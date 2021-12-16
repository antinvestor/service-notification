package events

import (
	"context"
	"errors"
	"github.com/antinvestor/service-notification/service/models"
	"github.com/antinvestor/service-notification/service/repository"
	"github.com/pitabwire/frame"
)

type NotificationStatusSave struct {
	Service *frame.Service
}

func (e *NotificationStatusSave) Name() string {
	return "notificationStatus.save"
}

func (e *NotificationStatusSave) PayloadType() interface{} {
	return &models.NotificationStatus{}
}

func (e *NotificationStatusSave) Validate(ctx context.Context, payload interface{}) error {
	notificationStatus, ok := payload.(*models.NotificationStatus)
	if !ok {
		return errors.New(" payload is not of type models.NotificationStatus")
	}

	if notificationStatus.GetID() == "" {
		return errors.New(" notificationStatus Id should already have been set ")
	}

	return nil
}

func (e *NotificationStatusSave) Execute(ctx context.Context, payload interface{}) error {

	nStatus := payload.(*models.NotificationStatus)

	err := e.Service.DB(ctx, false).Save(nStatus).Error

	if err != nil {
		return err
	}

	notificationRepo := repository.NewNotificationRepository(ctx, e.Service)
	n, err := notificationRepo.GetByID(nStatus.NotificationID)
	if err != nil {
		return err
	}

	n.StatusID = nStatus.ID
	n.State = nStatus.State
	if n.TransientID == "" {
		n.TransientID = nStatus.TransientID
	}

	err = notificationRepo.Save(n)
	if err != nil {
		return err
	}

	return nil
}
