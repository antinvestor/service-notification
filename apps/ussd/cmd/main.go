package main

import (
	"context"
	"net/http"

	"buf.build/gen/go/antinvestor/notification/connectrpc/go/notification/v1/notificationv1connect"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	apis "github.com/antinvestor/common/v2"
	"github.com/antinvestor/common/v2/connection"
	"github.com/antinvestor/common/v2/servicecatalog"
	aconfig "github.com/antinvestor/service-notification/apps/ussd/config"
	"github.com/antinvestor/service-notification/apps/ussd/service/business"
	ussdEvents "github.com/antinvestor/service-notification/apps/ussd/service/events"
	"github.com/antinvestor/service-notification/apps/ussd/service/handlers"
	"github.com/antinvestor/service-notification/apps/ussd/service/repository"
	"github.com/antinvestor/service-notification/pkg/events"
	"github.com/pitabwire/frame/v2"
	"github.com/pitabwire/frame/v2/config"
	"github.com/pitabwire/frame/v2/datastore"
	"github.com/pitabwire/util"
)

func main() {
	tmpCtx := context.Background()

	cfg, err := config.LoadWithOIDC[aconfig.UssdConfig](tmpCtx)
	if err != nil {
		util.Log(tmpCtx).With("err", err).Error("could not process configs")
		return
	}

	if cfg.Name() == "" {
		cfg.ServiceName = "service_ussd"
	}

	ctx, svc := frame.NewServiceWithContext(
		tmpCtx,
		frame.WithConfig(&cfg),
		frame.WithDatastore(),
	)
	defer svc.Stop(ctx)
	log := util.Log(ctx)

	dbManager := svc.DatastoreManager()
	evtsMan := svc.EventsManager()
	sm := svc.SecurityManager()

	// Handle database migration if requested
	if cfg.DoDatabaseMigrate() {
		errMigrate := repository.Migrate(ctx, dbManager, cfg.GetDatabaseMigrationPath())
		if errMigrate != nil {
			log.WithError(errMigrate).Fatal("main -- Could not migrate successfully")
		}
		return
	}

	// Setup external service clients
	notificationCli, err := setupNotificationClient(ctx, cfg)
	if err != nil {
		log.WithError(err).Fatal("main -- Could not setup notification client")
	}

	_, err = setupProfileClient(ctx, cfg)
	if err != nil {
		log.WithError(err).Fatal("main -- Could not setup profile client")
	}

	// Get database pool
	dbPool := dbManager.GetPool(ctx, datastore.DefaultPoolName)
	if dbPool == nil {
		log.Fatal("Database pool is nil - check DATABASE_PRIMARY_URL environment variable")
	}

	workMan := svc.WorkManager()

	// Initialise repositories
	menuRepo := repository.NewMenuRepository(ctx, dbPool, workMan)
	translationRepo := repository.NewTranslationRepository(ctx, dbPool, workMan)
	sessionRepo := repository.NewSessionRepository(ctx, dbPool, workMan)
	queryRepo := repository.NewQueryRepository(ctx, dbPool, workMan)
	serviceConfigRepo := repository.NewServiceConfigRepository(ctx, dbPool, workMan)

	// Create business logic
	ussdBusiness := business.NewUssdBusiness(ctx, evtsMan,
		menuRepo, translationRepo, sessionRepo, queryRepo, serviceConfigRepo,
		cfg.DefaultLanguageCode, cfg.SessionExpiryMinutes)

	// Setup HTTP handlers
	gatewayServer := handlers.NewGatewayServer(ussdBusiness)
	managementServer := handlers.NewManagementServer(ussdBusiness)

	mux := http.NewServeMux()

	// Mount gateway routes (telco-facing — uses auth_key, not JWT)
	gatewayMux := gatewayServer.NewRouter()
	mux.Handle("/ussd/", gatewayMux)

	// Mount management API routes behind authentication
	mgmtMux := http.NewServeMux()
	managementServer.RegisterRoutes(mgmtMux)
	authenticator := sm.GetAuthenticator(ctx)
	mux.Handle("/api/", handlers.AuthMiddleware(authenticator, mgmtMux))

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	serviceOptions := []frame.Option{
		frame.WithHTTPHandler(mux),
		frame.WithRegisterEvents(
			events.NewNotificationStatusUpdate(ctx, notificationCli),
			ussdEvents.NewSessionComplete(ctx, notificationCli),
		),
	}

	svc.Init(ctx, serviceOptions...)

	log.Info("Starting USSD service")
	err = svc.Run(ctx, "")
	if err != nil {
		log.WithError(err).Fatal("could not run Server")
	}
}

func setupNotificationClient(
	ctx context.Context,
	cfg aconfig.UssdConfig,
) (notificationv1connect.NotificationServiceClient, error) {
	return connection.NewServiceClient(ctx, &cfg, apis.ServiceTarget{
		Endpoint:              cfg.NotificationServiceURI,
		WorkloadAPITargetPath: cfg.NotificationServiceWorkloadAPITargetPath,
		ServiceID:             servicecatalog.ServiceNotification,
	}, notificationv1connect.NewNotificationServiceClient)
}

func setupProfileClient(
	ctx context.Context,
	cfg aconfig.UssdConfig,
) (profilev1connect.ProfileServiceClient, error) {
	return connection.NewServiceClient(ctx, &cfg, apis.ServiceTarget{
		Endpoint:              cfg.ProfileServiceURI,
		WorkloadAPITargetPath: cfg.ProfileServiceWorkloadAPITargetPath,
		ServiceID:             servicecatalog.ServiceProfile,
	}, profilev1connect.NewProfileServiceClient)
}
