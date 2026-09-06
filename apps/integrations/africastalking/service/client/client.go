package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	profilev1 "buf.build/gen/go/antinvestor/profile/protocolbuffers/go/profile/v1"
	"buf.build/gen/go/antinvestor/settingz/connectrpc/go/settings/v1/settingsv1connect"
	"github.com/antinvestor/service-notification/apps/integrations/africastalking/config"
	"github.com/antinvestor/service-notification/pkg/constants"
	"github.com/antinvestor/service-notification/pkg/integration/credentials"
	"github.com/antinvestor/service-notification/pkg/utility"
)

// requestTimeout bounds one Africa's Talking API call so a stalled provider cannot pin a
// queue worker indefinitely.
const requestTimeout = 30 * time.Second

type Client struct {
	cfg        *config.AfricasTalkingConfig
	httpClient http.Client

	profileCli  profilev1connect.ProfileServiceClient
	credentials *credentials.Resolver
}

func NewClient(cfg *config.AfricasTalkingConfig, profileCli profilev1connect.ProfileServiceClient, settingsCli settingsv1connect.SettingsServiceClient) (*Client, error) {

	return &Client{
		cfg:         cfg,
		httpClient:  http.Client{Timeout: requestTimeout},
		profileCli:  profileCli,
		credentials: credentials.New(settingsCli, cfg.SettingsIntegrationName, cfg.SettingsIntegrationID),
	}, nil
}

// extractCredentials returns the Africa's Talking credentials for a queued message.
// Explicit key headers win; otherwise the connection (or route) named in the headers is
// resolved from the settings service.
func (ms *Client) extractCredentials(ctx context.Context, headers map[string]string) (map[string]string, error) {
	apiKey, hasKey := headers[constants.APIKeyHeaderName]
	if hasKey {
		apiSenderID, ok := headers[constants.APISenderIDHeaderName]
		if !ok {
			return nil, fmt.Errorf("no api sender id specified for message")
		}
		apiUserName, ok := headers[constants.APIUserNameHeaderName]
		if !ok {
			return nil, fmt.Errorf("no api username has been specified")
		}

		return map[string]string{
			constants.APIKeyHeaderName:      apiKey,
			constants.APISenderIDHeaderName: apiSenderID,
			constants.APIUserNameHeaderName: apiUserName,
		}, nil
	}

	values, err := ms.credentials.Resolve(ctx, headers)
	if err != nil {
		return nil, err
	}

	for _, required := range []string{constants.APIKeyHeaderName, constants.APIUserNameHeaderName, constants.APISenderIDHeaderName} {
		if values[required] == "" {
			return nil, fmt.Errorf("connection %q credentials are missing %s", credentials.ConnectionName(headers), required)
		}
	}
	return values, nil
}

func (ms *Client) Send(ctx context.Context, headers map[string]string, notification *notificationv1.Notification) (*ResponsePayload, error) {

	credentials, err := ms.extractCredentials(ctx, headers)
	if err != nil {
		return nil, err
	}

	recipient, err := utility.PopulateContactLink(ctx, ms.profileCli, notification.GetRecipient(), profilev1.ContactType_MSISDN)
	if err != nil {
		return nil, err
	}

	payload := RequestPayload{
		Username:     credentials[constants.APIUserNameHeaderName],
		SenderID:     credentials[constants.APISenderIDHeaderName],
		PhoneNumbers: []string{recipient.GetDetail()},
		Message:      notification.GetData()}

	apiKey := credentials[constants.APIKeyHeaderName]
	idempotencyKey := notification.GetId()

	response, err := ms.SendBulkSMS(ctx, apiKey, idempotencyKey, payload)
	if err != nil {
		return nil, err
	}
	return response, nil

}

type RecipientPayload struct {
	StatusCode int    `json:"statusCode"`
	Number     string `json:"number"`
	Status     string `json:"status"`
	Cost       string `json:"cost"`
	MessageId  string `json:"messageId"`
}

type ResponsePayload struct {
	SMSMessageData struct {
		Message    string             `json:"Message"`
		Recipients []RecipientPayload `json:"Recipients"`
	} `json:"SMSMessageData"`
}

// RequestPayload defines the structure for the JSON payload.
type RequestPayload struct {
	Username     string   `json:"username"`
	Message      string   `json:"message"`
	SenderID     string   `json:"senderId"`
	PhoneNumbers []string `json:"phoneNumbers"`
}

// SendBulkSMS performs the equivalent of the curl command to send bulk messages.
//
// apiKey: Your Africa's Talking API key.
// username: Your Africa's Talking application username.
// message: The text message to be sent.
// senderID: Your registered short code or alphanumeric sender ID.
// phoneNumbers: A slice of strings containing the recipient phone numbers.
func (ms *Client) SendBulkSMS(ctx context.Context, apiKey, idempotencyKey string, payloadData RequestPayload) (*ResponsePayload, error) {

	jsonData, err := json.Marshal(payloadData)
	if err != nil {
		return nil, fmt.Errorf("error marshalling request data: %w", err)
	}

	//    The bytes.NewBuffer function creates a reader from the byte slice.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ms.cfg.ATServerURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error creating new request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apiKey", apiKey)
	req.Header.Set("Idempotency-Key", idempotencyKey)

	resp, err := ms.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	// 7. Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	// 8. Check for a successful status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API request failed with status code %d: %s", resp.StatusCode, string(body))
	}

	var response ResponsePayload
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (ms *Client) Categorise(ctx context.Context, payload map[string]any) string {
	// Check for Delivery Report
	// Fields: id, status, phoneNumber, networkCode, failureReason
	_, hasId := payload["id"]
	if hasId {
		if _, hasStatus := payload["status"]; hasStatus {
			return DeliveryReport
		}
	}

	// Check for Incoming Messages
	// Fields: text, from, to, id, linkId, date
	if _, hasText := payload["text"]; hasId && hasText {
		if _, hasFrom := payload["from"]; hasFrom {
			return IncomingMessages
		}
	}

	// Check for Bulk SMS Opt Out
	// Fields: phoneNumber, optOutCode, optOutType, optOutSource, optOutDate
	_, hasPhoneNumber := payload["phoneNumber"]
	if hasPhoneNumber {
		if _, hasOptOutCode := payload["optOutCode"]; hasOptOutCode {
			return BulkSMSOptOut
		}
	}

	// Check for Subscription Notifications
	// Fields: phoneNumber, shortCode, keyword, updateType
	if _, hasUpdateType := payload["updateType"]; hasPhoneNumber && hasUpdateType {
		return SubscriptionNotifications
	}

	// If we can't categorise, return empty string
	return ""
}
