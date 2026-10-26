package repository

import (
	"context"
	"errors"

	"github.com/antinvestor/service-notification/apps/ussd/service/models"
	"github.com/pitabwire/frame/v3/datastore"
	"github.com/pitabwire/frame/v3/datastore/pool"
	"github.com/pitabwire/frame/v3/workerpool"
	"gorm.io/gorm"
)

// ServiceConfigRepository provides data access for per-service USSD configuration.
type ServiceConfigRepository interface {
	datastore.BaseRepository[*models.UssdServiceConfig]
	GetByServiceID(ctx context.Context, serviceID string) (map[string]string, error)
	// Upsert sets one setting. A service is owned by the account that
	// configured it first; anyone else gets ErrServiceOwnedByAnother.
	Upsert(ctx context.Context, cfg *models.UssdServiceConfig) error
	OwnerOf(ctx context.Context, serviceID string) (string, error)
}

// ErrServiceOwnedByAnother is returned when an account changes the settings
// of a service another account configured.
var ErrServiceOwnedByAnother = errors.New("ussd service is configured by another account")

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
	return r.Pool().DB(ctx, false).Transaction(func(tx *gorm.DB) error {
		// Serialise writers of one service so two first configurers cannot
		// both claim it.
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "ussd-service:"+cfg.ServiceID).Error; err != nil {
			return err
		}
		var rows []*models.UssdServiceConfig
		if err := tx.Where("service_id = ?", cfg.ServiceID).Find(&rows).Error; err != nil {
			return err
		}
		// A service is claimed by the first account to configure it. Settings
		// written before ownership was recorded have no owner and are not
		// claimable by whoever writes next.
		for _, row := range rows {
			if row.OwnerID != cfg.OwnerID {
				return ErrServiceOwnedByAnother
			}
		}

		for _, row := range rows {
			if row.Name == cfg.Name {
				row.Value = cfg.Value
				row.Description = cfg.Description
				return tx.Save(row).Error
			}
		}
		cfg.GenID(ctx)
		return tx.Create(cfg).Error
	})
}

// OwnerOf returns the account that configured serviceID, or "" when no
// owner is recorded.
func (r *serviceConfigRepository) OwnerOf(ctx context.Context, serviceID string) (string, error) {
	var owners []string
	err := r.Pool().DB(ctx, true).Model(&models.UssdServiceConfig{}).
		Where("service_id = ? AND owner_id <> ''", serviceID).
		Distinct().Pluck("owner_id", &owners).Error
	if err != nil || len(owners) == 0 {
		return "", err
	}
	return owners[0], nil
}
