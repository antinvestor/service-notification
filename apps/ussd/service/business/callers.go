package business

import (
	"context"
	"errors"
	"fmt"
	"strings"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/v2/profile/v1/profilev1connect"
	profilev1 "buf.build/gen/go/antinvestor/profile/protocolbuffers/go/profile/v1"

	"github.com/antinvestor/service-notification/apps/ussd/service/engine"
)

// ErrCallerUnknown is returned when no profile holds the dialling number.
var ErrCallerUnknown = errors.New("no profile holds this number")

// profileCallers resolves a dialling number to the profile contact that
// holds it, through the profile service.
type profileCallers struct {
	profiles profilev1connect.ProfileServiceClient
}

// NewProfileCallers returns a CallerResolver backed by the profile service,
// or nil when no client is configured (callers then stay anonymous).
func NewProfileCallers(profiles profilev1connect.ProfileServiceClient) engine.CallerResolver {
	if profiles == nil {
		return nil
	}
	return &profileCallers{profiles: profiles}
}

func (p *profileCallers) Resolve(ctx context.Context, msisdn string) (engine.Caller, error) {
	resp, err := p.profiles.GetByContact(ctx, &profilev1.GetByContactRequest{Contact: msisdn})
	if err != nil {
		return engine.Caller{}, fmt.Errorf("look up caller: %w", err)
	}
	profile := resp.GetData()
	if profile.GetId() == "" {
		return engine.Caller{}, ErrCallerUnknown
	}
	for _, contact := range profile.GetContacts() {
		if contact.GetType() == profilev1.ContactType_MSISDN && sameNumber(contact.GetDetail(), msisdn) &&
			contact.GetState() != commonv1.STATE_DELETED {
			return engine.Caller{ProfileID: profile.GetId(), ContactID: contact.GetId()}, nil
		}
	}
	// The profile matched but the number is not one of its contacts: act for
	// nobody rather than for the wrong contact.
	return engine.Caller{}, ErrCallerUnknown
}

func sameNumber(a, b string) bool {
	clean := func(v string) string {
		return strings.TrimPrefix(strings.ReplaceAll(strings.TrimSpace(v), " ", ""), "+")
	}
	return a != "" && clean(a) == clean(b)
}
