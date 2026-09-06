package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	"github.com/antinvestor/service-notification/apps/integrations/whatsapp/config"
	"github.com/antinvestor/service-notification/apps/integrations/whatsapp/service/client"
	"github.com/antinvestor/service-notification/apps/integrations/whatsapp/service/internal/testsupport"
	"github.com/stretchr/testify/require"
)

const (
	routeID   = "route-wa-rx"
	appSecret = "app-secret"
)

func newServer(t *testing.T, cfg *config.WhatsAppConfig, stub *testsupport.NotificationStub) *httptest.Server {
	t.Helper()
	settings := &testsupport.SettingsStub{Values: map[string]string{
		routeID: `{"access_token":"tok","phone_number_id":"555","app_secret":"` + appSecret + `","verify_token":"stored-verify"}`,
	}}
	cfg.SettingsIntegrationName = "WhatsApp"
	cfg.SettingsIntegrationID = "notification.whatsapp"
	cfg.GraphAPIURL = "http://graph.invalid"
	cfg.GraphAPIVersion = "v21.0"

	whatsAppCli := client.NewClient(cfg, nil, testsupport.NewSettingsClient(t, settings))
	ws := NewWebhookServer(cfg, testsupport.NewNotificationClient(t, stub), whatsAppCli)
	srv := httptest.NewServer(ws.NewRouterV1())
	t.Cleanup(srv.Close)
	return srv
}

func post(t *testing.T, srv *httptest.Server, route, body, signature string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/receive/notification/"+route, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if signature != "" {
		req.Header.Set(client.SignatureHeader, signature)
	}
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func TestVerifyHandshake(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		cfg        *config.WhatsAppConfig
		route      string
		query      string
		wantStatus int
		wantBody   string
	}{
		{name: "env token accepted", cfg: &config.WhatsAppConfig{VerifyToken: "env-verify", AppSecret: "x"}, route: routeID,
			query: "hub.mode=subscribe&hub.verify_token=env-verify&hub.challenge=12345", wantStatus: 200, wantBody: "12345"},
		{name: "stored token accepted", cfg: &config.WhatsAppConfig{}, route: routeID,
			query: "hub.mode=subscribe&hub.verify_token=stored-verify&hub.challenge=999", wantStatus: 200, wantBody: "999"},
		{name: "wrong token", cfg: &config.WhatsAppConfig{VerifyToken: "env-verify"}, route: routeID,
			query: "hub.mode=subscribe&hub.verify_token=nope&hub.challenge=1", wantStatus: 403},
		{name: "wrong mode", cfg: &config.WhatsAppConfig{VerifyToken: "env-verify"}, route: routeID,
			query: "hub.mode=unsubscribe&hub.verify_token=env-verify&hub.challenge=1", wantStatus: 403},
		{name: "unknown route and no env token", cfg: &config.WhatsAppConfig{}, route: "route-unknown",
			query: "hub.mode=subscribe&hub.verify_token=stored-verify&hub.challenge=1", wantStatus: 403},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := newServer(t, tt.cfg, &testsupport.NotificationStub{})
			resp, err := srv.Client().Get(srv.URL + "/receive/notification/" + tt.route + "?" + tt.query)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()
			require.Equal(t, tt.wantStatus, resp.StatusCode)
			if tt.wantBody != "" {
				buf := make([]byte, 64)
				n, _ := resp.Body.Read(buf)
				require.Equal(t, tt.wantBody, string(buf[:n]))
			}
		})
	}
}

const inboundPayload = `{
  "object": "whatsapp_business_account",
  "entry": [{"id": "WABA", "changes": [{"field": "messages", "value": {
    "messaging_product": "whatsapp",
    "metadata": {"display_phone_number": "254700000001", "phone_number_id": "555"},
    "contacts": [{"profile": {"name": "Asha"}, "wa_id": "254712345678"}],
    "messages": [
      {"from": "254712345678", "id": "wamid.IN1", "timestamp": "1725600000", "type": "text", "text": {"body": "Hi, balance?"}},
      {"from": "254712345678", "id": "wamid.IN2", "timestamp": "1725600001", "type": "image",
       "image": {"id": "media-9", "mime_type": "image/jpeg", "caption": "receipt"},
       "context": {"from": "254700000001", "id": "wamid.OUT0"}},
      {"from": "", "id": "wamid.BAD", "type": "text", "text": {"body": "no sender"}}
    ]
  }}]}]
}`

func TestReceiveInboundMessages(t *testing.T) {
	t.Parallel()

	stub := &testsupport.NotificationStub{}
	srv := newServer(t, &config.WhatsAppConfig{}, stub)

	resp := post(t, srv, routeID, inboundPayload, client.Sign(appSecret, []byte(inboundPayload)))
	require.Equal(t, http.StatusOK, resp.StatusCode, "a bad item must not fail the whole delivery")

	received, statuses := stub.Snapshot()
	require.Empty(t, statuses)
	require.Len(t, received, 2)

	first := received[0]
	require.Equal(t, "whatsapp", first.GetType())
	require.Equal(t, routeID, first.GetRouteId())
	require.Equal(t, "+254712345678", first.GetSource().GetDetail())
	require.Equal(t, "Asha", first.GetSource().GetProfileName())
	require.Equal(t, "+254700000001", first.GetRecipient().GetDetail())
	require.Equal(t, "Hi, balance?", first.GetData())
	require.Regexp(t, `^wa_[0-9a-f]+$`, first.GetId())
	extras := first.GetExtras().AsMap()
	require.Equal(t, "wamid.IN1", extras["wamid"])
	require.Equal(t, "text", extras["message_type"])

	second := received[1]
	require.Equal(t, "receipt", second.GetData())
	extras = second.GetExtras().AsMap()
	require.Equal(t, "media-9", extras["media_id"])
	require.Equal(t, "image/jpeg", extras["media_mime_type"])
	require.Equal(t, "wamid.OUT0", extras["context_wamid"])
	require.NotEqual(t, first.GetId(), second.GetId())

	// Redelivery produces the same ids so the notification service can deduplicate.
	post(t, srv, routeID, inboundPayload, client.Sign(appSecret, []byte(inboundPayload)))
	again, _ := stub.Snapshot()
	require.Len(t, again, 4)
	require.Equal(t, first.GetId(), again[2].GetId())
}

