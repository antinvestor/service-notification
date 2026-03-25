package main

import (
	"context"

	"buf.build/gen/go/antinvestor/notification/connectrpc/go/notification/v1/notificationv1connect"
	"buf.build/gen/go/antinvestor/partition/connectrpc/go/partition/v1/partitionv1connect"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	apis "github.com/antinvestor/common"
	"github.com/antinvestor/common/connection"
	"github.com/antinvestor/service-notification/apps/integrations/smpp/config"
	"github.com/antinvestor/service-notification/apps/integrations/smpp/service"
	"github.com/antinvestor/service-notification/apps/integrations/smpp/service/events"
	"github.com/antinvestor/service-notification/apps/integrations/smpp/service/models"
	"github.com/pitabwire/frame"
	fconfig "github.com/pitabwire/frame/config"
	"github.com/pitabwire/frame/datastore"
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

	audienceList := cfg.GetOauth2ServiceAudience()

	profileCli, err := setupProfileClient(ctx, cfg, audienceList)
	if err != nil {
		log.WithError(err).Fatal("could not setup profile client")
	}

	partitionCli, err := setupPartitionClient(ctx, cfg, audienceList)
	if err != nil {
		log.WithError(err).Fatal("could not setup partition client")
	}

	notificationCli, err := setupNotificationClient(ctx, cfg, audienceList)
	if err != nil {
		log.WithError(err).Fatal("could not setup notification client")
	}

	dbPool := dbManager.GetPool(ctx, datastore.DefaultPoolName)
	if dbPool == nil {
		log.Fatal("database pool is nil - check DATABASE_URL environment variable")
		return
	}

	authServiceHandlers := service.NewAuthRouterV1(svc, &cfg, profileCli, partitionCli, notificationCli)

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
	audiences []string,
) (profilev1connect.ProfileServiceClient, error) {
	return connection.NewServiceClient(ctx, &cfg, apis.ServiceTarget{
		Endpoint:              cfg.ProfileServiceURI,
		WorkloadAPITargetPath: cfg.ProfileServiceWorkloadAPITargetPath,
		Audiences:             audiences,
	}, profilev1connect.NewProfileServiceClient)
}

func setupPartitionClient(
	ctx context.Context,
	cfg config.TemplateConfig,
	audiences []string,
) (partitionv1connect.PartitionServiceClient, error) {
	return connection.NewServiceClient(ctx, &cfg, apis.ServiceTarget{
		Endpoint:              cfg.PartitionServiceURI,
		WorkloadAPITargetPath: cfg.PartitionServiceWorkloadAPITargetPath,
		Audiences:             audiences,
	}, partitionv1connect.NewPartitionServiceClient)
}

func setupNotificationClient(
	ctx context.Context,
	cfg config.TemplateConfig,
	audiences []string,
) (notificationv1connect.NotificationServiceClient, error) {
	return connection.NewServiceClient(ctx, &cfg, apis.ServiceTarget{
		Endpoint:              cfg.NotificationServiceURI,
		WorkloadAPITargetPath: cfg.NotificationServiceWorkloadAPITargetPath,
		Audiences:             audiences,
	}, notificationv1connect.NewNotificationServiceClient)
}
