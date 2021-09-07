package repository

import (
	"context"
	"github.com/antinvestor/service-notification/service/models"
	"github.com/pitabwire/frame"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TemplateRepository interface {
	GetByID(id string) (*models.Templete, error)
	GetByNameAndPartitionID(id string, partitionId string) ([]models.Templete, error)
	GetByNamePartitionIDAndLanguageID(id string, partitionId string, languageId string) (*models.Templete, error)
	Save(template *models.Templete) error
}

type templateRepository struct {
	readDb  *gorm.DB
	writeDb *gorm.DB
}

func NewTemplateRepository(ctx context.Context, service *frame.Service) TemplateRepository {
	return &templateRepository{readDb: service.DB(ctx,true), writeDb: service.DB(ctx,false)}
}

func (repo *templateRepository) GetByNameAndPartitionID(name string, partitionId string) ([]models.Templete, error) {
	var templetes []models.Templete
	err := repo.readDb.Find(&templetes, "name = ? AND partition_id = ?", name, partitionId).Error
	if err != nil {
		return nil, err
	}
	return templetes, nil
}

func (repo *templateRepository) GetByNamePartitionIDAndLanguageID(name string, partitionId string, languageId string) (*models.Templete, error) {
	templete := models.Templete{}
	err := repo.readDb.First(&templete, "name = ? AND partition_id = ? AND language_id =?", name, partitionId, languageId).Error
	if err != nil {
		return nil, err
	}
	return &templete, nil
}

func (repo *templateRepository) GetByID(id string) (*models.Templete, error) {
	template := models.Templete{}
	err := repo.readDb.Preload(clause.Associations).First(&template, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &template, nil
}


func (repo *templateRepository) Save(template *models.Templete) error {
	return repo.writeDb.Save(template).Error
}
