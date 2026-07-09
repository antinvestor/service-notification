package repository

import (
	"context"
	"fmt"

	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/pitabwire/frame/v2/data"
	"github.com/pitabwire/frame/v2/datastore"
	"github.com/pitabwire/frame/v2/datastore/pool"
	"github.com/pitabwire/frame/v2/workerpool"
)

type LanguageRepository interface {
	datastore.BaseRepository[*models.Language]
	GetByIDList(ctx context.Context, id ...string) ([]*models.Language, error)
	GetByName(ctx context.Context, name string) (*models.Language, error)
	GetByCode(ctx context.Context, code string) (*models.Language, error)
	GetOrCreateByCode(ctx context.Context, code string) (*models.Language, error)
}

type languageRepository struct {
	datastore.BaseRepository[*models.Language]
}

func NewLanguageRepository(ctx context.Context, dbPool pool.Pool, workMan workerpool.Manager) LanguageRepository {
	return &languageRepository{
		BaseRepository: datastore.NewBaseRepository[*models.Language](
			ctx, dbPool, workMan, func() *models.Language { return &models.Language{} },
		),
	}
}

func (repo *languageRepository) GetByIDList(ctx context.Context, id ...string) ([]*models.Language, error) {
	var languages []*models.Language
	err := repo.Pool().DB(ctx, true).Find(&languages, "id IN ?", id).Error
	if err != nil {
		return nil, err
	}
	return languages, nil
}

func (repo *languageRepository) GetOrCreateByCode(ctx context.Context, languageCode string) (*models.Language, error) {

	if languageCode == "" {
		// Default fallback since we no longer have direct service access
		languageCode = "en"
	}

	lang, err := repo.GetByCode(ctx, languageCode)
	if err != nil {
		// Only "no rows" should trigger auto-creation — any other error is
		// a real failure. The previous logic was inverted (it returned on
		// no-rows and created on real errors), so the first notification in
		// a partition that had no language row yet failed with "record not
		// found" instead of provisioning the language.
		if !data.ErrorIsNoRows(err) {
			return nil, err
		}

		lang = &models.Language{
			Name:        fmt.Sprintf("Edit - %s", languageCode),
			Code:        languageCode,
			Description: "Auto created partition language",
		}
		lang.GenID(ctx)

		if createErr := repo.Create(ctx, lang); createErr != nil {
			return nil, createErr
		}
	}

	return lang, nil
}

func (repo *languageRepository) GetByCode(ctx context.Context, code string) (*models.Language, error) {
	var language models.Language
	err := repo.Pool().DB(ctx, true).First(&language, "code = ?", code).Error
	if err != nil {
		return nil, err
	}
	return &language, nil
}

func (repo *languageRepository) GetByName(ctx context.Context, name string) (*models.Language, error) {
	var language models.Language
	err := repo.Pool().DB(ctx, true).Find(&language, "name = ?", name).Error
	if err != nil {
		return nil, err
	}
	return &language, nil
}
