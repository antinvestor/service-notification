package business

import (
	"context"
	"github.com/antinvestor/apis/common"
	notificationV1 "github.com/antinvestor/service-notification-api"
	"github.com/antinvestor/service-notification/service/events"
	"github.com/antinvestor/service-notification/service/models"
	"github.com/antinvestor/service-notification/service/repository"
	partitionV1 "github.com/antinvestor/service-partition-api"
	profileV1 "github.com/antinvestor/service-profile-api"
	"github.com/golang/mock/gomock"
	"github.com/pitabwire/frame"
	"testing"
	"time"
)

func getService(ctx context.Context, serviceName string) *frame.Service {
	dbURL := frame.GetEnv("TEST_DATABASE_URL", "postgres://ant:secret@localhost:5436/service_notification?sslmode=disable")
	testDb := frame.Datastore(ctx, dbURL, false)

	service := frame.NewService(serviceName, testDb, frame.NoopDriver())

	eventList := frame.RegisterEvents(
		&events.NotificationSave{Service: service},
		&events.NotificationStatusSave{Service: service})
	service.Init(eventList)
	_ = service.Run(ctx, "")
	return service
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
		Return(&partitionV1.AccessObject{
			AccessId: "test_access-id",
			Partition: &partitionV1.PartitionObject{
				PartitionId: "test_partition-id",
				TenantId:    "test_tenant-id",
			},
		}, nil).AnyTimes()

	profileCli := partitionV1.InstantiatePartitionsClient(nil, mockPartitionService)
	return profileCli
}

func TestNewNotificationBusiness(t *testing.T) {

	ctx := context.Background()

	profileCli := getProfileCli(t)
	partitionCli := getPartitionCli(t)

	type args struct {
		ctx          context.Context
		service      *frame.Service
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
				ctx:          ctx,
				service:      getService(ctx, "NewNotificationBusinessTest"),
				profileCli:   profileCli,
				partitionCli: partitionCli},
			expectErr: false},

		{name: "NewNotificationBusinessWithNils",
			args: args{
				ctx:        ctx,
				service:    nil,
				profileCli: nil,
			},
			expectErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := NewNotificationBusiness(tt.args.ctx, tt.args.service, tt.args.profileCli, tt.args.partitionCli); !tt.expectErr && (err != nil || got == nil) {
				t.Errorf("NewNotificationBusiness() = could not get a valid notificationBusiness at %s", tt.name)
			}
		})
	}
}

