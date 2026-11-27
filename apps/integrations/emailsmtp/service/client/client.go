package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strings"
	"sync"
	"time"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/v2/profile/v1/profilev1connect"
	profilev1 "buf.build/gen/go/antinvestor/profile/protocolbuffers/go/profile/v1"
	"buf.build/gen/go/antinvestor/settingz/connectrpc/go/v2/settings/v1/settingsv1connect"
	"github.com/antinvestor/service-notification/apps/integrations/emailsmtp/config"
	"github.com/antinvestor/service-notification/pkg/constants"
	"github.com/antinvestor/service-notification/pkg/utility"
	"github.com/pitabwire/util"
	"github.com/wneessen/go-mail"
)

const (
	connectionTTL = 5 * time.Minute
)

type connectedClient struct {
	client      *mail.Client
	connectedAt time.Time
	mu          sync.Mutex
}

func (cc *connectedClient) isExpired() bool {
	return time.Since(cc.connectedAt) > connectionTTL
}

type Client struct {
	cfg         *config.EmailSMTPConfig
	logger      *util.LogEntry
	profileCli  profilev1connect.ProfileServiceClient
	settingsCli settingsv1connect.SettingsServiceClient
	connMap     sync.Map
	connMu      sync.Mutex
	tlsMode     TLSMode
	rootCAs     *x509.CertPool
}

// TLSMode is the resolved transport security used for the SMTP connection.
type TLSMode string

const (
	// TLSModeSTARTTLS connects in plaintext and mandatorily upgrades via STARTTLS.
	TLSModeSTARTTLS TLSMode = "starttls"
	// TLSModeImplicit wraps the connection in TLS from the first byte (SMTPS).
	TLSModeImplicit TLSMode = "implicit"

	tlsModeAuto = "auto"

	implicitTLSPort = 465
)

// resolveTLSMode maps the configured mode string and port to a concrete TLSMode.
// An empty or "auto" mode selects implicit TLS on port 465 and STARTTLS otherwise.
// Unknown values are an error rather than a silent fallback.
func resolveTLSMode(mode string, port int) (TLSMode, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", tlsModeAuto:
		if port == implicitTLSPort {
			return TLSModeImplicit, nil
		}
		return TLSModeSTARTTLS, nil
	case string(TLSModeSTARTTLS):
		return TLSModeSTARTTLS, nil
	case string(TLSModeImplicit):
		return TLSModeImplicit, nil
	default:
		return "", fmt.Errorf("invalid SMTP_SERVER_TLS_MODE %q: must be one of auto, starttls, implicit", mode)
	}
}

// Option customises a Client.
type Option func(*Client)

// WithRootCAs sets the trusted root CAs used to verify the SMTP server certificate.
// Intended for tests with self-signed certificates; production uses the system pool.
func WithRootCAs(pool *x509.CertPool) Option {
	return func(c *Client) { c.rootCAs = pool }
}

