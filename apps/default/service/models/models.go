package models

import (
	"context"
	"time"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"github.com/pitabwire/frame/v2/data"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	RouteModeTransmit   = "tx"
	RouteModeReceive    = "rx"
	RouteModeTransceive = "trx"

	RouteTypeAny          = "any"
	RouteTypeEmailForm    = "email"
	RouteTypeSMSForm      = "sms"
	RouteTypeWhatsAppForm = "whatsapp"

	// MessageBodyTextKey and MessageBodyDefaultKey are channel-agnostic template body types.
	MessageBodyTextKey    = "text"
	MessageBodyDefaultKey = "default"

	// ExtraKeyWhatsAppTemplate carries the JSON-encoded WhatsApp template definition
	// (template.extra["whatsapp"]) to the WhatsApp integration.
	ExtraKeyWhatsAppTemplate = "whatsapp_template"
	// ExtraKeySubject carries the rendered subject for channels that use one.
	ExtraKeySubject = "subject"
	// ExtraKeySupportPrefix prefixes partition support contacts merged into the message map.
	ExtraKeySupportPrefix = "support_"
)

// messageBodyFallbackOrder lists channel-agnostic body types tried after the exact
// notification type, in order, before any remaining channel body is used.
var messageBodyFallbackOrder = []string{MessageBodyTextKey, MessageBodyDefaultKey, RouteTypeSMSForm}

// SelectMessageBody picks the rendered body for a notification type from the
// rendered message map: exact type, then text, default, sms, then any channel body.
// Keys that are not channel bodies (subject, support contacts, template metadata) are ignored.
func SelectMessageBody(notificationType string, message map[string]string) string {
	if len(message) == 0 {
		return ""
	}
	if body, ok := message[notificationType]; ok && notificationType != "" {
		return body
	}
	for _, key := range messageBodyFallbackOrder {
		if body, ok := message[key]; ok {
			return body
		}
	}
	for _, key := range []string{RouteTypeWhatsAppForm, RouteTypeEmailForm, RouteTypeAny} {
		if body, ok := message[key]; ok {
			return body
		}
	}
	return ""
}

// Language Our simple table holding all the supported languages
type Language struct {
	data.BaseModel
	Name        string `gorm:"type:varchar(50)"`
	Code        string `gorm:"type:varchar(10)"`
	Description string `gorm:"type:text"`
}

func (l *Language) ToApi() *notificationv1.Language {

	extraData, _ := structpb.NewStruct(map[string]any{"description": l.Description})

	return &notificationv1.Language{
		Id:    l.GetID(),
		Code:  l.Code,
		Name:  l.Name,
		Extra: extraData,
	}
}

// Template Table holds the template details
type Template struct {
	data.BaseModel

	Name  string `gorm:"type:varchar(255)"`
	Extra data.JSONMap
}

func (t *Template) ToApi(templateDataList []*notificationv1.TemplateData) *notificationv1.Template {

	return &notificationv1.Template{
		Id:    t.GetID(),
		Name:  t.Name,
		Data:  templateDataList,
		Extra: t.Extra.ToProtoStruct(),
	}
}

type TemplateData struct {
	data.BaseModel

	TemplateID string `gorm:"type:varchar(50);unique_index:uq_template_by_type"`
	LanguageID string `gorm:"type:varchar(50);unique_index:uq_template_by_type"`
	Type       string `gorm:"type:varchar(10);unique_index:uq_template_by_type"`
	Detail     string `gorm:"type:text"`
	Subject    string `gorm:"type:varchar(250)"`
}

func (td *TemplateData) ToApi(language *notificationv1.Language) *notificationv1.TemplateData {

	tData := &notificationv1.TemplateData{
		Id:       td.GetID(),
		Type:     td.Type,
		Detail:   td.Detail,
		Language: language,
	}

	return tData
}

// Notification table holding all the payload of message in transit in and out of the system
type Notification struct {
	data.BaseModel

	ParentID string `gorm:"type:varchar(50)"`

	SenderProfileID   string `gorm:"type:varchar(250)"`
	SenderProfileType string `gorm:"type:varchar(50)"`
	SenderContactID   string `gorm:"type:varchar(50)"`

	RecipientProfileID   string `gorm:"type:varchar(50)"`
	RecipientProfileType string `gorm:"type:varchar(50)"`
	RecipientContactID   string `gorm:"type:varchar(50)"`

	RouteID  string `gorm:"type:varchar(50)"`
	OutBound bool

	LanguageID string `gorm:"type:varchar(50)"`
	TemplateID string `gorm:"type:varchar(50)"`

	NotificationType string `gorm:"type:varchar(10)"`
	Message          string `gorm:"type:text"`
	Payload          data.JSONMap

	ReleasedAt  *time.Time
	State       int32
	TransientID string `gorm:"type:varchar(50)"`
	ExternalID  string `gorm:"type:varchar(50)"`

	StatusID string `gorm:"type:varchar(50)"`
	Priority int32
}

func (model *Notification) IsReleased() bool {
	return model.ReleasedAt != nil && !model.ReleasedAt.IsZero()
}

