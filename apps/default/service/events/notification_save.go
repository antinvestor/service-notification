package events

import (
	"context"
	"errors"

	commonv1 "github.com/antinvestor/apis/go/common/v1"
	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/pitabwire/frame"
)

// NotificationSaveEvent is the event name for saving notification records
const NotificationSaveEvent = "notification.save"

type NotificationSave struct {
	Service *frame.Service
}

// NewNotificationSave creates a new NotificationSave event handler
func NewNotificationSave(ctx context.Context, service *frame.Service) *NotificationSave {
	return &NotificationSave{
		Service: service,
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

	logger := e.Service.Log(ctx).WithField("type", e.Name())
	logger.WithField("payload", notification).Debug("handling event")

	result := e.Service.DB(ctx, false).Create(notification)
	err := result.Error
	if err != nil {
		logger.WithError(err).Warn("could not save to db")
		return err
	}
	logger.WithField("rows affected", result.RowsAffected).Debug("successfully saved notification to db")

	if !notification.OutBound {
		err = e.Service.Emit(ctx, NotificationInRouteEvent, notification.GetID())
		if err != nil {
			return err
		}

		return nil
	}

	if notification.IsReleased() {
		err = e.Service.Emit(ctx, NotificationOutRouteEvent, notification.GetID())
		if err != nil {
			logger.WithError(err).Warn("could not emit for queue out")
			return err
		}
	} else {
		nStatus := models.NotificationStatus{
			NotificationID: notification.GetID(),
			State:          int32(commonv1.STATE_CHECKED.Number()),
			Status:         int32(commonv1.STATUS_QUEUED.Number()),
		}

		nStatus.GenID(ctx)

		// Queue out notification status for further processing
		err = e.Service.Emit(ctx, NotificationStatusSaveEvent, nStatus)
		if err != nil {
			logger.WithError(err).Warn("could not emit status")
			return err
		}
	}
	return nil
}
