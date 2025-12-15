package events

import (
	"context"
	"errors"
	"log/slog"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	"buf.build/gen/go/antinvestor/notification/connectrpc/go/notification/v1/notificationv1connect"
	"connectrpc.com/connect"
	"github.com/pitabwire/util"
)

// NotificationStatusUpdateEvent is the event name for updating notification statuses
const NotificationStatusUpdateEvent = "notificationStatus.update"

type NotificationStatusUpdate struct {
	NotificationCli notificationv1connect.NotificationServiceClient
}

// NewNotificationStatusUpdate creates a new NotificationStatusUpdate event handler
func NewNotificationStatusUpdate(ctx context.Context, notificationCli notificationv1connect.NotificationServiceClient) *NotificationStatusUpdate {

	return &NotificationStatusUpdate{
		NotificationCli: notificationCli,
	}
}

func (e *NotificationStatusUpdate) Name() string {
	return NotificationStatusUpdateEvent
}

func (e *NotificationStatusUpdate) PayloadType() any {
	return &commonv1.StatusUpdateRequest{}
}

func (e *NotificationStatusUpdate) Validate(_ context.Context, payload any) error {
	statusUpdateRequest, ok := payload.(*commonv1.StatusUpdateRequest)
	if !ok {
		return errors.New(" payload is not of type models.NotificationStatus")
	}

	if statusUpdateRequest.GetId() == "" {
		return errors.New(" statusUpdateRequest Id should already have been set ")
	}

	return nil
}

func (e *NotificationStatusUpdate) Execute(ctx context.Context, payload any) error {
	statusUpdateRequest := payload.(*commonv1.StatusUpdateRequest)

	logger := util.Log(ctx).WithField("type", e.Name())
	defer logger.Release()
	if logger.Enabled(ctx, slog.LevelDebug) {

		logger.WithField("payload", statusUpdateRequest).Debug("handling event")
	}
	_, err := e.NotificationCli.StatusUpdate(ctx, connect.NewRequest(statusUpdateRequest))
	if err != nil {
		logger.WithError(err).WithField("notification_id", statusUpdateRequest.GetId()).Warn("could not update status")
		return nil
	}

	return nil
}
