package repository

import (
	"context"

	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/pitabwire/frame/datastore"
	"github.com/pitabwire/frame/datastore/pool"
	"github.com/pitabwire/frame/workerpool"
)

type NotificationRepository interface {
	datastore.BaseRepository[*models.Notification]
	GetByIDList(ctx context.Context, id ...string) ([]*models.Notification, error)
}

type notificationRepository struct {
	datastore.BaseRepository[*models.Notification]
}

func NewNotificationRepository(ctx context.Context, dbPool pool.Pool, workMan workerpool.Manager) NotificationRepository {
	return &notificationRepository{
		BaseRepository: datastore.NewBaseRepository[*models.Notification](
			ctx, dbPool, workMan, func() *models.Notification { return &models.Notification{} },
		),
	}
}

func (repo *notificationRepository) GetByPartitionAndID(ctx context.Context, partitionID string, id string) (*models.Notification, error) {
	notification := models.Notification{}
	err := repo.Pool().DB(ctx, true).First(&notification, "partition_id = ? AND id = ?", partitionID, id).Error
	if err != nil {
		return nil, err
	}
	return &notification, nil
}

func (repo *notificationRepository) GetByIDList(ctx context.Context, id ...string) ([]*models.Notification, error) {
	var notifications []*models.Notification
	err := repo.Pool().DB(ctx, true).Find(&notifications, "id IN ?", id).Error
	if err != nil {
		return nil, err
	}
	return notifications, nil
}
