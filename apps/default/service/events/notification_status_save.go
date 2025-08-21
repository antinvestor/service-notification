package events

import (
	"context"
	"errors"

	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/antinvestor/service-notification/apps/default/service/repository"
	"github.com/pitabwire/frame"
)

// NotificationStatusSaveEvent is the event name for saving notification status records
const NotificationStatusSaveEvent = "notificationStatus.save"

type NotificationStatusSave struct {
	Service          *frame.Service
	NotificationRepo repository.NotificationRepository
}

// NewNotificationStatusSave creates a new NotificationStatusSave event handler
func NewNotificationStatusSave(ctx context.Context, service *frame.Service) *NotificationStatusSave {
	return &NotificationStatusSave{
		Service:          service,
		NotificationRepo: repository.NewNotificationRepository(ctx, service),
	}
}

func (e *NotificationStatusSave) Name() string {
	return NotificationStatusSaveEvent
}

func (e *NotificationStatusSave) PayloadType() any {
	return &models.NotificationStatus{}
}

func (e *NotificationStatusSave) Validate(_ context.Context, payload any) error {
	notificationStatus, ok := payload.(*models.NotificationStatus)
	if !ok {
		return errors.New(" payload is not of type models.NotificationStatus")
	}

	if notificationStatus.GetID() == "" {
		return errors.New(" notificationStatus Id should already have been set ")
	}

	return nil
}

func (e *NotificationStatusSave) Execute(ctx context.Context, payload any) error {
	nStatus := payload.(*models.NotificationStatus)

	logger := e.Service.Log(ctx).WithField("payload", nStatus).WithField("type", e.Name())
	logger.Debug("handling event")

	result := e.Service.DB(ctx, false).Create(nStatus)

	err := result.Error
	if err != nil {
		logger.WithError(err).Warn("could not save notification status to db")
		return err
	}
	logger.WithField("rows affected", result.RowsAffected).Debug("successfully saved record to db")

	n, err := e.NotificationRepo.GetByID(ctx, nStatus.NotificationID)
	if err != nil {
		return err
	}

	n.StatusID = nStatus.ID
	n.State = nStatus.State
	if n.TransientID == "" {
		n.TransientID = nStatus.TransientID
	}

	err = e.NotificationRepo.Save(ctx, n)
	if err != nil {
		logger.WithError(err).Warn("could not save notification update to db")

		return err
	}

	return nil
}
