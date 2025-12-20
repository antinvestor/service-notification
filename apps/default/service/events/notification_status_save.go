package events

import (
	"context"
	"errors"

	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/antinvestor/service-notification/apps/default/service/repository"
	"github.com/pitabwire/util"
)

// NotificationStatusSaveEvent is the event name for saving notification status records
const NotificationStatusSaveEvent = "notificationStatus.save"

type NotificationStatusSave struct {
	NotificationRepo       repository.NotificationRepository
	notificationStatusRepo repository.NotificationStatusRepository
}

// NewNotificationStatusSave creates a new NotificationStatusSave event handler
func NewNotificationStatusSave(ctx context.Context, notificationRepo repository.NotificationRepository, notificationStatusRepo repository.NotificationStatusRepository) *NotificationStatusSave {

	return &NotificationStatusSave{
		NotificationRepo:       notificationRepo,
		notificationStatusRepo: notificationStatusRepo,
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

	logger := util.Log(ctx).WithField("type", e.Name()).WithField("notification_id", nStatus.NotificationID)
	logger.Debug("event handler started")

	err := e.notificationStatusRepo.Create(ctx, nStatus)
	if err != nil {
		logger.WithError(err).Error("could not save notification status to db")
		return err
	}

	n, err := e.NotificationRepo.GetByID(ctx, nStatus.NotificationID)
	if err != nil {
		logger.WithError(err).Error("could not get notification from db")
		return err
	}

	n.StatusID = nStatus.ID
	n.State = nStatus.State
	if n.TransientID == "" {
		n.TransientID = nStatus.TransientID
	}

	_, err = e.NotificationRepo.Update(ctx, n, "status_id", "state", "transient_id")
	if err != nil {
		logger.WithError(err).Error("could not save notification update to db")
		return err
	}

	logger.Debug("event handler completed successfully")
	return nil
}
