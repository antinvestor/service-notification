package business

import (
	"context"
	"fmt"
	"net/http"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"github.com/antinvestor/service-notification/apps/ussd/service/engine"
	ussdEvents "github.com/antinvestor/service-notification/apps/ussd/service/events"
	"github.com/antinvestor/service-notification/apps/ussd/service/models"
	"github.com/antinvestor/service-notification/apps/ussd/service/repository"
	"github.com/pitabwire/frame/v3/data"
	fevents "github.com/pitabwire/frame/v3/events"
	"github.com/pitabwire/util"
	"google.golang.org/protobuf/types/known/structpb"
)

// UssdBusiness defines the interface for USSD service operations.
type UssdBusiness interface {
	// ProcessUSSD handles a USSD request from a telco gateway.
	ProcessUSSD(ctx context.Context, req engine.USSDRequest) engine.USSDResponse

	// Menu CRUD operations
	CreateMenu(ctx context.Context, menu *models.UssdMenu) error
	UpdateMenu(ctx context.Context, menu *models.UssdMenu) error
	GetMenu(ctx context.Context, menuID string) (*models.UssdMenu, error)
	GetMenuChildren(ctx context.Context, menuID string) ([]*models.UssdMenu, error)
	GetRootMenus(ctx context.Context, ownerID string) ([]*models.UssdMenu, error)
	DeleteMenu(ctx context.Context, menuID string) error
	DuplicateMenu(ctx context.Context, sourceMenuID, newOwnerID, newParentID string, maxDepth ...int) (*models.UssdMenu, error)

	// Translation operations
	SetTranslation(ctx context.Context, t *models.UssdTranslation) error
	GetTranslations(ctx context.Context, menuID, langCode string) (*models.UssdTranslation, error)

	// Session operations
	GetSession(ctx context.Context, sessionID string) (*models.UssdSession, error)
	CleanupExpiredSessions(ctx context.Context) (int64, error)

	// Query operations. Rows are written by the calling account (OwnerID)
	// for a caller's contact; ReplaceQueries swaps the whole set under a name
	// in one step, which is how a service pushes the current state.
	CreateQuery(ctx context.Context, q *models.UssdQuery) error
	ReplaceQueries(ctx context.Context, key repository.QueryKey, rows []*models.UssdQuery) error
	DeactivateQueries(ctx context.Context, key repository.QueryKey) error

	// Service config operations
	GetServiceConfig(ctx context.Context, serviceID string) (map[string]string, error)
	SetServiceConfig(ctx context.Context, cfg *models.UssdServiceConfig) error
	ServiceOwner(ctx context.Context, serviceID string) (string, error)
}

// NewUssdBusiness creates a new USSD business layer instance.
func NewUssdBusiness(
	_ context.Context,
	eventsMan fevents.Manager,
	menuRepo repository.MenuRepository,
	translationRepo repository.TranslationRepository,
	sessionRepo repository.SessionRepository,
	queryRepo repository.QueryRepository,
	serviceConfigRepo repository.ServiceConfigRepository,
	httpClient *http.Client,
	callers engine.CallerResolver,
	defaultLang string,
	sessionExpiryMinutes int,
) UssdBusiness {
	processor := engine.NewProcessor(
		menuRepo,
		translationRepo,
		sessionRepo,
		serviceConfigRepo,
		queryRepo,
		httpClient,
		callers,
		defaultLang,
		sessionExpiryMinutes,
	)

	ub := &ussdBusiness{
		eventsMan:         eventsMan,
		menuRepo:          menuRepo,
		translationRepo:   translationRepo,
		sessionRepo:       sessionRepo,
		queryRepo:         queryRepo,
		serviceConfigRepo: serviceConfigRepo,
		processor:         processor,
	}

	// Register completion handler for notification emission and preference persistence
	processor.SetCompletionHandler(ub.onSessionComplete)

	return ub
}

type ussdBusiness struct {
	eventsMan         fevents.Manager
	menuRepo          repository.MenuRepository
	translationRepo   repository.TranslationRepository
	sessionRepo       repository.SessionRepository
	queryRepo         repository.QueryRepository
	serviceConfigRepo repository.ServiceConfigRepository
	processor         *engine.Processor
}

// onSessionComplete is called when a USSD session finishes data collection.
// It emits a notification with the collected data and persists any preferences.
func (ub *ussdBusiness) onSessionComplete(ctx context.Context, session *models.UssdSession, result *engine.ProcessingResult) {
	logger := util.Log(ctx).WithFields(map[string]any{
		"session_id": session.GetID(),
		"msisdn":     session.MSISDN,
		"service_id": session.ServiceID,
	})

	// Persist preferences if this result is flagged as a preference
	if result.IsPreference && result.CollectionKey != "" {
		ub.persistPreference(ctx, session, result.CollectionKey, result.CollectedData)
	}

	// Emit notification with all collected session data
	ub.emitSessionNotification(ctx, session, logger)
}

