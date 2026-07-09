package repository

import (
	"context"

	"github.com/antinvestor/service-notification/apps/ussd/service/models"
	"github.com/pitabwire/frame/v2/datastore"
	"github.com/pitabwire/frame/v2/datastore/pool"
	"github.com/pitabwire/frame/v2/workerpool"
)

// TranslationRepository provides data access for USSD menu translations.
type TranslationRepository interface {
	datastore.BaseRepository[*models.UssdTranslation]
	GetByMenuAndCode(ctx context.Context, menuID, code string) (*models.UssdTranslation, error)
	GetByMenuIDs(ctx context.Context, menuIDs []string, code string) ([]*models.UssdTranslation, error)
	Upsert(ctx context.Context, t *models.UssdTranslation) error
}

type translationRepository struct {
	datastore.BaseRepository[*models.UssdTranslation]
}

func NewTranslationRepository(ctx context.Context, dbPool pool.Pool, workMan workerpool.Manager) TranslationRepository {
	return &translationRepository{
		BaseRepository: datastore.NewBaseRepository[*models.UssdTranslation](
			ctx, dbPool, workMan, func() *models.UssdTranslation { return &models.UssdTranslation{} },
		),
	}
}

func (r *translationRepository) GetByMenuAndCode(ctx context.Context, menuID, code string) (*models.UssdTranslation, error) {
	var t models.UssdTranslation
	err := r.Pool().DB(ctx, true).
		Where("menu_id = ? AND code = ?", menuID, code).
		First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *translationRepository) GetByMenuIDs(ctx context.Context, menuIDs []string, code string) ([]*models.UssdTranslation, error) {
	if len(menuIDs) == 0 {
		return nil, nil
	}
	var translations []*models.UssdTranslation
	db := r.Pool().DB(ctx, true).Where("menu_id IN ?", menuIDs)
	if code != "" {
		db = db.Where("code = ?", code)
	}
	err := db.Find(&translations).Error
	return translations, err
}

func (r *translationRepository) Upsert(ctx context.Context, t *models.UssdTranslation) error {
	existing := models.UssdTranslation{}
	err := r.Pool().DB(ctx, false).Where("menu_id = ? AND code = ?", t.MenuID, t.Code).First(&existing).Error
	if err == nil {
		existing.Name = t.Name
		existing.Message = t.Message
		existing.ErrorMessage = t.ErrorMessage
		return r.Pool().DB(ctx, false).Save(&existing).Error
	}
	t.GenID(ctx)
	return r.Pool().DB(ctx, false).Create(t).Error
}
