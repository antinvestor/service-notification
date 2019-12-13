package utils

import (
	"context"
	"github.com/InVisionApp/go-health/v2"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
	otgorm "github.com/smacker/opentracing-gorm"
)

// Env Context object supplied around the applications lifetime
type Env struct {
	wDb              *gorm.DB
	rDb              *gorm.DB
	Logger          *logrus.Entry
	Health   		*health.Health

}

func (env *Env) SetWriteDb(db *gorm.DB) {
	env.wDb = db
}


func (env *Env) SetReadDb(db *gorm.DB) {
	env.rDb = db
}

func (env *Env) GeWtDb(ctx context.Context) *gorm.DB{
	return otgorm.SetSpanToGorm(ctx, env.wDb)
}


func (env *Env) GetRDb(ctx context.Context) *gorm.DB{
	return otgorm.SetSpanToGorm(ctx, env.rDb)
}

