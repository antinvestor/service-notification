package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/pitabwire/frame"
)

type NotificationRepository interface {
	GetByID(ctx context.Context, id string) (*models.Notification, error)
	GetByIDList(ctx context.Context, id ...string) ([]*models.Notification, error)
	Search(ctx context.Context, query string) ([]*models.Notification, error)
	Save(ctx context.Context, notification *models.Notification) error
}

type notificationRepository struct {
	abstractRepository
}

func NewNotificationRepository(ctx context.Context, service *frame.Service) NotificationRepository {
	return &notificationRepository{abstractRepository{service: service}}
}

func (repo *notificationRepository) GetByPartitionAndID(ctx context.Context, partitionID string, id string) (*models.Notification, error) {
	notification := models.Notification{}
	err := repo.readDb(ctx).First(&notification, "partition_id = ? AND id = ?", partitionID, id).Error
	if err != nil {
		return nil, err
	}
	return &notification, nil
}

func (repo *notificationRepository) GetByID(ctx context.Context, id string) (*models.Notification, error) {
	notification := models.Notification{}
	err := repo.readDb(ctx).First(&notification, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &notification, nil
}

func (repo *notificationRepository) GetByIDList(ctx context.Context, id ...string) ([]*models.Notification, error) {
	var notifications []*models.Notification
	err := repo.readDb(ctx).Find(&notifications, "id IN ?", id).Error
	if err != nil {
		return nil, err
	}
	return notifications, nil
}

func (repo *notificationRepository) Search(ctx context.Context, query string) ([]*models.Notification, error) {
	query = strings.TrimSpace(query)
	var notifications []*models.Notification
	notificationQuery := repo.readDb(ctx)
	if query != "" {
		searchQ := fmt.Sprintf("%%%s%%", query)

		notificationQuery = notificationQuery.
			Where(" id ILIKE ? OR external_id ILIKE ? OR transient_id ILIKE ?", searchQ, searchQ, searchQ)
	}

	err := notificationQuery.Find(&notifications).Error
	if err != nil {
		return nil, err
	}
	return notifications, nil
}
func (repo *notificationRepository) Save(ctx context.Context, notification *models.Notification) error {
	return repo.writeDb(ctx).Save(notification).Error
}
