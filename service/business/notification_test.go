package business

import (
	"context"
	commonv1 "github.com/antinvestor/apis/go/common/v1"
	notificationV1 "github.com/antinvestor/apis/go/notification/v1"
	partitionV1 "github.com/antinvestor/apis/go/partition/v1"
	profileV1 "github.com/antinvestor/apis/go/profile/v1"
	"github.com/antinvestor/service-notification/config"
	"github.com/antinvestor/service-notification/service/events"
	"github.com/antinvestor/service-notification/service/models"
	"github.com/antinvestor/service-notification/service/repository"
	"github.com/pitabwire/frame"
	"go.uber.org/mock/gomock"
	"testing"
	"time"
)

func getService(serviceName string) *ctxSrv {

	dbURL := frame.GetEnv("TEST_DATABASE_URL", "postgres://ant:secret@localhost:5436/service_notification?sslmode=disable")
	testDb := frame.DatastoreCon(dbURL, false)

	var ncfg config.NotificationConfig
	_ = frame.ConfigProcess("", &ncfg)

	ctx, service := frame.NewService(serviceName, testDb, frame.Config(&ncfg), frame.NoopDriver())

	eventList := frame.RegisterEvents(
		&events.NotificationSave{Service: service},
		&events.NotificationStatusSave{Service: service})
	service.Init(eventList)
	_ = service.Run(ctx, "")
	return &ctxSrv{
		ctx, service,
	}
}

type ctxSrv struct {
	ctx context.Context
	srv *frame.Service
}

func getProfileCli(t *testing.T) *profileV1.ProfileClient {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockProfileService := profileV1.NewMockProfileServiceClient(ctrl)
	profileCli := profileV1.InstantiateProfileClient(nil, mockProfileService)
	return profileCli
}

func getPartitionCli(t *testing.T) *partitionV1.PartitionClient {
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

	profileCli := partitionV1.InstantiatePartitionsClient(nil, mockPartitionService)
	return profileCli
}

func TestNewNotificationBusiness(t *testing.T) {

	profileCli := getProfileCli(t)
	partitionCli := getPartitionCli(t)

	type args struct {
		ctxService   *ctxSrv
		profileCli   *profileV1.ProfileClient
		partitionCli *partitionV1.PartitionClient
	}
	tests := []struct {
		name      string
		args      args
		want      NotificationBusiness
		expectErr bool
	}{

		{name: "NewNotificationBusiness",
			args: args{
				ctxService:   getService("NewNotificationBusinessTest"),
				profileCli:   profileCli,
				partitionCli: partitionCli},
			expectErr: false},

		{name: "NewNotificationBusinessWithNils",
			args: args{
				ctxService: &ctxSrv{
					ctx: context.Background(),
				},
				profileCli: nil,
			},
			expectErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := NewNotificationBusiness(tt.args.ctxService.ctx, tt.args.ctxService.srv, tt.args.profileCli, tt.args.partitionCli); !tt.expectErr && (err != nil || got == nil) {
				t.Errorf("NewNotificationBusiness() = could not get a valid notificationBusiness at %s", tt.name)
			}
		})
	}
}

