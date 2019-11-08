package service

import (
	"fmt"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/rs/xid"
)

// AntBaseModel Our simple table holding all the migration data
type AntBaseModel struct {
	CreatedAt  time.Time
	ModifiedAt time.Time
	Version    uint32 `gorm:"DEFAULT 0"`
	DeletedAt  *time.Time
}
//IDGen genereate unigue id format
func (model *AntBaseModel) IDGen(uniqueCode string) string {
	return fmt.Sprintf("%s_%s", uniqueCode, xid.New().String())
}
// BeforeCreate Ensures we update a migrations time stamps
func (model *AntBaseModel) BeforeCreate(scope *gorm.Scope) error {
	
		if err := scope.SetColumn("CreatedAt", time.Now()); err != nil{
			return err
		}
		if err := scope.SetColumn("ModifiedAt", time.Now()); err != nil{
			return err
		}
		return scope.SetColumn("Version", 1)
	 }

// BeforeUpdate Updates time stamp every time we update status of a migration
func (model *AntBaseModel) BeforeUpdate(scope *gorm.Scope) error {
		if err := scope.SetColumn("Version", model.Version+1); err != nil{
			return err
		}
		 return scope.SetColumn("ModifiedAt", time.Now())
	 }

	 // AntMigration Our simple table holding all the migration data
type AntMigration struct {
		AntBaseModel
	
		AntMigrationID string `gorm:"type:varchar(50);primary_key"`
		Name           string `gorm:"type:varchar(50);unique_index"`
		Patch          string `gorm:"type:text"`
		AppliedAt      *time.Time
	}
	
	// BeforeCreate Ensures we update a migrations time stamps
func (model *AntMigration) BeforeCreate(scope *gorm.Scope) error {
	
		if err := model.AntBaseModel.BeforeCreate(scope); err != nil{
			return err
		}
		return scope.SetColumn("AntMigrationID", model.IDGen("mg"))
	}
	
	
// MessageTemplete Our simple table holding all the templete details
type MessageTemplete struct {

	AntBaseModel

	MessageTempleteID string `gorm:"type:varchar(50);primary_key"`
	LanguageID          string `gorm:"type:text"`
	TempleteName      string `gorm:"type:varchar(50);unique_index"`
	TempleteValue     string `gorm:"type:text"`
	AppliedAt         *time.Time

}

// BeforeCreate Ensures we update a migrations time stamps
func (model *MessageTemplete) BeforeCreate(scope *gorm.Scope) error {

	if err := model.AntBaseModel.BeforeCreate(scope); err != nil{
		return err
	}	
	 
	return  scope.SetColumn("MessageTempleteID",  model.IDGen("mt"))
}

// Language Our simple table holding all the supported language and details
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
	if err := model.AntBaseModel.BeforeCreate(scope); err != nil{
		return err
	}
	return scope.SetColumn("LanguageID", model.IDGen("Lg"))
}

 

// Notification Our simple table holding all the payload of message in transit in and out of the system
type Notification struct {

	AntBaseModel

	NotificationID   string `gorm:"type:varchar(50);primary_key"`
	ProfileID        string `gorm:"type:text"`
	ChannelsID          string `gorm:"type:text"`
	LanguageID       string `gorm:"type:text"`
	ProductID        string `gorm:"type:text"`
	Autosend         string `gorm:"type:text"`
	Status           string `gorm:"type:text"`
	Messagetype      string `gorm:"type:text"`
	Messagevariables string `gorm:"type:text"`
	Payload          string `sql:"type:jsonb"`
	AppliedAt        *time.Time
	 
}

// BeforeCreate Ensures we update a migrations time stamps
func (model *Notification) BeforeCreate(scope *gorm.Scope) error {
	if err := model.AntBaseModel.BeforeCreate(scope); err != nil{
		return err
	}
	return nil //scope.SetColumn("NotificationID", model.IDGen("Lg"))
}

 

// Channels Our simple table holding all the payload of message in transit in and out of the system
type Channels struct {

	AntBaseModel

	ChannelsID  string `gorm:"type:varchar(50);primary_key"`
	Channel     string `gorm:"type:text"`
	Description string `gorm:"type:text"`
	AppliedAt   *time.Time
	 
}

// BeforeCreate Ensures we update a migrations time stamps
func (model *Channels) BeforeCreate(scope *gorm.Scope) error {
	if err := model.AntBaseModel.BeforeCreate(scope); err != nil{
		return err
	}
	return scope.SetColumn("ChannelsID", model.IDGen("ch"))
}

 
