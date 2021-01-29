package repository

import (
	"context"
	"github.com/antinvestor/service-notification/service/repository/models"
	"github.com/go-errors/errors"
	"github.com/pitabwire/frame"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TemplateRepository interface {
	GetByID(id string) (*models.Templete, error)
	GetByNameAndProductID(id string, productId string) ([]models.Templete, error)
	GetByNameProductIDAndLanguageID(id string, productId string, languageId string) (*models.Templete, error)
	Save(template *models.Templete) error
}

type templateRepository struct {
	readDb  *gorm.DB
	writeDb *gorm.DB
}

func NewTemplateRepository(ctx context.Context, service *frame.Service) TemplateRepository {
	return &templateRepository{readDb: service.DB(ctx,true), writeDb: service.DB(ctx,false)}
}

func (repo *templateRepository) GetByNameAndProductID(name string, productId string) ([]models.Templete, error) {
	var templetes []models.Templete
	err := repo.readDb.Find(&templetes, "name = ? and product_id = ?", name, productId).Error
	if err != nil {
		return nil, errors.Wrap(err, 1)
	}
	return templetes, nil
}

func (repo *templateRepository) GetByNameProductIDAndLanguageID(name string, productId string, languageId string) (*models.Templete, error) {
	templete := models.Templete{}
	err := repo.readDb.First(&templete, "name = ? and product_id = ? and language_id =?", name, productId, languageId).Error
	if err != nil {
		return nil, errors.Wrap(err, 1)
	}
	return &templete, nil
}

func (repo *templateRepository) GetByID(id string) (*models.Templete, error) {
	template := models.Templete{}
	err := repo.readDb.Preload(clause.Associations).First(&template, "id = ?", id).Error
	if err != nil {
		return nil, errors.Wrap(err, 1)
	}
	return &template, nil
}


func (repo *templateRepository) Save(template *models.Templete) error {
	return repo.writeDb.Save(template).Error
}
