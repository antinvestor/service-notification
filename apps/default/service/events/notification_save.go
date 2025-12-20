package events

import (
	"context"
	"errors"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/antinvestor/service-notification/apps/default/service/repository"
	"github.com/pitabwire/frame/data"
	"github.com/pitabwire/frame/events"
	"github.com/pitabwire/util"
)

// NotificationSaveEvent is the event name for saving notification records
const NotificationSaveEvent = "notification.save"

type NotificationSave struct {
	eventMan         events.Manager
	notificationRepo repository.NotificationRepository
}

// NewNotificationSave creates a new NotificationSave event handler
func NewNotificationSave(_ context.Context, eventMan events.Manager, notificationRepo repository.NotificationRepository) *NotificationSave {
	return &NotificationSave{
		eventMan:         eventMan,
		notificationRepo: notificationRepo,
	}
}

func (e *NotificationSave) Name() string {
	return NotificationSaveEvent
}

func (e *NotificationSave) PayloadType() any {
	return &models.Notification{}
}

func (e *NotificationSave) Validate(ctx context.Context, payload any) error {
	notification, ok := payload.(*models.Notification)
	if !ok {
		return errors.New(" payload is not of type models.Notification")
	}

	if notification.GetID() == "" {
		return errors.New(" notification Id should already have been set ")
	}

	return nil
}

func (e *NotificationSave) Execute(ctx context.Context, payload any) error {
	notification := payload.(*models.Notification)

	logger := util.Log(ctx).WithField("type", e.Name()).WithField("notification_id", notification.GetID())
	defer logger.Release()
	logger.Debug("event handler started")

	err := e.notificationRepo.Create(ctx, notification)
	if err != nil {
		logger.WithError(err).Error("could not save to db")
		return err
	}

	if !notification.OutBound {
		err = e.eventMan.Emit(ctx, NotificationInRouteEvent, notification.GetID())
		if err != nil {
			logger.WithError(err).Error("could not emit for in route")
			return err
		}

		logger.Debug("event handler completed successfully")
		return nil
	}

	if notification.IsReleased() {
		err = e.eventMan.Emit(ctx, NotificationOutRouteEvent, notification.GetID())
		if err != nil {
			logger.WithError(err).Error("could not emit for queue out")
			return err
		}
	} else {
		nStatus := models.NotificationStatus{
			NotificationID: notification.GetID(),
			State:          int32(commonv1.STATE_CHECKED.Number()),
			Status:         int32(commonv1.STATUS_QUEUED.Number()),
			Extra: data.JSONMap{
				"step": "pending_release",
			},
		}

		nStatus.GenID(ctx)

		// Queue out notification status for further processing
		err = e.eventMan.Emit(ctx, NotificationStatusSaveEvent, &nStatus)
		if err != nil {
			logger.WithError(err).Error("could not emit status")
			return err
		}
	}

	logger.Debug("event handler completed successfully")
	return nil
}
