package repository

import (
	"context"
	"fmt"

	"github.com/antinvestor/service-notification/service/models"
	"github.com/pitabwire/frame"
)

type LanguageRepository interface {
	GetByID(ctx context.Context, id string) (*models.Language, error)
	GetByName(ctx context.Context, name string) (*models.Language, error)
	GetByCode(ctx context.Context, code string) (*models.Language, error)
	GetOrCreateByCode(ctx context.Context, code string) (*models.Language, error)
	Save(ctx context.Context, language *models.Language) error
}

type languageRepository struct {
	abstractRepository
}

func NewLanguageRepository(_ context.Context, service *frame.Service) LanguageRepository {
	return &languageRepository{abstractRepository{service: service}}
}

func (repo *languageRepository) GetOrCreateByCode(ctx context.Context, languageCode string) (*models.Language, error) {
	lang, err := repo.GetByCode(ctx, languageCode)
	if err != nil {
		if !frame.DBErrorIsRecordNotFound(err) || languageCode == "" {
			return nil, err
		}

		lang = &models.Language{
			Name:        fmt.Sprintf("Edit - %s", languageCode),
			Code:        languageCode,
			Description: "Auto created partition language",
		}
		lang.GenID(ctx)

		err = repo.Save(ctx, lang)
		if err != nil {
			return nil, err

		}
	}

	return lang, nil
}

func (repo *languageRepository) GetByCode(ctx context.Context, code string) (*models.Language, error) {
	var language models.Language
	err := repo.readDb(ctx).First(&language, "code = ?", code).Error
	if err != nil {
		return nil, err
	}
	return &language, nil
}

func (repo *languageRepository) GetByName(ctx context.Context, name string) (*models.Language, error) {
	var language models.Language
	err := repo.readDb(ctx).Find(&language, "name = ?", name).Error
	if err != nil {
		return nil, err
	}
	return &language, nil
}

func (repo *languageRepository) GetByID(ctx context.Context, id string) (*models.Language, error) {
	language := models.Language{}
	err := repo.readDb(ctx).First(&language, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &language, nil
}

func (repo *languageRepository) Save(ctx context.Context, language *models.Language) error {
	return repo.writeDb(ctx).Save(language).Error
}
