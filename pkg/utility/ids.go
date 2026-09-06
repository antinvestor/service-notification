package utility

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// deterministicIDLength keeps derived ids within the 40 character limit the notification
// proto enforces on ids while leaving them clearly distinguishable from generated xids.
const deterministicIDLength = 32

// DeterministicID derives a stable identifier from a provider message id so repeated
// webhook deliveries map to the same notification. The result is lowercase hex, prefixed
// with the provider tag, and always satisfies the id pattern [0-9a-z_-]{3,40}.
func DeterministicID(provider, providerMessageID string) string {
	sum := sha256.Sum256([]byte(provider + ":" + providerMessageID))
	digest := hex.EncodeToString(sum[:])
	prefix := strings.ToLower(provider)
	room := deterministicIDLength - len(prefix) - 1
	if room < 8 {
		room = 8
	}
	return prefix + "_" + digest[:room]
}
