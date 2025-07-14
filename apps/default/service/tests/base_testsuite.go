package tests

import (
	"context"
	"testing"

	"github.com/antinvestor/apis/go/common"
	"github.com/antinvestor/apis/go/common/mocks"
	commonv1 "github.com/antinvestor/apis/go/common/v1"
	notificationv1 "github.com/antinvestor/apis/go/notification/v1"
	partitionv1 "github.com/antinvestor/apis/go/partition/v1"
	profilev1 "github.com/antinvestor/apis/go/profile/v1"
	"github.com/antinvestor/service-notification/apps/default/config"
	events2 "github.com/antinvestor/service-notification/apps/default/service/events"
	"github.com/antinvestor/service-notification/apps/default/service/repository"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/tests"
	"github.com/pitabwire/frame/tests/deps/testpostgres"
	"github.com/pitabwire/frame/tests/testdef"
	"github.com/pitabwire/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
)

const PostgresqlDBImage = "paradedb/paradedb:latest"

const (
	DefaultRandomStringLength = 8
)

type BaseTestSuite struct {
	tests.FrameBaseTestSuite
}

func initResources(_ context.Context) []testdef.TestResource {
	pg := testpostgres.NewPGDepWithCred(PostgresqlDBImage, "ant", "s3cr3t", "service_profile")
	resources := []testdef.TestResource{pg}
	return resources
}

func (bs *BaseTestSuite) SetupSuite() {
	bs.InitResourceFunc = initResources
	bs.FrameBaseTestSuite.SetupSuite()
}

func (bs *BaseTestSuite) CreateService(
	t *testing.T,
	depOpts *testdef.DependancyOption,
) (*frame.Service, context.Context) {
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	profileConfig, err := frame.ConfigFromEnv[config.NotificationConfig]()
	require.NoError(t, err)

	profileConfig.LogLevel = "debug"
	profileConfig.RunServiceSecurely = false
	profileConfig.ServerPort = ""

	for _, res := range depOpts.Database() {
		testDS, cleanup, err0 := res.GetRandomisedDS(t.Context(), depOpts.Prefix())
		require.NoError(t, err0)

		t.Cleanup(func() {
			cleanup(t.Context())
		})

		profileConfig.DatabasePrimaryURL = []string{testDS.String()}
		profileConfig.DatabaseReplicaURL = []string{testDS.String()}
	}

	ctx, svc := frame.NewServiceWithContext(t.Context(), "profile tests",
		frame.WithConfig(&profileConfig),
		frame.WithDatastore(),
		frame.WithNoopDriver())

	profileCli := bs.GetProfileCli(ctx)

	svc.Init(ctx, frame.WithRegisterEvents(
		&events2.NotificationSave{Service: svc},
		&events2.NotificationStatusSave{Service: svc},
		&events2.NotificationInRoute{Service: svc},
		&events2.NotificationInQueue{Service: svc, ProfileCli: profileCli},
		&events2.NotificationOutRoute{Service: svc, ProfileCli: profileCli},
		&events2.NotificationOutQueue{Service: svc, ProfileCli: profileCli}))

	err = repository.Migrate(ctx, svc, "../../migrations/0001")
	require.NoError(t, err)

	err = svc.Run(ctx, "")
	require.NoError(t, err)

	return svc, ctx
}

func (bs *BaseTestSuite) GetNotificationCli(_ context.Context) *notificationv1.NotificationClient {
	mockNotificationService := notificationv1.NewMockNotificationServiceClient(bs.Ctrl)
	mockNotificationService.EXPECT().Send(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, _ *notificationv1.SendRequest, _ ...grpc.CallOption) (grpc.ServerStreamingClient[notificationv1.SendResponse], error) {
			// Return a successful response with a generated message ID
			const randomIDLength = 6
			resp := &notificationv1.SendResponse{
				Data: []*commonv1.StatusResponse{
					{
						Id:         util.IDString(),
						State:      commonv1.STATE_ACTIVE,
						Status:     commonv1.STATUS_SUCCESSFUL,
						ExternalId: util.RandomString(randomIDLength),
					},
				},
			}

			// Create a custom mock implementation
			mockStream := mocks.NewMockServerStreamingClient[notificationv1.SendResponse](ctx)
			err := mockStream.SendMsg(resp)
			if err != nil {
				return nil, err
			}

			return mockStream, nil
		}).
		AnyTimes()
	notificationCli := notificationv1.Init(&common.GrpcClientBase{}, mockNotificationService)

	return notificationCli
}

func (bs *BaseTestSuite) GetProfileCli(_ context.Context) *profilev1.ProfileClient {

	t := bs.T()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockProfileService := profilev1.NewMockProfileServiceClient(ctrl)
	mockProfileService.EXPECT().
		GetById(gomock.Any(), gomock.Any()).
		Return(&profilev1.GetByIdResponse{
			Data: &profilev1.ProfileObject{
				Id: "test_profile-id",
			},
		}, nil).AnyTimes()
	mockProfileService.EXPECT().
		GetByContact(gomock.Any(), gomock.Any()).
		Return(&profilev1.GetByContactResponse{
			Data: &profilev1.ProfileObject{
				Id: "test_profile-id",
			},
		}, nil).AnyTimes()

	profileCli := profilev1.Init(&common.GrpcClientBase{}, mockProfileService)
	return profileCli
}

func (bs *BaseTestSuite) GetPartitionCli(_ context.Context) *partitionv1.PartitionClient {

	t := bs.T()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPartitionService := partitionv1.NewMockPartitionServiceClient(ctrl)

	mockPartitionService.EXPECT().
		GetAccess(gomock.Any(), gomock.Any()).
		Return(&partitionv1.GetAccessResponse{Data: &partitionv1.AccessObject{
			AccessId: "test_access-id",
			Partition: &partitionv1.PartitionObject{
				Id:       "test_partition-id",
				TenantId: "test_tenant-id",
			},
		}}, nil).AnyTimes()

	profileCli := partitionv1.Init(&common.GrpcClientBase{}, mockPartitionService)
	return profileCli
}

func (bs *BaseTestSuite) TearDownSuite() {
	bs.FrameBaseTestSuite.TearDownSuite()
}

// WithTestDependancies Creates subtests with each known DependancyOption.
func (bs *BaseTestSuite) WithTestDependancies(t *testing.T, testFn func(t *testing.T, dep *testdef.DependancyOption)) {
	options := []*testdef.DependancyOption{
		testdef.NewDependancyOption("default", util.RandomString(DefaultRandomStringLength), bs.Resources()),
	}

	tests.WithTestDependancies(t, options, testFn)
}
