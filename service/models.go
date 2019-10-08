package service

import (
	"time"

	"github.com/jinzhu/gorm"
	"github.com/rs/xid"
)

// AntMigration Our simple table holding all the migration data
type AntMigration struct {
	AntMigrationID string `gorm:"type:varchar(50);primary_key"`
	Name           string `gorm:"type:varchar(50);unique_index"`
	Patch          string `gorm:"type:text"`
	AppliedAt      *time.Time
	CreatedAt      time.Time
	ModifiedAt     time.Time
	Version        uint32 `gorm:"DEFAULT 0"`
}

// BeforeCreate Ensures we update a migrations time stamps
func (model *AntMigration) BeforeCreate(scope *gorm.Scope) error {
	scope.SetColumn("AntMigrationID", xid.New().String())
	scope.SetColumn("CreatedAt", time.Now())
	return scope.SetColumn("ModifiedAt", time.Now())
}

// BeforeUpdate Updates time stamp every time we update status of a migration
func (model *AntMigration) BeforeUpdate(scope *gorm.Scope) error {
	scope.SetColumn("Version", model.Version+1)
	return scope.SetColumn("ModifiedAt", time.Now())
}
