package repository

import (
	"context"

	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/pitabwire/frame/datastore"
	"github.com/pitabwire/frame/datastore/pool"
	"github.com/pitabwire/frame/workerpool"
)

type TemplateDataRepository interface {
	datastore.BaseRepository[*models.TemplateData]
	GetByTemplateID(ctx context.Context, templateId ...string) ([]*models.TemplateData, error)
	GetByTemplateIDAndLanguage(ctx context.Context, languageId string, templateId ...string) ([]*models.TemplateData, error)
}

type templateDataRepository struct {
	datastore.BaseRepository[*models.TemplateData]
}

func NewTemplateDataRepository(ctx context.Context, dbPool pool.Pool, workMan workerpool.Manager) TemplateDataRepository {
	return &templateDataRepository{
		BaseRepository: datastore.NewBaseRepository[*models.TemplateData](
			ctx, dbPool, workMan, func() *models.TemplateData { return &models.TemplateData{} },
		),
	}
}

func (tr *templateDataRepository) GetByTemplateID(ctx context.Context, templateId ...string) ([]*models.TemplateData, error) {
	var templateDataList []*models.TemplateData
	err := tr.Pool().DB(ctx, true).Where("template_id IN ?", templateId).Find(&templateDataList).Error
	if err != nil {
		return nil, err
	}
	return templateDataList, nil
}

func (tr *templateDataRepository) GetByTemplateIDAndLanguage(ctx context.Context, languageId string, templateId ...string) ([]*models.TemplateData, error) {
	var templateDataList []*models.TemplateData
	err := tr.Pool().DB(ctx, true).Where(" language_id = ? AND template_id IN ?", templateId, languageId).Find(&templateDataList).Error
	if err != nil {
		return nil, err
	}
	return templateDataList, nil
}
