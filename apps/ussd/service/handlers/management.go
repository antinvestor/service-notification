package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/antinvestor/service-notification/apps/ussd/service/business"
	"github.com/antinvestor/service-notification/apps/ussd/service/models"
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

// RegisterRoutes adds management API routes to the given mux.
// NOTE: These endpoints should be protected by an authentication middleware
// (e.g. Frame's security interceptors or an API gateway) in production.
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
	mux.HandleFunc("POST /api/v1/queries", ms.CreateQuery)
	mux.HandleFunc("DELETE /api/v1/queries", ms.DeactivateQueries)

	// Session management
	mux.HandleFunc("GET /api/v1/sessions/{sessionID}", ms.GetSession)
	mux.HandleFunc("POST /api/v1/sessions/cleanup", ms.CleanupSessions)
}

// --- Menu endpoints ---

func (ms *ManagementServer) GetMenu(w http.ResponseWriter, r *http.Request) {
	menuID := r.PathValue("menuID")
	menu, err := ms.ussdBusiness.GetMenu(r.Context(), menuID)
	if err != nil {
		writeError(w, http.StatusNotFound, "menu not found")
		return
	}
	writeJSON(w, http.StatusOK, menu)
}

func (ms *ManagementServer) GetMenuChildren(w http.ResponseWriter, r *http.Request) {
	menuID := r.PathValue("menuID")
	children, err := ms.ussdBusiness.GetMenuChildren(r.Context(), menuID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get children")
		return
	}
	writeJSON(w, http.StatusOK, children)
}

func (ms *ManagementServer) GetRootMenus(w http.ResponseWriter, r *http.Request) {
	ownerID := r.URL.Query().Get("owner_id")
	if ownerID == "" {
		// Default to the authenticated user's ID
		ownerID = OwnerIDFromRequest(r)
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
	OwnerID       string `json:"owner_id"`
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

	ownerID := req.OwnerID
	if ownerID == "" {
		ownerID = OwnerIDFromRequest(r)
	}

	menu := models.UssdMenuFromParams(r.Context(),
		ownerID, req.ParentID, req.Action, req.Validator,
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
	menuID := r.PathValue("menuID")

	existing, err := ms.ussdBusiness.GetMenu(r.Context(), menuID)
	if err != nil {
		writeError(w, http.StatusNotFound, "menu not found")
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
	if err := ms.ussdBusiness.DeleteMenu(r.Context(), menuID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete menu")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type duplicateMenuRequest struct {
	OwnerID  string `json:"owner_id"`
	ParentID string `json:"parent_id"`
}

func (ms *ManagementServer) DuplicateMenu(w http.ResponseWriter, r *http.Request) {
	menuID := r.PathValue("menuID")

	var req duplicateMenuRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	newMenu, err := ms.ussdBusiness.DuplicateMenu(r.Context(), menuID, req.OwnerID, req.ParentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to duplicate menu")
		return
	}
	writeJSON(w, http.StatusCreated, newMenu)
}

// --- Translation endpoints ---

func (ms *ManagementServer) GetTranslation(w http.ResponseWriter, r *http.Request) {
	menuID := r.PathValue("menuID")
	langCode := r.PathValue("langCode")

	t, err := ms.ussdBusiness.GetTranslations(r.Context(), menuID, langCode)
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
	langCode := r.PathValue("langCode")

	var req setTranslationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	t := &models.UssdTranslation{
		MenuID:       menuID,
		Code:         langCode,
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

func (ms *ManagementServer) GetServiceConfig(w http.ResponseWriter, r *http.Request) {
	serviceID := r.PathValue("serviceID")
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

	cfg := &models.UssdServiceConfig{
		ServiceID:   serviceID,
		Name:        req.Name,
		Value:       req.Value,
		Description: req.Description,
	}

	if err := ms.ussdBusiness.SetServiceConfig(r.Context(), cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config")
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// --- Query endpoints ---

type createQueryRequest struct {
	MSISDN  string         `json:"msisdn"`
	UserID  string         `json:"user_id"`
	BotID   string         `json:"bot_id"`
	Name    string         `json:"name"`
	Payload map[string]any `json:"payload"`
	IsMany  bool           `json:"is_many"`
}

func (ms *ManagementServer) CreateQuery(w http.ResponseWriter, r *http.Request) {
	var req createQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	q := &models.UssdQuery{
		MSISDN:   req.MSISDN,
		UserID:   req.UserID,
		BotID:    req.BotID,
		Name:     req.Name,
		Payload:  req.Payload,
		IsMany:   req.IsMany,
		IsActive: true,
	}

	if err := ms.ussdBusiness.CreateQuery(r.Context(), q); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create query")
		return
	}
	writeJSON(w, http.StatusCreated, q)
}

type deactivateQueriesRequest struct {
	Name   string `json:"name"`
	MSISDN string `json:"msisdn"`
	UserID string `json:"user_id"`
}

func (ms *ManagementServer) DeactivateQueries(w http.ResponseWriter, r *http.Request) {
	var req deactivateQueriesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := ms.ussdBusiness.DeactivateQueries(r.Context(), req.Name, req.MSISDN, req.UserID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to deactivate queries")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Session endpoints ---

func (ms *ManagementServer) GetSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	session, err := ms.ussdBusiness.GetSession(r.Context(), sessionID)
	if err != nil {
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
