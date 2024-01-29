package repository

import (
	"context"
	"fmt"
	"github.com/pitabwire/frame"
	"strings"

	"github.com/antinvestor/service-notification/service/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TemplateRepository interface {
	GetByID(id string) (*models.Template, error)
	GetByPartitionIDAndName(partitionId string, name string) (*models.Template, error)
	SearchByPartitionIDName(partitionId string, query string, page int, count int) ([]*models.Template, error)
	Save(template *models.Template) error
	SaveTemplateData(templateData *models.TemplateData) error
}

type templateRepository struct {
	readDb  *gorm.DB
	writeDb *gorm.DB
}

func NewTemplateRepository(ctx context.Context, service *frame.Service) TemplateRepository {
	return &templateRepository{readDb: service.DB(ctx, true), writeDb: service.DB(ctx, false)}
}

func (tr *templateRepository) GetByID(id string) (*models.Template, error) {
	template := models.Template{}
	err := tr.readDb.Preload(clause.Associations).First(&template, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &template, nil
}

func (tr *templateRepository) GetByPartitionIDAndName(partitionId string, name string) (*models.Template, error) {
	template := models.Template{}
	err := tr.readDb.Find(&template, "partition_id = ? AND name = ?", partitionId, name).Error
	if err != nil {
		return nil, err
	}
	return &template, nil
}

func (tr *templateRepository) SearchByPartitionIDName(partitionId string, query string, page int, count int) ([]*models.Template, error) {

	query = strings.TrimSpace(query)

	var templateList []*models.Template
	templateSearchQuery := tr.readDb.Where("partition_id = ? ", partitionId)

	if query != "" {

		searchQ := fmt.Sprintf("%%%s%%", query)

		templateSearchQuery = templateSearchQuery.
			Where(" name ILIKE ? ", searchQ)
	}

	err := templateSearchQuery.Find(&templateList).Offset(page * count).Limit(count).Error
	if err != nil {
		return nil, err
	}

	return templateList, nil
}

func (tr *templateRepository) Save(template *models.Template) error {
	return tr.writeDb.Save(template).Error
}

func (tr *templateRepository) SaveTemplateData(templateData *models.TemplateData) error {
	return tr.writeDb.Save(templateData).Error
}
