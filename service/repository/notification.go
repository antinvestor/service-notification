package repository

import (
	"antinvestor.com/service/notification/service/repository/models"
	"antinvestor.com/service/notification/utils"
	"context"
	"github.com/jinzhu/gorm"
)

type NotificationRepository interface {
	GetByID(id string) (*models.Notification, error)
	GetByIDAndProductID(id string, productId string) (*models.Notification, error)
	Search(query string, productId string) ([]models.Notification, error)
	Save(notification *models.Notification) error
}

type notificationRepository struct {
	readDb  *gorm.DB
	writeDb *gorm.DB
}

func NewNotificationRepository(ctx context.Context, env *utils.Env) NotificationRepository {
	return &notificationRepository{readDb: env.GetRDb(ctx), writeDb: env.GeWtDb(ctx)}
}

func (repo *notificationRepository) GetByIDAndProductID(id string, productId string) (*models.Notification, error) {
	notification := models.Notification{}
	err := repo.readDb.First(&notification, "notification_id = ? and product_id = ?", id, productId).Error
	if err != nil {
		return nil, err
	}
	return &notification, nil
}

func (repo *notificationRepository) GetByID(id string) (*models.Notification, error) {
	notification := models.Notification{}
	err := repo.readDb.First(&notification, "notification_id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &notification, nil
}

func (repo *notificationRepository) Search(query string, productId string) ([]models.Notification, error) {
	var notifications []models.Notification

	err := repo.readDb.Find(&notifications,
		"product_id = ? AND (notification_id LIKE ? OR external_id LIKE ? OR transient_id LIKE ?)",
		productId, query).Error
	if err != nil {
		return nil, err
	}
	return notifications, nil
}

func (repo *notificationRepository) Save(notification *models.Notification) error {
	return repo.writeDb.Save(notification).Error
}