func NewClient(logger *util.LogEntry, cfg *config.EmailSMTPConfig, profileCli profilev1connect.ProfileServiceClient, settingsCli settingsv1connect.SettingsServiceClient, opts ...Option) (*Client, error) {
	tlsMode, err := resolveTLSMode(cfg.SMTPServerTLSMode, cfg.SMTPServerPORT)
	if err != nil {
		return nil, err
	}

	c := &Client{
		tlsMode:     tlsMode,
		cfg:         cfg,
		logger:      logger,
		profileCli:  profileCli,
		settingsCli: settingsCli,
		connMap:     sync.Map{},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

func (ms *Client) createMailClient() (*mail.Client, error) {
	cfg := ms.cfg
	opts := []mail.Option{
		mail.WithPort(cfg.SMTPServerPORT),
		mail.WithTLSPolicy(mail.TLSMandatory),
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithUsername(cfg.SMTPServerAccessKey),
		mail.WithPassword(cfg.SMTPServerSecretKey),
		mail.WithTimeout(15 * time.Second),
	}
	if ms.tlsMode == TLSModeImplicit {
		opts = append(opts, mail.WithSSL())
	}
	if ms.rootCAs != nil {
		opts = append(opts, mail.WithTLSConfig(&tls.Config{
			ServerName: cfg.SMTPServerHOST,
			MinVersion: tls.VersionTLS12,
			RootCAs:    ms.rootCAs,
		}))
	}
	return mail.NewClient(cfg.SMTPServerHOST, opts...)
}

func (ms *Client) getConnectedClient(ctx context.Context, credentials map[string]string) (*connectedClient, error) {
	partitionID := credentials[constants.PartitionIDHeaderName]

	if connObj, ok := ms.connMap.Load(partitionID); ok {
		if conn, cok := connObj.(*connectedClient); cok && !conn.isExpired() {
			return conn, nil
		}
		if conn, cok := connObj.(*connectedClient); cok {
			_ = conn.client.Close()
		}
		ms.connMap.Delete(partitionID)
	}

	ms.connMu.Lock()
	defer ms.connMu.Unlock()

	if connObj, ok := ms.connMap.Load(partitionID); ok {
		if conn, cok := connObj.(*connectedClient); cok && !conn.isExpired() {
			return conn, nil
		}
		if conn, cok := connObj.(*connectedClient); cok {
			_ = conn.client.Close()
		}
		ms.connMap.Delete(partitionID)
	}

	cli, err := ms.createMailClient()
	if err != nil {
		return nil, err
	}

	if err := cli.DialWithContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to establish SMTP connection: %w", err)
	}

	conn := &connectedClient{
		client:      cli,
		connectedAt: time.Now(),
	}
	ms.connMap.Store(partitionID, conn)

	return conn, nil
}

func (ms *Client) Send(ctx context.Context, credentials map[string]string, notification *notificationv1.Notification) error {

	recipient, err := utility.PopulateContactLink(ctx, ms.profileCli, notification.GetRecipient(), profilev1.ContactType_EMAIL)
	if err != nil {
		return err
	}

	sender, err := utility.PopulateContactLink(ctx, ms.profileCli, notification.GetSource(), profilev1.ContactType_EMAIL)
	if err != nil {
		return err
	}

	extrasData := notification.GetExtras().AsMap()

	if sender.GetDetail() == "" {

		// Check existence of support email and use that:
		if dt, ok := extrasData["support_email"]; ok {
			if s, supportOk := dt.(string); supportOk {
				sender.Detail = s
			}
		}
	}

	if sender.GetDetail() == "" {
		return fmt.Errorf("email sender is empty: possibly configure support contacts in partition")
	}

	notificationSubject := ""
	if dt, ok := extrasData["subject"]; ok {
		if s, subjectOk := dt.(string); subjectOk {
			notificationSubject = s
		}
	}

	htmlBody := ""
	if dt, ok := extrasData["html"]; ok {
		if s, htmlOk := dt.(string); htmlOk {
			htmlBody = s
		}
	}

	textBody := ""
	if dt, ok := extrasData["text"]; ok {
		if s, textOk := dt.(string); textOk {
			textBody = s
		}
	}

	if textBody == "" {
		textBody = notification.GetData()
	}

	if textBody == "" && htmlBody == "" {
		return fmt.Errorf("email body is empty: provide either text or html content")
	}

	conn, err := ms.getConnectedClient(ctx, credentials)
	if err != nil {
		return err
	}

	err = ms.sendEmailWithRetry(ctx, credentials, conn, notification.GetId(), sender, recipient, notificationSubject, textBody, htmlBody)
	if err != nil {
		return err
	}
	return nil
}

func (ms *Client) sendEmailWithRetry(ctx context.Context, credentials map[string]string, conn *connectedClient, messageID string, sender, recipient *commonv1.ContactLink, subject, textBody, htmlBody string) error {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	msg := mail.NewMsg()

	if sender != nil && sender.GetDetail() != "" {
		if err := msg.From(sender.GetDetail()); err != nil {
			return err
		}
	}
	if err := msg.To(recipient.GetDetail()); err != nil {
		return err
	}

	msg.SetGenHeader("X-PM-Metadata-notification-id", messageID)
	msg.Subject(subject)

	if textBody != "" {
		msg.SetBodyString(mail.TypeTextPlain, textBody)
	}
	if htmlBody != "" {
		msg.AddAlternativeString(mail.TypeTextHTML, htmlBody)
	}

	err := conn.client.Send(msg)
	if err != nil {
		partitionID := credentials[constants.PartitionIDHeaderName]
		ms.connMap.Delete(partitionID)
		_ = conn.client.Close()

		newConn, dialErr := ms.getConnectedClient(ctx, credentials)
		if dialErr != nil {
			return fmt.Errorf("send failed and reconnect failed: %w (original: %v)", dialErr, err)
		}

		newConn.mu.Lock()
		defer newConn.mu.Unlock()
		if retryErr := newConn.client.Send(msg); retryErr != nil {
			return fmt.Errorf("send failed after retry: %w", retryErr)
		}
	}

	return nil
}
