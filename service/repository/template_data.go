package repository

import (
	"context"
	"github.com/antinvestor/service-notification/service/models"
	"github.com/pitabwire/frame"
	"gorm.io/gorm"
)

type TemplateDataRepository interface {
	GetByID(id string) (*models.TemplateData, error)
	GetByTemplateID(templateId string) ([]*models.TemplateData, error)
	GetByTemplateIDAndLanguage(templateId string, languageId string) ([]*models.TemplateData, error)
	Save(templateData *models.TemplateData) error
}

type templateDataRepository struct {
	readDb  *gorm.DB
	writeDb *gorm.DB
}

func NewTemplateDataRepository(ctx context.Context, service *frame.Service) TemplateDataRepository {
	return &templateDataRepository{readDb: service.DB(ctx, true), writeDb: service.DB(ctx, false)}
}

func (tr *templateDataRepository) GetByID(id string) (*models.TemplateData, error) {
	template := models.TemplateData{}
	err := tr.readDb.First(&template, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &template, nil
}

func (tr *templateDataRepository) GetByTemplateID(templateId string) ([]*models.TemplateData, error) {
	var templateDataList []*models.TemplateData
	err := tr.readDb.Where("template_id = ?", templateId).Find(&templateDataList).Error
	if err != nil {
		return nil, err
	}
	return templateDataList, nil
}

func (tr *templateDataRepository) GetByTemplateIDAndLanguage(templateId string, languageId string) ([]*models.TemplateData, error) {
	var templateDataList []*models.TemplateData
	err := tr.readDb.Where("template_id = ? AND language_id = ?", templateId, languageId).Find(&templateDataList).Error
	if err != nil {
		return nil, err
	}
	return templateDataList, nil
}

func (tr *templateDataRepository) Save(templateData *models.TemplateData) error {
	return tr.writeDb.Save(templateData).Error
}