// persistPreference saves a collected value as the caller's preference. It
// is stored under the service's own owner so the service's menus read it.
func (ub *ussdBusiness) persistPreference(ctx context.Context, session *models.UssdSession, key, value string) {
	logger := util.Log(ctx).WithField("preference_key", key)

	contactID := session.CollectedData.GetString(models.SessionKeyContactID)
	if contactID == "" {
		logger.Debug("caller has no profile contact; preference not kept")
		return
	}
	owner, err := ub.serviceConfigRepo.OwnerOf(ctx, session.ServiceID)
	if err != nil || owner == "" {
		logger.WithError(err).Debug("service has no owner; preference not kept")
		return
	}

	name := "preference:" + key
	pref := &models.UssdQuery{
		OwnerID:   owner,
		ContactID: contactID,
		ProfileID: session.CollectedData.GetString(models.SessionKeyProfileID),
		Name:      name,
		Payload:   data.JSONMap{key: value},
		IsActive:  true,
	}
	pref.GenID(ctx)
	if err = ub.queryRepo.Replace(ctx, repository.QueryKey{OwnerID: owner, Name: name, ContactID: contactID},
		[]*models.UssdQuery{pref}); err != nil {
		logger.WithError(err).Warn("failed to persist preference")
	}
}

// emitSessionNotification hands the collected session data to the
// notification service as an inbound "ussd" message from the caller. The
// caller is named by profile and contact id; their number is not sent.
func (ub *ussdBusiness) emitSessionNotification(ctx context.Context, session *models.UssdSession, logger *util.LogEntry) {
	if ub.eventsMan == nil {
		return
	}
	if session.CollectedData.GetString(models.SessionKeyProfileID) == "" ||
		session.CollectedData.GetString(models.SessionKeyContactID) == "" {
		// Nobody to act for: the number belongs to no profile.
		logger.Info("session completed by a caller without a profile; nothing to hand on")
		return
	}

	payload, err := structpb.NewStruct(session.CollectedData)
	if err != nil {
		logger.WithError(err).Warn("failed to convert collected data to struct")
		payload, _ = structpb.NewStruct(map[string]any{})
	}

	notification := &notificationv1.Notification{
		Source: &commonv1.ContactLink{
			ProfileId: session.CollectedData.GetString(models.SessionKeyProfileID),
			ContactId: session.CollectedData.GetString(models.SessionKeyContactID),
		},
		Type:     "ussd",
		Data:     session.ServiceID,
		Payload:  payload,
		Language: session.Language,
		OutBound: false,
	}

	if err = ub.eventsMan.Emit(ctx, ussdEvents.SessionCompleteEvent, notification); err != nil {
		logger.WithError(err).Error("failed to emit session complete notification")
	}
}

func (ub *ussdBusiness) ProcessUSSD(ctx context.Context, req engine.USSDRequest) engine.USSDResponse {
	return ub.processor.ProcessRequest(ctx, req)
}

func (ub *ussdBusiness) CreateMenu(ctx context.Context, menu *models.UssdMenu) error {
	menu.GenID(ctx)
	return ub.menuRepo.Create(ctx, menu)
}

func (ub *ussdBusiness) UpdateMenu(ctx context.Context, menu *models.UssdMenu) error {
	_, err := ub.menuRepo.Update(ctx, menu)
	return err
}

func (ub *ussdBusiness) GetMenu(ctx context.Context, menuID string) (*models.UssdMenu, error) {
	return ub.menuRepo.GetByID(ctx, menuID)
}

func (ub *ussdBusiness) GetMenuChildren(ctx context.Context, menuID string) ([]*models.UssdMenu, error) {
	return ub.menuRepo.GetByParentID(ctx, menuID)
}

func (ub *ussdBusiness) GetRootMenus(ctx context.Context, ownerID string) ([]*models.UssdMenu, error) {
	return ub.menuRepo.GetRootMenus(ctx, ownerID)
}

func (ub *ussdBusiness) DeleteMenu(ctx context.Context, menuID string) error {
	return ub.menuRepo.DeleteWithDescendants(ctx, menuID)
}

const maxDuplicateDepth = 20

func (ub *ussdBusiness) DuplicateMenu(ctx context.Context, sourceMenuID, newOwnerID, newParentID string, maxDepth ...int) (*models.UssdMenu, error) {
	depth := maxDuplicateDepth
	if len(maxDepth) > 0 && maxDepth[0] > 0 {
		depth = maxDepth[0]
	}
	return ub.duplicateMenuRecursive(ctx, sourceMenuID, newOwnerID, newParentID, depth)
}

