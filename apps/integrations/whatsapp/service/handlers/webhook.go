// Package handlers exposes the Cloud API webhook: subscription verification, inbound user
// messages and delivery statuses.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	"buf.build/gen/go/antinvestor/notification/connectrpc/go/notification/v1/notificationv1connect"
	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"connectrpc.com/connect"
	"github.com/antinvestor/service-notification/apps/integrations/whatsapp/config"
	"github.com/antinvestor/service-notification/apps/integrations/whatsapp/service/client"
	"github.com/antinvestor/service-notification/pkg/apperrors"
	"github.com/antinvestor/service-notification/pkg/utility"
	"github.com/pitabwire/util"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	channelWhatsApp = "whatsapp"
	providerTag     = "wa"
	maxWebhookBody  = 4 << 20
)

// WebhookServer handles Meta webhook traffic for every connection routed to this service.
type WebhookServer struct {
	cfg             *config.WhatsAppConfig
	notificationCli notificationv1connect.NotificationServiceClient
	whatsAppCli     *client.Client
}

func NewWebhookServer(cfg *config.WhatsAppConfig, notificationCli notificationv1connect.NotificationServiceClient, whatsAppCli *client.Client) *WebhookServer {
	return &WebhookServer{cfg: cfg, notificationCli: notificationCli, whatsAppCli: whatsAppCli}
}

// NewRouterV1 mounts the webhook at the same path shape the other integrations use, so a
// route's callback URL is /receive/notification/{routeID}.
func (ws *WebhookServer) NewRouterV1() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /receive/notification/{routeID}", ws.Verify)
	mux.HandleFunc("POST /receive/notification/{routeID}", ws.Receive)
	return mux
}

func (ws *WebhookServer) writeError(ctx context.Context, w http.ResponseWriter, err error, code int) {
	util.Log(ctx).WithField("code", code).WithError(err).Error("webhook request rejected")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

// connectionSecrets returns the app secret and verify token for a route: the env values
// when set, otherwise the connection's stored credentials.
func (ws *WebhookServer) connectionSecrets(ctx context.Context, routeID string) (appSecret, verifyToken string) {
	appSecret, verifyToken = ws.cfg.AppSecret, ws.cfg.VerifyToken
	if appSecret != "" && verifyToken != "" {
		return appSecret, verifyToken
	}

	creds, err := ws.whatsAppCli.Credentials().ResolveByName(ctx, routeID)
	if err != nil {
		util.Log(ctx).WithError(err).WithField("route_id", routeID).Debug("no stored credentials for route, using service defaults")
		return appSecret, verifyToken
	}
	if appSecret == "" {
		appSecret = creds[client.CredentialAppSecret]
	}
	if verifyToken == "" {
		verifyToken = creds[client.CredentialVerifyToken]
	}
	return appSecret, verifyToken
}

// Verify answers the subscription handshake Meta performs when a webhook URL is registered.
func (ws *WebhookServer) Verify(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	routeID := r.PathValue("routeID")
	query := r.URL.Query()

	_, verifyToken := ws.connectionSecrets(ctx, routeID)
	if verifyToken == "" {
		ws.writeError(ctx, w, errors.New("webhook verify token is not configured"), http.StatusForbidden)
		return
	}

	if query.Get("hub.mode") != "subscribe" || query.Get("hub.verify_token") != verifyToken {
		ws.writeError(ctx, w, errors.New("webhook verification failed"), http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(query.Get("hub.challenge")))
}

// Receive ingests one webhook delivery. Individual message or status failures are logged
// and do not fail the request; Meta retries the whole payload otherwise.
func (ws *WebhookServer) Receive(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	routeID := r.PathValue("routeID")
	log := util.Log(ctx).WithField("route_id", routeID)

	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBody))
	if err != nil {
		ws.writeError(ctx, w, err, http.StatusBadRequest)
		return
	}

	appSecret, _ := ws.connectionSecrets(ctx, routeID)
	if appSecret != "" {
		if !client.VerifySignature(appSecret, body, r.Header.Get(client.SignatureHeader)) {
			ws.writeError(ctx, w, errors.New("invalid webhook signature"), http.StatusUnauthorized)
			return
		}
	} else {
		log.Warn("no app secret configured, accepting unsigned webhook")
	}

	var payload client.WebhookPayload
	if err = json.Unmarshal(body, &payload); err != nil {
		ws.writeError(ctx, w, err, http.StatusBadRequest)
		return
	}

	messages, statuses, failures := 0, 0, 0
	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			value := change.Value
			for _, msg := range value.Messages {
				messages++
				if handleErr := ws.handleMessage(ctx, routeID, value, msg); handleErr != nil {
					failures++
					log.WithError(handleErr).WithField("wamid", msg.ID).Error("could not forward inbound message")
				}
			}
			for _, status := range value.Statuses {
				statuses++
				if handleErr := ws.handleStatus(ctx, routeID, status); handleErr != nil {
					failures++
					log.WithError(handleErr).WithField("wamid", status.ID).Error("could not apply delivery status")
				}
			}
		}
	}

	log.WithFields(map[string]any{"messages": messages, "statuses": statuses, "failures": failures}).
		Debug("webhook payload processed")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]int{"messages": messages, "statuses": statuses, "failures": failures})
}

