package repository

import (
	"context"
	"github.com/antinvestor/service-notification/service/repository/models"
	"github.com/pitabwire/frame"
	"gorm.io/gorm"
)

type LanguageRepository interface {
	GetByID(id string) (*models.Language, error)
	GetByName(name string) (*models.Language, error)
	GetByCode(code string) (*models.Language, error)
	Save(language *models.Language) error
}

type languageRepository struct {
	readDb  *gorm.DB
	writeDb *gorm.DB
}

func NewLanguageRepository(ctx context.Context, service *frame.Service) LanguageRepository {
	return &languageRepository{readDb: service.DB(ctx,true), writeDb: service.DB(ctx,false)}
}

func (repo *languageRepository) GetByCode(code string) (*models.Language, error) {
	var language models.Language
	err := repo.readDb.First(&language, "code = ?", code).Error
	if err != nil {
		return nil, err
	}
	return &language, nil
}

func (repo *languageRepository) GetByName(name string) (*models.Language, error) {
	var language models.Language
	err := repo.readDb.First(&language, "code = ?", name).Error
	if err != nil {
		return nil, err
	}
	return &language, nil
}

func (repo *languageRepository) GetByID(id string) (*models.Language, error) {
	language := models.Language{}
	err := repo.readDb.First(&language, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &language, nil
}


func (repo *languageRepository) Save(language *models.Language) error {
	return repo.writeDb.Save(language).Error
}