func (ub *ussdBusiness) duplicateMenuRecursive(ctx context.Context, sourceMenuID, newOwnerID, newParentID string, remainingDepth int) (*models.UssdMenu, error) {
	if remainingDepth <= 0 {
		return nil, fmt.Errorf("menu tree exceeds maximum duplication depth of %d", maxDuplicateDepth)
	}

	logger := util.Log(ctx)

	source, err := ub.menuRepo.GetByID(ctx, sourceMenuID)
	if err != nil {
		return nil, err
	}

	newMenu := &models.UssdMenu{
		OwnerID:       newOwnerID,
		ParentID:      newParentID,
		Order:         source.Order,
		Action:        source.Action,
		Validator:     source.Validator,
		Name:          source.Name,
		Message:       source.Message,
		CollectionKey: source.CollectionKey,
		Extra:         source.Extra,
		Regex:         source.Regex,
		ErrorMessage:  source.ErrorMessage,
		IsPreference:  source.IsPreference,
		IsPublic:      false,
		IsActive:      true,
	}
	newMenu.GenID(ctx)

	if errCreate := ub.menuRepo.Create(ctx, newMenu); errCreate != nil {
		return nil, errCreate
	}

	if errCopy := ub.copyTranslations(ctx, source.GetID(), newMenu.GetID()); errCopy != nil {
		logger.WithError(errCopy).Warn("failed to copy translations")
	}

	children, err := ub.menuRepo.GetByParentID(ctx, sourceMenuID)
	if err != nil {
		return newMenu, nil
	}

	for _, child := range children {
		if _, err := ub.duplicateMenuRecursive(ctx, child.GetID(), newOwnerID, newMenu.GetID(), remainingDepth-1); err != nil {
			logger.WithError(err).WithField("child_id", child.GetID()).Warn("failed to duplicate child")
		}
	}

	return newMenu, nil
}

func (ub *ussdBusiness) copyTranslations(ctx context.Context, sourceMenuID, targetMenuID string) error {
	translations, err := ub.translationRepo.GetByMenuIDs(ctx, []string{sourceMenuID}, "")
	if err != nil {
		return nil
	}
	for _, t := range translations {
		newT := &models.UssdTranslation{
			MenuID:       targetMenuID,
			Code:         t.Code,
			Name:         t.Name,
			Message:      t.Message,
			ErrorMessage: t.ErrorMessage,
		}
		newT.GenID(ctx)
		if err := ub.translationRepo.Create(ctx, newT); err != nil {
			if !data.ErrorIsDuplicateKey(err) {
				return err
			}
		}
	}
	return nil
}

func (ub *ussdBusiness) SetTranslation(ctx context.Context, t *models.UssdTranslation) error {
	return ub.translationRepo.Upsert(ctx, t)
}

func (ub *ussdBusiness) GetTranslations(ctx context.Context, menuID, langCode string) (*models.UssdTranslation, error) {
	return ub.translationRepo.GetByMenuAndCode(ctx, menuID, langCode)
}

func (ub *ussdBusiness) GetSession(ctx context.Context, sessionID string) (*models.UssdSession, error) {
	return ub.sessionRepo.GetByID(ctx, sessionID)
}

func (ub *ussdBusiness) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	return ub.sessionRepo.CleanupExpired(ctx)
}

func (ub *ussdBusiness) CreateQuery(ctx context.Context, q *models.UssdQuery) error {
	q.GenID(ctx)
	return ub.queryRepo.Create(ctx, q)
}

func (ub *ussdBusiness) ReplaceQueries(ctx context.Context, key repository.QueryKey, rows []*models.UssdQuery) error {
	for _, row := range rows {
		row.OwnerID = key.OwnerID
		row.ContactID = key.ContactID
		row.Name = key.Name
		row.IsActive = true
		row.GenID(ctx)
	}
	return ub.queryRepo.Replace(ctx, key, rows)
}

func (ub *ussdBusiness) DeactivateQueries(ctx context.Context, key repository.QueryKey) error {
	return ub.queryRepo.DeactivateByName(ctx, key)
}

func (ub *ussdBusiness) GetServiceConfig(ctx context.Context, serviceID string) (map[string]string, error) {
	return ub.serviceConfigRepo.GetByServiceID(ctx, serviceID)
}

func (ub *ussdBusiness) SetServiceConfig(ctx context.Context, cfg *models.UssdServiceConfig) error {
	return ub.serviceConfigRepo.Upsert(ctx, cfg)
}

func (ub *ussdBusiness) ServiceOwner(ctx context.Context, serviceID string) (string, error) {
	return ub.serviceConfigRepo.OwnerOf(ctx, serviceID)
}
