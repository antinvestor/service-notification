package repository

import (
	"context"

	"github.com/antinvestor/service-notification/service/models"
	"github.com/pitabwire/frame"
	"gorm.io/gorm"
)

type NotificationStatusRepository interface {
	GetByID(id string) (*models.NotificationStatus, error)
	GetByNotificationID(notificationId string) ([]models.NotificationStatus, error)
	Save(notification *models.NotificationStatus) error

}

type notificationStatusRepository struct {
	readDb  *gorm.DB
	writeDb *gorm.DB
}

func NewNotificationStatusRepository(ctx context.Context, service *frame.Service) NotificationStatusRepository {
	return &notificationStatusRepository{readDb: service.DB(ctx, true), writeDb: service.DB(ctx, false)}
}


func (repo *notificationStatusRepository) GetByID(id string) (*models.NotificationStatus, error) {
	notificationStatus := models.NotificationStatus{}
	err := repo.readDb.First(&notificationStatus, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &notificationStatus, nil
}

func (repo *notificationStatusRepository) GetByNotificationID(notificationId string) ([]models.NotificationStatus, error) {
	var notificationStatusList []models.NotificationStatus

	err := repo.readDb.Find(&notificationStatusList,
		"notification_id = ? ",	notificationId).Error
	if err != nil {
		return nil, err
	}
	return notificationStatusList, nil
}

func (repo *notificationStatusRepository) Save(notificationStatus *models.NotificationStatus) error {
	return repo.writeDb.Save(notificationStatus).Error
}
