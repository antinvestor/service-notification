package engine

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/antinvestor/service-notification/apps/ussd/service/models"
	"github.com/antinvestor/service-notification/apps/ussd/service/repository"
	"github.com/pitabwire/util"
)

// CompletionHandler is called when a USSD session completes data collection.
// It receives the session with all collected data and the final result.
type CompletionHandler func(ctx context.Context, session *models.UssdSession, result *ProcessingResult)

// Processor is the top-level USSD request processor.
// It orchestrates authentication, session management, navigation, and response building.
type Processor struct {
	sessionManager    *SessionManager
	navigator         *Navigator
	serviceConfigRepo repository.ServiceConfigRepository
	queryRepo         repository.QueryRepository
	defaultLang       string
	configCache       sync.Map // serviceID -> *configCacheEntry
	completionHandler CompletionHandler
}

type configCacheEntry struct {
	config    map[string]string
	expiresAt time.Time
}

const configCacheTTL = 2 * time.Minute

// NewProcessor creates the main USSD processor with all dependencies.
func NewProcessor(
	menuRepo repository.MenuRepository,
	translationRepo repository.TranslationRepository,
	sessionRepo repository.SessionRepository,
	serviceConfigRepo repository.ServiceConfigRepository,
	queryRepo repository.QueryRepository,
	httpClient *http.Client,
	defaultLang string,
	sessionExpiryMinutes int,
) *Processor {
	queryResolver := NewQueryResolver(queryRepo)
	validator := NewInputValidator(httpClient)
	navigator := NewNavigator(menuRepo, translationRepo, queryResolver, validator)
	sessionManager := NewSessionManager(sessionRepo, sessionExpiryMinutes)

	return &Processor{
		sessionManager:    sessionManager,
		navigator:         navigator,
		serviceConfigRepo: serviceConfigRepo,
		queryRepo:         queryRepo,
		defaultLang:       defaultLang,
	}
}

// USSDRequest represents a parsed incoming USSD request.
type USSDRequest struct {
	ServiceID       string
	MSISDN          string
	SessionExternal string
	UserInput       string
	Lang            string
	AuthKey         string
	IP              string
	IsFinal         bool
}

// USSDResponse is the final response sent to the telco gateway.
type USSDResponse struct {
	Message   string
	IsEnd     bool
	SessionID string
}

// ProcessRequest handles a complete USSD request/response cycle.
func (p *Processor) ProcessRequest(ctx context.Context, req USSDRequest) USSDResponse {
	logger := util.Log(ctx).WithFields(map[string]any{
		"service_id": req.ServiceID,
		"msisdn":     maskMSISDN(req.MSISDN),
	})

	// Load service configuration (cached)
	svcConfig, err := p.getServiceConfig(ctx, req.ServiceID)
	if err != nil || len(svcConfig) == 0 {
		logger.WithError(err).Error("failed to load service configuration")
		return USSDResponse{Message: "Service not configured", IsEnd: true}
	}

	// Authenticate
	if err := p.authenticate(req, svcConfig); err != nil {
		logger.WithError(err).Warn("authentication failed")
		return USSDResponse{Message: "Authentication failed", IsEnd: true}
	}

	// Determine initial menu and language
	initialMenuID := svcConfig[models.ConfigInitialMenu]
	if initialMenuID == "" {
		logger.Error("no initial menu configured")
		return USSDResponse{Message: "Service misconfigured", IsEnd: true}
	}

	lang := p.resolveLang(req, svcConfig)
	sessionExpiryMinutes := p.resolveSessionExpiry(svcConfig)

	// Get or create session
	sessionState, err := p.sessionManager.GetOrCreateSession(ctx, req.MSISDN, req.ServiceID, req.SessionExternal, initialMenuID, lang, sessionExpiryMinutes)
	if err != nil {
		logger.WithError(err).Error("session management failed")
		return USSDResponse{Message: "Service error", IsEnd: true}
	}
	session := sessionState.Session

	// Apply language from session if available
	if session.Language != "" {
		lang = session.Language
	}

	// Handle final/cleanup request
	if req.IsFinal {
		_ = p.sessionManager.DestroySession(ctx, session.GetID())
		return USSDResponse{IsEnd: true, SessionID: session.GetID()}
	}

	// Clean user input
	userInput := strings.TrimSpace(req.UserInput)

	// Handle special navigation inputs
	switch userInput {
	case models.NavExit:
		_ = p.sessionManager.DestroySession(ctx, session.GetID())
		return p.buildResponse(svcConfig, "Thank you. Goodbye.", false, true, session.GetID())

	case models.NavRestart, models.NavRestartH:
		newState, err := p.sessionManager.ResetSession(ctx, session, initialMenuID, lang, sessionExpiryMinutes)
		if err != nil {
			return USSDResponse{Message: "Service error", IsEnd: true}
		}
		session = newState.Session
		sessionState = newState
		userInput = "" // Display root menu

	case models.NavBack:
		if session.PreviousMenuID != "" {
			session.CurrentMenuID = session.PreviousMenuID
			session.PreviousMenuID = ""
		}
		userInput = "" // Re-display current menu
	}

	// Process the navigation turn
	navCtx := NavigationContext{
		ServiceID: req.ServiceID,
		UserID:    session.CollectedData.GetString("user_id"),
		BotID:     session.CollectedData.GetString("bot_id"),
		MSISDN:    session.MSISDN,
		Lang:      lang,
		MenuID:    session.CurrentMenuID,
		UserInput: userInput,
	}

	result, err := p.navigator.ProcessTurn(ctx, navCtx)
	if err != nil {
		logger.WithError(err).Error("navigation error")
		_ = p.sessionManager.DestroySession(ctx, session.GetID())
		return USSDResponse{Message: "Service error", IsEnd: true}
	}

	// Handle the processing result
	switch result.Outcome {
	case OutcomeContinue:
		if err := p.sessionManager.UpdateSession(ctx, session, result, sessionExpiryMinutes); err != nil {
			logger.WithError(err).Error("failed to update session")
			return USSDResponse{Message: "Service error", IsEnd: true}
		}
		message := SubstituteSessionData(result.Message, session.CollectedData)
		return p.buildResponse(svcConfig, message, sessionState.IsBeginning, false, session.GetID())

	case OutcomeDone:
		// Save final collected data
		p.saveCollectedData(session, result)
		if p.completionHandler != nil {
			p.completionHandler(ctx, session, result)
		}
		_ = p.sessionManager.DestroySession(ctx, session.GetID())
		message := SubstituteSessionData(result.Message, session.CollectedData)
		return p.buildResponse(svcConfig, message, false, true, session.GetID())

	case OutcomeDoneContinue:
		// Data collection done, but session resets to start for another round
		p.saveCollectedData(session, result)
		if p.completionHandler != nil {
			p.completionHandler(ctx, session, result)
		}
		// Reset session to initial menu for next flow
		newState, resetErr := p.sessionManager.ResetSession(ctx, session, initialMenuID, lang, sessionExpiryMinutes)
		if resetErr != nil {
			logger.WithError(resetErr).Error("failed to reset session after done-continue")
			return USSDResponse{Message: "Service error", IsEnd: true}
		}
		// Show the initial menu again
		message := SubstituteSessionData(result.Message, session.CollectedData)
		return p.buildResponse(svcConfig, message, false, false, newState.Session.GetID())

	case OutcomeTerminate, OutcomeCanceled:
		_ = p.sessionManager.DestroySession(ctx, session.GetID())
		message := result.Message
		if message == "" {
			message = "Thank you. Goodbye."
		}
		message = SubstituteSessionData(message, session.CollectedData)
		return p.buildResponse(svcConfig, message, false, true, session.GetID())

	default:
		_ = p.sessionManager.DestroySession(ctx, session.GetID())
		return USSDResponse{Message: "Service error", IsEnd: true}
	}
}

