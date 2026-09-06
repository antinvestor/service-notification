package events

import (
	"testing"

	profilev1 "buf.build/gen/go/antinvestor/profile/protocolbuffers/go/profile/v1"
	"github.com/antinvestor/service-notification/apps/default/service/models"
)

func TestChannelTypesForContact(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		contactType profilev1.ContactType
		want        []string
	}{
		{
			name:        "msisdn prefers whatsapp then sms",
			contactType: profilev1.ContactType_MSISDN,
			want:        []string{models.RouteTypeWhatsAppForm, models.RouteTypeSMSForm},
		},
		{
			name:        "email maps to email",
			contactType: profilev1.ContactType_EMAIL,
			want:        []string{models.RouteTypeEmailForm},
		},
		{
			name:        "unknown maps to any",
			contactType: profilev1.ContactType(99),
			want:        []string{models.RouteTypeAny},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := channelTypesForContact(tt.contactType)
			if len(got) != len(tt.want) {
				t.Fatalf("channelTypesForContact() len = %d, want %d (%v)", len(got), len(tt.want), got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("channelTypesForContact()[%d] = %q, want %q (full=%v)", i, got[i], tt.want[i], got)
				}
			}
		})
	}
}
