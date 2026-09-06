package events

import (
	"context"
	"errors"
	"testing"

	"github.com/antinvestor/service-notification/apps/default/service/models"
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
