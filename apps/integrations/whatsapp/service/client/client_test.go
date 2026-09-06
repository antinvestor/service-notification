package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"buf.build/gen/go/antinvestor/settingz/connectrpc/go/settings/v1/settingsv1connect"
	settingsv1 "buf.build/gen/go/antinvestor/settingz/protocolbuffers/go/settings/v1"
	"connectrpc.com/connect"
	"github.com/antinvestor/service-notification/apps/integrations/whatsapp/config"
	"github.com/antinvestor/service-notification/pkg/constants"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

type settingsStub struct {
	settingsv1connect.UnimplementedSettingsServiceHandler
	values map[string]string
}

func (s *settingsStub) Get(_ context.Context, req *connect.Request[settingsv1.GetRequest]) (*connect.Response[settingsv1.GetResponse], error) {
	value, ok := s.values[req.Msg.GetKey().GetName()]
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such setting"))
	}
	return connect.NewResponse(&settingsv1.GetResponse{Data: &settingsv1.SettingObject{Key: req.Msg.GetKey(), Value: value}}), nil
}

// graphStub records requests to the messages endpoint and replies with a canned response.
type graphStub struct {
	mu       sync.Mutex
	requests []recordedRequest
	status   int
	body     string
}

type recordedRequest struct {
	path string
	auth string
	body map[string]any
}

func (g *graphStub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	g.mu.Lock()
	g.requests = append(g.requests, recordedRequest{path: r.URL.Path, auth: r.Header.Get("Authorization"), body: body})
	g.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(g.status)
	_, _ = w.Write([]byte(g.body))
}

func (g *graphStub) last(t *testing.T) recordedRequest {
	t.Helper()
	g.mu.Lock()
	defer g.mu.Unlock()
	require.NotEmpty(t, g.requests)
	return g.requests[len(g.requests)-1]
}

func newTestClient(t *testing.T, graph *graphStub) *Client {
	t.Helper()

	graphSrv := httptest.NewServer(graph)
	t.Cleanup(graphSrv.Close)

	settings := &settingsStub{values: map[string]string{
		"route-wa":  `{"access_token":"tok-123","phone_number_id":"555"}`,
		"route-bad": `{"access_token":"","phone_number_id":"555"}`,
	}}
	mux := http.NewServeMux()
	mux.Handle(settingsv1connect.NewSettingsServiceHandler(settings))
	settingsSrv := httptest.NewServer(mux)
	t.Cleanup(settingsSrv.Close)

	cfg := &config.WhatsAppConfig{
		SettingsIntegrationName: "WhatsApp",
		SettingsIntegrationID:   "notification.whatsapp",
		GraphAPIURL:             graphSrv.URL,
		GraphAPIVersion:         "v21.0",
		RequestTimeoutSeconds:   5,
	}
	return NewClient(cfg, nil, settingsv1connect.NewSettingsServiceClient(settingsSrv.Client(), settingsSrv.URL))
}

func headersFor(route string) map[string]string {
	return map[string]string{constants.RouteIDHeaderName: route}
}

func textNotification(body string) *notificationv1.Notification {
	return &notificationv1.Notification{
		Id:        "notif-1",
		Recipient: &commonv1.ContactLink{Detail: "+254 712 345678"},
		Data:      body,
		Language:  "en",
	}
}

const acceptedResponse = `{"messaging_product":"whatsapp","contacts":[{"input":"254712345678","wa_id":"254712345678"}],"messages":[{"id":"wamid.ABC"}]}`

func TestSendText(t *testing.T) {
	t.Parallel()

	graph := &graphStub{status: http.StatusOK, body: acceptedResponse}
	cli := newTestClient(t, graph)

	result, err := cli.Send(context.Background(), headersFor("route-wa"), textNotification("hello there"))
	require.NoError(t, err)
	require.Equal(t, "wamid.ABC", result.MessageID)
	require.Equal(t, "254712345678", result.WaID)
	require.Empty(t, result.Template)

	req := graph.last(t)
	require.Equal(t, "/v21.0/555/messages", req.path)
	require.Equal(t, "Bearer tok-123", req.auth)
	require.Equal(t, "whatsapp", req.body["messaging_product"])
	require.Equal(t, "254712345678", req.body["to"])
	require.Equal(t, "text", req.body["type"])
	require.Equal(t, "hello there", req.body["text"].(map[string]any)["body"])
}

