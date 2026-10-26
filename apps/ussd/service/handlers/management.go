package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/antinvestor/service-notification/apps/ussd/service/business"
	"github.com/antinvestor/service-notification/apps/ussd/service/models"
	"github.com/antinvestor/service-notification/apps/ussd/service/repository"
	"github.com/pitabwire/util"
)

// ManagementServer provides REST API endpoints for managing USSD menus, translations,
// service configs, and queries.
type ManagementServer struct {
	ussdBusiness business.UssdBusiness
}

// NewManagementServer creates a new management API handler.
func NewManagementServer(ussdBusiness business.UssdBusiness) *ManagementServer {
	return &ManagementServer{ussdBusiness: ussdBusiness}
}

// RegisterRoutes adds management API routes to the given mux. They are
// served behind AuthMiddleware, which admits internal service accounts only;
// each resource is further limited to the account that owns it.
func (ms *ManagementServer) RegisterRoutes(mux *http.ServeMux) {
	// Menu management
	mux.HandleFunc("GET /api/v1/menus/{menuID}", ms.GetMenu)
	mux.HandleFunc("GET /api/v1/menus/{menuID}/children", ms.GetMenuChildren)
	mux.HandleFunc("GET /api/v1/menus", ms.GetRootMenus)
	mux.HandleFunc("POST /api/v1/menus", ms.CreateMenu)
	mux.HandleFunc("PUT /api/v1/menus/{menuID}", ms.UpdateMenu)
	mux.HandleFunc("DELETE /api/v1/menus/{menuID}", ms.DeleteMenu)
	mux.HandleFunc("POST /api/v1/menus/{menuID}/duplicate", ms.DuplicateMenu)

	// Translation management
	mux.HandleFunc("GET /api/v1/menus/{menuID}/translations/{langCode}", ms.GetTranslation)
	mux.HandleFunc("PUT /api/v1/menus/{menuID}/translations/{langCode}", ms.SetTranslation)

	// Service config management
	mux.HandleFunc("GET /api/v1/services/{serviceID}/config", ms.GetServiceConfig)
	mux.HandleFunc("PUT /api/v1/services/{serviceID}/config", ms.SetServiceConfig)

	// Query management
	mux.HandleFunc("PUT /api/v1/queries", ms.ReplaceQueries)
	mux.HandleFunc("DELETE /api/v1/queries", ms.DeactivateQueries)

	// Session management
	mux.HandleFunc("GET /api/v1/sessions/{sessionID}", ms.GetSession)
	mux.HandleFunc("POST /api/v1/sessions/cleanup", ms.CleanupSessions)
}

// --- Ownership ---

// ownedMenu loads a menu the caller owns. Menus belong to the account that
// created them; another account's menu is reported as not found.
func (ms *ManagementServer) ownedMenu(w http.ResponseWriter, r *http.Request, menuID string) (*models.UssdMenu, bool) {
	menu, err := ms.ussdBusiness.GetMenu(r.Context(), menuID)
	if err != nil || !ownsMenu(r, menu) {
		writeError(w, http.StatusNotFound, "menu not found")
		return nil, false
	}
	return menu, true
}

func ownsMenu(r *http.Request, menu *models.UssdMenu) bool {
	caller := OwnerIDFromRequest(r)
	return menu != nil && (caller == "" && devMode(r) || menu.OwnerID == caller)
}

// ownsService reports whether the caller configured serviceID. A service
// nobody configured yet is open to its first configurer.
func (ms *ManagementServer) ownsService(r *http.Request, serviceID string) (bool, error) {
	owner, err := ms.ussdBusiness.ServiceOwner(r.Context(), serviceID)
	if err != nil {
		return false, err
	}
	caller := OwnerIDFromRequest(r)
	return owner == "" || owner == caller || caller == "" && devMode(r), nil
}

// --- Menu endpoints ---

