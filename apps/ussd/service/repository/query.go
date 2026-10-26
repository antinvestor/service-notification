package repository

import (
	"context"

	"github.com/antinvestor/service-notification/apps/ussd/service/models"
	"github.com/pitabwire/frame/v3/datastore"
	"github.com/pitabwire/frame/v3/datastore/pool"
	"github.com/pitabwire/frame/v3/workerpool"
	"gorm.io/gorm"
)

// QueryKey addresses the rows a menu reads for one caller. With a ContactID
// the rows are those OwnerID wrote for that contact; without one the legacy
// MSISDN and UserID addressing applies.
type QueryKey struct {
	OwnerID   string
	Name      string
	ContactID string
	MSISDN    string
	UserID    string
}

func (k QueryKey) scope(db *gorm.DB) *gorm.DB {
	db = db.Where("name = ? AND is_active = true", k.Name)
	if k.ContactID != "" {
		return db.Where("owner_id = ? AND contact_id = ?", k.OwnerID, k.ContactID)
	}
	return db.Where("msisdn = ? AND user_id = ?", k.MSISDN, k.UserID)
}

// QueryRepository provides data access for USSD dynamic query data.
type QueryRepository interface {
	datastore.BaseRepository[*models.UssdQuery]
	FindByName(ctx context.Context, key QueryKey) (*models.UssdQuery, error)
	FindAllByName(ctx context.Context, key QueryKey) ([]*models.UssdQuery, error)
	DeactivateByName(ctx context.Context, key QueryKey) error
	// Replace deactivates the rows under the key of rows[0] and stores rows
	// in their place, atomically, so a caller never sees a half-written set.
	Replace(ctx context.Context, key QueryKey, rows []*models.UssdQuery) error
}

type queryRepository struct {
	datastore.BaseRepository[*models.UssdQuery]
}

func NewQueryRepository(ctx context.Context, dbPool pool.Pool, workMan workerpool.Manager) QueryRepository {
	return &queryRepository{
		BaseRepository: datastore.NewBaseRepository[*models.UssdQuery](
			ctx, dbPool, workMan, func() *models.UssdQuery { return &models.UssdQuery{} },
		),
	}
}

func (r *queryRepository) FindByName(ctx context.Context, key QueryKey) (*models.UssdQuery, error) {
	var q models.UssdQuery
	err := key.scope(r.Pool().DB(ctx, true)).Order("created_at DESC, id DESC").First(&q).Error
	if err != nil {
		return nil, err
	}
	return &q, nil
}

func (r *queryRepository) FindAllByName(ctx context.Context, key QueryKey) ([]*models.UssdQuery, error) {
	var queries []*models.UssdQuery
	err := key.scope(r.Pool().DB(ctx, true)).Order("created_at DESC, id DESC").Find(&queries).Error
	return queries, err
}

// queriesTable is used for bulk updates: an empty model would have its
// primary key filled by the frame BaseModel hooks and match no row.
const queriesTable = "ussd_queries"

func (r *queryRepository) DeactivateByName(ctx context.Context, key QueryKey) error {
	return key.scope(r.Pool().DB(ctx, false).Table(queriesTable)).
		Update("is_active", false).Error
}

func (r *queryRepository) Replace(ctx context.Context, key QueryKey, rows []*models.UssdQuery) error {
	return r.Pool().DB(ctx, false).Transaction(func(tx *gorm.DB) error {
		if err := key.scope(tx.Table(queriesTable)).Update("is_active", false).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(rows).Error
	})
}
