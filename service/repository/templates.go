package repository

import (
	"context"
	"fmt"
	"github.com/pitabwire/frame"
	"strings"

	"github.com/antinvestor/service-notification/service/models"
)

type TemplateRepository interface {
	GetByID(ctx context.Context, id string) (*models.Template, error)
	GetByName(ctx context.Context, name string) (*models.Template, error)
	SearchByName(ctx context.Context, query string, page int, count int) ([]*models.Template, error)
	Save(ctx context.Context, template *models.Template) error
}

type templateRepository struct {
	abstractRepository
}

func NewTemplateRepository(_ context.Context, service *frame.Service) TemplateRepository {
	return &templateRepository{
		abstractRepository{service: service},
	}
}

func (tr *templateRepository) GetByID(ctx context.Context, id string) (*models.Template, error) {
	template := models.Template{}
	err := tr.readDb(ctx).First(&template, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &template, nil
}

func (tr *templateRepository) GetByName(ctx context.Context, name string) (*models.Template, error) {
	template := models.Template{}

	err := tr.readDb(ctx).Find(&template, "name = ?", name).Error
	if err != nil {
		return nil, err
	}
	return &template, nil
}

func (tr *templateRepository) SearchByName(ctx context.Context, query string, page int, count int) ([]*models.Template, error) {

	query = strings.TrimSpace(query)

	var templateList []*models.Template
	templateSearchQuery := tr.readDb(ctx)

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

func (tr *templateRepository) Save(ctx context.Context, template *models.Template) error {
	return tr.writeDb(ctx).Save(template).Error
}
