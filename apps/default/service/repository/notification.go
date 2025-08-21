package repository

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/framedata"
)

var permittedSearchKeys = []string{"parent_id", "id", "partition_id", "sender_contact_id", "receiver_contact_id", "template_id", "message", "payload"}

type NotificationRepository interface {
	GetByID(ctx context.Context, id string) (*models.Notification, error)
	GetByIDList(ctx context.Context, id ...string) ([]*models.Notification, error)
	Search(ctx context.Context, query *framedata.SearchQuery) (frame.JobResultPipe[[]*models.Notification], error)
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

func (repo *notificationRepository) Search(ctx context.Context, query *framedata.SearchQuery,
) (frame.JobResultPipe[[]*models.Notification], error) {
	return framedata.StableSearch[models.Notification](ctx, repo.service, query, func(
		ctx context.Context,
		query *framedata.SearchQuery,
	) ([]*models.Notification, error) {
		var notificationList []*models.Notification

		paginator := query.Pagination

		db := repo.service.DB(ctx, true).
			Limit(paginator.Limit).Offset(paginator.Offset)

		if query.Fields != nil {
			startAt, sok := query.Fields["start_date"]
			stopAt, stok := query.Fields["end_date"]
			if sok && startAt != nil && stok && stopAt != nil {
				startDate, ok1 := startAt.(*time.Time)
				endDate, ok2 := stopAt.(*time.Time)
				if ok1 && ok2 {
					db = db.Where(
						"created_at BETWEEN ? AND ? ",
						startDate.Format("2020-01-31T00:00:00Z"),
						endDate.Format("2020-01-31T00:00:00Z"),
					)
				}
			}

			for key, value := range query.Fields {
				if !slices.Contains(permittedSearchKeys, key) {
					continue
				}

				if key == "message" {
					messageVal, ok := value.(string)
					if ok {
						messageVal = strings.TrimSpace(messageVal)
						if messageVal != "" {
							db = db.Where(" message ILIKE ?", fmt.Sprintf("%%%s%%", messageVal))
						}
					}
					continue
				}

				db = db.Where(fmt.Sprintf("%s = ?", key), value)
			}
		}

		if query.Query != "" {
			searchQuery := strings.TrimSpace(query.Query)
			searchQuery = fmt.Sprintf("%%%s%%", searchQuery)
			db = db.Where(" id ILIKE ? OR external_id ILIKE ? OR transient_id ILIKE ?", searchQuery, searchQuery, searchQuery)
		}

		err := db.Find(&notificationList).Error
		if err != nil {
			return nil, err
		}

		return notificationList, nil
	})
}

func (repo *notificationRepository) Save(ctx context.Context, notification *models.Notification) error {
	return repo.writeDb(ctx).Save(notification).Error
}
