package tests

import (
	"buf.build/go/protovalidate"
	"context"
	"fmt"
	apis "github.com/antinvestor/apis/go/common"
	notificationV1 "github.com/antinvestor/apis/go/notification/v1"
	partitionv1 "github.com/antinvestor/apis/go/partition/v1"
	profilev1 "github.com/antinvestor/apis/go/profile/v1"
	"github.com/antinvestor/service-notification/apps/default/config"
	events2 "github.com/antinvestor/service-notification/apps/default/service/events"
	"github.com/antinvestor/service-notification/apps/default/service/handlers"
	"github.com/antinvestor/service-notification/apps/default/service/models"
	protovalidateinterceptor "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/protovalidate"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/util"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	tcNats "github.com/testcontainers/testcontainers-go/modules/nats"
	tcPostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log/slog"
	"strings"
	"testing"
)

// StdoutLogConsumer is a LogConsumer that prints the log to stdout
type StdoutLogConsumer struct{}

// Accept prints the log to stdout
func (lc *StdoutLogConsumer) Accept(l testcontainers.Log) {
	fmt.Print(string(l.Content))
}

// NotificationTestSuite provides a base test suite with all necessary test components
type NotificationTestSuite struct {
	suite.Suite

	postgresContainer *tcPostgres.PostgresContainer
	natsContainer     *tcNats.NATSContainer

	MockCtrl *gomock.Controller
}

// SetupSuite initializes the test environment for the test suite
func (s *NotificationTestSuite) SetupSuite() {

	t := s.T()

	s.MockCtrl = gomock.NewController(t)

	ctx := t.Context()
	// Use the shared test resources
	err := s.setupTestResources(ctx)
	require.NoError(t, err, "could not setup tests")

}

// TearDownSuite cleans up resources after all tests are completed
func (s *NotificationTestSuite) TearDownSuite() {

	if s.MockCtrl != nil {
		s.MockCtrl.Finish()
	}

	t := s.T()
	ctx := t.Context()

	s.cleanupTestResources(ctx)
}

func (s *NotificationTestSuite) GetService(t *testing.T, testOpts DependancyOption) (context.Context, *config.NotificationConfig, *frame.Service, error) {

	ctx := t.Context()

	randomnessPrefix := strings.ToLower(util.RandomString(7))

	pgUri, dbCloseFunc, dbErr := s.prepareDatabaseConnection(ctx, randomnessPrefix, testOpts)
	if dbErr != nil {
		return nil, nil, nil, dbErr
	}
	t.Cleanup(func() { dbCloseFunc(ctx) })

	natsUri, qCloseFunc, qErr := s.prepareQueueConnection(ctx, randomnessPrefix, testOpts)
	if qErr != nil {
		return nil, nil, nil, qErr
	}
	t.Cleanup(func() { qCloseFunc(ctx) })
	// Set environment variables that will be used by frame.ConfigurationDefault
	t.Setenv("DATABASE_URL", pgUri)
	t.Setenv("DATABASE_MAX_CONN", "10")
	t.Setenv("DATABASE_MIN_CONN", "1")
	t.Setenv("DATABASE_MAX_IDLE", "3")

	// Queue configuration
	t.Setenv("QUEUE_URL", natsUri)
	t.Setenv("QUEUE_CLUSTER_ID", NatsCluster)

	// Service ports
	t.Setenv("HTTP_SERVER_PORT", "8081")
	t.Setenv("GRPC_SERVER_PORT", "9091")

	// JWT settings for testing
	t.Setenv("OAUTH2_JWT_VERIFY_ISSUER", "test-issuer")
	t.Setenv("OAUTH2_JWT_VERIFY_AUDIENCE", "service_notifications")

	// NoopDriver settings
	t.Setenv("DRIVER_ENABLED", "false") // This is critical to use NoopDriver

	// Configure service
	cfg := &config.NotificationConfig{}
	cfg.DatabasePrimaryURL = []string{pgUri}

	serviceName := "service_notifications_test"

	// Add debug logging
	slog.Info("Creating service with database URL", "url", pgUri)

	ctx, svc := frame.NewServiceWithContext(ctx, serviceName,
		frame.WithConfig(cfg),
		frame.WithDatastore(),
		frame.WithNoopDriver())

	// Initialize mock clients
	profileCli := s.getProfileCli()
	partitionCli := s.getPartitionCli()

	validator, err := protovalidate.New()
	require.NoError(t, err, "could not load validator for proto messages")

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			recovery.UnaryServerInterceptor(recovery.WithRecoveryHandlerContext(frame.RecoveryHandlerFun)),
			svc.UnaryAuthInterceptor(serviceName, cfg.Oauth2JwtVerifyIssuer),
			protovalidateinterceptor.UnaryServerInterceptor(validator),
		),
		grpc.ChainStreamInterceptor(
			recovery.StreamServerInterceptor(recovery.WithRecoveryHandlerContext(frame.RecoveryHandlerFun)),
			svc.StreamAuthInterceptor(serviceName, cfg.Oauth2JwtVerifyIssuer),
			protovalidateinterceptor.StreamServerInterceptor(validator),
		),
	)

	implementation := &handlers.NotificationServer{

		Service:      svc,
		ProfileCli:   profileCli,
		PartitionCli: partitionCli,
	}

	notificationV1.RegisterNotificationServiceServer(grpcServer, implementation)

	grpcServerOpt := frame.WithGRPCServer(grpcServer)
	serviceOptions := []frame.Option{grpcServerOpt}

	proxyOptions := apis.ProxyOptions{
		GrpcServerEndpoint: fmt.Sprintf("localhost:%s", cfg.GrpcServerPort),
		GrpcServerDialOpts: []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
	}

	proxyMux, err := notificationV1.CreateProxyHandler(ctx, proxyOptions)
	require.NoError(t, err, "could not create proxy handler")

	proxyServerOpt := frame.WithHTTPHandler(proxyMux)
	serviceOptions = append(serviceOptions, proxyServerOpt)

	// Register event handlers - don't try to capture the return value
	svc.Init(ctx,
		frame.WithRegisterEvents(
			&events2.NotificationSave{Service: svc},
			&events2.NotificationStatusSave{Service: svc},
			&events2.NotificationInRoute{Service: svc},
			&events2.NotificationInQueue{Service: svc, ProfileCli: profileCli},
			&events2.NotificationOutRoute{Service: svc, ProfileCli: profileCli},
			&events2.NotificationOutQueue{Service: svc, ProfileCli: profileCli},
		))

	// Explicitly initialize the database pool to avoid nil pointer dereference
	svc.Init(ctx, serviceOptions...)

	err = svc.MigrateDatastore(ctx, cfg.GetDatabaseMigrationPath(),
		&models.Route{}, &models.Language{}, &models.Template{},
		&models.TemplateData{}, &models.Notification{}, &models.NotificationStatus{})
	if err != nil {
		return nil, nil, nil, err
	}
	// Run service
	err = svc.Run(ctx, "")
	if err != nil {
		return nil, nil, nil, err
	}

	t.Cleanup(func() {
		svc.Stop(ctx)
	})

	return ctx, cfg, svc, err
}

