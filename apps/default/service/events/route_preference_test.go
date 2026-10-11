package events

import (
	"context"
	"errors"
	"testing"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	profilev1 "buf.build/gen/go/antinvestor/profile/protocolbuffers/go/profile/v1"
	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/stretchr/testify/require"
)

func notificationWithPartition() *models.Notification {
	n := &models.Notification{}
	n.PartitionID = "p1"
	return n
}

// stubRouteLookup implements routeLookup for unit tests.
type stubRouteLookup struct {
	byID   map[string]*models.Route
	byType map[string][]*models.Route
	err    error
}

func (s *stubRouteLookup) GetByID(_ context.Context, id string) (*models.Route, error) {
	if s.err != nil {
		return nil, s.err
	}
	route, ok := s.byID[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return route, nil
}

func (s *stubRouteLookup) GetByModeTypeAndPartitionID(_ context.Context, _ string, routeType string, _ string) ([]*models.Route, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.byType[routeType], nil
}

func TestRouteWithChannelPreference(t *testing.T) {
	t.Parallel()

	waRoute := &models.Route{Name: "wa", RouteType: models.RouteTypeWhatsAppForm, Uri: "mem://wa"}
	waRoute.ID = "route-wa"
	smsRoute := &models.Route{Name: "sms", RouteType: models.RouteTypeSMSForm, Uri: "mem://sms"}
	smsRoute.ID = "route-sms"

	tests := []struct {
		name         string
		repo         *stubRouteLookup
		notification *models.Notification
		channelTypes []string
		wantRouteID  string
		wantChannel  string
		wantFallback bool
		wantErr      bool
	}{
		{
			name: "uses whatsapp when available",
			repo: &stubRouteLookup{
				byType: map[string][]*models.Route{
					models.RouteTypeWhatsAppForm: {waRoute},
					models.RouteTypeSMSForm:      {smsRoute},
				},
			},
			notification: notificationWithPartition(),
			channelTypes: []string{models.RouteTypeWhatsAppForm, models.RouteTypeSMSForm},
			wantRouteID:  "route-wa",
			wantChannel:  models.RouteTypeWhatsAppForm,
			wantFallback: false,
		},
		{
			name: "falls back to sms when whatsapp missing",
			repo: &stubRouteLookup{
				byType: map[string][]*models.Route{
					models.RouteTypeSMSForm: {smsRoute},
				},
			},
			notification: notificationWithPartition(),
			channelTypes: []string{models.RouteTypeWhatsAppForm, models.RouteTypeSMSForm},
			wantRouteID:  "route-sms",
			wantChannel:  models.RouteTypeSMSForm,
			wantFallback: true,
		},
		{
			name: "errors when neither channel has a route",
			repo: &stubRouteLookup{
				byType: map[string][]*models.Route{},
			},
			notification: notificationWithPartition(),
			channelTypes: []string{models.RouteTypeWhatsAppForm, models.RouteTypeSMSForm},
			wantErr:      true,
		},
		{
			name: "explicit route id wins",
			repo: &stubRouteLookup{
				byID: map[string]*models.Route{
					"route-sms": smsRoute,
				},
			},
			notification: func() *models.Notification {
				n := notificationWithPartition()
				n.RouteID = "route-sms"
				n.NotificationType = models.RouteTypeSMSForm
				return n
			}(),
			channelTypes: []string{models.RouteTypeWhatsAppForm, models.RouteTypeSMSForm},
			wantRouteID:  "route-sms",
			wantChannel:  models.RouteTypeSMSForm,
			wantFallback: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			n := tt.notification
			n.ID = "notif-1"
			route, channel, fallback, err := routeWithChannelPreference(
				context.Background(),
				tt.repo,
				models.RouteModeTransmit,
				n,
				tt.channelTypes,
			)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if route.ID != tt.wantRouteID {
				t.Fatalf("route.ID = %q, want %q", route.ID, tt.wantRouteID)
			}
			if channel != tt.wantChannel {
				t.Fatalf("channel = %q, want %q", channel, tt.wantChannel)
			}
			if fallback != tt.wantFallback {
				t.Fatalf("fallback = %v, want %v", fallback, tt.wantFallback)
			}
			if n.NotificationType != tt.wantChannel {
				t.Fatalf("notification type = %q, want %q", n.NotificationType, tt.wantChannel)
			}
		})
	}
}

func TestReachableContactsAndChannels(t *testing.T) {
	t.Parallel()

	profile := &profilev1.ProfileObject{Contacts: []*profilev1.ContactObject{
		{Id: "mail-unverified", Type: profilev1.ContactType_EMAIL},
		{Id: "phone-deleted", Type: profilev1.ContactType_MSISDN, Verified: true, State: commonv1.STATE_DELETED},
		{Id: "phone-optout", Type: profilev1.ContactType_MSISDN, Verified: true, CommunicationLevel: profilev1.CommunicationLevel_NO_CONTACT},
		{Id: "phone", Type: profilev1.ContactType_MSISDN, Verified: true},
		{Id: ""},
	}}

	contacts := reachableContacts(profile)
	require.Len(t, contacts, 2)
	require.Equal(t, "phone", contacts[0].GetId(), "verified contacts come first")
	require.Equal(t, "mail-unverified", contacts[1].GetId())

	require.Equal(t,
		[]string{models.RouteTypeWhatsAppForm, models.RouteTypeSMSForm, models.RouteTypeEmailForm},
		channelTypesForContacts(contacts))

	require.Equal(t, "phone", contactForChannel(contacts, models.RouteTypeSMSForm).GetId())
	require.Equal(t, "mail-unverified", contactForChannel(contacts, models.RouteTypeEmailForm).GetId())
	require.Nil(t, contactForChannel(contacts, "push"))
	require.Empty(t, channelTypesForContacts(nil))
}