const statusPayload = `{
  "object": "whatsapp_business_account",
  "entry": [{"id": "WABA", "changes": [{"field": "messages", "value": {
    "messaging_product": "whatsapp",
    "metadata": {"display_phone_number": "254700000001", "phone_number_id": "555"},
    "statuses": [
      {"id": "wamid.S1", "status": "sent", "timestamp": "1", "recipient_id": "254712345678",
       "conversation": {"id": "conv-1", "origin": {"type": "utility"}}, "pricing": {"billable": true, "category": "utility"}},
      {"id": "wamid.S2", "status": "delivered", "timestamp": "2", "recipient_id": "254712345678"},
      {"id": "wamid.S3", "status": "read", "timestamp": "3", "recipient_id": "254712345678"},
      {"id": "wamid.S4", "status": "failed", "timestamp": "4", "recipient_id": "254712345678",
       "errors": [{"code": 131047, "title": "Re-engagement message", "message": "Re-engagement message", "error_data": {"details": "outside window"}}]},
      {"id": "wamid.S5", "status": "failed", "timestamp": "5", "recipient_id": "254712345678",
       "errors": [{"code": 131000, "title": "Something went wrong"}]},
      {"id": "wamid.UNKNOWN", "status": "delivered", "timestamp": "6", "recipient_id": "254712345678"}
    ]
  }}]}]
}`

func TestReceiveStatuses(t *testing.T) {
	t.Parallel()

	stub := &testsupport.NotificationStub{UnknownExternalIDs: map[string]bool{"wamid.UNKNOWN": true}}
	srv := newServer(t, &config.WhatsAppConfig{}, stub)

	resp := post(t, srv, routeID, statusPayload, client.Sign(appSecret, []byte(statusPayload)))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	received, statuses := stub.Snapshot()
	require.Empty(t, received)
	require.Len(t, statuses, 5, "unknown external ids are ignored, not errors")

	byID := map[string]*commonv1.StatusUpdateRequest{}
	for _, s := range statuses {
		require.Empty(t, s.GetId(), "statuses resolve by external id only")
		byID[s.GetExternalId()] = s
	}

	require.Equal(t, commonv1.STATUS_IN_PROCESS, byID["wamid.S1"].GetStatus())
	require.Equal(t, commonv1.STATE_ACTIVE, byID["wamid.S1"].GetState())
	require.Equal(t, "conv-1", byID["wamid.S1"].GetExtras().AsMap()["conversation_id"])

	require.Equal(t, commonv1.STATUS_SUCCESSFUL, byID["wamid.S2"].GetStatus())
	require.Equal(t, commonv1.STATE_INACTIVE, byID["wamid.S2"].GetState())

	require.Equal(t, commonv1.STATUS_SUCCESSFUL, byID["wamid.S3"].GetStatus())
	require.Equal(t, true, byID["wamid.S3"].GetExtras().AsMap()["read"])

	failed := byID["wamid.S4"]
	require.Equal(t, commonv1.STATUS_FAILED, failed.GetStatus())
	require.Equal(t, "sms", failed.GetExtras().AsMap()["fallback_channel"])
	require.Equal(t, float64(131047), failed.GetExtras().AsMap()["error_code"])

	require.Equal(t, commonv1.STATUS_FAILED, byID["wamid.S5"].GetStatus())
	require.Nil(t, byID["wamid.S5"].GetExtras().AsMap()["fallback_channel"])
}

func TestReceiveRejectsBadRequests(t *testing.T) {
	t.Parallel()

	stub := &testsupport.NotificationStub{}
	srv := newServer(t, &config.WhatsAppConfig{}, stub)

	resp := post(t, srv, routeID, inboundPayload, "")
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode, "missing signature")

	resp = post(t, srv, routeID, inboundPayload, client.Sign("wrong", []byte(inboundPayload)))
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode, "wrong signature")

	resp = post(t, srv, routeID, `{"object":`, client.Sign(appSecret, []byte(`{"object":`)))
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "malformed json")

	received, statuses := stub.Snapshot()
	require.Empty(t, received)
	require.Empty(t, statuses)
}

func TestReceiveUnsignedWhenNoSecretConfigured(t *testing.T) {
	t.Parallel()

	stub := &testsupport.NotificationStub{}
	srv := newServer(t, &config.WhatsAppConfig{}, stub)

	// A route with no stored credentials and no env secret accepts unsigned payloads.
	resp := post(t, srv, "route-dev", inboundPayload, "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	received, _ := stub.Snapshot()
	require.Len(t, received, 2)
	require.Equal(t, "route-dev", received[0].GetRouteId())
}
