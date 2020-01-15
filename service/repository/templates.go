package repository

import (
	"antinvestor.com/service/notification/service/repository/models"
	"antinvestor.com/service/notification/utils"
	"context"
	"github.com/jinzhu/gorm"
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

func NewTemplateRepository(ctx context.Context, env *utils.Env) TemplateRepository {
	return &templateRepository{readDb: env.GetRDb(ctx), writeDb: env.GeWtDb(ctx)}
}

func (repo *templateRepository) GetByNameAndProductID(id string, productId string) ([]models.Templete, error) {
	var templetes []models.Templete
	err := repo.readDb.Find(&templetes, "template_id = ? and product_id = ?", id, productId).Error
	if err != nil {
		return nil, err
	}
	return templetes, nil
}

func (repo *templateRepository) GetByNameProductIDAndLanguageID(id string, productId string, languageId string) (*models.Templete, error) {
	templete := models.Templete{}
	err := repo.readDb.First(&templete, "template_id = ? and product_id = ? and language_id =?", id, productId, languageId).Error
	if err != nil {
		return nil, err
	}
	return &templete, nil
}

func (repo *templateRepository) GetByID(id string) (*models.Templete, error) {
	template := models.Templete{}
	templateData := []models.TempleteData{}
	err := repo.readDb.First(&template, "template_id = ?", id).Related(&templateData).Error
	if err != nil {
		return nil, err
	}
	template.DataList = templateData
	return &template, nil
}


func (repo *templateRepository) Save(template *models.Templete) error {
	return repo.writeDb.Save(template).Error
}
