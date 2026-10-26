package engine

import (
	"context"
	"strings"
)

// Caller is who is dialling, as the profile service knows them. Both ids are
// empty for a number no profile holds.
type Caller struct {
	ProfileID string
	ContactID string
}

// CallerResolver looks a caller's phone number up as a profile contact. The
// engine addresses everything it stores about a caller by these ids; the
// number itself stays on the session row the gateway needs to find.
type CallerResolver interface {
	Resolve(ctx context.Context, msisdn string) (Caller, error)
}

// lastInput returns the input for this turn when the gateway sends the whole
// path typed so far joined by sep, as Africa's Talking does ("1*2*3").
func lastInput(raw, sep string) string {
	if sep == "" {
		return raw
	}
	if i := strings.LastIndex(raw, sep); i >= 0 {
		return raw[i+len(sep):]
	}
	return raw
}
