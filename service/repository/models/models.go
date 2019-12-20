package models

import (
	"github.com/jinzhu/gorm/dialects/postgres"
	"github.com/qor/transition"
	"time"

	"github.com/jinzhu/gorm"
)

// MessageTemplete Our simple table holding all the templete details
type MessageTemplete struct {
	AntBaseModel

	MessageTempleteID string `gorm:"type:varchar(50);primary_key"`
	LanguageID        string `gorm:"type:text"`
	TempleteName      string `gorm:"type:varchar(50);unique_index"`
	TempleteValue     string `gorm:"type:text"`
	AppliedAt         *time.Time
}

// BeforeCreate Ensures we update a migrations time stamps
func (model *MessageTemplete) BeforeCreate(scope *gorm.Scope) error {

	if err := model.AntBaseModel.BeforeCreate(scope); err != nil {
		return err
	}

	return scope.SetColumn("MessageTempleteID", model.IDGen("mt"))
}

// Language Our simple table holding all the supported languages
type Language struct {
	AntBaseModel

	LanguageID  string `gorm:"type:varchar(50);primary_key"`
	Name        string `gorm:"type:varchar(50);unique_index"`
	Description string `gorm:"type:text"`
	Region      string `gorm:"type:text"`
	AppliedAt   *time.Time
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

	ProfileID      string `gorm:"type:varchar(50)"`
	ContactID      string `gorm:"type:varchar(50)"`

	ProductID      string `gorm:"type:varchar(50)"`
	ChannelID      string `gorm:"type:varchar(50)"`
	OutBound bool

	LanguageID     string `gorm:"type:varchar(50)"`
	Type     string `gorm:"type:varchar(10)"`
	Template string `gorm:"type:varchar(10)"`
	Payload  postgres.Jsonb

	Message string `gorm:"type:text"`

	TransientID string `gorm:"type:varchar(50)"`
	ExternalID  string `gorm:"type:varchar(50)"`
	Extra      string `gorm:"type:text"`
	Status      string `gorm:"type:varchar(10)"`
	ReleasedAt  *time.Time
	transition.Transition
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

	ChannelID   string `gorm:"type:varchar(50);primary_key"`
	Channel     string `gorm:"type:text"`
	Description string `gorm:"type:text"`
	AppliedAt   *time.Time
}

// BeforeCreate Ensures we update a migrations time stamps
func (model *Channel) BeforeCreate(scope *gorm.Scope) error {
	if err := model.AntBaseModel.BeforeCreate(scope); err != nil {
		return err
	}
	return scope.SetColumn("ChannelID", model.IDGen("ch"))
}
