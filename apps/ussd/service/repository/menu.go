package repository

import (
	"context"

	"github.com/antinvestor/service-notification/apps/ussd/service/models"
	"github.com/pitabwire/frame/datastore"
	"github.com/pitabwire/frame/datastore/pool"
	"github.com/pitabwire/frame/workerpool"
	"gorm.io/gorm"
)

// MenuRepository provides data access for USSD menu items.
type MenuRepository interface {
	datastore.BaseRepository[*models.UssdMenu]
	GetByParentID(ctx context.Context, parentID string) ([]*models.UssdMenu, error)
	GetWithChildren(ctx context.Context, menuID string) (*models.UssdMenu, []*models.UssdMenu, error)
	GetRootMenus(ctx context.Context, ownerID string) ([]*models.UssdMenu, error)
	GetByOwnerID(ctx context.Context, ownerID string) ([]*models.UssdMenu, error)
	DeleteWithDescendants(ctx context.Context, menuID string) error
}

type menuRepository struct {
	datastore.BaseRepository[*models.UssdMenu]
}

func NewMenuRepository(ctx context.Context, dbPool pool.Pool, workMan workerpool.Manager) MenuRepository {
	return &menuRepository{
		BaseRepository: datastore.NewBaseRepository[*models.UssdMenu](
			ctx, dbPool, workMan, func() *models.UssdMenu { return &models.UssdMenu{} },
		),
	}
}

func (r *menuRepository) GetByParentID(ctx context.Context, parentID string) ([]*models.UssdMenu, error) {
	var items []*models.UssdMenu
	err := r.Pool().DB(ctx, true).
		Where("parent_id = ? AND is_active = true", parentID).
		Order("\"order\" DESC, id ASC").
		Find(&items).Error
	return items, err
}

func (r *menuRepository) GetWithChildren(ctx context.Context, menuID string) (*models.UssdMenu, []*models.UssdMenu, error) {
	var items []*models.UssdMenu
	err := r.Pool().DB(ctx, true).
		Where("(id = ? OR parent_id = ?) AND is_active = true", menuID, menuID).
		Order("\"order\" DESC, id ASC").
		Find(&items).Error
	if err != nil {
		return nil, nil, err
	}

	var parent *models.UssdMenu
	var children []*models.UssdMenu
	for _, item := range items {
		if item.GetID() == menuID {
			parent = item
		} else {
			children = append(children, item)
		}
	}

	return parent, children, nil
}

func (r *menuRepository) GetRootMenus(ctx context.Context, ownerID string) ([]*models.UssdMenu, error) {
	var items []*models.UssdMenu
	err := r.Pool().DB(ctx, true).
		Where("(owner_id = ? OR is_public = true) AND (parent_id = '' OR parent_id IS NULL) AND is_active = true", ownerID).
		Order("\"order\" DESC, id ASC").
		Find(&items).Error
	return items, err
}

func (r *menuRepository) GetByOwnerID(ctx context.Context, ownerID string) ([]*models.UssdMenu, error) {
	var items []*models.UssdMenu
	err := r.Pool().DB(ctx, true).
		Where("owner_id = ? AND is_active = true", ownerID).
		Order("parent_id ASC, \"order\" DESC, id ASC").
		Find(&items).Error
	return items, err
}

func (r *menuRepository) DeleteWithDescendants(ctx context.Context, menuID string) error {
	db := r.Pool().DB(ctx, false)

	// Collect all descendant IDs iteratively
	allIDs := []string{menuID}
	parentIDs := []string{menuID}

	for len(parentIDs) > 0 {
		var childIDs []string
		if err := db.Model(&models.UssdMenu{}).
			Where("parent_id IN ?", parentIDs).
			Pluck("id", &childIDs).Error; err != nil {
			return err
		}
		if len(childIDs) == 0 {
			break
		}
		allIDs = append(allIDs, childIDs...)
		parentIDs = childIDs
	}

	// Delete in a transaction to ensure atomicity
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM ussd_translations WHERE menu_id IN ?", allIDs).Error; err != nil {
			return err
		}
		return tx.Exec("DELETE FROM ussd_menus WHERE id IN ?", allIDs).Error
	})
}