// authenticate verifies auth key and IP whitelist.
func (p *Processor) authenticate(req USSDRequest, config map[string]string) error {
	authKey := config[models.ConfigAuthKey]
	if authKey != "" && subtle.ConstantTimeCompare([]byte(req.AuthKey), []byte(authKey)) != 1 {
		return &AuthError{Message: "invalid auth key"}
	}

	whitelist := config[models.ConfigWhitelistIP]
	if whitelist != "" && req.IP != "" {
		allowed := false
		for _, ip := range strings.Split(whitelist, ",") {
			if strings.TrimSpace(ip) == req.IP {
				allowed = true
				break
			}
		}
		if !allowed {
			return &AuthError{Message: "IP not whitelisted"}
		}
	}

	return nil
}

func (p *Processor) resolveLang(req USSDRequest, config map[string]string) string {
	if req.Lang != "" {
		return req.Lang
	}
	if defaultLang := config[models.ConfigDefaultLang]; defaultLang != "" {
		return defaultLang
	}
	return p.defaultLang
}

func (p *Processor) resolveSessionExpiry(config map[string]string) int {
	if v, ok := config[models.ConfigSessionExpiryMinutes]; ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return defaultSessionExpiryMinutes
}

func (p *Processor) buildResponse(config map[string]string, message string, isBeginning, isEnd bool, sessionID string) USSDResponse {
	rb := NewResponseBuilder(
		config[models.ConfigDataBeginTemplate],
		config[models.ConfigDataContinueTemplate],
		config[models.ConfigDataEndTemplate],
	)
	resp := rb.Build(message, isBeginning, isEnd)
	resp.SessionID = sessionID
	return USSDResponse{
		Message:   resp.Message,
		IsEnd:     resp.IsEnd,
		SessionID: sessionID,
	}
}

// SetCompletionHandler registers a callback invoked when a session completes data collection.
func (p *Processor) SetCompletionHandler(handler CompletionHandler) {
	p.completionHandler = handler
}

func (p *Processor) saveCollectedData(session *models.UssdSession, result *ProcessingResult) {
	if result.CollectionKey != "" {
		if session.CollectedData == nil {
			session.CollectedData = map[string]any{}
		}
		session.CollectedData[result.CollectionKey] = result.CollectedData
	}
}

func (p *Processor) getServiceConfig(ctx context.Context, serviceID string) (map[string]string, error) {
	if cached, ok := p.configCache.Load(serviceID); ok {
		entry := cached.(*configCacheEntry)
		if time.Now().Before(entry.expiresAt) {
			return entry.config, nil
		}
		p.configCache.Delete(serviceID)
	}

	config, err := p.serviceConfigRepo.GetByServiceID(ctx, serviceID)
	if err != nil {
		return nil, err
	}

	p.configCache.Store(serviceID, &configCacheEntry{
		config:    config,
		expiresAt: time.Now().Add(configCacheTTL),
	})

	return config, nil
}

// AuthError indicates an authentication failure.
type AuthError struct {
	Message string
}

func (e *AuthError) Error() string {
	return "authentication error: " + e.Message
}