func (ms *ManagementServer) GetMenu(w http.ResponseWriter, r *http.Request) {
	menu, ok := ms.ownedMenu(w, r, r.PathValue("menuID"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, menu)
}

func (ms *ManagementServer) GetMenuChildren(w http.ResponseWriter, r *http.Request) {
	menuID := r.PathValue("menuID")
	if _, ok := ms.ownedMenu(w, r, menuID); !ok {
		return
	}
	children, err := ms.ussdBusiness.GetMenuChildren(r.Context(), menuID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get children")
		return
	}
	writeJSON(w, http.StatusOK, children)
}

func (ms *ManagementServer) GetRootMenus(w http.ResponseWriter, r *http.Request) {
	ownerID := OwnerIDFromRequest(r)
	if ownerID == "" {
		ownerID = r.URL.Query().Get("owner_id")
	}
	if ownerID == "" {
		writeError(w, http.StatusBadRequest, "owner_id required")
		return
	}
	menus, err := ms.ussdBusiness.GetRootMenus(r.Context(), ownerID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get menus")
		return
	}
	writeJSON(w, http.StatusOK, menus)
}

type createMenuRequest struct {
	ParentID      string `json:"parent_id"`
	Order         int    `json:"order"`
	Action        string `json:"action"`
	Validator     string `json:"validator"`
	Name          string `json:"name"`
	Message       string `json:"message"`
	CollectionKey string `json:"collection_key"`
	Extra         string `json:"extra"`
	Regex         string `json:"regex"`
	ErrorMessage  string `json:"error_message"`
	IsPreference  bool   `json:"is_preference"`
	IsPublic      bool   `json:"is_public"`
}

// CreateMenu adds a menu owned by the caller. A child goes only under a
// parent the caller owns.
func (ms *ManagementServer) CreateMenu(w http.ResponseWriter, r *http.Request) {
	var req createMenuRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.Action == "" {
		writeError(w, http.StatusBadRequest, "name and action are required")
		return
	}
	if req.ParentID != "" {
		if _, ok := ms.ownedMenu(w, r, req.ParentID); !ok {
			return
		}
	}

	menu := models.UssdMenuFromParams(r.Context(),
		OwnerIDFromRequest(r), req.ParentID, req.Action, req.Validator,
		req.Name, req.Message, req.CollectionKey, req.Extra,
		req.Regex, req.ErrorMessage, req.IsPreference, req.IsPublic,
	)
	menu.Order = req.Order

	if err := ms.ussdBusiness.CreateMenu(r.Context(), menu); err != nil {
		util.Log(r.Context()).WithError(err).Error("failed to create menu")
		writeError(w, http.StatusInternalServerError, "failed to create menu")
		return
	}
	writeJSON(w, http.StatusCreated, menu)
}

func (ms *ManagementServer) UpdateMenu(w http.ResponseWriter, r *http.Request) {
	existing, ok := ms.ownedMenu(w, r, r.PathValue("menuID"))
	if !ok {
		return
	}

	var req createMenuRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Action != "" {
		existing.Action = req.Action
	}
	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.Message != "" {
		existing.Message = req.Message
	}
	existing.Validator = req.Validator
	existing.CollectionKey = req.CollectionKey
	existing.Extra = req.Extra
	existing.Regex = req.Regex
	existing.ErrorMessage = req.ErrorMessage
	existing.IsPreference = req.IsPreference
	existing.IsPublic = req.IsPublic
	existing.Order = req.Order

	if err := ms.ussdBusiness.UpdateMenu(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update menu")
		return
	}
	writeJSON(w, http.StatusOK, existing)
}

func (ms *ManagementServer) DeleteMenu(w http.ResponseWriter, r *http.Request) {
	menuID := r.PathValue("menuID")
	if _, ok := ms.ownedMenu(w, r, menuID); !ok {
		return
	}
	if err := ms.ussdBusiness.DeleteMenu(r.Context(), menuID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete menu")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type duplicateMenuRequest struct {
	ParentID string `json:"parent_id"`
}

// DuplicateMenu copies a menu tree. The source must be the caller's or
// public; the copy is always the caller's.
func (ms *ManagementServer) DuplicateMenu(w http.ResponseWriter, r *http.Request) {
	menuID := r.PathValue("menuID")
	source, err := ms.ussdBusiness.GetMenu(r.Context(), menuID)
	if err != nil || (!source.IsPublic && !ownsMenu(r, source)) {
		writeError(w, http.StatusNotFound, "menu not found")
		return
	}

	var req duplicateMenuRequest
	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ParentID != "" {
		if _, ok := ms.ownedMenu(w, r, req.ParentID); !ok {
			return
		}
	}

	newMenu, err := ms.ussdBusiness.DuplicateMenu(r.Context(), menuID, OwnerIDFromRequest(r), req.ParentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to duplicate menu")
		return
	}
	writeJSON(w, http.StatusCreated, newMenu)
}

// --- Translation endpoints ---

func (ms *ManagementServer) GetTranslation(w http.ResponseWriter, r *http.Request) {
	menuID := r.PathValue("menuID")
	if _, ok := ms.ownedMenu(w, r, menuID); !ok {
		return
	}
	t, err := ms.ussdBusiness.GetTranslations(r.Context(), menuID, r.PathValue("langCode"))
	if err != nil {
		writeError(w, http.StatusNotFound, "translation not found")
		return
	}
	writeJSON(w, http.StatusOK, t)
}

type setTranslationRequest struct {
	Name         string `json:"name"`
	Message      string `json:"message"`
	ErrorMessage string `json:"error_message"`
}

func (ms *ManagementServer) SetTranslation(w http.ResponseWriter, r *http.Request) {
	menuID := r.PathValue("menuID")
	if _, ok := ms.ownedMenu(w, r, menuID); !ok {
		return
	}

	var req setTranslationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	t := &models.UssdTranslation{
		MenuID:       menuID,
		Code:         r.PathValue("langCode"),
		Name:         req.Name,
		Message:      req.Message,
		ErrorMessage: req.ErrorMessage,
	}

	if err := ms.ussdBusiness.SetTranslation(r.Context(), t); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save translation")
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// --- Service config endpoints ---

// GetServiceConfig returns a service's settings, its auth key included, so
// only the account that configured the service may read them.
func (ms *ManagementServer) GetServiceConfig(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
	owns, err := ms.ownsService(r, serviceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get config")
		return
	}
	if !owns {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}
	config, err := ms.ussdBusiness.GetServiceConfig(r.Context(), serviceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get config")
		return
	}
	writeJSON(w, http.StatusOK, config)
}

type setServiceConfigRequest struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

func (ms *ManagementServer) SetServiceConfig(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")

	var req setServiceConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	cfg := &models.UssdServiceConfig{
		ServiceID:   serviceID,
		OwnerID:     OwnerIDFromRequest(r),
		Name:        req.Name,
		Value:       req.Value,
		Description: req.Description,
	}

	err := ms.ussdBusiness.SetServiceConfig(r.Context(), cfg)
	switch {
	case err == nil:
	case errors.Is(err, repository.ErrServiceOwnedByAnother):
		writeError(w, http.StatusForbidden, "service is configured by another account")
		return
	default:
		writeError(w, http.StatusInternalServerError, "failed to save config")
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// --- Query endpoints ---

type queryRow struct {
	Payload map[string]any `json:"payload"`
	IsMany  bool           `json:"is_many"`
}

type replaceQueriesRequest struct {
	Name      string     `json:"name"`
	ContactID string     `json:"contact_id"`
	ProfileID string     `json:"profile_id"`
	Rows      []queryRow `json:"rows"`
}

// ReplaceQueries sets what the caller's menus show a contact under name:
// the rows given replace whatever the caller stored there before. An empty
// list clears it.
func (ms *ManagementServer) ReplaceQueries(w http.ResponseWriter, r *http.Request) {
	var req replaceQueriesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.ContactID == "" {
		writeError(w, http.StatusBadRequest, "name and contact_id are required")
		return
	}

	rows := make([]*models.UssdQuery, 0, len(req.Rows))
	for _, row := range req.Rows {
		rows = append(rows, &models.UssdQuery{
			ProfileID: req.ProfileID,
			Payload:   row.Payload,
			IsMany:    row.IsMany || len(req.Rows) > 1,
		})
	}
	key := repository.QueryKey{OwnerID: OwnerIDFromRequest(r), Name: req.Name, ContactID: req.ContactID}
	if err := ms.ussdBusiness.ReplaceQueries(r.Context(), key, rows); err != nil {
		util.Log(r.Context()).WithError(err).Error("failed to replace queries")
		writeError(w, http.StatusInternalServerError, "failed to replace queries")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"stored": len(rows)})
}

type deactivateQueriesRequest struct {
	Name      string `json:"name"`
	ContactID string `json:"contact_id"`
}

func (ms *ManagementServer) DeactivateQueries(w http.ResponseWriter, r *http.Request) {
	var req deactivateQueriesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.ContactID == "" {
		writeError(w, http.StatusBadRequest, "name and contact_id are required")
		return
	}

	key := repository.QueryKey{OwnerID: OwnerIDFromRequest(r), Name: req.Name, ContactID: req.ContactID}
	if err := ms.ussdBusiness.DeactivateQueries(r.Context(), key); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to deactivate queries")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Session endpoints ---

func (ms *ManagementServer) GetSession(w http.ResponseWriter, r *http.Request) {
	session, err := ms.ussdBusiness.GetSession(r.Context(), r.PathValue("sessionID"))
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	if owns, ownErr := ms.ownsService(r, session.ServiceID); ownErr != nil || !owns {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (ms *ManagementServer) CleanupSessions(w http.ResponseWriter, r *http.Request) {
	count, err := ms.ussdBusiness.CleanupExpiredSessions(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cleanup failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"cleaned": count})
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
