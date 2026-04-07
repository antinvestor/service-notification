package handlers

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/antinvestor/service-notification/apps/ussd/service/business"
	"github.com/antinvestor/service-notification/apps/ussd/service/engine"
	"github.com/pitabwire/util"
)

// GatewayServer handles incoming USSD requests from telco gateways.
// It supports form-encoded, JSON, and XML request formats.
type GatewayServer struct {
	ussdBusiness business.UssdBusiness
}

// NewGatewayServer creates a new gateway handler.
func NewGatewayServer(ussdBusiness business.UssdBusiness) *GatewayServer {
	return &GatewayServer{ussdBusiness: ussdBusiness}
}

// NewRouter builds the HTTP router for the gateway endpoints.
func (gs *GatewayServer) NewRouter() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /ussd/gateway/{serviceID}", gs.HandleUSSD)
	mux.HandleFunc("POST /ussd/gateway/{serviceID}/{protocol}", gs.HandleUSSD)
	mux.HandleFunc("GET /ussd/health", gs.HealthCheck)
	return mux
}

// HandleUSSD is the main entry point for telco gateway USSD requests.
func (gs *GatewayServer) HandleUSSD(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := util.Log(ctx)

	serviceID := r.PathValue("serviceID")
	if serviceID == "" {
		http.Error(w, "service ID required", http.StatusBadRequest)
		return
	}

	protocol := r.PathValue("protocol")
	if protocol == "" {
		protocol = detectProtocol(r)
	}

	var req engine.USSDRequest
	var err error

	switch protocol {
	case "json":
		req, err = gs.parseJSONRequest(r, serviceID)
	case "xml":
		req, err = gs.parseXMLRequest(r, serviceID)
	default:
		req, err = gs.parseFormRequest(r, serviceID)
	}

	if err != nil {
		logger.WithError(err).Warn("failed to parse USSD request")
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	// Extract client IP
	req.IP = extractIP(r)

	// Process the USSD request
	resp := gs.ussdBusiness.ProcessUSSD(ctx, req)

	// Write response in the same format as the request
	switch protocol {
	case "json":
		gs.writeJSONResponse(w, resp)
	case "xml":
		gs.writeXMLResponse(w, resp)
	default:
		gs.writeTextResponse(w, resp)
	}
}

// HealthCheck provides a simple health endpoint.
func (gs *GatewayServer) HealthCheck(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// --- Form-encoded request parsing ---

func (gs *GatewayServer) parseFormRequest(r *http.Request, serviceID string) (engine.USSDRequest, error) {
	if err := r.ParseForm(); err != nil {
		return engine.USSDRequest{}, err
	}

	return engine.USSDRequest{
		ServiceID:       serviceID,
		MSISDN:          firstNonEmpty(r.FormValue("msisdn"), r.FormValue("MSISDN"), r.FormValue("phoneNumber")),
		SessionExternal: firstNonEmpty(r.FormValue("sessionId"), r.FormValue("session_id"), r.FormValue("sessionID")),
		UserInput:       firstNonEmpty(r.FormValue("input"), r.FormValue("text"), r.FormValue("data"), r.FormValue("ussdString")),
		Lang:            r.FormValue("lang"),
		AuthKey:         firstNonEmpty(r.FormValue("authKey"), r.FormValue("auth_key"), r.Header.Get("X-Auth-Key")),
		IsFinal:         r.FormValue("clean") == "clean-session",
	}, nil
}

// --- JSON request parsing ---

type jsonUSSDRequest struct {
	MSISDN    string `json:"msisdn"`
	SessionID string `json:"sessionId"`
	Input     string `json:"input"`
	Lang      string `json:"lang"`
	AuthKey   string `json:"authKey"`
	IsFinal   bool   `json:"isFinal"`
}

// maxRequestBody is the maximum allowed request body size (64KB — USSD payloads are tiny).
const maxRequestBody = 64 * 1024

func (gs *GatewayServer) parseJSONRequest(r *http.Request, serviceID string) (engine.USSDRequest, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBody))
	if err != nil {
		return engine.USSDRequest{}, err
	}

	var jr jsonUSSDRequest
	if err := json.Unmarshal(body, &jr); err != nil {
		return engine.USSDRequest{}, err
	}

	authKey := jr.AuthKey
	if authKey == "" {
		authKey = r.Header.Get("X-Auth-Key")
	}

	return engine.USSDRequest{
		ServiceID:       serviceID,
		MSISDN:          jr.MSISDN,
		SessionExternal: jr.SessionID,
		UserInput:       jr.Input,
		Lang:            jr.Lang,
		AuthKey:         authKey,
		IsFinal:         jr.IsFinal,
	}, nil
}

// --- XML request parsing ---

type xmlUSSDRequest struct {
	XMLName   xml.Name `xml:"request"`
	MSISDN    string   `xml:"msisdn"`
	SessionID string   `xml:"sessionId"`
	Input     string   `xml:"input"`
	Lang      string   `xml:"lang"`
	AuthKey   string   `xml:"authKey"`
	IsFinal   bool     `xml:"isFinal"`
}

func (gs *GatewayServer) parseXMLRequest(r *http.Request, serviceID string) (engine.USSDRequest, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBody))
	if err != nil {
		return engine.USSDRequest{}, err
	}

	var xr xmlUSSDRequest
	if err := xml.Unmarshal(body, &xr); err != nil {
		return engine.USSDRequest{}, err
	}

	authKey := xr.AuthKey
	if authKey == "" {
		authKey = r.Header.Get("X-Auth-Key")
	}

	return engine.USSDRequest{
		ServiceID:       serviceID,
		MSISDN:          xr.MSISDN,
		SessionExternal: xr.SessionID,
		UserInput:       xr.Input,
		Lang:            xr.Lang,
		AuthKey:         authKey,
		IsFinal:         xr.IsFinal,
	}, nil
}

// --- Response writers ---

func (gs *GatewayServer) writeTextResponse(w http.ResponseWriter, resp engine.USSDResponse) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	// Flares/standard USSD gateway headers
	if resp.IsEnd {
		w.Header().Set("Freeflow", "FB")
	} else {
		w.Header().Set("Freeflow", "FC")
	}
	w.Header().Set("Charge", "N")
	w.Header().Set("Amount", "0")

	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, resp.Message)
}

type jsonUSSDResponse struct {
	Message   string `json:"message"`
	SessionID string `json:"sessionId,omitempty"`
	IsEnd     bool   `json:"isEnd"`
}

func (gs *GatewayServer) writeJSONResponse(w http.ResponseWriter, resp engine.USSDResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(jsonUSSDResponse{
		Message:   resp.Message,
		SessionID: resp.SessionID,
		IsEnd:     resp.IsEnd,
	})
}

type xmlUSSDResponse struct {
	XMLName   xml.Name `xml:"response"`
	Message   string   `xml:"message"`
	SessionID string   `xml:"sessionId,omitempty"`
	IsEnd     bool     `xml:"isEnd"`
}

func (gs *GatewayServer) writeXMLResponse(w http.ResponseWriter, resp engine.USSDResponse) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(xmlUSSDResponse{
		Message:   resp.Message,
		SessionID: resp.SessionID,
		IsEnd:     resp.IsEnd,
	})
}

// --- Utilities ---

func detectProtocol(r *http.Request) string {
	ct := r.Header.Get("Content-Type")
	switch {
	case strings.Contains(ct, "json"):
		return "json"
	case strings.Contains(ct, "xml"):
		return "xml"
	default:
		return "form"
	}
}

func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Strip port from RemoteAddr (handles both IPv4 and IPv6)
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
