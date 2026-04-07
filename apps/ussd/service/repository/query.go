package repository

import (
	"context"

	"github.com/antinvestor/service-notification/apps/ussd/service/models"
	"github.com/pitabwire/frame/datastore"
	"github.com/pitabwire/frame/datastore/pool"
	"github.com/pitabwire/frame/workerpool"
)

// QueryRepository provides data access for USSD dynamic query data.
type QueryRepository interface {
	datastore.BaseRepository[*models.UssdQuery]
	FindByName(ctx context.Context, name, msisdn, userID string) (*models.UssdQuery, error)
	FindAllByName(ctx context.Context, name, msisdn, userID string) ([]*models.UssdQuery, error)
	DeactivateByName(ctx context.Context, name, msisdn, userID string) error
}

type queryRepository struct {
	datastore.BaseRepository[*models.UssdQuery]
}

func NewQueryRepository(ctx context.Context, dbPool pool.Pool, workMan workerpool.Manager) QueryRepository {
	return &queryRepository{
		BaseRepository: datastore.NewBaseRepository[*models.UssdQuery](
			ctx, dbPool, workMan, func() *models.UssdQuery { return &models.UssdQuery{} },
		),
	}
}

func (r *queryRepository) FindByName(ctx context.Context, name, msisdn, userID string) (*models.UssdQuery, error) {
	var q models.UssdQuery
	err := r.Pool().DB(ctx, true).
		Where("name = ? AND msisdn = ? AND user_id = ? AND is_active = true", name, msisdn, userID).
		Order("created_at DESC").
		First(&q).Error
	if err != nil {
		return nil, err
	}
	return &q, nil
}

func (r *queryRepository) FindAllByName(ctx context.Context, name, msisdn, userID string) ([]*models.UssdQuery, error) {
	var queries []*models.UssdQuery
	err := r.Pool().DB(ctx, true).
		Where("name = ? AND msisdn = ? AND user_id = ? AND is_active = true", name, msisdn, userID).
		Order("created_at DESC").
		Find(&queries).Error
	return queries, err
}

func (r *queryRepository) DeactivateByName(ctx context.Context, name, msisdn, userID string) error {
	return r.Pool().DB(ctx, false).
		Model(&models.UssdQuery{}).
		Where("name = ? AND msisdn = ? AND user_id = ? AND is_active = true", name, msisdn, userID).
		Update("is_active", false).Error
}
