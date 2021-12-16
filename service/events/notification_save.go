package events

import (
	"context"
	"errors"
	"github.com/antinvestor/apis/common"
	"github.com/antinvestor/service-notification/service/models"
	"github.com/pitabwire/frame"
)

type NotificationSave struct {
	Service *frame.Service
}

func (e *NotificationSave) Name() string {
	return "notification.save"
}

func (e *NotificationSave) PayloadType() interface{} {
	return &models.Notification{}
}

func (e *NotificationSave) Validate(ctx context.Context, payload interface{}) error {
	notification, ok := payload.(*models.Notification)
	if !ok {
		return errors.New(" payload is not of type models.Notification")
	}

	if notification.GetID() == "" {
		return errors.New(" notification Id should already have been set ")
	}

	return nil
}

func (e *NotificationSave) Execute(ctx context.Context, payload interface{}) error {

	notification := payload.(*models.Notification)
	err := e.Service.DB(ctx, false).Save(notification).Error

	if err != nil {
		return err
	}

	if notification.OutBound {

		if notification.IsReleased() {

			event := NotificationOutRoute{}
			err = e.Service.Emit(ctx, event.Name(), notification.GetID())
			if err != nil {
				return err
			}
		} else {
			nStatus := models.NotificationStatus{
				NotificationID: notification.GetID(),
				State:          int32(common.STATE_CHECKED.Number()),
				Status:         int32(common.STATUS_QUEUED.Number()),
			}

			nStatus.GenID(ctx)

			// Queue out notification status for further processing
			eventStatus := NotificationStatusSave{}
			err = e.Service.Emit(ctx, eventStatus.Name(), nStatus)
			if err != nil {
				return err
			}

		}
	} else {

		event := NotificationInRoute{}
		err = e.Service.Emit(ctx, event.Name(), notification.GetID())
		if err != nil {
			return err
		}
	}

	return nil
}
