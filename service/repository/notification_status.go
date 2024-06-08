package repository

import (
	"context"

	"github.com/antinvestor/service-notification/service/models"
	"github.com/pitabwire/frame"
)

type NotificationStatusRepository interface {
	GetByID(ctx context.Context, id string) (*models.NotificationStatus, error)
	GetByNotificationID(ctx context.Context, notificationId string) ([]models.NotificationStatus, error)
	Save(ctx context.Context, notification *models.NotificationStatus) error
}

type notificationStatusRepository struct {
	abstractRepository
}

func NewNotificationStatusRepository(ctx context.Context, service *frame.Service) NotificationStatusRepository {
	return &notificationStatusRepository{abstractRepository{service: service}}
}

func (repo *notificationStatusRepository) GetByID(ctx context.Context, id string) (*models.NotificationStatus, error) {
	notificationStatus := models.NotificationStatus{}
	err := repo.readDb(ctx).First(&notificationStatus, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &notificationStatus, nil
}

func (repo *notificationStatusRepository) GetByNotificationID(ctx context.Context, notificationId string) ([]models.NotificationStatus, error) {
	var notificationStatusList []models.NotificationStatus

	err := repo.readDb(ctx).Find(&notificationStatusList,
		"notification_id = ? ", notificationId).Error
	if err != nil {
		return nil, err
	}
	return notificationStatusList, nil
}

func (repo *notificationStatusRepository) Save(ctx context.Context, notificationStatus *models.NotificationStatus) error {
	return repo.writeDb(ctx).Save(notificationStatus).Error
}
