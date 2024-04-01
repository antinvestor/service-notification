package repository

import (
	"context"
	"fmt"
	"github.com/pitabwire/frame"
	"strings"

	"github.com/antinvestor/service-notification/service/models"
	"gorm.io/gorm"
)

type TemplateRepository interface {
	GetByID(id string) (*models.Template, error)
	GetByName(name string) (*models.Template, error)
	SearchByName(query string, page int, count int) ([]*models.Template, error)
	Save(template *models.Template) error
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
	err := tr.readDb.First(&template, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &template, nil
}

func (tr *templateRepository) GetByName(name string) (*models.Template, error) {
	template := models.Template{}

	err := tr.readDb.Find(&template, "name = ?", name).Error
	if err != nil {
		return nil, err
	}
	return &template, nil
}

func (tr *templateRepository) SearchByName(query string, page int, count int) ([]*models.Template, error) {

	query = strings.TrimSpace(query)

	var templateList []*models.Template
	templateSearchQuery := tr.readDb

	if count <= 0 {
		count = 10
	}

	if query != "" {

		searchQ := fmt.Sprintf("%%%s%%", query)

		templateSearchQuery = templateSearchQuery.
			Where(" name ILIKE ? ", searchQ)
	}

	err := templateSearchQuery.Offset(page * count).Limit(count).Find(&templateList).Error
	if err != nil {
		return nil, err
	}

	return templateList, nil
}

func (tr *templateRepository) Save(template *models.Template) error {
	return tr.writeDb.Save(template).Error
}
