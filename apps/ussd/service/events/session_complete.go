package events

import (
	"context"
	"errors"

	"buf.build/gen/go/antinvestor/notification/connectrpc/go/notification/v1/notificationv1connect"
	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"connectrpc.com/connect"
	"github.com/pitabwire/util"
)

// SessionCompleteEvent is the event name for USSD session completion.
const SessionCompleteEvent = "ussd.session.complete"

// SessionComplete handles the emission of a notification when a USSD session
// finishes collecting data. It forwards the collected data to the notification
// service as an inbound notification.
type SessionComplete struct {
	notificationCli notificationv1connect.NotificationServiceClient
}

// NewSessionComplete creates a new session complete event handler.
func NewSessionComplete(_ context.Context, notificationCli notificationv1connect.NotificationServiceClient) *SessionComplete {
	return &SessionComplete{
		notificationCli: notificationCli,
	}
}

func (e *SessionComplete) Name() string {
	return SessionCompleteEvent
}

func (e *SessionComplete) PayloadType() any {
	return &notificationv1.Notification{}
}

func (e *SessionComplete) Validate(_ context.Context, payload any) error {
	notification, ok := payload.(*notificationv1.Notification)
	if !ok {
		return errors.New("payload is not of type notificationv1.Notification")
	}
	if notification.GetSource() == nil || notification.GetSource().GetContactId() == "" {
		return errors.New("notification source contact (MSISDN) is required")
	}
	return nil
}

func (e *SessionComplete) Execute(ctx context.Context, payload any) error {
	notification := payload.(*notificationv1.Notification)

	logger := util.Log(ctx).WithFields(map[string]any{
		"type":   e.Name(),
		"msisdn": notification.GetSource().GetContactId(),
	})
	defer logger.Release()
	logger.Debug("event handler started")

	if e.notificationCli == nil {
		logger.Debug("no notification client configured, skipping")
		return nil
	}

	stream, err := e.notificationCli.Receive(ctx, connect.NewRequest(&notificationv1.ReceiveRequest{
		Data: []*notificationv1.Notification{notification},
	}))
	if err != nil {
		logger.WithError(err).Warn("failed to send session completion to notification service")
		return nil
	}

	// Drain the stream
	for stream.Receive() {
		// Response received
	}
	if err := stream.Err(); err != nil {
		logger.WithError(err).Warn("error reading notification response stream")
	}

	logger.Debug("event handler completed successfully")
	return nil
}
