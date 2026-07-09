package main

import (
	"context"

	"buf.build/gen/go/antinvestor/notification/connectrpc/go/notification/v1/notificationv1connect"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	"buf.build/gen/go/antinvestor/tenancy/connectrpc/go/tenancy/v1/tenancyv1connect"
	apis "github.com/antinvestor/common/v2"
	"github.com/antinvestor/common/v2/connection"
	"github.com/antinvestor/common/v2/servicecatalog"
	"github.com/antinvestor/service-notification/apps/integrations/smpp/config"
	"github.com/antinvestor/service-notification/apps/integrations/smpp/service"
	"github.com/antinvestor/service-notification/apps/integrations/smpp/service/events"
	"github.com/antinvestor/service-notification/apps/integrations/smpp/service/models"
	"github.com/pitabwire/frame/v2"
	fconfig "github.com/pitabwire/frame/v2/config"
	"github.com/pitabwire/frame/v2/datastore"
	"github.com/pitabwire/util"
)

func main() {
	tmpCtx := context.Background()

	cfg, err := fconfig.LoadWithOIDC[config.TemplateConfig](tmpCtx)
	if err != nil {
		util.Log(tmpCtx).With("err", err).Error("could not process configs")
		return
	}

	if cfg.Name() == "" {
		cfg.ServiceName = "template_service"
	}

	ctx, svc := frame.NewServiceWithContext(
		tmpCtx,
		frame.WithConfig(&cfg),
		frame.WithDatastore(),
	)
	defer svc.Stop(ctx)

	log := util.Log(ctx)
	dbManager := svc.DatastoreManager()

	if cfg.DoDatabaseMigrate() {
		dbPool := dbManager.GetPool(ctx, datastore.DefaultMigrationPoolName)
		if dbPool == nil {
			log.Fatal("database pool is nil - check DATABASE_URL environment variable")
			return
		}
		err = dbManager.Migrate(ctx, dbPool, cfg.GetDatabaseMigrationPath(),
			models.Template{})
		if err != nil {
			log.WithError(err).Fatal("could not migrate successfully")
		}
		return
	}

	profileCli, err := setupProfileClient(ctx, cfg)
	if err != nil {
		log.WithError(err).Fatal("could not setup profile client")
	}

	tenancyCli, err := setupTenancyClient(ctx, cfg)
	if err != nil {
		log.WithError(err).Fatal("could not setup partition client")
	}

	notificationCli, err := setupNotificationClient(ctx, cfg)
	if err != nil {
		log.WithError(err).Fatal("could not setup notification client")
	}

	dbPool := dbManager.GetPool(ctx, datastore.DefaultPoolName)
	if dbPool == nil {
		log.Fatal("database pool is nil - check DATABASE_URL environment variable")
		return
	}

	authServiceHandlers := service.NewAuthRouterV1(svc, &cfg, profileCli, tenancyCli, notificationCli)

	serviceOptions := []frame.Option{
		frame.WithHTTPHandler(authServiceHandlers),
		frame.WithRegisterEvents(
			events.NewTemplateSave(ctx, dbPool),
		),
	}

	svc.Init(ctx, serviceOptions...)

	serverPort := cfg.Port()
	if serverPort == "" {
		serverPort = ":7020"
	}

	log.With("port", serverPort).Info("initiating server operations")
	err = svc.Run(ctx, serverPort)
	if err != nil {
		log.WithError(err).Error("could not run Server")
	}
}

func setupProfileClient(
	ctx context.Context,
	cfg config.TemplateConfig,
) (profilev1connect.ProfileServiceClient, error) {
	return connection.NewServiceClient(ctx, &cfg, apis.ServiceTarget{
		Endpoint:              cfg.ProfileServiceURI,
		WorkloadAPITargetPath: cfg.ProfileServiceWorkloadAPITargetPath,
		ServiceID:             servicecatalog.ServiceProfile,
	}, profilev1connect.NewProfileServiceClient)
}

func setupTenancyClient(
	ctx context.Context,
	cfg config.TemplateConfig,
) (tenancyv1connect.TenancyServiceClient, error) {
	return connection.NewServiceClient(ctx, &cfg, apis.ServiceTarget{
		Endpoint:              cfg.TenancyServiceURI,
		WorkloadAPITargetPath: cfg.TenancyServiceWorkloadAPITargetPath,
		ServiceID:             servicecatalog.ServiceTenancy,
	}, tenancyv1connect.NewTenancyServiceClient)
}

func setupNotificationClient(
	ctx context.Context,
	cfg config.TemplateConfig,
) (notificationv1connect.NotificationServiceClient, error) {
	return connection.NewServiceClient(ctx, &cfg, apis.ServiceTarget{
		Endpoint:              cfg.NotificationServiceURI,
		WorkloadAPITargetPath: cfg.NotificationServiceWorkloadAPITargetPath,
		ServiceID:             servicecatalog.ServiceNotification,
	}, notificationv1connect.NewNotificationServiceClient)
}
