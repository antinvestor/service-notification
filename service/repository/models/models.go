package models

import (
	"github.com/antinvestor/apis/common"
	napi "github.com/antinvestor/service-notification-api"
	"github.com/pitabwire/frame"
	"gorm.io/datatypes"
	"time"
)

const (
	ChannelModeTransmit   = "tx"
	ChannelModeReceive    = "rx"
	ChannelModeTransceive = "trx"

	ChannelTypeEmail = "email"
	ChannelTypeSms   = "sms"
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

	ChannelID string `gorm:"type:varchar(50)"`
	OutBound  bool

	LanguageID string `gorm:"type:varchar(50)"`

	TemplateID string `gorm:"type:varchar(50)"`
	Payload    datatypes.JSONMap

	Type string `gorm:"type:varchar(10)"`

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

		//State: model.ToStatusApi(),
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

// Channel Our simple table holding all the payload of message in transit in and out of the system
type Channel struct {
	frame.BaseModel

	CounterChannelID string `gorm:"type:varchar(50)"`
	Name             string `gorm:"type:varchar(50)"`
	Description      string `gorm:"type:text"`
	Type             string `gorm:"type:varchar(10)"`
	Mode             string `gorm:"type:varchar(10)"`
}
