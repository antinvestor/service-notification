package events

import (
	"context"
	"errors"
	"github.com/antinvestor/apis/common"
	"github.com/antinvestor/service-notification/service/models"
	"github.com/pitabwire/frame"
	"github.com/sirupsen/logrus"
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
	logger := logrus.WithField("payload", payload).WithField("type", e.Name())
	logger.Info("handling event")

	notification := payload.(*models.Notification)
	err := e.Service.DB(ctx, false).Save(notification).Error
	if err != nil {
		logger.WithError(err).Warn("could not save to db")
		return err
	}

	if !notification.OutBound {
		event := NotificationInRoute{}
		err = e.Service.Emit(ctx, event.Name(), notification.GetID())
		if err != nil {
			return err
		}
	}

	if notification.IsReleased() {
		event := NotificationOutRoute{}
		err = e.Service.Emit(ctx, event.Name(), notification.GetID())
		if err != nil {
			logger.WithError(err).Warn("could not emit for queue out")
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
			logger.WithError(err).Warn("could not emit status")
			return err
		}
	}
	return nil
}
