package tests

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	aconfig "github.com/antinvestor/service-notification/apps/default/config"
	"github.com/antinvestor/service-notification/apps/default/service/authz"
	"github.com/antinvestor/service-notification/apps/default/service/business"
	"github.com/antinvestor/service-notification/apps/default/service/events"
	"github.com/antinvestor/service-notification/apps/default/service/repository"
	"github.com/antinvestor/service-notification/apps/default/tests/testketo"
	internaltests "github.com/antinvestor/service-notification/internal/tests"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/config"
	"github.com/pitabwire/frame/datastore"
	"github.com/pitabwire/frame/frametests"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/pitabwire/frame/frametests/deps/testpostgres"
	"github.com/pitabwire/frame/security"
	"github.com/pitabwire/util"
	"github.com/stretchr/testify/require"
)

type BaseTestSuite struct {
	internaltests.BaseTestSuite
	AuthzMiddleware authz.Middleware
	ketoReadURI     string
	ketoWriteURI    string
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

func (bs *BaseTestSuite) SetupSuite() {
	bs.InitResourceFunc = func(_ context.Context) []definition.TestResource {
		pg := testpostgres.NewWithOpts("service_notification",
			definition.WithUserName("ant"), definition.WithCredential("s3cr3t"))
		keto := testketo.NewWithOpts(
			definition.WithDependancies(pg),
			definition.WithEnableLogging(true),
		)
		return []definition.TestResource{pg, keto}
	}
	bs.BaseTestSuite.SetupSuite()

	ctx := bs.T().Context()

	// Find Keto dependency and extract read/write URIs
	var ketoDep definition.DependancyConn
	for _, res := range bs.Resources() {
		if res.Name() == testketo.ImageName {
			ketoDep = res
			break
		}
	}
	bs.Require().NotNil(ketoDep, "keto dependency should be available")

	// Write API: default port (4467/tcp, first in port list)
	writeURL, err := url.Parse(string(ketoDep.GetDS(ctx)))
	bs.Require().NoError(err)
	bs.ketoWriteURI = writeURL.Host

	// Read API: port 4466/tcp (second in port list)
	readPort, err := ketoDep.PortMapping(ctx, "4466/tcp")
	bs.Require().NoError(err)
	bs.ketoReadURI = fmt.Sprintf("%s:%s", writeURL.Hostname(), readPort)
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
	cfg.DatabaseTraceQueries = false
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

	// Configure real Keto authoriser URIs
	cfg.AuthorizationServiceReadURI = bs.ketoReadURI
	cfg.AuthorizationServiceWriteURI = bs.ketoWriteURI

	ctx, svc := frame.NewServiceWithContext(ctx,
		frame.WithConfig(&cfg),
		frame.WithDatastore(),
		frametests.WithNoopDriver())

	// Use real Keto authoriser via SecurityManager
	sm := svc.SecurityManager()
	bs.AuthzMiddleware = authz.NewMiddleware(sm.GetAuthorizer(ctx))

	profileCli := bs.GetProfileCli(ctx)
	partitionCli := bs.GetPartitionCli(ctx)

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
		events.NewNotificationOutQueue(ctx, qMan, evtsMan, profileCli, partitionCli, notificationRepo, notificationStatusRepo, languageRepo, templateDataRepo, routeRepo)))

	// Get absolute path to migrations directory using source file location
	// This file is in apps/default/service/tests, so migrations are at ../../migrations/0001
	migrationPath := "../../migrations/0001"
	t.Logf("Migration path: %s", migrationPath)

	err = repository.Migrate(ctx, svc.DatastoreManager(), migrationPath)
	require.NoError(t, err)

	err = svc.Run(ctx, "")
	require.NoError(t, err)

	// Create business object with all dependencies
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

// WithAuthClaims adds authentication claims to a context for testing.
func (bs *BaseTestSuite) WithAuthClaims(ctx context.Context, tenantID, profileID string) context.Context {
	claims := &security.AuthenticationClaims{
		TenantID:  tenantID,
		AccessID:  util.IDString(),
		ContactID: profileID,
		SessionID: util.IDString(),
		DeviceID:  "test-device",
	}
	claims.Subject = profileID
	return claims.ClaimsToContext(ctx)
}

// SeedTenantRole writes a tenant-level ReBAC tuple granting the given role to a profile.
func (bs *BaseTestSuite) SeedTenantRole(ctx context.Context, svc *frame.Service, tenantID, profileID, role string) {
	auth := svc.SecurityManager().GetAuthorizer(ctx)
	err := auth.WriteTuple(ctx, security.RelationTuple{
		Object:   security.ObjectRef{Namespace: authz.NamespaceTenant, ID: tenantID},
		Relation: role,
		Subject:  security.SubjectRef{Namespace: authz.NamespaceProfile, ID: profileID},
	})
	bs.Require().NoError(err, "failed to seed tenant role")
}
