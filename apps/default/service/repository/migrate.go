package repository

import (
	"context"

	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/pitabwire/frame/datastore"
)

func Migrate(ctx context.Context, dbManager datastore.Manager, migrationPath string) error {
	dbPool := dbManager.GetPool(ctx, datastore.DefaultMigrationPoolName)

	return dbManager.Migrate(ctx, dbPool, migrationPath,
		&models.Route{}, &models.Language{}, &models.Template{},
		&models.TemplateData{}, &models.Notification{}, &models.NotificationStatus{})
}
