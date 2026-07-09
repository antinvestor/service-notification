package repository

import (
	"context"

	"github.com/antinvestor/service-notification/apps/ussd/service/models"
	"github.com/pitabwire/frame/v2/datastore"
	"github.com/pitabwire/frame/v2/datastore/pool"
	"github.com/pitabwire/frame/v2/workerpool"
)

// ServiceConfigRepository provides data access for per-service USSD configuration.
type ServiceConfigRepository interface {
	datastore.BaseRepository[*models.UssdServiceConfig]
	GetByServiceID(ctx context.Context, serviceID string) (map[string]string, error)
	Upsert(ctx context.Context, cfg *models.UssdServiceConfig) error
}

type serviceConfigRepository struct {
	datastore.BaseRepository[*models.UssdServiceConfig]
}

func NewServiceConfigRepository(ctx context.Context, dbPool pool.Pool, workMan workerpool.Manager) ServiceConfigRepository {
	return &serviceConfigRepository{
		BaseRepository: datastore.NewBaseRepository[*models.UssdServiceConfig](
			ctx, dbPool, workMan, func() *models.UssdServiceConfig { return &models.UssdServiceConfig{} },
		),
	}
}

func (r *serviceConfigRepository) GetByServiceID(ctx context.Context, serviceID string) (map[string]string, error) {
	var configs []*models.UssdServiceConfig
	err := r.Pool().DB(ctx, true).
		Where("service_id = ?", serviceID).
		Find(&configs).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(configs))
	for _, cfg := range configs {
		result[cfg.Name] = cfg.Value
	}
	return result, nil
}

func (r *serviceConfigRepository) Upsert(ctx context.Context, cfg *models.UssdServiceConfig) error {
	existing := models.UssdServiceConfig{}
	err := r.Pool().DB(ctx, false).Where("service_id = ? AND name = ?", cfg.ServiceID, cfg.Name).First(&existing).Error
	if err == nil {
		existing.Value = cfg.Value
		existing.Description = cfg.Description
		return r.Pool().DB(ctx, false).Save(&existing).Error
	}
	cfg.GenID(ctx)
	return r.Pool().DB(ctx, false).Create(cfg).Error
}