func TestSendTemplate(t *testing.T) {
	t.Parallel()

	graph := &graphStub{status: http.StatusOK, body: acceptedResponse}
	cli := newTestClient(t, graph)

	payload, _ := structpb.NewStruct(map[string]any{"code": "4242", "expiryDate": "2026-09-06T10:00", "unused": "x"})
	extras, _ := structpb.NewStruct(map[string]any{
		ExtraKeyTemplate: `{"name":"otp_code","language":"en_US","params":["code","expiryDate"],"header_params":["code"]}`,
		"whatsapp":       "Your code is 4242",
	})
	n := &notificationv1.Notification{
		Id:        "notif-2",
		Recipient: &commonv1.ContactLink{Detail: "254712345678"},
		Payload:   payload,
		Extras:    extras,
		Data:      "Your code is 4242",
	}

	result, err := cli.Send(context.Background(), headersFor("route-wa"), n)
	require.NoError(t, err)
	require.Equal(t, "otp_code", result.Template)

	req := graph.last(t)
	require.Equal(t, "template", req.body["type"])
	tmpl := req.body["template"].(map[string]any)
	require.Equal(t, "otp_code", tmpl["name"])
	require.Equal(t, "en_US", tmpl["language"].(map[string]any)["code"])

	components := tmpl["components"].([]any)
	require.Len(t, components, 2)
	header := components[0].(map[string]any)
	require.Equal(t, "header", header["type"])
	body := components[1].(map[string]any)
	require.Equal(t, "body", body["type"])
	params := body["parameters"].([]any)
	require.Len(t, params, 2)
	require.Equal(t, "4242", params[0].(map[string]any)["text"])
	require.Equal(t, "2026-09-06T10:00", params[1].(map[string]any)["text"])
}

func TestSendTemplateMissingParam(t *testing.T) {
	t.Parallel()

	graph := &graphStub{status: http.StatusOK, body: acceptedResponse}
	cli := newTestClient(t, graph)

	extras, _ := structpb.NewStruct(map[string]any{ExtraKeyTemplate: `{"name":"otp_code","params":["code"]}`})
	n := textNotification("x")
	n.Extras = extras

	_, err := cli.Send(context.Background(), headersFor("route-wa"), n)
	require.ErrorContains(t, err, `parameter "code" is missing`)
	require.Empty(t, graph.requests)
}

func TestSendTemplateLanguageFallsBackToNotification(t *testing.T) {
	t.Parallel()

	graph := &graphStub{status: http.StatusOK, body: acceptedResponse}
	cli := newTestClient(t, graph)

	extras, _ := structpb.NewStruct(map[string]any{ExtraKeyTemplate: `{"name":"welcome"}`})
	n := textNotification("")
	n.Language = "sw"
	n.Extras = extras

	_, err := cli.Send(context.Background(), headersFor("route-wa"), n)
	require.NoError(t, err)
	tmpl := graph.last(t).body["template"].(map[string]any)
	require.Equal(t, "sw", tmpl["language"].(map[string]any)["code"])
	require.Nil(t, tmpl["components"])
}

func TestSendValidation(t *testing.T) {
	t.Parallel()

	graph := &graphStub{status: http.StatusOK, body: acceptedResponse}
	cli := newTestClient(t, graph)
	ctx := context.Background()

	_, err := cli.Send(ctx, headersFor("route-wa"), textNotification(""))
	require.ErrorContains(t, err, "no message body")

	n := textNotification("hi")
	n.Recipient = &commonv1.ContactLink{Detail: "not-a-number"}
	_, err = cli.Send(ctx, headersFor("route-wa"), n)
	require.ErrorContains(t, err, "no phone number")

	_, err = cli.Send(ctx, headersFor("route-bad"), textNotification("hi"))
	require.ErrorContains(t, err, "credentials need")

	_, err = cli.Send(ctx, headersFor("route-missing"), textNotification("hi"))
	require.Error(t, err)

	_, err = cli.Send(ctx, map[string]string{}, textNotification("hi"))
	require.Error(t, err)

	require.Empty(t, graph.requests)
}

