package repository

import (
	"context"
	"github.com/antinvestor/service-notification/service/repository/models"
	"github.com/go-errors/errors"
	"github.com/pitabwire/frame"
	"gorm.io/gorm"
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

func NewNotificationRepository(ctx context.Context, service *frame.Service) NotificationRepository {
	return &notificationRepository{readDb: service.DB(ctx,true), writeDb: service.DB(ctx,false)}
}

func (repo *notificationRepository) GetByIDAndProductID(id string, productId string) (*models.Notification, error) {
	notification := models.Notification{}
	err := repo.readDb.First(&notification, "id = ? and product_id = ?", id, productId).Error
	if err != nil {
		return nil, errors.Wrap(err, 1)
	}
	return &notification, nil
}

func (repo *notificationRepository) GetByID(id string) (*models.Notification, error) {
	notification := models.Notification{}
	err := repo.readDb.First(&notification, "id = ?", id).Error
	if err != nil {
		return nil, errors.Wrap(err, 1)
	}
	return &notification, nil
}

func (repo *notificationRepository) Search(query string, productId string) ([]models.Notification, error) {
	var notifications []models.Notification

	err := repo.readDb.Find(&notifications,
		"product_id = ? AND (id LIKE ? OR external_id LIKE ? OR transient_id LIKE ?)",
		productId, query).Error
	if err != nil {
		return nil, errors.Wrap(err, 1)
	}
	return notifications, nil
}

func (repo *notificationRepository) Save(notification *models.Notification) error {
	return repo.writeDb.Save(notification).Error
}