func Test_notificationBusiness_QueueIn(t *testing.T) {

	ctx := context.Background()

	type fields struct {
		service     *frame.Service
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
		want    *notificationV1.StatusResponse
		wantErr bool
	}{
		{name: "NormalPassingQueueIn",
			fields: fields{
				service:     getService(ctx, "NormalQueueInTest"),
				profileCli:  getProfileCli(t),
				partitionCl: getPartitionCli(t),
			},
			args: args{
				ctx: ctx,
				message: &notificationV1.Notification{
					ID:       "justtestingId",
					Contact:  &notificationV1.Notification_ContactID{ContactID: "epochTesting"},
					OutBound: true,
					Data:     "Hello we are just testing things out",
					AccessID: "testingAccessData",
				},
			},
			wantErr: false,
			want: &notificationV1.StatusResponse{
				ID:     "123456",
				State:  common.STATE_CREATED,
				Status: common.STATUS_QUEUED,
			},
		},
		{name: "NormalWithIDQueueIn",
			fields: fields{
				service:     getService(ctx, "NormalQueueInTest"),
				profileCli:  getProfileCli(t),
				partitionCl: getPartitionCli(t),
			},
			args: args{
				ctx: ctx,
				message: &notificationV1.Notification{
					ID:       "c2f4j7au6s7f91uqnojg",
					Contact:  &notificationV1.Notification_ContactID{ContactID: "epochTesting"},
					Data:     "Hello we are just testing things out",
					AccessID: "testingAccessData",
				},
			},
			wantErr: false,
			want: &notificationV1.StatusResponse{
				ID:     "c2f4j7au6s7f91uqnojg",
				State:  common.STATE_CREATED,
				Status: common.STATUS_QUEUED,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nb := &notificationBusiness{
				service:      tt.fields.service,
				profileCli:   tt.fields.profileCli,
				partitionCli: tt.fields.partitionCl,
			}
			got, err := nb.QueueIn(tt.args.ctx, tt.args.message)
			if (err != nil) != tt.wantErr {
				t.Errorf("QueueIn() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.GetStatus() != tt.want.GetStatus() || got.GetState() != tt.want.GetState() {
				t.Errorf("QueueIn() got = %v, want %v", got, tt.want)
			}

			if tt.name == "NormalWithIDQueueIn" && got.GetID() != tt.want.GetID() {
				t.Errorf("QueueIn() expecting id %s to be reused, got : %s", tt.want.GetID(), got.GetID())
			}
		})
	}
}

func Test_notificationBusiness_QueueOut(t *testing.T) {
	ctx := context.Background()

	type fields struct {
		service     *frame.Service
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
		want    *notificationV1.StatusResponse
		wantErr bool
	}{
		{name: "NormalQueueOut",
			fields: fields{
				service:     getService(ctx, "NormalQueueOutTest"),
				profileCli:  getProfileCli(t),
				partitionCl: getPartitionCli(t),
			},
			args: args{
				ctx: ctx,
				message: &notificationV1.Notification{
					ID:       "testingQueue_out",
					Contact:  &notificationV1.Notification_ContactID{ContactID: "epochTesting"},
					Data:     "Hello we are just testing things out",
					AccessID: "testingAccessData",
				},
			},
			wantErr: false,
			want: &notificationV1.StatusResponse{
				ID:     "c2f4j7au6s7f91uqnojg",
				State:  common.STATE_CREATED,
				Status: common.STATUS_QUEUED,
			},
		},

		{name: "NormalQueueOutWithXID",
			fields: fields{
				service:     getService(ctx, "NormalQueueOutWithXIDTest"),
				profileCli:  getProfileCli(t),
				partitionCl: getPartitionCli(t),
			},
			args: args{
				ctx: ctx,
				message: &notificationV1.Notification{
					ID:       "c2f4j7au6s7f91uqnojg",
					Contact:  &notificationV1.Notification_ContactID{ContactID: "epochTesting"},
					Data:     "Hello we are just testing things out",
					AccessID: "testingAccessData",
				},
			},
			wantErr: false,
			want: &notificationV1.StatusResponse{
				ID:     "c2f4j7au6s7f91uqnojg",
				State:  common.STATE_CREATED,
				Status: common.STATUS_QUEUED,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nb := &notificationBusiness{
				service:      tt.fields.service,
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

			if tt.name == "NormalQueueOutWithXID" && got.GetID() != tt.want.GetID() {
				t.Errorf("QueueOut() expecting id %s to be reused, got : %s", tt.want.GetID(), got.GetID())
			}
		})
	}
}

func Test_notificationBusiness_Release(t *testing.T) {

	ctx := context.Background()

	type fields struct {
		service     *frame.Service
		profileCli  *profileV1.ProfileClient
		partitionCl *partitionV1.PartitionClient
	}
	type args struct {
		ctx        context.Context
		releaseReq *notificationV1.ReleaseRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *notificationV1.StatusResponse
		wantErr bool
	}{
		{name: "NormalRelease",
			fields: fields{
				service:     getService(ctx, "NormalReleaseTest"),
				profileCli:  getProfileCli(t),
				partitionCl: getPartitionCli(t),
			},
			args: args{
				ctx: ctx,
				releaseReq: &notificationV1.ReleaseRequest{
					ID:       "testingQueue_out",
					AccessID: "testingAccessData",
					Comment:  "testing releasing messages",
				},
			},
			wantErr: false,
			want: &notificationV1.StatusResponse{
				ID:         "c2f4j7au6s7f91uqnojg",
				State:      common.STATE_ACTIVE,
				Status:     common.STATUS_QUEUED,
				ExternalID: "total_externalization",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nb := &notificationBusiness{
				service:      tt.fields.service,
				profileCli:   tt.fields.profileCli,
				partitionCli: tt.fields.partitionCl,
			}

			n := models.Notification{
				ContactID:        "epochTesting",
				Message:          "Hello we are just testing statuses out",
				NotificationType: "email",
				State:            int32(common.STATE_ACTIVE.Number()),
			}
			n.AccessID = "testingAccessData"
			n.PartitionID = "test_partition-id"

			nRepo := repository.NewNotificationRepository(ctx, nb.service)
			err := nRepo.Save(&n)
			if err != nil {
				t.Errorf("Status() error = %v could not store a notification for status checking", err)
				return
			}
			tt.args.releaseReq.ID = n.GetID()

			got, err := nb.Release(tt.args.ctx, tt.args.releaseReq)
			if (err != nil) != tt.wantErr {
				t.Errorf("Release() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got.GetStatus() != tt.want.GetStatus() || got.GetState() != tt.want.GetState() {
				t.Errorf("Release() got = %v, want %v", got, tt.want)
			}

			if got.GetID() != n.GetID() {
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
		service     *frame.Service
		profileCli  *profileV1.ProfileClient
		partitionCl *partitionV1.PartitionClient
		leastCount  int
	}
	type args struct {
		search *notificationV1.SearchRequest
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
				service:     getService(ctx, "NormalSearchTest"),
				profileCli:  getProfileCli(t),
				partitionCl: getPartitionCli(t),
				leastCount:  1,
			},
			args: args{
				search: &notificationV1.SearchRequest{Query: "", AccessID: "testingAccessData"},
				stream: nsSs,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			nsSs.EXPECT().Send(gomock.Any()).MinTimes(tt.fields.leastCount)

			nb := &notificationBusiness{
				service:      tt.fields.service,
				profileCli:   tt.fields.profileCli,
				partitionCli: tt.fields.partitionCl,
			}

			n := models.Notification{
				ContactID:        "epochTesting",
				Message:          "Hello we are just testing statuses out",
				NotificationType: "email",
				State:            int32(common.STATE_ACTIVE.Number()),
			}
			n.AccessID = tt.args.search.GetAccessID()
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
		service     *frame.Service
		profileCli  *profileV1.ProfileClient
		partitionCl *partitionV1.PartitionClient
	}
	type args struct {
		ctx       context.Context
		statusReq *notificationV1.StatusRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *notificationV1.StatusResponse
		wantErr bool
	}{
		{name: "NormalStatus",
			fields: fields{
				service:     getService(ctx, "NormalStatusTest"),
				profileCli:  getProfileCli(t),
				partitionCl: getPartitionCli(t),
			},
			args: args{
				ctx: ctx,
				statusReq: &notificationV1.StatusRequest{
					ID:       "testingQueue_out",
					AccessID: "testingAccessData",
				},
			},
			wantErr: false,
			want: &notificationV1.StatusResponse{
				ID:     "c2f4j7au6s7f91uqnojg",
				State:  common.STATE_DELETED,
				Status: common.STATUS_FAILED,
			},
		},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {

			nb := &notificationBusiness{
				service:      tt.fields.service,
				profileCli:   tt.fields.profileCli,
				partitionCli: tt.fields.partitionCl,
			}

			nStatus := models.NotificationStatus{
				State:  int32(common.STATE_DELETED.Number()),
				Status: int32(common.STATUS_FAILED.Number()),
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

			tt.args.statusReq.ID = n.GetID()

			got, err := nb.Status(tt.args.ctx, tt.args.statusReq)
			if (err != nil) != tt.wantErr {
				t.Errorf("Status() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.GetStatus() != tt.want.GetStatus() || got.GetState() != tt.want.GetState() {
				t.Errorf("Status() got = %v, want %v", got, tt.want)
			}

			if got.GetID() != n.GetID() {
				t.Errorf("Status() expecting notification id to be reused")
			}
		})
	}
}

func Test_notificationBusiness_StatusUpdate(t *testing.T) {

	ctx := context.Background()

	type fields struct {
		service     *frame.Service
		profileCli  *profileV1.ProfileClient
		partitionCl *partitionV1.PartitionClient
	}
	type args struct {
		ctx       context.Context
		statusReq *notificationV1.StatusUpdateRequest
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *notificationV1.StatusResponse
		wantErr bool
	}{
		{name: "NormalStatusUpdate",
			fields: fields{
				service:     getService(ctx, "NormalStatusUpdateTest"),
				profileCli:  getProfileCli(t),
				partitionCl: getPartitionCli(t),
			},
			args: args{
				ctx: ctx,
				statusReq: &notificationV1.StatusUpdateRequest{
					ID:         "testingQueue_out",
					AccessID:   "testingAccessData",
					State:      common.STATE_INACTIVE,
					Status:     common.STATUS_SUCCESSFUL,
					ExternalID: "total_externalization",
				},
			},
			wantErr: false,
			want: &notificationV1.StatusResponse{
				ID:         "c2f4j7au6s7f91uqnojg",
				State:      common.STATE_INACTIVE,
				Status:     common.STATUS_SUCCESSFUL,
				ExternalID: "total_externalization",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nb := &notificationBusiness{
				service:      tt.fields.service,
				profileCli:   tt.fields.profileCli,
				partitionCli: tt.fields.partitionCl,
			}

			releaseDate := time.Now()
			n := models.Notification{
				ContactID:        "epochTesting",
				Message:          "Hello we are just testing statuses out",
				NotificationType: "email",
				State:            int32(common.STATE_ACTIVE.Number()),
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

			tt.args.statusReq.ID = n.GetID()

			got, err := nb.StatusUpdate(tt.args.ctx, tt.args.statusReq)
			if (err != nil) != tt.wantErr {
				t.Errorf("StatusUpdate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.GetState() != tt.want.GetState() {
				t.Errorf("Status() got = %v, want %v", got, tt.want)
			}

			if got.GetExternalID() != tt.want.GetExternalID() {
				t.Errorf("Status() got =%v, want %v", got.GetExternalID(), tt.want.GetExternalID())
			}

			if got.GetID() != n.GetID() {
				t.Errorf("Status() expecting notification id to be reused")
			}
		})
	}
}
