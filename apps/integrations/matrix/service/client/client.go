package client

import (
	"context"
	"fmt"
	"maps"
	"strings"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"github.com/antinvestor/gomatrix"
	"github.com/antinvestor/service-notification/apps/integrations/matrix/config"
	"github.com/pitabwire/util"
)

const (
	ExtraMatrixEventTypeKey = "matrix_event_type"

	EventTypeMessage = "message" // Regular message
	EventTypeCustom  = "custom"  // System notice

)

type Client struct {
	cfg *config.NotificationMatrixConfig

	serverNoticeURI string

	logger *util.LogEntry
	matrix *gomatrix.Client
}

func NewClient(logger *util.LogEntry, cfg *config.NotificationMatrixConfig) (*Client, error) {

	matrix, err := gomatrix.NewClient(cfg.MatrixServerURL, cfg.MatrixUserID, cfg.MatrixAccessToken)
	if err != nil {
		return nil, err
	}

	serverNoticeURI := fmt.Sprintf("%s/_synapse/admin/v1/send_server_notice", cfg.MatrixServerURL)

	return &Client{
		logger:          logger,
		cfg:             cfg,
		serverNoticeURI: serverNoticeURI,
		matrix:          matrix,
	}, nil
}

func (ms *Client) groupIDToRoomID(_ context.Context, contact *commonv1.ContactLink) string {
	return fmt.Sprintf("!%s:%s", contact.GetProfileId(), ms.cfg.MatrixServerDomain)
}

func (ms *Client) profileIDToUserID(_ context.Context, contact *commonv1.ContactLink) string {
	return fmt.Sprintf("@%s:%s", contact.GetProfileId(), ms.cfg.MatrixServerDomain)
}

func (ms *Client) Send(ctx context.Context, notification *notificationv1.Notification) (*gomatrix.RespSendEvent, error) {

	recipient := notification.GetRecipient()

	// Determine event type from metadata or payload
	extrasData := notification.GetExtras().AsMap()
	matrixEventType := extrasData[ExtraMatrixEventTypeKey]

	profileType := strings.ToLower(recipient.GetProfileType())

	switch profileType {
	case "group", "room":
		// For groups or rooms, use the profile ID as the room ID
		roomID := ms.groupIDToRoomID(ctx, recipient)

		matrixEventTypeStr := matrixEventType.(string)

		if matrixEventType != "" {
			return ms.sendEvent(ctx, roomID, matrixEventTypeStr, notification)
		}

		return ms.sendMessage(ctx, roomID, notification)
	case "profile", "user":
		// For users, we need to get or create a direct message room
		userID := ms.profileIDToUserID(ctx, recipient)
		return ms.sendUserNotice(ctx, userID, notification)
	default:
		return nil, fmt.Errorf("unsupported profile type: %s", profileType)
	}
}

// sendEvent sends a custom activity event
func (ms *Client) sendEvent(_ context.Context, roomID string, eventType string, notification *notificationv1.Notification) (*gomatrix.RespSendEvent, error) {

	extrasData := notification.GetExtras().AsMap()
	if metaType, ok := extrasData["event_type"]; ok {
		eventType = metaType.(string)
	}

	content := notification.GetPayload().AsMap()

	// Send custom room event - use MatrixCustomActivityEvent as the event type
	return ms.matrix.SendStateEvent(roomID, eventType, "", content)
}

// sendMessage sends a regular message event
func (ms *Client) sendMessage(ctx context.Context, roomID string, notification *notificationv1.Notification) (*gomatrix.RespSendEvent, error) {
	content := ms.extractMessageContent(ctx, notification)
	// Send the message to the room
	return ms.matrix.SendMessageEvent(roomID, "m.room.message", content)
}

// sendUserNotice sends a system notice event (appears differently than regular messages)
func (ms *Client) sendUserNotice(ctx context.Context, userID string, notification *notificationv1.Notification) (*gomatrix.RespSendEvent, error) {

	content := ms.extractMessageContent(ctx, notification)

	payload := map[string]interface{}{
		"user_id": userID,
		"content": content,
	}

	urlPath := ms.serverNoticeURI
	var resp gomatrix.RespSendEvent
	err := ms.matrix.MakeRequest("POST", urlPath, payload, &resp)
	return &resp, err
}

func (ms *Client) extractMessageContent(_ context.Context, notification *notificationv1.Notification) map[string]any {
	content := notification.GetPayload().AsMap()
	maps.Insert(content, maps.All(map[string]any{
		"msgtype": "m.text",
		"body":    notification.GetData(),
	}))

	return content
}
