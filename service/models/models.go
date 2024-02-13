package models

import (
	"time"

	commonv1 "github.com/antinvestor/apis/go/common/v1"
	notificationV1 "github.com/antinvestor/apis/go/notification/v1"
	"github.com/pitabwire/frame"
	"gorm.io/datatypes"
)

const (
	RouteModeTransmit   = "tx"
	RouteModeReceive    = "rx"
	RouteModeTransceive = "trx"

	RouteTypeAny       = "any"
	RouteTypeLongForm  = "l"
	RouteTypeShortForm = "s"
)

// Language Our simple table holding all the supported languages
type Language struct {
	frame.BaseModel

	Name        string `gorm:"type:varchar(50);unique_index"`
	Code        string `gorm:"type:varchar(10);unique_index"`
	Description string `gorm:"type:text"`
}

func (l *Language) ToApi() *notificationV1.Language {
	return &notificationV1.Language{
		Id:    l.GetID(),
		Code:  l.Code,
		Name:  l.Name,
		Extra: map[string]string{"description": l.Description},
	}
}

// Template Table holds the template details
type Template struct {
	frame.BaseModel

	Name  string `gorm:"type:varchar(255)"`
	Extra datatypes.JSONMap
}

func (t *Template) ToApi(templateDataList []*notificationV1.TemplateData) *notificationV1.Template {

	return &notificationV1.Template{
		Id:    t.GetID(),
		Name:  t.Name,
		Data:  templateDataList,
		Extra: frame.DBPropertiesToMap(t.Extra),
	}
}

type TemplateData struct {
	frame.BaseModel

	TemplateID string `gorm:"type:varchar(50);unique_index:uq_template_by_type"`
	LanguageID string `gorm:"type:varchar(50);unique_index:uq_template_by_type"`
	Type       string `gorm:"type:varchar(10);unique_index:uq_template_by_type"`
	Detail     string `gorm:"type:text"`
}

func (td *TemplateData) ToApi(language *notificationV1.Language) *notificationV1.TemplateData {

	tData := &notificationV1.TemplateData{
		Id:       td.GetID(),
		Type:     td.Type,
		Detail:   td.Detail,
		Language: language,
	}

	return tData
}

// Notification table holding all the payload of message in transit in and out of the system
type Notification struct {
	frame.BaseModel

	Source string `gorm:"type:varchar(250)"`

	ProfileType string `gorm:"type:varchar(50)"`

	ProfileID string `gorm:"type:varchar(50)"`
	ContactID string `gorm:"type:varchar(50)"`

	RouteID  string `gorm:"type:varchar(50)"`
	OutBound bool

	LanguageID string `gorm:"type:varchar(50)"`

	TemplateID string `gorm:"type:varchar(50)"`
	Payload    datatypes.JSONMap

	NotificationType string `gorm:"type:varchar(10)"`

	Message string `gorm:"type:text"`

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

func (model *Notification) ToApi(status *NotificationStatus, language *Language, message map[string]string) *notificationV1.Notification {

	extra := make(map[string]string)
	if model.IsReleased() {
		extra["ReleaseDate"] = model.ReleasedAt.String()
	}

	messageData := model.Message

	if len(message) > 0 {
		for key, val := range message {
			extra[key] = val

			if key == RouteTypeShortForm && model.Message == "" {
				messageData = val
			}
		}
	}

	notification := notificationV1.Notification{
		Id:          model.ID,
		Contact:     &notificationV1.Notification_ContactId{ContactId: model.ContactID},
		ProfileType: model.ProfileType,
		ProfileId:   model.ProfileID,
		Type:        model.NotificationType,
		Template:    model.TemplateID,
		Payload:     frame.DBPropertiesToMap(model.Payload),
		Data:        messageData,
		Language:    language.Code,
		OutBound:    model.OutBound,
		AutoRelease: model.IsReleased(),
		RouteId:     model.RouteID,
		Status:      status.ToStatusAPI(),
		Extras:      extra,
		Priority:    notificationV1.PRIORITY(model.Priority),
	}
	return &notification
}

// NotificationStatus table holding all the statuses of notifications in our system
type NotificationStatus struct {
	frame.BaseModel
	NotificationID string `gorm:"type:varchar(50)"`

	TransientID string `gorm:"type:varchar(50)"`
	ExternalID  string `gorm:"type:varchar(50)"`
	Extra       datatypes.JSONMap
	State       int32
	Status      int32
}

func (model *NotificationStatus) ToStatusAPI() *commonv1.StatusResponse {

	extra := frame.DBPropertiesToMap(model.Extra)
	extra["CreatedAt"] = model.CreatedAt.String()
	extra["StatusID"] = model.ID

	status := commonv1.StatusResponse{
		Id:          model.NotificationID,
		State:       commonv1.STATE(model.State),
		Status:      commonv1.STATUS(model.Status),
		TransientId: model.TransientID,
		ExternalId:  model.ExternalID,
		Extras:      extra,
	}

	return &status
}

// Route Our simple table holding all the payload of message in transit in and out of the system
type Route struct {
	frame.BaseModel

	CounterID   string `gorm:"type:varchar(50)"`
	Name        string `gorm:"type:varchar(50)"`
	Description string `gorm:"type:text"`
	RouteType   string `gorm:"type:varchar(10)"`
	Mode        string `gorm:"type:varchar(10)"`
	Uri         string `gorm:"type:varchar(255)"`
}
