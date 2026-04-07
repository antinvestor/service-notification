package repository

import (
	"context"
	"time"

	"github.com/antinvestor/service-notification/apps/ussd/service/models"
	"github.com/pitabwire/frame/datastore"
	"github.com/pitabwire/frame/datastore/pool"
	"github.com/pitabwire/frame/workerpool"
)

// SessionRepository provides data access for USSD sessions.
type SessionRepository interface {
	datastore.BaseRepository[*models.UssdSession]
	FindActiveSession(ctx context.Context, msisdn, serviceID, sessionExternal string) (*models.UssdSession, error)
	DeactivateSession(ctx context.Context, sessionID string) error
	CleanupExpired(ctx context.Context) (int64, error)
}

type sessionRepository struct {
	datastore.BaseRepository[*models.UssdSession]
}

func NewSessionRepository(ctx context.Context, dbPool pool.Pool, workMan workerpool.Manager) SessionRepository {
	return &sessionRepository{
		BaseRepository: datastore.NewBaseRepository[*models.UssdSession](
			ctx, dbPool, workMan, func() *models.UssdSession { return &models.UssdSession{} },
		),
	}
}

func (r *sessionRepository) FindActiveSession(ctx context.Context, msisdn, serviceID, sessionExternal string) (*models.UssdSession, error) {
	var session models.UssdSession
	db := r.Pool().DB(ctx, true).
		Where("msisdn = ? AND service_id = ? AND state = ? AND expires_at > ?",
			msisdn, serviceID, models.SessionStateActive, time.Now())

	if sessionExternal != "" {
		db = db.Where("session_external = ?", sessionExternal)
	}

	err := db.Order("created_at DESC").First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *sessionRepository) DeactivateSession(ctx context.Context, sessionID string) error {
	return r.Pool().DB(ctx, false).
		Model(&models.UssdSession{}).
		Where("id = ?", sessionID).
		Update("state", models.SessionStateInactive).Error
}

func (r *sessionRepository) CleanupExpired(ctx context.Context) (int64, error) {
	result := r.Pool().DB(ctx, false).
		Model(&models.UssdSession{}).
		Where("state = ? AND expires_at < ?", models.SessionStateActive, time.Now()).
		Update("state", models.SessionStateInactive)
	return result.RowsAffected, result.Error
}