func Test_notificationBusiness_QueueIn(t *testing.T) {

	type fields struct {
		ctxService  *ctxSrv
		profileCli  *profileV1.ProfileClient
		partitionCl *partitionV1.PartitionClient
	}
	type args struct {
		message *notificationV1.Notification
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *commonv1.StatusResponse
		wantErr bool
	}{
		{name: "NormalPassingQueueIn",
			fields: fields{
				ctxService:  getService("NormalQueueInTest"),
				profileCli:  getProfileCli(t),
				partitionCl: getPartitionCli(t),
			},
			args: args{
				message: &notificationV1.Notification{
					Id:       "justtestingId",
					Contact:  &notificationV1.Notification_ContactId{ContactId: "epochTesting"},
					OutBound: true,
					Data:     "Hello we are just testing things out",
					AccessId: "testingAccessData",
				},
			},
			wantErr: false,
			want: &commonv1.StatusResponse{
				Id:     "123456",
				State:  commonv1.STATE_CREATED,
				Status: commonv1.STATUS_QUEUED,
			},
		},
		{name: "NormalWithIDQueueIn",
			fields: fields{
				ctxService:  getService("NormalQueueInTest"),
				profileCli:  getProfileCli(t),
				partitionCl: getPartitionCli(t),
			},
			args: args{
				message: &notificationV1.Notification{
					Id:       "c2f4j7au6s7f91uqnojg",
					Contact:  &notificationV1.Notification_ContactId{ContactId: "epochTesting"},
					Data:     "Hello we are just testing things out",
					AccessId: "testingAccessData",
				},
			},
			wantErr: false,
			want: &commonv1.StatusResponse{
				Id:     "c2f4j7au6s7f91uqnojg",
				State:  commonv1.STATE_CREATED,
				Status: commonv1.STATUS_QUEUED,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nb := &notificationBusiness{
				service:      tt.fields.ctxService.srv,
				profileCli:   tt.fields.profileCli,
				partitionCli: tt.fields.partitionCl,
			}
			got, err := nb.QueueIn(tt.fields.ctxService.ctx, tt.args.message)
			if (err != nil) != tt.wantErr {
				t.Errorf("QueueIn() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.GetStatus() != tt.want.GetStatus() || got.GetState() != tt.want.GetState() {
				t.Errorf("QueueIn() got = %v, want %v", got, tt.want)
			}

			if tt.name == "NormalWithIDQueueIn" && got.GetId() != tt.want.GetId() {
				t.Errorf("QueueIn() expecting id %s to be reused, got : %s", tt.want.GetId(), got.GetId())
			}
		})
	}
}

func Test_notificationBusiness_QueueOut(t *testing.T) {
	ctx := context.Background()

	type fields struct {
		ctxService *ctxSrv

		profileCli  *profileV1.ProfileClient
		partitionCl *partitionV1.PartitionClient
	}
	type args struct {
		ctx     context.Context
		message *notificationV1.Notification
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *commonv1.StatusResponse
		wantErr bool
	}{
		{name: "NormalQueueOut",
			fields: fields{
				ctxService: getService(
					"NormalQueueOutTest"),
				profileCli:  getProfileCli(t),
				partitionCl: getPartitionCli(t),
			},
			args: args{
				ctx: ctx,
				message: &notificationV1.Notification{
					Id:       "testingQueue_out",
					Contact:  &notificationV1.Notification_ContactId{ContactId: "epochTesting"},
					Data:     "Hello we are just testing things out",
					AccessId: "testingAccessData",
				},
			},
			wantErr: false,
			want: &commonv1.StatusResponse{
				Id:     "c2f4j7au6s7f91uqnojg",
				State:  commonv1.STATE_CREATED,
				Status: commonv1.STATUS_QUEUED,
			},
		},

		{name: "NormalQueueOutWithXID",
			fields: fields{
				ctxService: getService(
					"NormalQueueOutWithXIDTest"),
				profileCli:  getProfileCli(t),
				partitionCl: getPartitionCli(t),
			},
			args: args{
				ctx: ctx,
				message: &notificationV1.Notification{
					Id:       "c2f4j7au6s7f91uqnojg",
					Contact:  &notificationV1.Notification_ContactId{ContactId: "epochTesting"},
					Data:     "Hello we are just testing things out",
					AccessId: "testingAccessData",
				},
			},
			wantErr: false,
			want: &commonv1.StatusResponse{
				Id:     "c2f4j7au6s7f91uqnojg",
				State:  commonv1.STATE_CREATED,
				Status: commonv1.STATUS_QUEUED,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nb := &notificationBusiness{
				service:      tt.fields.ctxService.srv,
				profileCli:   tt.fields.profileCli,
				partitionCli: tt.fields.partitionCl,
			}
			got, err := nb.QueueOut(tt.args.ctx, tt.args.message)
			if (err != nil) != tt.wantErr {
				t.Errorf("QueueOut() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.GetStatus() != tt.want.GetStatus() || got.GetState() != tt.want.GetState() {
				t.Errorf("QueueOut() got = %v, want %v", got, tt.want)
			}

			if tt.name == "NormalQueueOutWithXID" && got.GetId() != tt.want.GetId() {
				t.Errorf("QueueOut() expecting id %s to be reused, got : %s", tt.want.GetId(), got.GetId())
			}
		})
	}
}

func Test_notificationBusiness_Release(t *testing.T) {

	ctx := context.Background()

	type fields struct {
		ctxService  *ctxSrv
		profileCli  *profileV1.ProfileClient
		partitionCl *partitionV1.PartitionClient
	}
	type args struct {
		releaseReq *notificationV1.ReleaseRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *commonv1.StatusResponse
		wantErr bool
	}{
		{name: "NormalRelease",
			fields: fields{
				ctxService: getService(
					"NormalReleaseTest"),
				profileCli:  getProfileCli(t),
				partitionCl: getPartitionCli(t),
			},
			args: args{
				releaseReq: &notificationV1.ReleaseRequest{
					Id:       "testingQueue_out",
					AccessId: "testingAccessData",
					Comment:  "testing releasing messages",
				},
			},
			wantErr: false,
			want: &commonv1.StatusResponse{
				Id:         "c2f4j7au6s7f91uqnojg",
				State:      commonv1.STATE_ACTIVE,
				Status:     commonv1.STATUS_QUEUED,
				ExternalId: "total_externalization",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nb := &notificationBusiness{
				service:      tt.fields.ctxService.srv,
				profileCli:   tt.fields.profileCli,
				partitionCli: tt.fields.partitionCl,
			}

			n := models.Notification{
				ContactID:        "epochTesting",
				Message:          "Hello we are just testing statuses out",
				NotificationType: "email",
				State:            int32(commonv1.STATE_ACTIVE.Number()),
			}
			n.AccessID = "testingAccessData"
			n.PartitionID = "test_partition-id"

			nRepo := repository.NewNotificationRepository(ctx, nb.service)
			err := nRepo.Save(&n)
			if err != nil {
				t.Errorf("Status() error = %v could not store a notification for status checking", err)
				return
			}
			tt.args.releaseReq.Id = n.GetID()

			got, err := nb.Release(tt.fields.ctxService.ctx, tt.args.releaseReq)
			if (err != nil) != tt.wantErr {
				t.Errorf("Release() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got.GetStatus() != tt.want.GetStatus() || got.GetState() != tt.want.GetState() {
				t.Errorf("Release() got = %v, want %v", got, tt.want)
			}

			if got.GetId() != n.GetID() {
				t.Errorf("Release() expecting notification id to be reused")
			}
		})
	}
}

func Test_notificationBusiness_Search(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	nsSs := notificationV1.NewMockNotificationService_SearchServer(ctrl)
	nsSs.EXPECT().Context().Return(ctx).AnyTimes()

	type fields struct {
		ctxService *ctxSrv

		profileCli  *profileV1.ProfileClient
		partitionCl *partitionV1.PartitionClient
		leastCount  int
	}
	type args struct {
		search *commonv1.SearchRequest
		stream notificationV1.NotificationService_SearchServer
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Normal Search",
			fields: fields{
				ctxService: getService(
					"NormalSearchTest"),
				profileCli:  getProfileCli(t),
				partitionCl: getPartitionCli(t),
				leastCount:  1,
			},
			args: args{
				search: &commonv1.SearchRequest{Query: ""},
				stream: nsSs,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			nsSs.EXPECT().Send(gomock.Any()).MinTimes(tt.fields.leastCount)

			nb := &notificationBusiness{
				service:      tt.fields.ctxService.srv,
				profileCli:   tt.fields.profileCli,
				partitionCli: tt.fields.partitionCl,
			}

			n := models.Notification{
				ContactID:        "epochTesting",
				Message:          "Hello we are just testing statuses out",
				NotificationType: "email",
				State:            int32(commonv1.STATE_ACTIVE.Number()),
			}
			n.PartitionID = "test_partition-id"

			nRepo := repository.NewNotificationRepository(ctx, nb.service)
			err := nRepo.Save(&n)
			if err != nil {
				t.Errorf("Search() error = %v could not store a notification", err)
				return
			}

			if err := nb.Search(tt.args.search, tt.args.stream); (err != nil) != tt.wantErr {
				t.Errorf("Search() error = %v, wantErr %v", err, tt.wantErr)
			}

		})
	}
}

func Test_notificationBusiness_Status(t *testing.T) {
	ctx := context.Background()
	type fields struct {
		ctxService *ctxSrv

		profileCli  *profileV1.ProfileClient
		partitionCl *partitionV1.PartitionClient
	}
	type args struct {
		ctx       context.Context
		statusReq *commonv1.StatusRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *commonv1.StatusResponse
		wantErr bool
	}{
		{name: "NormalStatus",
			fields: fields{
				ctxService: getService(
					"NormalStatusTest"),
				profileCli:  getProfileCli(t),
				partitionCl: getPartitionCli(t),
			},
			args: args{
				ctx: ctx,
				statusReq: &commonv1.StatusRequest{
					Id: "testingQueue_out",
				},
			},
			wantErr: false,
			want: &commonv1.StatusResponse{
				Id:     "c2f4j7au6s7f91uqnojg",
				State:  commonv1.STATE_DELETED,
				Status: commonv1.STATUS_FAILED,
			},
		},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {

			nb := &notificationBusiness{
				service:      tt.fields.ctxService.srv,
				profileCli:   tt.fields.profileCli,
				partitionCli: tt.fields.partitionCl,
			}

			nStatus := models.NotificationStatus{
				State:  int32(commonv1.STATE_DELETED.Number()),
				Status: int32(commonv1.STATUS_FAILED.Number()),
			}

			nStatus.AccessID = "testingAccessData"
			nStatus.PartitionID = "test_partition-id"
			nStatus.GenID(ctx)

			releaseDate := time.Now()
			n := models.Notification{
				ContactID:        "epochTesting",
				Message:          "Hello we are just testing statuses out",
				NotificationType: "email",
				StatusID:         nStatus.GetID(),
				ReleasedAt:       &releaseDate,
			}

			n.ID = "c2f4j7au6s7f91uqnojg"
			n.AccessID = "testingAccessData"
			n.PartitionID = "test_partition-id"

			nRepo := repository.NewNotificationRepository(ctx, nb.service)
			err := nRepo.Save(&n)
			if err != nil {
				t.Errorf("Status() error = %v could not store a notification for status checking", err)
				return
			}

			nStatus.NotificationID = n.GetID()
			nSRepo := repository.NewNotificationStatusRepository(ctx, nb.service)
			err = nSRepo.Save(&nStatus)
			if err != nil {
				t.Errorf("Status() error = %v could not store a notification Status for status checking", err)
				return
			}

			tt.args.statusReq.Id = n.GetID()

			got, err := nb.Status(tt.args.ctx, tt.args.statusReq)
			if (err != nil) != tt.wantErr {
				t.Errorf("Status() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.GetStatus() != tt.want.GetStatus() || got.GetState() != tt.want.GetState() {
				t.Errorf("Status() got = %v, want %v", got, tt.want)
			}

			if got.GetId() != n.GetID() {
				t.Errorf("Status() expecting notification id to be reused")
			}
		})
	}
}

func Test_notificationBusiness_StatusUpdate(t *testing.T) {

	ctx := context.Background()

	type fields struct {
		ctxService *ctxSrv

		profileCli  *profileV1.ProfileClient
		partitionCl *partitionV1.PartitionClient
	}
	type args struct {
		ctx       context.Context
		statusReq *commonv1.StatusUpdateRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *commonv1.StatusResponse
		wantErr bool
	}{
		{name: "NormalStatusUpdate",
			fields: fields{
				ctxService: getService(
					"NormalStatusUpdateTest"),
				profileCli:  getProfileCli(t),
				partitionCl: getPartitionCli(t),
			},
			args: args{
				ctx: ctx,
				statusReq: &commonv1.StatusUpdateRequest{
					Id:         "testingQueue_out",
					AccessId:   "testingAccessData",
					State:      commonv1.STATE_INACTIVE,
					Status:     commonv1.STATUS_SUCCESSFUL,
					ExternalId: "total_externalization",
				},
			},
			wantErr: false,
			want: &commonv1.StatusResponse{
				Id:         "c2f4j7au6s7f91uqnojg",
				State:      commonv1.STATE_INACTIVE,
				Status:     commonv1.STATUS_SUCCESSFUL,
				ExternalId: "total_externalization",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nb := &notificationBusiness{
				service:      tt.fields.ctxService.srv,
				profileCli:   tt.fields.profileCli,
				partitionCli: tt.fields.partitionCl,
			}

			releaseDate := time.Now()
			n := models.Notification{
				ContactID:        "epochTesting",
				Message:          "Hello we are just testing statuses out",
				NotificationType: "email",
				State:            int32(commonv1.STATE_ACTIVE.Number()),
				ReleasedAt:       &releaseDate,
			}
			n.AccessID = "testingAccessData"
			n.PartitionID = "test_partition-id"

			nRepo := repository.NewNotificationRepository(ctx, nb.service)
			err := nRepo.Save(&n)
			if err != nil {
				t.Errorf("Status() error = %v could not store a notification for status checking", err)
				return
			}

			tt.args.statusReq.Id = n.GetID()

			got, err := nb.StatusUpdate(tt.args.ctx, tt.args.statusReq)
			if (err != nil) != tt.wantErr {
				t.Errorf("StatusUpdate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.GetState() != tt.want.GetState() {
				t.Errorf("Status() got = %v, want %v", got, tt.want)
			}

			if got.GetExternalId() != tt.want.GetExternalId() {
				t.Errorf("Status() got =%v, want %v", got.GetExternalId(), tt.want.GetExternalId())
			}

			if got.GetId() != n.GetID() {
				t.Errorf("Status() expecting notification id to be reused")
			}
		})
	}
}
