package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/antinvestor/service-notification/apps/ussd/service/models"
	"github.com/antinvestor/service-notification/apps/ussd/service/repository"
	"github.com/pitabwire/frame/v2/data"
	"github.com/pitabwire/util"
)

const (
	defaultSessionExpiryMinutes = 5
)

// SessionManager handles USSD session lifecycle: creation, retrieval, state updates, and destruction.
type SessionManager struct {
	sessionRepo   repository.SessionRepository
	defaultExpiry int
}

// NewSessionManager creates a session manager.
func NewSessionManager(sessionRepo repository.SessionRepository, defaultExpiryMinutes int) *SessionManager {
	if defaultExpiryMinutes <= 0 {
		defaultExpiryMinutes = defaultSessionExpiryMinutes
	}
	return &SessionManager{
		sessionRepo:   sessionRepo,
		defaultExpiry: defaultExpiryMinutes,
	}
}

// SessionState holds the working state of a session during processing.
type SessionState struct {
	Session       *models.UssdSession
	IsBeginning   bool
	ExpiryMinutes int
}

// GetOrCreateSession finds an active session or creates a new one.
// expiryMinutes is per-service; pass 0 to use the default.
func (sm *SessionManager) GetOrCreateSession(ctx context.Context, msisdn, serviceID, sessionExternal, initialMenuID, lang string, expiryMinutes int) (*SessionState, error) {
	logger := util.Log(ctx)

	if expiryMinutes <= 0 {
		expiryMinutes = sm.defaultExpiry
	}

	// Try to find an existing active session
	session, err := sm.sessionRepo.FindActiveSession(ctx, msisdn, serviceID, sessionExternal)
	if err == nil && session != nil && !session.IsExpired() {
		logger.WithField("session_id", session.GetID()).Debug("resuming active session")
		return &SessionState{Session: session, IsBeginning: false, ExpiryMinutes: expiryMinutes}, nil
	}

	// Create a new session
	session = &models.UssdSession{
		MSISDN:          normaliseMSISDN(msisdn),
		ServiceID:       serviceID,
		SessionExternal: sessionExternal,
		State:           models.SessionStateActive,
		CurrentMenuID:   initialMenuID,
		PreviousMenuID:  "",
		Language:        lang,
		CollectedData:   data.JSONMap{},
		ExpiresAt:       time.Now().Add(time.Duration(expiryMinutes) * time.Minute),
	}
	session.GenID(ctx)

	if err := sm.sessionRepo.Create(ctx, session); err != nil {
		logger.WithError(err).Error("failed to create session")
		return nil, err
	}

	logger.WithField("session_id", session.GetID()).Debug("created new session")
	return &SessionState{Session: session, IsBeginning: true, ExpiryMinutes: expiryMinutes}, nil
}

// UpdateSession persists session state changes after a turn.
func (sm *SessionManager) UpdateSession(ctx context.Context, session *models.UssdSession, result *ProcessingResult, expiryMinutes int) error {
	if result.CollectionKey != "" && result.CollectedData != "" {
		if session.CollectedData == nil {
			session.CollectedData = data.JSONMap{}
		}
		session.CollectedData[result.CollectionKey] = result.CollectedData
	}

	if result.NextMenuID != "" {
		session.PreviousMenuID = session.CurrentMenuID
		session.CurrentMenuID = result.NextMenuID
	}

	if expiryMinutes <= 0 {
		expiryMinutes = sm.defaultExpiry
	}

	// Extend session expiry on each successful turn
	session.ExpiresAt = time.Now().Add(time.Duration(expiryMinutes) * time.Minute)

	_, err := sm.sessionRepo.Update(ctx, session)
	return err
}

// DestroySession marks the session as inactive.
func (sm *SessionManager) DestroySession(ctx context.Context, sessionID string) error {
	return sm.sessionRepo.DeactivateSession(ctx, sessionID)
}

// ResetSession destroys the current session and starts fresh.
func (sm *SessionManager) ResetSession(ctx context.Context, session *models.UssdSession, initialMenuID, lang string, expiryMinutes int) (*SessionState, error) {
	if err := sm.DestroySession(ctx, session.GetID()); err != nil {
		return nil, err
	}
	return sm.GetOrCreateSession(ctx, session.MSISDN, session.ServiceID, session.SessionExternal, initialMenuID, lang, expiryMinutes)
}

// SubstituteSessionData replaces %(key)s and {key} placeholders in a message
// with values from the session's collected data.
func SubstituteSessionData(message string, collectedData map[string]any) string {
	if collectedData == nil || (!strings.Contains(message, "{") && !strings.Contains(message, "%(")) {
		return message
	}

	result := message
	for key, val := range collectedData {
		valStr := ""
		if val != nil {
			switch v := val.(type) {
			case string:
				valStr = v
			default:
				valStr = fmt.Sprintf("%v", v)
			}
		}

		// Support {key} style
		result = strings.ReplaceAll(result, "{"+key+"}", valStr)
		// Support %(key)s Python-style (backwards compatibility)
		result = strings.ReplaceAll(result, "%("+key+")s", valStr)
	}

	return result
}

// maskMSISDN masks the middle digits of a phone number for log privacy.
// "+254700123456" -> "+254***123456"
func maskMSISDN(msisdn string) string {
	if len(msisdn) <= 6 {
		return "***"
	}
	prefix := msisdn[:4]
	suffix := msisdn[len(msisdn)-6:]
	return prefix + "***" + suffix
}

func normaliseMSISDN(msisdn string) string {
	msisdn = strings.TrimSpace(msisdn)
	if msisdn != "" && !strings.HasPrefix(msisdn, "+") {
		msisdn = "+" + msisdn
	}
	return msisdn
}