// getProfileCli creates a mock profile client
func (s *NotificationTestSuite) getProfileCli() *profilev1.ProfileClient {
	slog.Info("Creating mock profile client")

	mockProfileService := profilev1.NewMockProfileServiceClient(s.MockCtrl)

	// Set up common expectations for the profile client
	mockProfileService.EXPECT().
		GetById(gomock.Any(), gomock.Any()).
		AnyTimes().
		Return(&profilev1.GetByIdResponse{
			Data: &profilev1.ProfileObject{
				Id: "test-profile-id",
			},
		}, nil)

	// Create a profile client with the mock service
	return profilev1.Init(nil, mockProfileService)
}

// getPartitionCli creates a mock partition client
func (s *NotificationTestSuite) getPartitionCli() *partitionv1.PartitionClient {
	slog.Info("Creating mock partition client")

	mockPartitionService := partitionv1.NewMockPartitionServiceClient(s.MockCtrl)

	// Set up common expectations for the partition client
	mockPartitionService.EXPECT().
		GetAccess(gomock.Any(), gomock.Any()).
		AnyTimes().
		Return(&partitionv1.GetAccessResponse{
			Data: &partitionv1.AccessObject{
				AccessId: "test-access-id",
				Partition: &partitionv1.PartitionObject{
					Id:       "test-partition-id",
					TenantId: "test-tenant-id",
				},
			},
		}, nil)

	// Create a partition client with the mock service
	return partitionv1.Init(nil, mockPartitionService)
}

// CreateTestModels creates test models in the database for use in tests
func (s *NotificationTestSuite) CreateTestModels(ctx context.Context, t *testing.T, svc *frame.Service) (
	*models.Language,
	*models.Route,
	*models.Template,
	*models.TemplateData,
	*models.Notification,
	*models.NotificationStatus,
) {

	db := svc.DB(ctx, false)

	// Create language
	language := &models.Language{
		Code: "en",
		Name: "English",
	}
	language.GenID(ctx)

	result := db.Where("code = ?", language.Code).FirstOrCreate(&language)
	require.NoError(t, result.Error)

	// Create route
	route := &models.Route{
		Name:        "test-route",
		RouteType:   models.RouteTypeShortForm,
		Mode:        models.RouteModeTransmit,
		Uri:         "test://example.com",
		Description: "Test route for SMS notifications",
	}
	route.GenID(ctx)

	result = db.Create(route)
	require.NoError(t, result.Error)

	// Create template
	template := &models.Template{
		Name:  "test-template",
		Extra: frame.JSONMap{},
	}
	template.GenID(ctx)

	result = db.Create(template)
	require.NoError(t, result.Error)

	// Create template data
	templateData := &models.TemplateData{
		TemplateID: template.ID,
		LanguageID: language.ID,
		Type:       models.RouteTypeShortForm,
		Detail:     "Hello {{name}}, this is a test template",
	}
	templateData.GenID(ctx)

	result = db.Create(templateData)
	require.NoError(t, result.Error)

	// Create notification
	notification := &models.Notification{
		RouteID:          route.ID,
		TemplateID:       template.ID,
		LanguageID:       language.ID,
		NotificationType: models.RouteTypeShortForm,
		Message:          "Test notification message",
		Payload:          frame.JSONMap{"name": "Test User"},
		State:            0, // Pending state
	}
	notification.GenID(ctx)

	result = db.Create(notification)
	require.NoError(t, result.Error)

	// Create notification status
	notificationStatus := &models.NotificationStatus{
		NotificationID: notification.ID,
		Status:         int32(0), // STATUS_PENDING
		State:          int32(0), // STATE_PENDING
	}
	notificationStatus.GenID(ctx)

	result = db.Create(notificationStatus)
	require.NoError(t, result.Error)

	// Update the notification with the status ID
	notification.StatusID = notificationStatus.ID
	result = db.Save(notification)
	require.NoError(t, result.Error)

	return language, route, template, templateData, notification, notificationStatus
}
