// Package client talks to the WhatsApp Business Cloud API (Meta Graph API).
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	profilev1 "buf.build/gen/go/antinvestor/profile/protocolbuffers/go/profile/v1"
	"buf.build/gen/go/antinvestor/settingz/connectrpc/go/settings/v1/settingsv1connect"
	"github.com/antinvestor/service-notification/apps/integrations/whatsapp/config"
	"github.com/antinvestor/service-notification/pkg/integration/credentials"
	"github.com/antinvestor/service-notification/pkg/utility"
)

// Credential keys stored per connection in the settings service.
const (
	CredentialAccessToken   = "access_token"
	CredentialPhoneNumberID = "phone_number_id"
	CredentialAppSecret     = "app_secret"
	CredentialVerifyToken   = "verify_token"
)

// ExtraKeyTemplate is the notification extra carrying the WhatsApp template definition
// the notification service copies from template.extra["whatsapp"].
const ExtraKeyTemplate = "whatsapp_template"

const (
	messagingProduct    = "whatsapp"
	maxResponseBody     = 1 << 20
	maxTextBodyLength   = 4096
	defaultTemplateLang = "en"
)

// TemplateDefinition is the shape of template.extra["whatsapp"].
type TemplateDefinition struct {
	// Name of the Meta-approved message template.
	Name string `json:"name"`
	// Language code registered with the template (e.g. en_US). Falls back to the notification language.
	Language string `json:"language"`
	// Params lists payload keys, in the order of the template body placeholders {{1}}, {{2}}...
	Params []string `json:"params"`
	// HeaderParams lists payload keys for header text placeholders, if the template has any.
	HeaderParams []string `json:"header_params"`
}

// SendResult is the provider acknowledgement of an accepted message.
type SendResult struct {
	MessageID string
	WaID      string
	Template  string
}

// Client sends messages through the Cloud API for any connection whose credentials the
// settings service holds.
type Client struct {
	cfg         *config.WhatsAppConfig
	httpClient  *http.Client
	profileCli  profilev1connect.ProfileServiceClient
	credentials *credentials.Resolver
}

// NewClient wires a Cloud API client. The HTTP timeout is set once at the client level.
func NewClient(cfg *config.WhatsAppConfig, profileCli profilev1connect.ProfileServiceClient, settingsCli settingsv1connect.SettingsServiceClient) *Client {
	timeout := time.Duration(cfg.RequestTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		cfg:         cfg,
		httpClient:  &http.Client{Timeout: timeout},
		profileCli:  profileCli,
		credentials: credentials.New(settingsCli, cfg.SettingsIntegrationName, cfg.SettingsIntegrationID),
	}
}

// Credentials exposes the resolver so the webhook can look up per-connection secrets.
func (c *Client) Credentials() *credentials.Resolver {
	return c.credentials
}

// Send delivers a queued notification: a template message when the notification carries a
// WhatsApp template definition, otherwise a free-form text message.
func (c *Client) Send(ctx context.Context, headers map[string]string, notification *notificationv1.Notification) (*SendResult, error) {
	creds, err := c.credentials.Resolve(ctx, headers)
	if err != nil {
		return nil, err
	}
	token, phoneNumberID := creds[CredentialAccessToken], creds[CredentialPhoneNumberID]
	if token == "" || phoneNumberID == "" {
		return nil, fmt.Errorf("connection %q credentials need %s and %s",
			credentials.ConnectionName(headers), CredentialAccessToken, CredentialPhoneNumberID)
	}

	recipient, err := utility.PopulateContactLink(ctx, c.profileCli, notification.GetRecipient(), profilev1.ContactType_MSISDN)
	if err != nil {
		return nil, err
	}
	to := NormaliseMSISDN(recipient.GetDetail())
	if to == "" {
		return nil, errors.New("recipient has no phone number")
	}

	body, templateName, err := buildMessage(to, notification)
	if err != nil {
		return nil, err
	}

	result, err := c.post(ctx, token, phoneNumberID, body)
	if err != nil {
		var sendErr *SendError
		if errors.As(err, &sendErr) && sendErr.Credential() {
			c.credentials.Forget(credentials.ConnectionName(headers))
		}
		return nil, err
	}
	result.Template = templateName
	return result, nil
}

