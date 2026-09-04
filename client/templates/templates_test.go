package templates_test

import (
	"context"
	"errors"
	"testing"

	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/antinvestor/service-notification/client/templates"
)

type fakeSaver struct {
	reqs []*notificationv1.TemplateSaveRequest
	err  error
}

func (f *fakeSaver) TemplateSave(_ context.Context, req *connect.Request[notificationv1.TemplateSaveRequest]) (*connect.Response[notificationv1.TemplateSaveResponse], error) {
	if f.err != nil {
		return nil, f.err
	}
	f.reqs = append(f.reqs, req.Msg)
	return connect.NewResponse(notificationv1.TemplateSaveResponse_builder{}.Build()), nil
}

func valid() templates.Template {
	return templates.Template{
		Name:      "template.demo.order.shipped",
		Subject:   "Order {{.reference}} shipped",
		Bodies:    map[string]string{templates.ChannelSMS: "Order {{.reference}} shipped.", templates.ChannelEmail: "<p>Order {{.reference}} shipped.</p>"},
		Variables: []string{"reference"},
	}
}

func TestSyncSendsSubjectChannelsAndExtra(t *testing.T) {
	saver := &fakeSaver{}
	n, err := templates.Sync(context.Background(), saver, "service_demo", []templates.Template{valid()})
	require.NoError(t, err)
	require.Equal(t, 1, n)
	req := saver.reqs[0]
	require.Equal(t, "template.demo.order.shipped", req.GetName())
	require.Equal(t, templates.DefaultLanguage, req.GetLanguageCode())
	data := req.GetData().AsMap()
	require.Equal(t, "Order {{.reference}} shipped", data[templates.SubjectKey])
	require.Contains(t, data, templates.ChannelSMS)
	require.Contains(t, data, templates.ChannelEmail)
	extra := req.GetExtra().AsMap()
	require.Equal(t, "service_demo", extra["owner"])
	require.Equal(t, []any{"reference"}, extra["variables"])
}

func TestValidateRejectsBadNamesBodiesAndChannels(t *testing.T) {
	bad := valid()
	bad.Name = "OrderShipped"
	require.Error(t, bad.Validate())

	bad = valid()
	bad.Bodies = map[string]string{"subject": "x"}
	require.Error(t, bad.Validate())

	bad = valid()
	bad.Bodies[templates.ChannelSMS] = "{{.unclosed"
	require.Error(t, bad.Validate())

	bad = valid()
	bad.Bodies["a-very-long-channel"] = "x"
	require.Error(t, bad.Validate())
}

func TestSyncStopsAtFirstFailureAndRejectsDuplicates(t *testing.T) {
	saver := &fakeSaver{err: errors.New("boom")}
	n, err := templates.Sync(context.Background(), saver, "svc", []templates.Template{valid()})
	require.Error(t, err)
	require.Equal(t, 0, n)

	n, err = templates.Sync(context.Background(), &fakeSaver{}, "svc", []templates.Template{valid(), valid()})
	require.Error(t, err)
	require.Equal(t, 1, n)
}
