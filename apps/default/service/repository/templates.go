package repository

import (
	"context"

	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/pitabwire/frame/datastore"
	"github.com/pitabwire/frame/datastore/pool"
	"github.com/pitabwire/frame/workerpool"
)

type TemplateRepository interface {
	datastore.BaseRepository[*models.Template]
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

	err := tr.Pool().DB(ctx, true).Find(&template, "name = ?", name).Error
	if err != nil {
		return nil, err
	}
	return &template, nil
}
