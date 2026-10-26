package main

import (
	"context"
	"net/http"
	"time"

	"buf.build/gen/go/antinvestor/notification/connectrpc/go/v2/notification/v1/notificationv1connect"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/v2/profile/v1/profilev1connect"
	apis "github.com/antinvestor/common/v3"
	"github.com/antinvestor/common/v3/connection"
	"github.com/antinvestor/common/v3/servicecatalog"
	aconfig "github.com/antinvestor/service-notification/apps/ussd/config"
	"github.com/antinvestor/service-notification/apps/ussd/service/business"
	ussdEvents "github.com/antinvestor/service-notification/apps/ussd/service/events"
	"github.com/antinvestor/service-notification/apps/ussd/service/handlers"
	"github.com/antinvestor/service-notification/apps/ussd/service/repository"
	"github.com/antinvestor/service-notification/pkg/events"
	"github.com/pitabwire/frame/v3"
	"github.com/pitabwire/frame/v3/client"
	"github.com/pitabwire/frame/v3/config"
	"github.com/pitabwire/frame/v3/datastore"
	"github.com/pitabwire/frame/v3/security/interceptors/httptor"
	"github.com/pitabwire/frame/v3/setup"
	"github.com/pitabwire/util"
)

// validatorTimeout bounds a menu's external validator call; a gateway drops
// a USSD turn that takes longer than a few seconds.
const validatorTimeout = 4 * time.Second

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
	svc.Setup().RegisterFunc(setup.NameMigrate, func(ctx context.Context) error {
		return repository.Migrate(ctx, svc.DatastoreManager(), cfg.GetDatabaseMigrationPath())
	})
	defer svc.Stop(ctx)
	log := util.Log(ctx)

	// Setup Job only: migrate, then exit. The USSD app exposes no Connect
	// API, so it registers no permission manifest.
	if frame.ShouldRunSetup(&cfg) {
		svc.Init(ctx)
		if setupErr := svc.RunSetupForProcess(ctx, &cfg); setupErr != nil {
			log.WithError(setupErr).Fatal("setup plan failed")
		}
		log.Info("setup plan complete — exiting")
		return
	}

	dbManager := svc.DatastoreManager()
	evtsMan := svc.EventsManager()
	sm := svc.SecurityManager()

	// Setup external service clients
	notificationCli, err := setupNotificationClient(ctx, cfg)
	if err != nil {
		log.WithError(err).Fatal("main -- Could not setup notification client")
	}

	profileCli, err := setupProfileClient(ctx, cfg)
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
	// External validators are anonymous HTTPS calls. A service's
	// session_start_url is called as this service account (OAuth from the
	// service config). Both are bounded tightly: a gateway drops a USSD turn
	// that takes longer than a few seconds.
	validatorHTTP := client.NewHTTPClient(ctx, client.WithHTTPTimeout(validatorTimeout), client.WithHTTPNoAuth())
	serviceHTTP := client.NewHTTPClient(ctx, client.WithHTTPTimeout(validatorTimeout))
	ussdBusiness := business.NewUssdBusiness(ctx, evtsMan,
		menuRepo, translationRepo, sessionRepo, queryRepo, serviceConfigRepo,
		validatorHTTP, serviceHTTP, cfg.TrustedServiceHosts, business.NewProfileCallers(profileCli),
		cfg.DefaultLanguageCode, cfg.SessionExpiryMinutes)

	// Setup HTTP handlers
	gatewayServer := handlers.NewGatewayServer(ussdBusiness)
	managementServer := handlers.NewManagementServer(ussdBusiness)

	mux := http.NewServeMux()

	// Mount gateway routes (telco-facing — uses auth_key, not JWT)
	gatewayMux := gatewayServer.NewRouter()
	mux.Handle("/ussd/", gatewayMux)
	mux.Handle("/gateway/", gatewayMux)

	// Mount management API routes behind authentication
	mgmtMux := http.NewServeMux()
	managementServer.RegisterRoutes(mgmtMux)
	// The bearer token is verified first (it sets the claims); the
	// management guard then admits internal service accounts only.
	authenticator := sm.GetAuthenticator(ctx)
	mux.Handle("/api/", httptor.AuthenticationMiddleware(
		handlers.AuthMiddleware(authenticator, mgmtMux), authenticator))

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
