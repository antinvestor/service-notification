package repository

import (
	"context"

	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/pitabwire/frame"
)

type TemplateDataRepository interface {
	GetByID(ctx context.Context, id string) (*models.TemplateData, error)
	GetByTemplateID(ctx context.Context, templateId string) ([]*models.TemplateData, error)
	GetByTemplateIDAndLanguage(ctx context.Context, templateId string, languageId string) ([]*models.TemplateData, error)
	Save(ctx context.Context, templateData *models.TemplateData) error
}

type templateDataRepository struct {
	abstractRepository
}

func NewTemplateDataRepository(ctx context.Context, service *frame.Service) TemplateDataRepository {
	return &templateDataRepository{abstractRepository{service: service}}
}

func (tr *templateDataRepository) GetByID(ctx context.Context, id string) (*models.TemplateData, error) {
	template := models.TemplateData{}
	err := tr.readDb(ctx).First(&template, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &template, nil
}

func (tr *templateDataRepository) GetByTemplateID(ctx context.Context, templateId string) ([]*models.TemplateData, error) {
	var templateDataList []*models.TemplateData
	err := tr.readDb(ctx).Where("template_id = ?", templateId).Find(&templateDataList).Error
	if err != nil {
		return nil, err
	}
	return templateDataList, nil
}

func (tr *templateDataRepository) GetByTemplateIDAndLanguage(ctx context.Context, templateId string, languageId string) ([]*models.TemplateData, error) {
	var templateDataList []*models.TemplateData
	err := tr.readDb(ctx).Where("template_id = ? AND language_id = ?", templateId, languageId).Find(&templateDataList).Error
	if err != nil {
		return nil, err
	}
	return templateDataList, nil
}

func (tr *templateDataRepository) Save(ctx context.Context, templateData *models.TemplateData) error {
	return tr.writeDb(ctx).Save(templateData).Error
}
