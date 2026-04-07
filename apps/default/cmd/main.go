package main

import (
	"context"
	_ "embed"
	"net/http"

	"buf.build/gen/go/antinvestor/notification/connectrpc/go/notification/v1/notificationv1connect"
	notificationpb "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	"buf.build/gen/go/antinvestor/tenancy/connectrpc/go/tenancy/v1/tenancyv1connect"
	"connectrpc.com/connect"
	apis "github.com/antinvestor/common"
	"github.com/antinvestor/common/connection"
	"github.com/antinvestor/common/permissions"
	aconfig "github.com/antinvestor/service-notification/apps/default/config"
	"github.com/antinvestor/service-notification/apps/default/service/authz"
	"github.com/antinvestor/service-notification/apps/default/service/business"
	events2 "github.com/antinvestor/service-notification/apps/default/service/events"
	"github.com/antinvestor/service-notification/apps/default/service/handlers"
	"github.com/antinvestor/service-notification/apps/default/service/repository"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/config"
	"github.com/pitabwire/frame/datastore"
	"github.com/pitabwire/frame/security"
	"github.com/pitabwire/frame/security/authorizer"
	connectInterceptors "github.com/pitabwire/frame/security/interceptors/connect"
	"github.com/pitabwire/frame/workerpool"
	"github.com/pitabwire/util"
)

//go:embed spec/notification.openapi.yaml
var notificationAPISpecFile []byte

func main() {
	tmpCtx := context.Background()

	// Initialise configuration
	cfg, err := config.LoadWithOIDC[aconfig.NotificationConfig](tmpCtx)
	if err != nil {
		util.Log(tmpCtx).With("err", err).Error("could not process configs")
		return
	}

	if cfg.Name() == "" {
		cfg.ServiceName = "service_notification"
	}

	// Create service
	ctx, svc := frame.NewServiceWithContext(
		tmpCtx,
		frame.WithConfig(&cfg),
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
	profileCli, err := setupProfileClient(ctx, cfg)
	if err != nil {
		log.WithError(err).Fatal("main -- Could not setup profile client")
	}

	tenancyCli, err := setupTenancyClient(ctx, cfg)
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
	notificationBusiness := business.NewNotificationBusiness(ctx, workMan, evtsMan, profileCli, tenancyCli,
		notificationRepo, notificationStatusRepo, languageRepo, templateRepo, templateDataRepo, routeRepo)

	// Setup Connect server
	connectHandler := setupConnectServer(ctx, sm, workMan, notificationBusiness)

	// Register permission manifest for the notification service namespace.
	notificationSD := notificationpb.File_notification_v1_notification_proto.Services().ByName("NotificationService")

	// Initialise the service with all options
	serviceOptions := []frame.Option{
		frame.WithHTTPHandler(connectHandler),
		frame.WithPermissionRegistration(notificationSD),
		frame.WithRegisterEvents(
			events2.NewNotificationSave(ctx, evtsMan, notificationRepo),
			events2.NewNotificationStatusSave(ctx, notificationRepo, notificationStatusRepo),
			events2.NewNotificationInRoute(ctx, qMan, evtsMan, notificationRepo, routeRepo),
			events2.NewNotificationInQueue(ctx, qMan, evtsMan, notificationRepo, routeRepo, profileCli),
			events2.NewNotificationOutRoute(ctx, evtsMan, profileCli, notificationRepo, routeRepo),
			events2.NewNotificationOutQueue(ctx, qMan, evtsMan, profileCli, tenancyCli,
				notificationRepo, notificationStatusRepo, languageRepo, templateDataRepo, routeRepo)),
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
	cfg aconfig.NotificationConfig) (profilev1connect.ProfileServiceClient, error) {
	return connection.NewServiceClient(ctx, &cfg, apis.ServiceTarget{
		Endpoint:              cfg.ProfileServiceURI,
		WorkloadAPITargetPath: cfg.ProfileServiceWorkloadAPITargetPath,
		Audiences:             []string{"service_profile"},
	}, profilev1connect.NewProfileServiceClient)
}

// setupTenancyClient creates and configures the partition client.
func setupTenancyClient(
	ctx context.Context,
	cfg aconfig.NotificationConfig) (tenancyv1connect.TenancyServiceClient, error) {
	return connection.NewServiceClient(ctx, &cfg, apis.ServiceTarget{
		Endpoint:              cfg.TenancyServiceURI,
		WorkloadAPITargetPath: cfg.TenancyServiceWorkloadAPITargetPath,
		Audiences:             []string{"service_tenancy"},
	}, tenancyv1connect.NewTenancyServiceClient)
}

// setupConnectServer initialises and configures the Connect RPC server.
func setupConnectServer(ctx context.Context, sm security.Manager, workMan workerpool.Manager, notificationBusiness business.NotificationBusiness) http.Handler {

	// Create handler with injected dependencies
	implementation := handlers.NewNotificationServer(workMan, notificationBusiness)

	auth := sm.GetAuthorizer(ctx)

	// Layer 1: TenancyAccessChecker verifies caller can access the partition.
	tenancyAccessChecker := authorizer.NewTenancyAccessChecker(auth, authz.NamespaceTenancyAccess)
	tenancyAccessInterceptor := connectInterceptors.NewTenancyAccessInterceptor(tenancyAccessChecker)

	// Layer 2: FunctionAccessInterceptor enforces per-RPC permissions from proto annotations.
	sd := notificationpb.File_notification_v1_notification_proto.Services().ByName("NotificationService")
	procMap := permissions.BuildProcedureMap(sd)
	functionChecker := authorizer.NewFunctionChecker(auth, permissions.ForService(sd).Namespace)
	functionAccessInterceptor := connectInterceptors.NewFunctionAccessInterceptor(functionChecker, procMap)

	defaultInterceptorList, err := connectInterceptors.DefaultList(
		ctx, sm.GetAuthenticator(ctx), tenancyAccessInterceptor, functionAccessInterceptor)
	if err != nil {
		util.Log(ctx).WithError(err).Fatal("main -- Could not create default interceptors")
	}

	_, serverHandler := notificationv1connect.NewNotificationServiceHandler(
		implementation, connect.WithInterceptors(defaultInterceptorList...))

	mux := http.NewServeMux()
	mux.Handle("/", serverHandler)
	mux.Handle("/openapi.yaml", apis.NewOpenAPIHandler(notificationAPISpecFile, nil))

	return mux
}
