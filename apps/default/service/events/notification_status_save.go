package events

import (
	"context"
	"errors"

	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/antinvestor/service-notification/apps/default/service/repository"
	"github.com/pitabwire/frame"
	"gorm.io/gorm/clause"
)

type NotificationStatusSave struct {
	Service *frame.Service
}

func (e *NotificationStatusSave) Name() string {
	return "notificationStatus.save"
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

	result := e.Service.DB(ctx, false).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).Create(nStatus)

	err := result.Error
	if err != nil {
		logger.WithError(err).Warn("could not save notification status to db")
		return err
	}
	logger.WithField("rows affected", result.RowsAffected).Debug("successfully saved record to db")

	notificationRepo := repository.NewNotificationRepository(ctx, e.Service)
	n, err := notificationRepo.GetByID(ctx, nStatus.NotificationID)
	if err != nil {
		return err
	}

	n.StatusID = nStatus.ID
	n.State = nStatus.State
	if n.TransientID == "" {
		n.TransientID = nStatus.TransientID
	}

	err = notificationRepo.Save(ctx, n)
	if err != nil {
		logger.WithError(err).Warn("could not save notification update to db")

		return err
	}

	return nil
}
