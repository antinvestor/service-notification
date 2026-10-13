// Package testsupport hosts an in-process notification service for integration tests.
package testsupport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	"buf.build/gen/go/antinvestor/notification/connectrpc/go/v2/notification/v1/notificationv1connect"
	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"buf.build/gen/go/antinvestor/settingz/connectrpc/go/v2/settings/v1/settingsv1connect"
	settingsv1 "buf.build/gen/go/antinvestor/settingz/protocolbuffers/go/settings/v1"
	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
)

// NotificationStub records what an integration sends to the notification service.
type NotificationStub struct {
	notificationv1connect.UnimplementedNotificationServiceHandler

	mu       sync.Mutex
	Received []*notificationv1.Notification
	Statuses []*commonv1.StatusUpdateRequest
	// UnknownExternalIDs makes StatusUpdate answer NotFound for these external ids.
	UnknownExternalIDs map[string]bool
}

func (s *NotificationStub) Receive(_ context.Context, req *notificationv1.ReceiveRequest, stream notificationv1connect.NotificationServiceReceiveServerStream) error {
	s.mu.Lock()
	s.Received = append(s.Received, req.GetData()...)
	s.mu.Unlock()

	responses := make([]*commonv1.StatusResponse, 0, len(req.GetData()))
	for _, n := range req.GetData() {
		responses = append(responses, &commonv1.StatusResponse{Id: n.GetId(), State: commonv1.STATE_CREATED})
	}
	return stream.Send(&notificationv1.ReceiveResponse{Data: responses})
}

func (s *NotificationStub) StatusUpdate(_ context.Context, req *commonv1.StatusUpdateRequest) (*commonv1.StatusUpdateResponse, error) {
	if s.UnknownExternalIDs[req.GetExternalId()] {
		return nil, connect.NewError(connect.CodeNotFound, "no notification for external id")
	}
	s.mu.Lock()
	s.Statuses = append(s.Statuses, req)
	s.mu.Unlock()
	return &commonv1.StatusUpdateResponse{Data: &commonv1.StatusResponse{
		Id: req.GetId(), ExternalId: req.GetExternalId(), State: req.GetState(), Status: req.GetStatus(),
	}}, nil
}

// Snapshot returns copies of the recorded calls.
func (s *NotificationStub) Snapshot() ([]*notificationv1.Notification, []*commonv1.StatusUpdateRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*notificationv1.Notification(nil), s.Received...), append([]*commonv1.StatusUpdateRequest(nil), s.Statuses...)
}

// NewNotificationClient serves the stub over HTTP and returns a Connect client for it.
func NewNotificationClient(t *testing.T, stub *NotificationStub) notificationv1connect.NotificationServiceClient {
	t.Helper()
	mux := http.NewServeMux()
	server := connect.NewServer()
	notificationv1connect.RegisterNotificationServiceHandler(server, stub)
	connecthttp.Mount(mux, server)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return notificationv1connect.NewNotificationServiceClient(connect.NewClient(connecthttp.NewTransport(srv.Client(), srv.URL)))
}

// SettingsStub answers settings lookups from a fixed map of connection name to JSON.
type SettingsStub struct {
	settingsv1connect.UnimplementedSettingsServiceHandler
	Values map[string]string
}

func (s *SettingsStub) Get(_ context.Context, req *settingsv1.GetRequest) (*settingsv1.GetResponse, error) {
	value, ok := s.Values[req.GetKey().GetName()]
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, "no such setting")
	}
	return &settingsv1.GetResponse{Data: &settingsv1.SettingObject{Key: req.GetKey(), Value: value}}, nil
}

// NewSettingsClient serves the stub over HTTP and returns a Connect client for it.
func NewSettingsClient(t *testing.T, stub *SettingsStub) settingsv1connect.SettingsServiceClient {
	t.Helper()
	mux := http.NewServeMux()
	server := connect.NewServer()
	settingsv1connect.RegisterSettingsServiceHandler(server, stub)
	connecthttp.Mount(mux, server)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return settingsv1connect.NewSettingsServiceClient(connect.NewClient(connecthttp.NewTransport(srv.Client(), srv.URL)))
}
