package repository

import (
	"context"
	"errors"

	"github.com/antinvestor/service-notification/apps/ussd/service/models"
	"github.com/pitabwire/frame/v2/datastore"
)

func Migrate(ctx context.Context, dbManager datastore.Manager, migrationPath string) error {
	dbPool := dbManager.GetPool(ctx, datastore.DefaultMigrationPoolName)
	if dbPool == nil {
		return errors.New("datastore pool is not initialised")
	}

	return dbManager.Migrate(ctx, dbPool, migrationPath,
		&models.UssdMenu{},
		&models.UssdTranslation{},
		&models.UssdSession{},
		&models.UssdQuery{},
		&models.UssdServiceConfig{},
	)
}
