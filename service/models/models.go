package models

import (
	"github.com/antinvestor/apis/common"
	napi "github.com/antinvestor/service-notification-api"
	"github.com/pitabwire/frame"
	"gorm.io/datatypes"
	"time"
)

const (
	RouteModeTransmit = "tx"
	RouteModeReceive  = "rx"
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

	AccessID string `gorm:"type:varchar(50)"`

	ProfileID string `gorm:"type:varchar(50)"`
	ContactID string `gorm:"type:varchar(50)"`

	RouteID  string `gorm:"type:varchar(50)"`
	OutBound bool

	LanguageID string `gorm:"type:varchar(50)"`

	TemplateID string `gorm:"type:varchar(50)"`
	Payload    datatypes.JSONMap

	NotificationType string `gorm:"type:varchar(10)"`

	Message string `gorm:"type:text"`

	TransientID string `gorm:"type:varchar(50)"`
	ExternalID  string `gorm:"type:varchar(50)"`
	Extra       datatypes.JSONMap
	ReleasedAt  *time.Time
	State       int32
	Status      int32
}

func (model *Notification) IsReleased() bool {
	return model.ReleasedAt != nil && !model.ReleasedAt.IsZero()
}

func (model *Notification) ToNotificationApi()  *napi.Notification {
	notification := napi.Notification{
		ID: model.ID,
		AccessID: model.AccessID,
		ContactID: model.ContactID,
		Type: model.NotificationType,
		Templete: model.TemplateID,
		Payload: frame.DBPropertiesToMap(model.Payload),
		Data: model.Message,
		Language: model.LanguageID,
		OutBound: model.OutBound,
		AutoRelease: model.IsReleased(),
		RouteID: model.RouteID,
		Status: model.ToStatusApi(),
	}
	return &notification
}

func (model *Notification) ToStatusApi()  *napi.StatusResponse {

	releaseDate := ""
	if model.IsReleased() {
		releaseDate = model.ReleasedAt.String()
	}

	status := napi.StatusResponse{
		ID:          model.ID,
		State:       common.STATE(model.State),
		Status:      common.STATUS(model.Status),
		TransientID: model.TransientID,
		ExternalID:  model.ExternalID,
		Extras:      frame.DBPropertiesToMap(model.Extra),
		ReleaseDate: releaseDate,
	}

	return &status
}

// Route Our simple table holding all the payload of message in transit in and out of the system
type Route struct {
	frame.BaseModel

	CounterID string `gorm:"type:varchar(50)"`
	Name      string `gorm:"type:varchar(50)"`
	Description      string `gorm:"type:text"`
	RouteType             string `gorm:"type:varchar(10)"`
	Mode             string `gorm:"type:varchar(10)"`
	Uri             string `gorm:"type:varchar(255)"`
}
