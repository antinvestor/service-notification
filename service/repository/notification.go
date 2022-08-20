package repository

import (
	"context"

	"github.com/antinvestor/service-notification/service/models"
	"github.com/pitabwire/frame"
	"gorm.io/gorm"
)

type NotificationRepository interface {
	GetByID(id string) (*models.Notification, error)
	GetByPartitionAndID(partitionID string, id string) (*models.Notification, error)
	SearchByPartition(partitionID string, query string) ([]models.Notification, error)
	Save(notification *models.Notification) error
}

type notificationRepository struct {
	readDb  *gorm.DB
	writeDb *gorm.DB
}

func NewNotificationRepository(ctx context.Context, service *frame.Service) NotificationRepository {
	return &notificationRepository{readDb: service.DB(ctx, true), writeDb: service.DB(ctx, false)}
}

func (repo *notificationRepository) GetByPartitionAndID(partitionId string, id string) (*models.Notification, error) {
	notification := models.Notification{}
	err := repo.readDb.First(&notification, "partition_id = ? AND id = ?", partitionId, id).Error
	if err != nil {
		return nil, err
	}
	return &notification, nil
}

func (repo *notificationRepository) GetByID(id string) (*models.Notification, error) {
	notification := models.Notification{}
	err := repo.readDb.First(&notification, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &notification, nil
}

func (repo *notificationRepository) SearchByPartition(partitionID string, query string) ([]models.Notification, error) {
	var notifications []models.Notification
	notificationQuery := repo.readDb.Where("partition_id = ?", partitionID)
	if query != "" {
		notificationQuery = notificationQuery.
			Where(" id ILIKE ? OR external_id ILIKE ? OR transient_id ILIKE ?", query, query, query)
	}

	err := notificationQuery.Find(&notifications).Error
	if err != nil {
		return nil, err
	}
	return notifications, nil
}
func (repo *notificationRepository) Save(notification *models.Notification) error {
	return repo.writeDb.Save(notification).Error
}
