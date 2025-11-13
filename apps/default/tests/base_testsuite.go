package tests

import (
	"context"
	"testing"

	aconfig "github.com/antinvestor/service-notification/apps/default/config"
	"github.com/antinvestor/service-notification/apps/default/service/business"
	"github.com/antinvestor/service-notification/apps/default/service/events"
	"github.com/antinvestor/service-notification/apps/default/service/repository"
	internaltests "github.com/antinvestor/service-notification/internal/tests"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/config"
	"github.com/pitabwire/frame/datastore"
	"github.com/pitabwire/frame/frametests"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/stretchr/testify/require"
)

type BaseTestSuite struct {
	internaltests.BaseTestSuite
}

// ServiceResources holds all repositories and business object for easy reuse in tests
type ServiceResources struct {
	// Repositories
	NotificationRepo       repository.NotificationRepository
	NotificationStatusRepo repository.NotificationStatusRepository
	LanguageRepo           repository.LanguageRepository
	TemplateRepo           repository.TemplateRepository
	TemplateDataRepo       repository.TemplateDataRepository
	RouteRepo              repository.RouteRepository

	// Business layer
	NotificationBusiness business.NotificationBusiness
}

func (bs *BaseTestSuite) CreateService(
	t *testing.T,
	depOpts *definition.DependencyOption,
) (*frame.Service, context.Context, *ServiceResources) {

	ctx := t.Context()
	cfg, err := config.FromEnv[aconfig.NotificationConfig]()
	require.NoError(t, err)

	cfg.LogLevel = "debug"
	cfg.DatabaseMigrate = true
	cfg.RunServiceSecurely = false
	cfg.ServerPort = ""

	res := depOpts.ByIsDatabase(ctx)
	testDS, cleanup, err0 := res.GetRandomisedDS(ctx, depOpts.Prefix())
	require.NoError(t, err0)

	t.Cleanup(func() {
		cleanup(ctx)
	})

	cfg.DatabasePrimaryURL = []string{testDS.String()}
	cfg.DatabaseReplicaURL = []string{testDS.String()}

	ctx, svc := frame.NewServiceWithContext(ctx,
		frame.WithConfig(&cfg),
		frame.WithDatastore(),
		frametests.WithNoopDriver())

	profileCli := bs.GetProfileCli(ctx)

	// Get managers from service (similar to main.go pattern)
	workMan := svc.WorkManager()
	dbPool := svc.DatastoreManager().GetPool(ctx, datastore.DefaultPoolName)
	evtsMan := svc.EventsManager()
	qMan := svc.QueueManager()

	// Initialise repositories (same as main.go lines 79-84)
	notificationRepo := repository.NewNotificationRepository(ctx, dbPool, workMan)
	notificationStatusRepo := repository.NewNotificationStatusRepository(ctx, dbPool, workMan)
	languageRepo := repository.NewLanguageRepository(ctx, dbPool, workMan)
	templateRepo := repository.NewTemplateRepository(ctx, dbPool, workMan)
	templateDataRepo := repository.NewTemplateDataRepository(ctx, dbPool, workMan)
	routeRepo := repository.NewRouteRepository(ctx, dbPool, workMan)

	// Register event handlers with proper dependencies (same as main.go lines 92-98)
	svc.Init(ctx, frame.WithRegisterEvents(
		events.NewNotificationSave(ctx, evtsMan, notificationRepo),
		events.NewNotificationStatusSave(ctx, notificationRepo, notificationStatusRepo),
		events.NewNotificationInRoute(ctx, qMan, evtsMan, notificationRepo, routeRepo),
		events.NewNotificationInQueue(ctx, qMan, evtsMan, notificationRepo, routeRepo, profileCli),
		events.NewNotificationOutRoute(ctx, evtsMan, profileCli, notificationRepo, routeRepo),
		events.NewNotificationOutQueue(ctx, qMan, evtsMan, profileCli, notificationRepo, notificationStatusRepo, languageRepo, templateDataRepo, routeRepo)))

	// Get absolute path to migrations directory using source file location
	// This file is in apps/default/service/tests, so migrations are at ../../migrations/0001
	migrationPath := "../../migrations/0001"
	t.Logf("Migration path: %s", migrationPath)

	err = repository.Migrate(ctx, svc.DatastoreManager(), migrationPath)
	require.NoError(t, err)

	err = svc.Run(ctx, "")
	require.NoError(t, err)

	// Create business object with all dependencies
	partitionCli := bs.GetPartitionCli(ctx)
	notificationBusiness := business.NewNotificationBusiness(
		ctx,
		workMan,
		evtsMan,
		profileCli,
		partitionCli,
		notificationRepo,
		notificationStatusRepo,
		languageRepo,
		templateRepo,
		templateDataRepo,
		routeRepo,
	)

	// Package all resources for easy reuse
	resources := &ServiceResources{
		NotificationRepo:       notificationRepo,
		NotificationStatusRepo: notificationStatusRepo,
		LanguageRepo:           languageRepo,
		TemplateRepo:           templateRepo,
		TemplateDataRepo:       templateDataRepo,
		RouteRepo:              routeRepo,
		NotificationBusiness:   notificationBusiness,
	}

	// Note: We don't call svc.Run() in tests since it tries to initialise publishers and start the server
	// Tests just need the service initialised with all dependencies

	return svc, ctx, resources
}
