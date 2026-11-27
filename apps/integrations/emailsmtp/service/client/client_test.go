package client

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/antinvestor/service-notification/apps/integrations/emailsmtp/config"
	"github.com/stretchr/testify/require"
)

func TestResolveTLSMode(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		port    int
		want    TLSMode
		wantErr bool
	}{
		{"auto 465 is implicit", "auto", 465, TLSModeImplicit, false},
		{"auto 587 is starttls", "auto", 587, TLSModeSTARTTLS, false},
		{"auto 25 is starttls", "auto", 25, TLSModeSTARTTLS, false},
		{"empty behaves as auto 465", "", 465, TLSModeImplicit, false},
		{"empty behaves as auto 587", "", 587, TLSModeSTARTTLS, false},
		{"explicit implicit on 587", "implicit", 587, TLSModeImplicit, false},
		{"explicit starttls on 465", "starttls", 465, TLSModeSTARTTLS, false},
		{"case and whitespace tolerant", "  Implicit ", 587, TLSModeImplicit, false},
		{"invalid value errors", "ssl", 465, "", true},
		{"invalid value errors on 587", "none", 587, "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveTLSMode(tc.mode, tc.port)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestNewClientRejectsInvalidTLSMode(t *testing.T) {
	_, err := NewClient(nil, &config.EmailSMTPConfig{SMTPServerPORT: 465, SMTPServerTLSMode: "bogus"}, nil, nil)
	require.Error(t, err)
}

// fakeSMTP is a minimal in-process SMTP server supporting STARTTLS, implicit TLS and AUTH PLAIN.
type fakeSMTP struct {
	ln       net.Listener
	tlsCfg   *tls.Config
	implicit bool

	mu       sync.Mutex
	authed   []string // decoded PLAIN payloads
	messages []string
	tlsUsed  bool
}

func newCert(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	leaf, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, pool
}

func startFakeSMTP(t *testing.T, implicit bool) (*fakeSMTP, *x509.CertPool, int) {
	t.Helper()
	cert, pool := newCert(t)
	s := &fakeSMTP{
		tlsCfg:   &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12},
		implicit: implicit,
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	s.ln = ln
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, aerr := ln.Accept()
			if aerr != nil {
				return
			}
			go s.serve(c)
		}
	}()
	return s, pool, ln.Addr().(*net.TCPAddr).Port
}

func (s *fakeSMTP) serve(raw net.Conn) {
	defer raw.Close()
	_ = raw.SetDeadline(time.Now().Add(3 * time.Second))
	conn := net.Conn(raw)
	isTLS := false
	if s.implicit {
		conn = tls.Server(raw, s.tlsCfg)
		isTLS = true
	}
	r := bufio.NewReader(conn)
	w := func(line string) { _, _ = conn.Write([]byte(line + "\r\n")) }
	w("220 localhost ESMTP")
	inData := false
	var body []string
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if inData {
			if line == "." {
				inData = false
				s.mu.Lock()
				s.messages = append(s.messages, strings.Join(body, "\n"))
				s.mu.Unlock()
				w("250 OK queued")
				continue
			}
			body = append(body, line)
			continue
		}
		cmd := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			w("250-localhost")
			if !s.implicit && !isTLS {
				w("250-STARTTLS")
			}
			if isTLS {
				w("250-AUTH PLAIN")
			}
			w("250 8BITMIME")
		case cmd == "STARTTLS":
			if s.implicit || isTLS {
				w("503 bad sequence")
				continue
			}
			w("220 ready")
			conn = tls.Server(raw, s.tlsCfg)
			r = bufio.NewReader(conn)
			w = func(l string) { _, _ = conn.Write([]byte(l + "\r\n")) }
			isTLS = true
		case strings.HasPrefix(cmd, "AUTH PLAIN"):
			if !isTLS {
				w("538 encryption required")
				continue
			}
			parts := strings.Fields(line)
			if len(parts) < 3 {
				w("501 syntax")
				continue
			}
			dec, derr := base64.StdEncoding.DecodeString(parts[2])
			if derr != nil {
				w("501 bad base64")
				continue
			}
			s.mu.Lock()
			s.authed = append(s.authed, string(dec))
			s.tlsUsed = true
			s.mu.Unlock()
			w("235 authenticated")
		case strings.HasPrefix(cmd, "MAIL FROM"), strings.HasPrefix(cmd, "RCPT TO"), cmd == "RSET", cmd == "NOOP":
			w("250 OK")
		case cmd == "DATA":
			inData = true
			w("354 go")
		case cmd == "QUIT":
			w("221 bye")
			return
		default:
			w("502 unsupported")
		}
	}
}

func (s *fakeSMTP) snapshot() (authed, messages []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.authed...), append([]string(nil), s.messages...)
}

func TestCreateMailClientAgainstFakeServer(t *testing.T) {
	tests := []struct {
		name       string
		serverTLS  bool // implicit server
		mode       string
		wantConnOK bool
	}{
		{"implicit mode against implicit server", true, "implicit", true},
		{"starttls mode against starttls server", false, "starttls", true},
		{"starttls mode against implicit server fails", true, "starttls", false},
		{"implicit mode against starttls server fails", false, "implicit", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, pool, port := startFakeSMTP(t, tc.serverTLS)
			cfg := &config.EmailSMTPConfig{
				SMTPServerHOST:      "localhost",
				SMTPServerPORT:      port,
				SMTPServerTLSMode:   tc.mode,
				SMTPServerAccessKey: "api_token",
				SMTPServerSecretKey: "s3cret",
			}
			cl, err := NewClient(nil, cfg, nil, nil, WithRootCAs(pool))
			require.NoError(t, err)

			mc, err := cl.createMailClient()
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			derr := mc.DialWithContext(ctx)
			if !tc.wantConnOK {
				require.Error(t, derr)
				return
			}
			require.NoError(t, derr)
			defer func() { _ = mc.Close() }()

			authed, _ := srv.snapshot()
			require.Equal(t, []string{"\x00api_token\x00s3cret"}, authed)
		})
	}
}

func TestCreateMailClientVerifiesCertificate(t *testing.T) {
	_, _, port := startFakeSMTP(t, true)
	cfg := &config.EmailSMTPConfig{
		SMTPServerHOST:    "localhost",
		SMTPServerPORT:    port,
		SMTPServerTLSMode: "implicit",
	}
	// No RootCAs injected: the self-signed cert must be rejected.
	cl, err := NewClient(nil, cfg, nil, nil)
	require.NoError(t, err)
	mc, err := cl.createMailClient()
	require.NoError(t, err)
	require.Error(t, mc.DialWithContext(t.Context()))
}