// contactName finds the display name Meta sent alongside the message, if any.
func contactName(contacts []client.Contact, waID string) string {
	for _, c := range contacts {
		if c.WaID == waID {
			return c.Profile.Name
		}
	}
	if len(contacts) == 1 {
		return contacts[0].Profile.Name
	}
	return ""
}

func (ws *WebhookServer) handleMessage(ctx context.Context, routeID string, value client.ChangeValue, msg client.Message) error {
	if msg.From == "" || msg.ID == "" {
		return apperrors.ErrMissingRequiredData.Extend("message has no sender or id")
	}

	extraData := map[string]any{
		"wamid":           msg.ID,
		"timestamp":       msg.Timestamp,
		"message_type":    msg.Type,
		"wa_id":           msg.From,
		"phone_number_id": value.Metadata.PhoneNumberID,
	}
	if msg.Context != nil {
		extraData["context_wamid"] = msg.Context.ID
	}
	if media := msg.MediaRef(); media != nil {
		extraData["media_id"] = media.ID
		extraData["media_mime_type"] = media.MimeType
		if media.Filename != "" {
			extraData["media_filename"] = media.Filename
		}
	}
	if reply := msg.ReplyPayload(); reply != "" {
		extraData["reply_payload"] = reply
	}
	if msg.Location != nil {
		extraData["latitude"] = msg.Location.Latitude
		extraData["longitude"] = msg.Location.Longitude
	}
	for i, e := range msg.Errors {
		extraData["error_"+strconv.Itoa(i)] = strconv.Itoa(e.Code) + " " + e.Title
	}

	extra, err := structpb.NewStruct(extraData)
	if err != nil {
		return err
	}

	notification := &notificationv1.Notification{
		Id: utility.DeterministicID(providerTag, msg.ID),
		Source: &commonv1.ContactLink{
			Detail:      "+" + client.NormaliseMSISDN(msg.From),
			ProfileName: contactName(value.Contacts, msg.From),
		},
		Recipient: &commonv1.ContactLink{Detail: "+" + client.NormaliseMSISDN(value.Metadata.DisplayPhoneNumber)},
		Type:      channelWhatsApp,
		Data:      msg.Body(),
		RouteId:   routeID,
		Extras:    extra,
	}

	stream, err := ws.notificationCli.Receive(ctx, connect.NewRequest(&notificationv1.ReceiveRequest{
		Data: []*notificationv1.Notification{notification},
	}))
	if err != nil {
		return err
	}
	defer func() { _ = stream.Close() }()
	for stream.Receive() {
		// The acknowledgement carries nothing the provider needs.
	}
	return stream.Err()
}

func (ws *WebhookServer) handleStatus(ctx context.Context, routeID string, status client.Status) error {
	if status.ID == "" {
		return apperrors.ErrMissingRequiredData.Extend("status has no message id")
	}

	extraData := map[string]any{
		"route":        routeID,
		"timestamp":    status.Timestamp,
		"recipient_id": status.RecipientID,
		"status":       status.Status,
	}
	if status.Conversation != nil {
		extraData["conversation_id"] = status.Conversation.ID
		extraData["conversation_origin"] = status.Conversation.Origin.Type
	}
	if status.Pricing != nil {
		extraData["billable"] = status.Pricing.Billable
		extraData["pricing_category"] = status.Pricing.Category
	}

	state, internalStatus := commonv1.STATE_ACTIVE, commonv1.STATUS_UNKNOWN
	switch status.Status {
	case "sent":
		internalStatus = commonv1.STATUS_IN_PROCESS
	case "delivered":
		state, internalStatus = commonv1.STATE_INACTIVE, commonv1.STATUS_SUCCESSFUL
	case "read":
		state, internalStatus = commonv1.STATE_INACTIVE, commonv1.STATUS_SUCCESSFUL
		extraData["read"] = true
	case "failed":
		state, internalStatus = commonv1.STATE_INACTIVE, commonv1.STATUS_FAILED
		for i, e := range status.Errors {
			extraData["error_"+strconv.Itoa(i)] = strconv.Itoa(e.Code) + " " + e.Title + " " + e.ErrorData.Details
			if i == 0 {
				extraData["error"] = e.Title
				extraData["error_code"] = e.Code
				if fallback := client.StatusFallback(e.Code); fallback != "" {
					extraData["fallback_channel"] = fallback
				}
			}
		}
	default:
		// "deleted" and unknown statuses are recorded but do not move the lifecycle.
	}

	extra, err := structpb.NewStruct(extraData)
	if err != nil {
		return err
	}

	_, err = ws.notificationCli.StatusUpdate(ctx, connect.NewRequest(&commonv1.StatusUpdateRequest{
		State:      state,
		Status:     internalStatus,
		ExternalId: status.ID,
		Extras:     extra,
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			util.Log(ctx).WithField("wamid", status.ID).Debug("status for a message this service did not send, ignoring")
			return nil
		}
		return err
	}
	return nil
}
