package main

import (
	"context"
	"net/http"

	"buf.build/gen/go/antinvestor/notification/connectrpc/go/notification/v1/notificationv1connect"
	"buf.build/gen/go/antinvestor/partition/connectrpc/go/partition/v1/partitionv1connect"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	"connectrpc.com/connect"
	apis "github.com/antinvestor/apis/go/common"
	"github.com/antinvestor/apis/go/partition"
	"github.com/antinvestor/apis/go/profile"
	aconfig "github.com/antinvestor/service-notification/apps/default/config"
	"github.com/antinvestor/service-notification/apps/default/service/business"
	events2 "github.com/antinvestor/service-notification/apps/default/service/events"
	"github.com/antinvestor/service-notification/apps/default/service/handlers"
	"github.com/antinvestor/service-notification/apps/default/service/repository"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/config"
	"github.com/pitabwire/frame/datastore"
	"github.com/pitabwire/frame/security"
	connectInterceptors "github.com/pitabwire/frame/security/interceptors/connect"
	"github.com/pitabwire/frame/security/openid"
	"github.com/pitabwire/frame/workerpool"
	"github.com/pitabwire/util"
)

func main() {
	tmpCtx := context.Background()

	// Initialise configuration
	cfg, err := config.LoadWithOIDC[aconfig.NotificationConfig](tmpCtx)
	if err != nil {
		util.Log(tmpCtx).With("err", err).Error("could not process configs")
		return
	}

	if cfg.Name() == "" {
		cfg.ServiceName = "service_notifications"
	}

	// Create service
	ctx, svc := frame.NewServiceWithContext(
		tmpCtx,
		frame.WithConfig(&cfg),
		frame.WithRegisterServerOauth2Client(),
		frame.WithDatastore(),
	)
	defer svc.Stop(ctx)
	log := util.Log(ctx)

	sm := svc.SecurityManager()
	dbManager := svc.DatastoreManager()
	workMan := svc.WorkManager()
	evtsMan := svc.EventsManager()
	qMan := svc.QueueManager()

	// Handle database migration if requested
	if handleDatabaseMigration(ctx, dbManager, cfg) {
		return
	}

	// Setup clients
	profileCli, err := setupProfileClient(ctx, sm, cfg)
	if err != nil {
		log.WithError(err).Fatal("main -- Could not setup profile client")
	}

	partitionCli, err := setupPartitionClient(ctx, sm, cfg)
	if err != nil {
		log.WithError(err).Fatal("main -- Could not setup partition client")
	}

	// Get database pool
	dbPool := dbManager.GetPool(ctx, datastore.DefaultPoolName)
	if dbPool == nil {
		log.Fatal("Database pool is nil - check DATABASE_PRIMARY_URL environment variable")
	}

	// Initialise repositories
	notificationRepo := repository.NewNotificationRepository(ctx, dbPool, workMan)
	notificationStatusRepo := repository.NewNotificationStatusRepository(ctx, dbPool, workMan)
	languageRepo := repository.NewLanguageRepository(ctx, dbPool, workMan)
	templateRepo := repository.NewTemplateRepository(ctx, dbPool, workMan)
	templateDataRepo := repository.NewTemplateDataRepository(ctx, dbPool, workMan)
	routeRepo := repository.NewRouteRepository(ctx, dbPool, workMan)

	// Create business logic with all dependencies
	notificationBusiness := business.NewNotificationBusiness(ctx, workMan, evtsMan, profileCli, partitionCli,
		notificationRepo, notificationStatusRepo, languageRepo, templateRepo, templateDataRepo, routeRepo)

	// Setup Connect server
	connectHandler := setupConnectServer(ctx, sm, workMan, notificationBusiness)

	// Initialise the service with all options
	serviceOptions := []frame.Option{
		frame.WithHTTPHandler(connectHandler),
		frame.WithRegisterEvents(
			events2.NewNotificationSave(ctx, evtsMan, notificationRepo),
			events2.NewNotificationStatusSave(ctx, notificationRepo, notificationStatusRepo),
			events2.NewNotificationInRoute(ctx, qMan, evtsMan, notificationRepo, routeRepo),
			events2.NewNotificationInQueue(ctx, qMan, evtsMan, notificationRepo, routeRepo, profileCli),
			events2.NewNotificationOutRoute(ctx, evtsMan, profileCli, notificationRepo, routeRepo),
			events2.NewNotificationOutQueue(ctx, qMan, evtsMan, profileCli, notificationRepo, notificationStatusRepo, languageRepo, templateDataRepo, routeRepo)),
	}

	svc.Init(ctx, serviceOptions...)

	// Start the service
	err = svc.Run(ctx, "")
	if err != nil {
		log.WithError(err).Fatal("could not run Server")
	}
}

// handleDatabaseMigration performs database migration if configured to do so.
func handleDatabaseMigration(
	ctx context.Context,
	dbManager datastore.Manager,
	cfg aconfig.NotificationConfig,
) bool {

	if cfg.DoDatabaseMigrate() {

		err := repository.Migrate(ctx, dbManager, cfg.GetDatabaseMigrationPath())
		if err != nil {
			util.Log(ctx).WithError(err).Fatal("main -- Could not migrate successfully")
		}
		return true
	}
	return false
}

// setupProfileClient creates and configures the profile client.
func setupProfileClient(
	ctx context.Context,
	clHolder security.InternalOauth2ClientHolder,
	cfg aconfig.NotificationConfig) (profilev1connect.ProfileServiceClient, error) {
	return profile.NewClient(ctx,
		apis.WithEndpoint(cfg.ProfileServiceURI),
		apis.WithTokenEndpoint(cfg.GetOauth2TokenEndpoint()),
		apis.WithTokenUsername(clHolder.JwtClientID()),
		apis.WithTokenPassword(clHolder.JwtClientSecret()),
		apis.WithScopes(openid.ConstSystemScopeInternal),
		apis.WithAudiences("service_profile"))
}

// setupPartitionClient creates and configures the partition client.
func setupPartitionClient(
	ctx context.Context,
	clHolder security.InternalOauth2ClientHolder,
	cfg aconfig.NotificationConfig) (partitionv1connect.PartitionServiceClient, error) {
	return partition.NewClient(ctx,
		apis.WithEndpoint(cfg.PartitionServiceURI),
		apis.WithTokenEndpoint(cfg.GetOauth2TokenEndpoint()),
		apis.WithTokenUsername(clHolder.JwtClientID()),
		apis.WithTokenPassword(clHolder.JwtClientSecret()),
		apis.WithScopes(openid.ConstSystemScopeInternal),
		apis.WithAudiences("service_partition"))
}

// setupConnectServer initialises and configures the Connect RPC server.
func setupConnectServer(ctx context.Context, sm security.Manager, workMan workerpool.Manager, notificationBusiness business.NotificationBusiness) http.Handler {

	// Create handler with injected dependencies
	implementation := handlers.NewNotificationServer(workMan, notificationBusiness)

	defaultInterceptorList, err := connectInterceptors.DefaultList(ctx, sm.GetAuthenticator(ctx))
	if err != nil {
		util.Log(ctx).WithError(err).Fatal("main -- Could not create default interceptors")
	}

	_, serverHandler := notificationv1connect.NewNotificationServiceHandler(
		implementation, connect.WithInterceptors(defaultInterceptorList...))

	return serverHandler
}
