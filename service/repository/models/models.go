package models

import (
	"github.com/jinzhu/gorm/dialects/postgres"
	"time"

	"github.com/jinzhu/gorm"
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
	AntBaseModel

	TempleteID string `gorm:"type:varchar(50);primary_key"`
	LanguageID string `gorm:"type:varchar(50)"`
	Name       string `gorm:"type:varchar(255)"`

	DataList []TempleteData
}

// BeforeCreate Ensures we update a migrations time stamps
func (model *Templete) BeforeCreate(scope *gorm.Scope) error {

	if err := model.AntBaseModel.BeforeCreate(scope); err != nil {
		return err
	}

	return scope.SetColumn("TempleteID", model.IDGen("tmp"))
}

type TempleteData struct {
	AntBaseModel
	TempleteDataID string `gorm:"type:varchar(50);primary_key"`
	TempleteID     string `gorm:"type:varchar(50);unique_index:uq_template_by_type"`
	Type           string `gorm:"type:varchar(10);unique_index:uq_template_by_type"`
	Detail         string `gorm:"type:text"`
}

// BeforeCreate Ensures we update a migrations time stamps
func (model *TempleteData) BeforeCreate(scope *gorm.Scope) error {

	if err := model.AntBaseModel.BeforeCreate(scope); err != nil {
		return err
	}

	return scope.SetColumn("TempleteDataID", model.IDGen("tmd"))
}

// Language Our simple table holding all the supported languages
type Language struct {
	AntBaseModel

	LanguageID  string `gorm:"type:varchar(50);primary_key"`
	Name        string `gorm:"type:varchar(50);unique_index"`
	Code        string `gorm:"type:varchar(10);unique_index"`
	Description string `gorm:"type:text"`
}

// BeforeCreate Ensures we update a migrations time stamps
func (model *Language) BeforeCreate(scope *gorm.Scope) error {
	if err := model.AntBaseModel.BeforeCreate(scope); err != nil {
		return err
	}
	return scope.SetColumn("LanguageID", model.IDGen("ln"))
}

// Notification table holding all the payload of message in transit in and out of the system
type Notification struct {
	AntBaseModel

	NotificationID string `gorm:"type:varchar(50);primary_key"`

	ProfileID string `gorm:"type:varchar(50)"`
	ContactID string `gorm:"type:varchar(50)"`

	ProductID string `gorm:"type:varchar(50)"`
	ChannelID string `gorm:"type:varchar(50)"`
	OutBound  bool

	LanguageID string `gorm:"type:varchar(50)"`

	TemplateID string `gorm:"type:varchar(50)"`
	Payload    postgres.Jsonb

	Type string `gorm:"type:varchar(10)"`

	Message string `gorm:"type:text"`

	TransientID string `gorm:"type:varchar(50)"`
	ExternalID  string `gorm:"type:varchar(50)"`
	Extra       string `gorm:"type:text"`
	ReleasedAt  *time.Time
	State       string `gorm:"type:varchar(10)"`
}

// BeforeCreate Ensures we update a migrations time stamps
func (model *Notification) BeforeCreate(scope *gorm.Scope) error {
	if err := model.AntBaseModel.BeforeCreate(scope); err != nil {
		return err
	}
	return scope.SetColumn("NotificationID", model.IDGen("nt"))
}

func (model *Notification) IsReleased() bool {
	return model.ReleasedAt != nil && !model.ReleasedAt.IsZero()
}

// Channel Our simple table holding all the payload of message in transit in and out of the system
type Channel struct {
	AntBaseModel

	ChannelID        string `gorm:"type:varchar(50);primary_key"`
	CounterChannelID string `gorm:"type:varchar(50)"`
	ProductID        string `gorm:"type:varchar(50)"`
	Name             string `gorm:"type:varchar(50)"`
	Description      string `gorm:"type:text"`
	Type             string `gorm:"type:varchar(10)"`
	Mode             string `gorm:"type:varchar(10)"`
}

// BeforeCreate Ensures we update a migrations time stamps
func (model *Channel) BeforeCreate(scope *gorm.Scope) error {
	if err := model.AntBaseModel.BeforeCreate(scope); err != nil {
		return err
	}
	return scope.SetColumn("ChannelID", model.IDGen("ch"))
}
