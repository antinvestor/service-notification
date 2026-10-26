package engine

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/nacl/box"
)

// SealedPrefix marks a collected value sealed for the service that owns the
// menu. Only that service, holding the private key, can open it.
const SealedPrefix = "sealed:"

// ErrNoSecretKey is returned when a menu collects a secret for a service
// that configured no key to seal it with.
var ErrNoSecretKey = errors.New("service has no secret_box_public_key to seal secret input with")

// SealSecret seals value with the service's X25519 public key (NaCl
// anonymous box), so a PIN typed into a USSD menu is never stored or handed
// on in the clear: sessions, completion payloads and the notification
// service only ever see the sealed form.
func SealSecret(publicKeyB64, value string) (string, error) {
	publicKeyB64 = strings.TrimSpace(publicKeyB64)
	if publicKeyB64 == "" {
		return "", ErrNoSecretKey
	}
	raw, err := base64.StdEncoding.DecodeString(publicKeyB64)
	if err != nil || len(raw) != 32 {
		return "", fmt.Errorf("secret_box_public_key is not a base64 X25519 key: %w", err)
	}
	var key [32]byte
	copy(key[:], raw)
	sealed, err := box.SealAnonymous(nil, []byte(value), &key, rand.Reader)
	if err != nil {
		return "", fmt.Errorf("seal secret input: %w", err)
	}
	return SealedPrefix + base64.StdEncoding.EncodeToString(sealed), nil
}
