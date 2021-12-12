package models

import (
	"time"

	"github.com/antinvestor/apis/common"
	notificationV1 "github.com/antinvestor/service-notification-api"
	"github.com/pitabwire/frame"
	"gorm.io/datatypes"
)

const (
	RouteModeTransmit   = "tx"
	RouteModeReceive    = "rx"
	RouteModeTransceive = "trx"

	RouteTypeEmail = "email"
	RouteTypeSms   = "sms"
)

// Templete Table holds the templete details
type Templete struct {
	frame.BaseModel

	LanguageID string `gorm:"type:varchar(50)"`
	Name       string `gorm:"type:varchar(255)"`

	DataList []TempleteData
}

type TempleteData struct {
	frame.BaseModel

	TempleteID string `gorm:"type:varchar(50);unique_index:uq_template_by_type"`
	Type       string `gorm:"type:varchar(10);unique_index:uq_template_by_type"`
	Detail     string `gorm:"type:text"`
}

// Language Our simple table holding all the supported languages
type Language struct {
	frame.BaseModel

	Name        string `gorm:"type:varchar(50);unique_index"`
	Code        string `gorm:"type:varchar(10);unique_index"`
	Description string `gorm:"type:text"`
}

// Notification table holding all the payload of message in transit in and out of the system
type Notification struct {
	frame.BaseModel

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
}

func (model *Notification) IsReleased() bool {
	return model.ReleasedAt != nil && !model.ReleasedAt.IsZero()
}

func (model *Notification) ToNotificationApi(status *NotificationStatus) *notificationV1.Notification {

	extra := make(map[string]string)
	if model.IsReleased() {
		extra["ReleaseDate"] = model.ReleasedAt.String()
	}

	notification := notificationV1.Notification{
		ID:          model.ID,
		AccessID:    model.AccessID,
		ContactID:   model.ContactID,
		Type:        model.NotificationType,
		Templete:    model.TemplateID,
		Payload:     frame.DBPropertiesToMap(model.Payload),
		Data:        model.Message,
		Language:    model.LanguageID,
		OutBound:    model.OutBound,
		AutoRelease: model.IsReleased(),
		RouteID:     model.RouteID,
		Status:      status.ToStatusApi(),
		Extras:      extra,
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

func (model *NotificationStatus) ToStatusApi() *notificationV1.StatusResponse {

	extra := frame.DBPropertiesToMap(model.Extra)
	extra["CreatedAt"] = model.CreatedAt.String()
	extra["StatusID"] = model.ID

	status := notificationV1.StatusResponse{
		ID:          model.NotificationID,
		State:       common.STATE(model.State),
		Status:      common.STATUS(model.Status),
		TransientID: model.TransientID,
		ExternalID:  model.ExternalID,
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
