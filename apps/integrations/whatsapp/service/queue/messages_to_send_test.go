package queue

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"github.com/antinvestor/service-notification/apps/integrations/whatsapp/config"
	"github.com/antinvestor/service-notification/apps/integrations/whatsapp/service/client"
	"github.com/antinvestor/service-notification/apps/integrations/whatsapp/service/internal/testsupport"
	"github.com/antinvestor/service-notification/pkg/constants"
	"github.com/antinvestor/service-notification/pkg/events"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

type recordingEmitter struct {
	mu      sync.Mutex
	updates []*commonv1.StatusUpdateRequest
}

func (r *recordingEmitter) Emit(_ context.Context, name string, payload any) error {
	if name != events.NotificationStatusUpdateEvent {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updates = append(r.updates, payload.(*commonv1.StatusUpdateRequest))
	return nil
}

func (r *recordingEmitter) last(t *testing.T) *commonv1.StatusUpdateRequest {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	require.NotEmpty(t, r.updates)
	return r.updates[len(r.updates)-1]
}

func newWorker(t *testing.T, graphStatus int, graphBody string) (*messageToSend, *recordingEmitter) {
	t.Helper()
	graph := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(graphStatus)
		_, _ = w.Write([]byte(graphBody))
	}))
	t.Cleanup(graph.Close)

	settings := &testsupport.SettingsStub{Values: map[string]string{"route-wa": `{"access_token":"tok","phone_number_id":"555"}`}}
	cfg := &config.WhatsAppConfig{SettingsIntegrationName: "WhatsApp", SettingsIntegrationID: "x",
		GraphAPIURL: graph.URL, GraphAPIVersion: "v21.0", RequestTimeoutSeconds: 5}
	cli := client.NewClient(cfg, nil, testsupport.NewSettingsClient(t, settings))

	emitter := &recordingEmitter{}
	return NewMessageToSend(emitter, cli).(*messageToSend), emitter
}

func encoded(t *testing.T, n *notificationv1.Notification) []byte {
	t.Helper()
	raw, err := proto.Marshal(n)
	require.NoError(t, err)
	return raw
}

func sampleNotification() *notificationv1.Notification {
	return &notificationv1.Notification{
		Id:        "notif-1",
		Recipient: &commonv1.ContactLink{Detail: "+254712345678"},
		Data:      "hello",
	}
}

var headers = map[string]string{constants.RouteIDHeaderName: "route-wa"}

func TestHandleSuccess(t *testing.T) {
	t.Parallel()
	worker, emitter := newWorker(t, http.StatusOK, `{"messages":[{"id":"wamid.OK"}],"contacts":[{"wa_id":"254712345678"}]}`)

	require.NoError(t, worker.Handle(context.Background(), headers, encoded(t, sampleNotification())))

	update := emitter.last(t)
	require.Equal(t, "notif-1", update.GetId())
	require.Equal(t, "wamid.OK", update.GetExternalId())
	require.Equal(t, commonv1.STATUS_QUEUED, update.GetStatus())
	require.Equal(t, commonv1.STATE_ACTIVE, update.GetState())
	require.Equal(t, "254712345678", update.GetExtras().AsMap()["wa_id"])
}

func TestHandleTransientFailureRequeues(t *testing.T) {
	t.Parallel()
	worker, emitter := newWorker(t, http.StatusBadRequest, `{"error":{"message":"rate","code":130429}}`)

	err := worker.Handle(context.Background(), headers, encoded(t, sampleNotification()))
	require.Error(t, err, "transient failures must return an error so the queue redelivers")

	update := emitter.last(t)
	require.Equal(t, commonv1.STATUS_UNKNOWN, update.GetStatus())
	require.Equal(t, commonv1.STATE_ACTIVE, update.GetState())
}

func TestHandlePermanentFailureWithFallback(t *testing.T) {
	t.Parallel()
	worker, emitter := newWorker(t, http.StatusBadRequest,
		`{"error":{"message":"not on whatsapp","code":131026,"fbtrace_id":"tr1"}}`)

	require.NoError(t, worker.Handle(context.Background(), headers, encoded(t, sampleNotification())))

	update := emitter.last(t)
	require.Equal(t, commonv1.STATUS_FAILED, update.GetStatus())
	require.Equal(t, commonv1.STATE_INACTIVE, update.GetState())
	extras := update.GetExtras().AsMap()
	require.Equal(t, "sms", extras["fallback_channel"])
	require.Equal(t, "tr1", extras["fbtrace_id"])
	require.Equal(t, float64(131026), extras["error_code"])
}

func TestHandlePermanentFailureWithoutFallback(t *testing.T) {
	t.Parallel()
	worker, emitter := newWorker(t, http.StatusBadRequest, `{"error":{"message":"bad param","code":100}}`)

	require.NoError(t, worker.Handle(context.Background(), headers, encoded(t, sampleNotification())))

	update := emitter.last(t)
	require.Equal(t, commonv1.STATUS_FAILED, update.GetStatus())
	_, hasFallback := update.GetExtras().AsMap()["fallback_channel"]
	require.False(t, hasFallback)
}

func TestHandleLocalValidationFailure(t *testing.T) {
	t.Parallel()
	worker, emitter := newWorker(t, http.StatusOK, `{}`)

	n := sampleNotification()
	n.Data = ""
	require.NoError(t, worker.Handle(context.Background(), headers, encoded(t, n)))

	update := emitter.last(t)
	require.Equal(t, commonv1.STATUS_FAILED, update.GetStatus())
	require.Contains(t, update.GetExtras().AsMap()["error"], "no message body")
}

func TestHandleDropsUndecodablePayload(t *testing.T) {
	t.Parallel()
	worker, emitter := newWorker(t, http.StatusOK, `{}`)

	garbage, _ := json.Marshal(map[string]any{"not": "proto"})
	require.NoError(t, worker.Handle(context.Background(), headers, append([]byte{0xff, 0xff}, garbage...)))
	require.Empty(t, emitter.updates)
}
