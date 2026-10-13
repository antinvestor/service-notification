package main

import (
	"context"
	_ "embed"
	"net/http"

	"buf.build/gen/go/antinvestor/notification/connectrpc/go/v2/notification/v1/notificationv1connect"
	notificationpb "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/v2/profile/v1/profilev1connect"
	"buf.build/gen/go/antinvestor/tenancy/connectrpc/go/v2/tenancy/v1/tenancyv1connect"
	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	apis "github.com/antinvestor/common/v3"
	"github.com/antinvestor/common/v3/connection"
	"github.com/antinvestor/common/v3/permissions"
	"github.com/antinvestor/common/v3/servicecatalog"
	aconfig "github.com/antinvestor/service-notification/apps/default/config"
	"github.com/antinvestor/service-notification/apps/default/service/authz"
	"github.com/antinvestor/service-notification/apps/default/service/business"
	events2 "github.com/antinvestor/service-notification/apps/default/service/events"
	"github.com/antinvestor/service-notification/apps/default/service/handlers"
	"github.com/antinvestor/service-notification/apps/default/service/repository"
	"github.com/pitabwire/frame/v3"
	"github.com/pitabwire/frame/v3/config"
	"github.com/pitabwire/frame/v3/datastore"
	"github.com/pitabwire/frame/v3/security"
	"github.com/pitabwire/frame/v3/security/authorizer"
	connectInterceptors "github.com/pitabwire/frame/v3/security/interceptors/connect"
	"github.com/pitabwire/frame/v3/setup"
	"github.com/pitabwire/frame/v3/workerpool"
	"github.com/pitabwire/util"
)

// maxRequestMessageBytes caps each received Connect message.
const maxRequestMessageBytes = 16 << 20

//go:embed spec/notification.openapi.yaml
var notificationAPISpecFile []byte

func main() {
	tmpCtx := context.Background()

	// Migration/setup jobs must not require OIDC or peer service clients.
	cfg, err := config.FromEnv[aconfig.NotificationConfig]()
	if err != nil {
		util.Log(tmpCtx).WithError(err).Fatal("could not process configs")
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

	svc.Setup().RegisterFunc(setup.NameMigrate, func(ctx context.Context) error {
		return repository.Migrate(ctx, svc.DatastoreManager(), cfg.GetDatabaseMigrationPath())
	})
	defer svc.Stop(ctx)
	log := util.Log(ctx)

	// Register permission manifest for the notification service namespace.
	notificationSD := notificationpb.File_notification_v1_notification_proto.Services().ByName("NotificationService")

	if frame.ShouldRunSetup(&cfg) {
		svc.Init(ctx, frame.WithPermissionRegistration(notificationSD))
		if setupErr := svc.RunSetupForProcess(ctx, &cfg); setupErr != nil {
			log.WithError(setupErr).Fatal("setup plan failed")
		}
		log.Info("setup plan complete — exiting")
		return
	}

	if err = cfg.LoadOauth2Config(ctx); err != nil {
		log.WithError(err).Fatal("could not load oauth2/oidc config")
		return
	}

	sm := svc.SecurityManager()
	dbManager := svc.DatastoreManager()
	workMan := svc.WorkManager()
	evtsMan := svc.EventsManager()
	qMan := svc.QueueManager()

	// Setup clients (runtime only)
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

	// Runtime only — permission manifests publish on the setup Job path above.
	serviceOptions := []frame.Option{
		frame.WithHTTPHandler(connectHandler),
		frame.WithRegisterEvents(
			events2.NewNotificationSave(ctx, evtsMan, notificationRepo),
			events2.NewNotificationStatusSave(ctx, evtsMan, notificationRepo, notificationStatusRepo),
			events2.NewNotificationInRoute(ctx, qMan, evtsMan, notificationRepo, routeRepo),
			events2.NewNotificationInQueue(ctx, qMan, evtsMan, notificationRepo, languageRepo, routeRepo, profileCli),
			events2.NewNotificationOutRoute(ctx, evtsMan, profileCli, notificationRepo, routeRepo),
			events2.NewNotificationOutQueue(ctx, qMan, evtsMan, profileCli, tenancyCli,
				notificationRepo, notificationStatusRepo, languageRepo, templateRepo, templateDataRepo, routeRepo)),
	}

	svc.Init(ctx, serviceOptions...)

	// Start the service
	err = svc.Run(ctx, "")
	if err != nil {
		log.WithError(err).Fatal("could not run Server")
	}
}

// setupProfileClient creates and configures the profile client.
func setupProfileClient(
	ctx context.Context,
	cfg aconfig.NotificationConfig) (profilev1connect.ProfileServiceClient, error) {
	return connection.NewServiceClient(ctx, &cfg, apis.ServiceTarget{
		Endpoint:              cfg.ProfileServiceURI,
		WorkloadAPITargetPath: cfg.ProfileServiceWorkloadAPITargetPath,
		ServiceID:             servicecatalog.ServiceProfile,
	}, profilev1connect.NewProfileServiceClient)
}

// setupTenancyClient creates and configures the partition client.
func setupTenancyClient(
	ctx context.Context,
	cfg aconfig.NotificationConfig) (tenancyv1connect.TenancyServiceClient, error) {
	return connection.NewServiceClient(ctx, &cfg, apis.ServiceTarget{
		Endpoint:              cfg.TenancyServiceURI,
		WorkloadAPITargetPath: cfg.TenancyServiceWorkloadAPITargetPath,
		ServiceID:             servicecatalog.ServiceTenancy,
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
		ctx, sm.GetAuthenticator(ctx),
		tenancyAccessInterceptor, functionAccessInterceptor)
	if err != nil {
		util.Log(ctx).WithError(err).Fatal("main -- Could not create default interceptors")
	}

	server := connect.NewServer(defaultInterceptorList...)
	notificationv1connect.RegisterNotificationServiceHandler(server, implementation)

	mux := http.NewServeMux()
	// Send and TemplateSave carry batches of rendered message bodies (HTML email
	// included), so allow more than connect v2's 4 MiB default per message.
	connecthttp.Mount(mux, server, connecthttp.WithReadMaxBytes(maxRequestMessageBytes))
	mux.Handle("/openapi.yaml", apis.NewOpenAPIHandler(notificationAPISpecFile, nil))

	return mux
}