// buildMessage renders the Cloud API request body for the notification.
func buildMessage(to string, notification *notificationv1.Notification) (map[string]any, string, error) {
	extras := notification.GetExtras().AsMap()

	if raw, ok := extras[ExtraKeyTemplate].(string); ok && raw != "" {
		var def TemplateDefinition
		if err := json.Unmarshal([]byte(raw), &def); err != nil {
			return nil, "", fmt.Errorf("invalid whatsapp template definition: %w", err)
		}
		if def.Name == "" {
			return nil, "", errors.New("whatsapp template definition has no name")
		}
		body, err := templateMessage(to, def, notification)
		if err != nil {
			return nil, "", err
		}
		return body, def.Name, nil
	}

	text := notification.GetData()
	if text == "" {
		return nil, "", errors.New("notification has no message body")
	}
	if len(text) > maxTextBodyLength {
		text = text[:maxTextBodyLength]
	}
	return map[string]any{
		"messaging_product": messagingProduct,
		"recipient_type":    "individual",
		"to":                to,
		"type":              "text",
		"text":              map[string]any{"preview_url": false, "body": text},
	}, "", nil
}

func templateMessage(to string, def TemplateDefinition, notification *notificationv1.Notification) (map[string]any, error) {
	payload := notification.GetPayload().AsMap()

	language := def.Language
	if language == "" {
		language = notification.GetLanguage()
	}
	if language == "" {
		language = defaultTemplateLang
	}

	var components []map[string]any
	if len(def.HeaderParams) > 0 {
		params, err := textParameters(def.HeaderParams, payload)
		if err != nil {
			return nil, err
		}
		components = append(components, map[string]any{"type": "header", "parameters": params})
	}
	if len(def.Params) > 0 {
		params, err := textParameters(def.Params, payload)
		if err != nil {
			return nil, err
		}
		components = append(components, map[string]any{"type": "body", "parameters": params})
	}

	template := map[string]any{
		"name":     def.Name,
		"language": map[string]any{"code": language},
	}
	if len(components) > 0 {
		template["components"] = components
	}

	return map[string]any{
		"messaging_product": messagingProduct,
		"recipient_type":    "individual",
		"to":                to,
		"type":              "template",
		"template":          template,
	}, nil
}

func textParameters(keys []string, payload map[string]any) ([]map[string]any, error) {
	params := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		value := utility.PayloadString(payload, key)
		if value == "" {
			return nil, fmt.Errorf("template parameter %q is missing from the notification payload", key)
		}
		params = append(params, map[string]any{"type": "text", "text": value})
	}
	return params, nil
}

// NormaliseMSISDN strips everything but digits: the Cloud API wants E.164 without '+'.
func NormaliseMSISDN(number string) string {
	var b strings.Builder
	for _, r := range number {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

type apiResponse struct {
	Contacts []struct {
		Input string `json:"input"`
		WaID  string `json:"wa_id"`
	} `json:"contacts"`
	Messages []struct {
		ID string `json:"id"`
	} `json:"messages"`
	Error *struct {
		Message   string `json:"message"`
		Type      string `json:"type"`
		Code      int    `json:"code"`
		Subcode   int    `json:"error_subcode"`
		ErrorData struct {
			Details string `json:"details"`
		} `json:"error_data"`
		TraceID string `json:"fbtrace_id"`
	} `json:"error"`
}

func (c *Client) messagesURL(phoneNumberID string) string {
	return fmt.Sprintf("%s/%s/%s/messages", strings.TrimRight(c.cfg.GraphAPIURL, "/"), c.cfg.GraphAPIVersion, phoneNumberID)
}

func (c *Client) post(ctx context.Context, token, phoneNumberID string, body map[string]any) (*SendResult, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encoding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.messagesURL(phoneNumberID), bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &SendError{HTTPStatus: http.StatusBadGateway, Message: err.Error()}
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, &SendError{HTTPStatus: http.StatusBadGateway, Message: "reading response: " + err.Error()}
	}

	var parsed apiResponse
	if len(raw) > 0 {
		if err = json.Unmarshal(raw, &parsed); err != nil {
			return nil, &SendError{HTTPStatus: resp.StatusCode, Message: "unparseable response: " + string(raw)}
		}
	}

	if parsed.Error != nil {
		return nil, &SendError{
			HTTPStatus: resp.StatusCode,
			Code:       parsed.Error.Code,
			Subcode:    parsed.Error.Subcode,
			Type:       parsed.Error.Type,
			Message:    parsed.Error.Message,
			Details:    parsed.Error.ErrorData.Details,
			TraceID:    parsed.Error.TraceID,
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &SendError{HTTPStatus: resp.StatusCode, Message: string(raw)}
	}
	if len(parsed.Messages) == 0 || parsed.Messages[0].ID == "" {
		return nil, &SendError{HTTPStatus: resp.StatusCode, Message: "response has no message id"}
	}

	result := &SendResult{MessageID: parsed.Messages[0].ID}
	if len(parsed.Contacts) > 0 {
		result.WaID = parsed.Contacts[0].WaID
	}
	return result, nil
}
