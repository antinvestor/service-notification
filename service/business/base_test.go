package business_test

import (
	"context"
	"fmt"
	"github.com/antinvestor/apis/go/common"
	partitionV1 "github.com/antinvestor/apis/go/partition/v1"
	profileV1 "github.com/antinvestor/apis/go/profile/v1"
	"github.com/antinvestor/service-notification/config"
	"github.com/antinvestor/service-notification/service/events"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/pitabwire/frame"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	tcPostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/mock/gomock"
	"net"
	"time"
)

const PostgresqlDbImage = "postgres:17"

// StdoutLogConsumer is a LogConsumer that prints the log to stdout
type StdoutLogConsumer struct{}

// Accept prints the log to stdout
func (lc *StdoutLogConsumer) Accept(l testcontainers.Log) {
	fmt.Print(string(l.Content))
}

type BaseTestSuite struct {
	suite.Suite
	service     *frame.Service
	ctx         context.Context
	pgContainer *tcPostgres.PostgresContainer
	networks    []string
	postgresUri string
}

func (bs *BaseTestSuite) SetupSuite() {
	ctx := context.Background()

	postgresContainer, err := bs.setupPostgres(ctx)
	assert.NoError(bs.T(), err)

	port, _ := nat.NewPort("tcp", "5432")
	port, _ = postgresContainer.MappedPort(ctx, port)
	fmt.Println(" successfully setup postgresql port : ", port.Port())

	bs.pgContainer = postgresContainer

	bs.networks, err = bs.pgContainer.Networks(ctx)
	assert.NoError(bs.T(), err)

	postgresqlIp, err := bs.pgContainer.ContainerIP(ctx)
	assert.NoError(bs.T(), err)

	bs.postgresUri = fmt.Sprintf("postgres://ant:secret@%s/service_notification?sslmode=disable", net.JoinHostPort(postgresqlIp, "5432"))

	databaseUriStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	assert.NoError(bs.T(), err)

	err = bs.setupMigrations(ctx)
	assert.NoError(bs.T(), err)

	nConfig := config.NotificationConfig{}

	err = frame.ConfigProcess("", &nConfig)
	assert.NoError(bs.T(), err)

	nConfig.LogLevel = "debug"
	nConfig.RunServiceSecurely = false
	nConfig.ServerPort = ""
	nConfig.DatabasePrimaryURL = []string{databaseUriStr}
	nConfig.DatabaseReplicaURL = []string{databaseUriStr}

	var service *frame.Service
	ctx, service = frame.NewServiceWithContext(ctx, "notification tests",
		frame.Config(&nConfig),
		frame.Datastore(ctx),
		frame.NoopDriver())

	profileCli := bs.getProfileCli(ctx)

	service.Init(frame.RegisterEvents(
		&events.NotificationSave{Service: service},
		&events.NotificationStatusSave{Service: service},
		&events.NotificationInRoute{Service: service},
		&events.NotificationInQueue{Service: service, ProfileCli: profileCli},
		&events.NotificationOutRoute{Service: service, ProfileCli: profileCli},
		&events.NotificationOutQueue{Service: service, ProfileCli: profileCli}))

	err = service.Run(ctx, "")
	bs.ctx = ctx
	bs.service = service

	assert.NoError(bs.T(), err)
}

func (bs *BaseTestSuite) getProfileCli(_ context.Context) *profileV1.ProfileClient {

	t := bs.T()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockProfileService := profileV1.NewMockProfileServiceClient(ctrl)
	mockProfileService.EXPECT().
		GetById(gomock.Any(), gomock.Any()).
		Return(&profileV1.GetByIdResponse{
			Data: &profileV1.ProfileObject{
				Id: "test_profile-id",
			},
		}, nil).AnyTimes()
	mockProfileService.EXPECT().
		GetByContact(gomock.Any(), gomock.Any()).
		Return(&profileV1.GetByContactResponse{
			Data: &profileV1.ProfileObject{
				Id: "test_profile-id",
			},
		}, nil).AnyTimes()

	profileCli := profileV1.Init(&common.GrpcClientBase{}, mockProfileService)
	return profileCli
}

func (bs *BaseTestSuite) getPartitionCli(_ context.Context) *partitionV1.PartitionClient {

	t := bs.T()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPartitionService := partitionV1.NewMockPartitionServiceClient(ctrl)

	mockPartitionService.EXPECT().
		GetAccess(gomock.Any(), gomock.Any()).
		Return(&partitionV1.GetAccessResponse{Data: &partitionV1.AccessObject{
			AccessId: "test_access-id",
			Partition: &partitionV1.PartitionObject{
				Id:       "test_partition-id",
				TenantId: "test_tenant-id",
			},
		}}, nil).AnyTimes()

	profileCli := partitionV1.Init(&common.GrpcClientBase{}, mockPartitionService)
	return profileCli
}

func (bs *BaseTestSuite) setupPostgres(ctx context.Context) (*tcPostgres.PostgresContainer, error) {

	postgresContainer, err := tcPostgres.Run(ctx, PostgresqlDbImage,
		tcPostgres.WithDatabase("service_notification"),
		tcPostgres.WithUsername("ant"),
		tcPostgres.WithPassword("secret"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
	)
	if err != nil {
		return nil, err
	}

	return postgresContainer, nil
}

func (bs *BaseTestSuite) setupMigrations(ctx context.Context) error {

	g := StdoutLogConsumer{}

	cRequest := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context: "../../",
		},
		ConfigModifier: func(config *container.Config) {
			config.Env = []string{
				"LOG_LEVEL=debug",
				"DO_MIGRATION=true",
				fmt.Sprintf("DATABASE_URL=%s", bs.postgresUri),
				"CONTACT_ENCRYPTION_KEY=ualgJEcb4GNXLn3jYV9TUGtgYrdTMg",
				"CONTACT_ENCRYPTION_SALT=VufLmnycUCgz",
			}
		},
		Networks:   bs.networks,
		WaitingFor: wait.ForExit().WithExitTimeout(10 * time.Second),
		LogConsumerCfg: &testcontainers.LogConsumerConfig{
			Opts:      []testcontainers.LogProductionOption{testcontainers.WithLogProductionTimeout(2 * time.Second)},
			Consumers: []testcontainers.LogConsumer{&g},
		},
	}

	migrationC, err := testcontainers.GenericContainer(ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: cRequest,
			Started:          true,
		})
	if err != nil {
		return err
	}

	return migrationC.Terminate(ctx)
}

func (bs *BaseTestSuite) TearDownSuite() {

	t := bs.T()
	if bs.pgContainer != nil {
		if err := bs.pgContainer.Terminate(bs.ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	}
}
