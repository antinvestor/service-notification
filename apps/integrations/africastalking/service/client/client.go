package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	commonv1 "github.com/antinvestor/apis/go/common/v1"
	notificationv1 "github.com/antinvestor/apis/go/notification/v1"
	profilev1 "github.com/antinvestor/apis/go/profile/v1"
	settingsv1 "github.com/antinvestor/apis/go/settings/v1"
	"github.com/antinvestor/service-notification/apps/integrations/africastalking/config"
	"github.com/antinvestor/service-notification/internal/constants"
	"github.com/pitabwire/util"
)

type Client struct {
	cfg        *config.AfricasTalkingConfig
	httpClient http.Client
	logger     *util.LogEntry

	profileCli  *profilev1.ProfileClient
	settingsCli *settingsv1.SettingsClient
}

func NewClient(logger *util.LogEntry, cfg *config.AfricasTalkingConfig, profileCli *profilev1.ProfileClient, settingsCli *settingsv1.SettingsClient) (*Client, error) {

	return &Client{
		cfg:         cfg,
		httpClient:  http.Client{},
		logger:      logger,
		profileCli:  profileCli,
		settingsCli: settingsCli,
	}, nil
}

func (ms *Client) contactLinkToPhoneNumber(ctx context.Context, contact *commonv1.ContactLink) (string, error) {

	if contact.GetDetail() != "" {
		return contact.GetDetail(), nil
	}

	result, err := ms.profileCli.Svc().GetByContact(ctx, &profilev1.GetByContactRequest{Contact: contact.GetContactId()})
	if err != nil {
		return "", err
	}

	profile := result.GetData()

	for _, c := range profile.GetContacts() {
		if c.GetId() == contact.GetContactId() {
			if c.GetType() == profilev1.ContactType_MSISDN {
				return c.GetDetail(), nil
			}
		}
	}

	return "", fmt.Errorf("no valid contact exists in request")
}

func (ms *Client) extractCredentials(ctx context.Context, headers map[string]string) (map[string]string, error) {
	var credentials map[string]string
	connection, ok := headers[constants.APIConnectionCredentialsHeaderName]
	if !ok {
		apiKey, ok0 := headers[constants.APIKeyHeaderName]
		if !ok0 {
			return nil, fmt.Errorf("no api key exists")
		}
		apiSenderID, ok0 := headers[constants.APISenderIDHeaderName]
		if !ok0 {
			return nil, fmt.Errorf("no api sender id specified for message")
		}
		apiUserName, ok0 := headers[constants.APIKeyHeaderName]
		if !ok0 {
			return nil, fmt.Errorf("no api username has been specified")
		}

		credentials = map[string]string{
			constants.APIKeyHeaderName:      apiKey,
			constants.APISenderIDHeaderName: apiSenderID,
			constants.APIUserNameHeaderName: apiUserName,
		}

		return credentials, nil
	}

	settingReq := &settingsv1.GetRequest{
		Key: &settingsv1.Setting{
			Name:     connection,
			Object:   ms.cfg.SettingsIntegrationName,
			ObjectId: ms.cfg.SettingsIntegrationID,
			Lang:     "",
			Module:   ms.cfg.SettingsIntegrationName,
		},
	}

	settingResp, err := ms.settingsCli.Svc().Get(ctx, settingReq)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(settingResp.GetData().GetValue()), &credentials)
	if err != nil {
		return nil, err
	}

	return credentials, nil
}

func (ms *Client) Send(ctx context.Context, headers map[string]string, notification *notificationv1.Notification) (*ResponsePayload, error) {

	credentials, err := ms.extractCredentials(ctx, headers)
	if err != nil {
		return nil, err
	}

	recipient := notification.GetRecipient()

	recipientMSISDN, err := ms.contactLinkToPhoneNumber(ctx, recipient)
	if err != nil {
		return nil, err
	}

	payload := RequestPayload{
		Username:     credentials[constants.APIUserNameHeaderName],
		SenderID:     credentials[constants.APISenderIDHeaderName],
		PhoneNumbers: []string{recipientMSISDN},
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
	req, err := http.NewRequest("POST", ms.cfg.ATServerURL, bytes.NewBuffer(jsonData))
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
