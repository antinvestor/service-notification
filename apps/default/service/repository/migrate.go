package repository

import (
	"context"

	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/pitabwire/frame"
)

func Migrate(ctx context.Context, svc *frame.Service, migrationPath string) error {
	return svc.MigrateDatastore(ctx, migrationPath,
		&models.Route{}, &models.Language{}, &models.Template{},
		&models.TemplateData{}, &models.Notification{}, &models.NotificationStatus{})
}
