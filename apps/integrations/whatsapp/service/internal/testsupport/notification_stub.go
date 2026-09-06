// Package testsupport hosts an in-process notification service for integration tests.
package testsupport

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	"buf.build/gen/go/antinvestor/notification/connectrpc/go/notification/v1/notificationv1connect"
	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"buf.build/gen/go/antinvestor/settingz/connectrpc/go/settings/v1/settingsv1connect"
	settingsv1 "buf.build/gen/go/antinvestor/settingz/protocolbuffers/go/settings/v1"
	"connectrpc.com/connect"
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

func (s *NotificationStub) Receive(_ context.Context, req *connect.Request[notificationv1.ReceiveRequest], stream *connect.ServerStream[notificationv1.ReceiveResponse]) error {
	s.mu.Lock()
	s.Received = append(s.Received, req.Msg.GetData()...)
	s.mu.Unlock()

	responses := make([]*commonv1.StatusResponse, 0, len(req.Msg.GetData()))
	for _, n := range req.Msg.GetData() {
		responses = append(responses, &commonv1.StatusResponse{Id: n.GetId(), State: commonv1.STATE_CREATED})
	}
	return stream.Send(&notificationv1.ReceiveResponse{Data: responses})
}

func (s *NotificationStub) StatusUpdate(_ context.Context, req *connect.Request[commonv1.StatusUpdateRequest]) (*connect.Response[commonv1.StatusUpdateResponse], error) {
	if s.UnknownExternalIDs[req.Msg.GetExternalId()] {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no notification for external id"))
	}
	s.mu.Lock()
	s.Statuses = append(s.Statuses, req.Msg)
	s.mu.Unlock()
	return connect.NewResponse(&commonv1.StatusUpdateResponse{Data: &commonv1.StatusResponse{
		Id: req.Msg.GetId(), ExternalId: req.Msg.GetExternalId(), State: req.Msg.GetState(), Status: req.Msg.GetStatus(),
	}}), nil
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
	mux.Handle(notificationv1connect.NewNotificationServiceHandler(stub))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return notificationv1connect.NewNotificationServiceClient(srv.Client(), srv.URL)
}

// SettingsStub answers settings lookups from a fixed map of connection name to JSON.
type SettingsStub struct {
	settingsv1connect.UnimplementedSettingsServiceHandler
	Values map[string]string
}

func (s *SettingsStub) Get(_ context.Context, req *connect.Request[settingsv1.GetRequest]) (*connect.Response[settingsv1.GetResponse], error) {
	value, ok := s.Values[req.Msg.GetKey().GetName()]
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such setting"))
	}
	return connect.NewResponse(&settingsv1.GetResponse{Data: &settingsv1.SettingObject{Key: req.Msg.GetKey(), Value: value}}), nil
}

// NewSettingsClient serves the stub over HTTP and returns a Connect client for it.
func NewSettingsClient(t *testing.T, stub *SettingsStub) settingsv1connect.SettingsServiceClient {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(settingsv1connect.NewSettingsServiceHandler(stub))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return settingsv1connect.NewSettingsServiceClient(srv.Client(), srv.URL)
}
