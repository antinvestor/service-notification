package repository

import (
	"context"

	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/pitabwire/frame/datastore"
	"github.com/pitabwire/frame/datastore/pool"
	"github.com/pitabwire/frame/workerpool"
)

type NotificationStatusRepository interface {
	datastore.BaseRepository[*models.NotificationStatus]
	GetByIDList(ctx context.Context, id ...string) ([]*models.NotificationStatus, error)
	GetByNotificationID(ctx context.Context, notificationId string) ([]models.NotificationStatus, error)
}

type notificationStatusRepository struct {
	datastore.BaseRepository[*models.NotificationStatus]
}

func NewNotificationStatusRepository(ctx context.Context, dbPool pool.Pool, workMan workerpool.Manager) NotificationStatusRepository {
	return &notificationStatusRepository{
		BaseRepository: datastore.NewBaseRepository[*models.NotificationStatus](
			ctx, dbPool, workMan, func() *models.NotificationStatus { return &models.NotificationStatus{} },
		),
	}
}

func (repo *notificationStatusRepository) GetByIDList(ctx context.Context, id ...string) ([]*models.NotificationStatus, error) {
	var notificationStatuses []*models.NotificationStatus
	err := repo.Pool().DB(ctx, true).Find(&notificationStatuses, "id IN ?", id).Error
	if err != nil {
		return nil, err
	}
	return notificationStatuses, nil
}

func (repo *notificationStatusRepository) GetByNotificationID(ctx context.Context, notificationId string) ([]models.NotificationStatus, error) {
	var notificationStatusList []models.NotificationStatus

	err := repo.Pool().DB(ctx, true).Find(&notificationStatusList,
		"notification_id = ? ", notificationId).Error
	if err != nil {
		return nil, err
	}
	return notificationStatusList, nil
}