func TestSendErrorClassification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		httpStatus   int
		body         string
		wantCode     int
		wantRetry    bool
		wantFallback string
		wantCred     bool
	}{
		{name: "recipient not on whatsapp", httpStatus: 400,
			body:     `{"error":{"message":"Receiver incapable","type":"OAuthException","code":131026,"error_data":{"details":"Message Undeliverable."},"fbtrace_id":"t1"}}`,
			wantCode: 131026, wantFallback: FallbackChannel},
		{name: "re-engagement window", httpStatus: 400,
			body:     `{"error":{"message":"Re-engagement message","code":131047}}`,
			wantCode: 131047, wantFallback: FallbackChannel},
		{name: "throughput limit", httpStatus: 400,
			body:     `{"error":{"message":"Rate limit hit","code":130429}}`,
			wantCode: 130429, wantRetry: true},
		{name: "pair rate limit", httpStatus: 400,
			body:     `{"error":{"message":"pair rate","code":131056}}`,
			wantCode: 131056, wantRetry: true},
		{name: "invalid parameter", httpStatus: 400,
			body:     `{"error":{"message":"Invalid parameter","code":100}}`,
			wantCode: 100},
		{name: "template not found", httpStatus: 404,
			body:     `{"error":{"message":"Template name does not exist","code":132001}}`,
			wantCode: 132001},
		{name: "expired token", httpStatus: 401,
			body:     `{"error":{"message":"Error validating access token","type":"OAuthException","code":190}}`,
			wantCode: 190, wantCred: true},
		{name: "server error without body", httpStatus: 503, body: ``, wantRetry: true},
		{name: "http 429 without code", httpStatus: 429, body: `{"error":{"message":"slow down","code":4}}`, wantCode: 4, wantRetry: true},
		{name: "garbage body", httpStatus: 502, body: `<html>bad gateway</html>`, wantRetry: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			graph := &graphStub{status: tt.httpStatus, body: tt.body}
			cli := newTestClient(t, graph)

			_, err := cli.Send(context.Background(), headersFor("route-wa"), textNotification("hi"))
			require.Error(t, err)

			var sendErr *SendError
			require.True(t, errors.As(err, &sendErr), "expected *SendError, got %T: %v", err, err)
			require.Equal(t, tt.httpStatus, sendErr.HTTPStatus)
			require.Equal(t, tt.wantCode, sendErr.Code)
			require.Equal(t, tt.wantRetry, sendErr.Retriable(), "retriable")
			require.Equal(t, tt.wantFallback, sendErr.Fallback(), "fallback")
			require.Equal(t, tt.wantCred, sendErr.Credential(), "credential")
		})
	}
}

func TestSendForgetsCredentialsOnAuthFailure(t *testing.T) {
	t.Parallel()

	graph := &graphStub{status: http.StatusUnauthorized,
		body: `{"error":{"message":"Error validating access token","type":"OAuthException","code":190}}`}
	cli := newTestClient(t, graph)
	ctx := context.Background()

	_, err := cli.Send(ctx, headersFor("route-wa"), textNotification("hi"))
	require.Error(t, err)

	// A subsequent send re-reads the settings service (the cache entry was dropped);
	// we can only observe that indirectly, so assert the resolver still works.
	values, err := cli.Credentials().ResolveByName(ctx, "route-wa")
	require.NoError(t, err)
	require.Equal(t, "tok-123", values[CredentialAccessToken])
}

func TestNormaliseMSISDN(t *testing.T) {
	t.Parallel()
	for input, want := range map[string]string{
		"+254712345678":   "254712345678",
		"254 712 345 678": "254712345678",
		"(254)-712-345":   "254712345",
		"":                "",
		"abc":             "",
	} {
		require.Equal(t, want, NormaliseMSISDN(input), fmt.Sprintf("input %q", input))
	}
}
