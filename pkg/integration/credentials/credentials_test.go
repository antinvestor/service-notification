package credentials

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"buf.build/gen/go/antinvestor/settingz/connectrpc/go/settings/v1/settingsv1connect"
	settingsv1 "buf.build/gen/go/antinvestor/settingz/protocolbuffers/go/settings/v1"
	"connectrpc.com/connect"
	"github.com/antinvestor/service-notification/pkg/constants"
	"github.com/stretchr/testify/require"
)

// settingsStub is an in-process Connect settings service.
type settingsStub struct {
	settingsv1connect.UnimplementedSettingsServiceHandler
	values  map[string]string
	calls   atomic.Int32
	lastKey *settingsv1.Setting
}

func (s *settingsStub) Get(_ context.Context, req *connect.Request[settingsv1.GetRequest]) (*connect.Response[settingsv1.GetResponse], error) {
	s.calls.Add(1)
	s.lastKey = req.Msg.GetKey()
	value, ok := s.values[req.Msg.GetKey().GetName()]
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such setting"))
	}
	return connect.NewResponse(&settingsv1.GetResponse{
		Data: &settingsv1.SettingObject{Key: req.Msg.GetKey(), Value: value},
	}), nil
}

func newResolver(t *testing.T, stub *settingsStub, ttl time.Duration) *Resolver {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(settingsv1connect.NewSettingsServiceHandler(stub))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	cli := settingsv1connect.NewSettingsServiceClient(srv.Client(), srv.URL)
	return New(cli, "WhatsApp", "notification.whatsapp").WithTTL(ttl)
}

func TestConnectionName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		headers map[string]string
		want    string
	}{
		{name: "connection header wins", headers: map[string]string{
			constants.APIConnectionCredentialsHeaderName: "conn-a", constants.RouteIDHeaderName: "route-1"}, want: "conn-a"},
		{name: "route id fallback", headers: map[string]string{constants.RouteIDHeaderName: "route-1"}, want: "route-1"},
		{name: "nothing", headers: map[string]string{}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, ConnectionName(tt.headers))
		})
	}
}

func TestResolve(t *testing.T) {
	t.Parallel()

	stub := &settingsStub{values: map[string]string{
		"route-1": `{"access_token":"tok","phone_number_id":"123"}`,
		"broken":  `not json`,
		"empty":   ``,
	}}
	resolver := newResolver(t, stub, time.Minute)
	ctx := context.Background()

	values, err := resolver.Resolve(ctx, map[string]string{constants.RouteIDHeaderName: "route-1"})
	require.NoError(t, err)
	require.Equal(t, map[string]string{"access_token": "tok", "phone_number_id": "123"}, values)
	require.Equal(t, "WhatsApp", stub.lastKey.GetObject())
	require.Equal(t, "notification.whatsapp", stub.lastKey.GetObjectId())

	// Second resolve of the same connection is served from cache.
	_, err = resolver.Resolve(ctx, map[string]string{constants.RouteIDHeaderName: "route-1"})
	require.NoError(t, err)
	require.Equal(t, int32(1), stub.calls.Load())

	// Forget forces a fresh lookup.
	resolver.Forget("route-1")
	_, err = resolver.Resolve(ctx, map[string]string{constants.RouteIDHeaderName: "route-1"})
	require.NoError(t, err)
	require.Equal(t, int32(2), stub.calls.Load())

	_, err = resolver.Resolve(ctx, map[string]string{})
	require.ErrorIs(t, err, ErrNoConnection)

	_, err = resolver.ResolveByName(ctx, "missing")
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))

	_, err = resolver.ResolveByName(ctx, "broken")
	require.ErrorContains(t, err, "not a JSON object")

	_, err = resolver.ResolveByName(ctx, "empty")
	require.ErrorContains(t, err, "no value")
}

func TestResolveWithoutCache(t *testing.T) {
	t.Parallel()

	stub := &settingsStub{values: map[string]string{"route-1": `{"k":"v"}`}}
	resolver := newResolver(t, stub, 0)
	ctx := context.Background()

	for range 3 {
		_, err := resolver.ResolveByName(ctx, "route-1")
		require.NoError(t, err)
	}
	require.Equal(t, int32(3), stub.calls.Load())
}