func NotificationFromAPI(ctx context.Context, notification *notificationv1.Notification) *Notification {
	if notification == nil {
		return nil
	}

	model := &Notification{
		ParentID:         notification.GetParentId(),
		TransientID:      notification.GetId(),
		TemplateID:       notification.GetTemplate(),
		LanguageID:       notification.GetLanguage(),
		NotificationType: notification.GetType(),
		Message:          notification.GetData(),
		OutBound:         notification.GetOutBound(),
		RouteID:          notification.GetRouteId(),
		Priority:         int32(notification.GetPriority()),
	}

	model.ID = notification.GetId()

	if notification.GetPayload() != nil {
		model.Payload = (&data.JSONMap{}).FromProtoStruct(notification.GetPayload())
	}

	if source := notification.GetSource(); source != nil {
		model.SenderProfileID = source.GetProfileId()
		model.SenderProfileType = source.GetProfileType()
		model.SenderContactID = source.GetContactId()
	}

	if recipient := notification.GetRecipient(); recipient != nil {
		model.RecipientProfileID = recipient.GetProfileId()
		model.RecipientProfileType = recipient.GetProfileType()
		model.RecipientContactID = recipient.GetContactId()
	}

	model.GenID(ctx)
	if model.ValidXID(notification.GetId()) {
		model.ID = notification.GetId()
	}

	return model
}

func (model *Notification) ToAPI(status *NotificationStatus, language *Language, message map[string]string) *notificationv1.Notification {

	extra := data.JSONMap{}
	extra["tenant_id"] = model.TenantID
	extra["partition_id"] = model.PartitionID
	extra["access_id"] = model.AccessID

	if model.IsReleased() {
		extra["ReleaseDate"] = model.ReleasedAt.String()
	}

	if len(message) != 0 {
		if model.Message == "" {
			model.Message = SelectMessageBody(model.NotificationType, message)
		}

		for key, val := range message {
			extra[key] = val
		}
	}

	languageCode := ""
	if language != nil {
		languageCode = language.Code
	}

	source := &commonv1.ContactLink{
		ProfileType: model.SenderProfileType,
		ProfileId:   model.SenderProfileID,
		ContactId:   model.SenderContactID,
	}

	recipient := &commonv1.ContactLink{
		ProfileType: model.RecipientProfileType,
		ProfileId:   model.RecipientProfileID,
		ContactId:   model.RecipientContactID,
	}

	notification := notificationv1.Notification{
		Id:          model.ID,
		Source:      source,
		Recipient:   recipient,
		Type:        model.NotificationType,
		Template:    model.TemplateID,
		Payload:     model.Payload.ToProtoStruct(),
		Data:        model.Message,
		Language:    languageCode,
		OutBound:    model.OutBound,
		AutoRelease: model.IsReleased(),
		RouteId:     model.RouteID,
		Status:      status.ToAPI(),
		Extras:      extra.ToProtoStruct(),
		Priority:    notificationv1.PRIORITY(model.Priority),
	}
	return &notification
}

// NotificationStatus table holding all the statuses of notifications in our system
type NotificationStatus struct {
	data.BaseModel
	NotificationID string `gorm:"type:varchar(50)"`

	TransientID string `gorm:"type:varchar(50)"`
	ExternalID  string `gorm:"type:varchar(50)"`
	Extra       data.JSONMap
	State       int32
	Status      int32
}

func (model *NotificationStatus) ToAPI() *commonv1.StatusResponse {

	if model == nil {
		return nil
	}

	extraData := data.JSONMap{
		"CreatedAt": model.CreatedAt.String(),
		"StatusID":  model.ID,
	}
	extraData = extraData.Update(model.Extra)

	status := commonv1.StatusResponse{
		Id:          model.NotificationID,
		State:       commonv1.STATE(model.State),
		Status:      commonv1.STATUS(model.Status),
		TransientId: model.TransientID,
		ExternalId:  model.ExternalID,
		Extras:      extraData.ToProtoStruct(),
	}

	return &status
}

// Route Our simple table holding all the payload of message in transit in and out of the system
type Route struct {
	data.BaseModel

	CounterID   string `gorm:"type:varchar(50)"`
	Name        string `gorm:"type:varchar(50)"`
	Description string `gorm:"type:text"`
	RouteType   string `gorm:"type:varchar(10)"`
	Mode        string `gorm:"type:varchar(10)"`
	Uri         string `gorm:"type:varchar(255)"`
}

// StatusExtraFallbackChannel is the status extra key an integration sets, together with
// STATUS_FAILED, to request one more delivery attempt on a different channel.
const StatusExtraFallbackChannel = "fallback_channel"

// StatusExtraFallbackNotificationID records on the failed parent's status the id of the
// child notification created for the fallback channel.
const StatusExtraFallbackNotificationID = "fallback_notification_id"

// NewFallbackNotification builds the child notification that retries a failed outbound
// parent on the given channel. It copies the parties, content and partition scope, links
// back through ParentID and is released immediately so it routes without another release.
func NewFallbackNotification(ctx context.Context, parent *Notification, channel string) *Notification {
	now := time.Now()
	child := &Notification{
		ParentID:             parent.GetID(),
		SenderProfileID:      parent.SenderProfileID,
		SenderProfileType:    parent.SenderProfileType,
		SenderContactID:      parent.SenderContactID,
		RecipientProfileID:   parent.RecipientProfileID,
		RecipientProfileType: parent.RecipientProfileType,
		RecipientContactID:   parent.RecipientContactID,
		OutBound:             true,
		LanguageID:           parent.LanguageID,
		TemplateID:           parent.TemplateID,
		NotificationType:     channel,
		Message:              parent.Message,
		Payload:              parent.Payload.Copy(),
		ReleasedAt:           &now,
		Priority:             parent.Priority,
	}
	child.TenantID = parent.TenantID
	child.PartitionID = parent.PartitionID
	child.AccessID = parent.AccessID
	child.GenID(ctx)
	return child
}
