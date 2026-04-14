package tests

import (
	"context"
	"testing"

	aconfig "github.com/antinvestor/service-notification/apps/ussd/config"
	"github.com/antinvestor/service-notification/apps/ussd/service/business"
	"github.com/antinvestor/service-notification/apps/ussd/service/repository"
	internaltests "github.com/antinvestor/service-notification/pkg/tests"
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

// ServiceResources holds all repositories and business object for easy reuse in tests.
type ServiceResources struct {
	MenuRepo          repository.MenuRepository
	TranslationRepo   repository.TranslationRepository
	SessionRepo       repository.SessionRepository
	QueryRepo         repository.QueryRepository
	ServiceConfigRepo repository.ServiceConfigRepository
	UssdBusiness      business.UssdBusiness
}

func (bs *BaseTestSuite) SetupSuite() {
	bs.BaseTestSuite.SetupSuite()
}

func (bs *BaseTestSuite) CreateService(
	t *testing.T,
	depOpts *definition.DependencyOption,
) (*frame.Service, context.Context, *ServiceResources) {
	ctx := t.Context()

	cfg, err := config.FromEnv[aconfig.UssdConfig]()
	require.NoError(t, err)

	cfg.LogLevel = "debug"
	cfg.DatabaseMigrate = true
	cfg.DatabaseTraceQueries = false
	cfg.RunServiceSecurely = false
	cfg.ServerPort = ""
	cfg.DefaultLanguageCode = "en"
	cfg.SessionExpiryMinutes = 5

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

	workMan := svc.WorkManager()
	evtsMan := svc.EventsManager()
	dbPool := svc.DatastoreManager().GetPool(ctx, datastore.DefaultPoolName)

	// Run migration
	err = repository.Migrate(ctx, svc.DatastoreManager(), "")
	require.NoError(t, err)

	// Initialise repositories
	menuRepo := repository.NewMenuRepository(ctx, dbPool, workMan)
	translationRepo := repository.NewTranslationRepository(ctx, dbPool, workMan)
	sessionRepo := repository.NewSessionRepository(ctx, dbPool, workMan)
	queryRepo := repository.NewQueryRepository(ctx, dbPool, workMan)
	serviceConfigRepo := repository.NewServiceConfigRepository(ctx, dbPool, workMan)

	svc.Init(ctx)

	err = svc.Run(ctx, "")
	require.NoError(t, err)

	// Create business logic
	ussdBiz := business.NewUssdBusiness(ctx, evtsMan,
		menuRepo, translationRepo, sessionRepo, queryRepo, serviceConfigRepo,
		"en", 5)

	resources := &ServiceResources{
		MenuRepo:          menuRepo,
		TranslationRepo:   translationRepo,
		SessionRepo:       sessionRepo,
		QueryRepo:         queryRepo,
		ServiceConfigRepo: serviceConfigRepo,
		UssdBusiness:      ussdBiz,
	}

	return svc, ctx, resources
}
