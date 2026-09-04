package repository

import (
	"context"

	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/pitabwire/frame/v2/datastore"
	"github.com/pitabwire/frame/v2/datastore/pool"
	"github.com/pitabwire/frame/v2/workerpool"
)

type TemplateRepository interface {
	datastore.BaseRepository[*models.Template]
	// GetByName returns the template with the given name within the caller's tenancy.
	// Returns gorm.ErrRecordNotFound when no such template exists.
	GetByName(ctx context.Context, name string) (*models.Template, error)
}

type templateRepository struct {
	datastore.BaseRepository[*models.Template]
}

func NewTemplateRepository(ctx context.Context, dbPool pool.Pool, workMan workerpool.Manager) TemplateRepository {
	return &templateRepository{
		BaseRepository: datastore.NewBaseRepository[*models.Template](
			ctx, dbPool, workMan, func() *models.Template { return &models.Template{} },
		),
	}
}

func (tr *templateRepository) GetByName(ctx context.Context, name string) (*models.Template, error) {
	template := models.Template{}

	// Oldest row wins so that any legacy duplicates resolve to a stable template id.
	err := tr.Pool().DB(ctx, true).Order("created_at ASC").First(&template, "name = ?", name).Error
	if err != nil {
		return nil, err
	}
	return &template, nil
}
