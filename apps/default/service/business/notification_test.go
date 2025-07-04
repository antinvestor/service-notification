package business_test

import (
	"context"
	"github.com/antinvestor/service-notification/apps/default/service/business"
	"github.com/antinvestor/service-notification/apps/default/service/models"
	repository2 "github.com/antinvestor/service-notification/apps/default/service/repository"
	"testing"
	"time"

	commonv1 "github.com/antinvestor/apis/go/common/v1"
	notificationV1 "github.com/antinvestor/apis/go/notification/v1"
	partitionV1 "github.com/antinvestor/apis/go/partition/v1"
	profileV1 "github.com/antinvestor/apis/go/profile/v1"
	"github.com/pitabwire/frame"
	"github.com/stretchr/testify/suite"
)

type NotificationTestSuite struct {
	BaseTestSuite
}

func (nts *NotificationTestSuite) SetupSuite() {
	nts.BaseTestSuite.SetupSuite()

}

func TestProfileSuite(t *testing.T) {
	suite.Run(t, new(NotificationTestSuite))
}

type ctxSrv struct {
	ctx context.Context
	srv *frame.Service
}

func (nts *NotificationTestSuite) TestNewNotificationBusiness() {

	t := nts.T()
	ctx := nts.ctx
	service := nts.service

	profileCli := nts.getProfileCli(ctx)
	partitionCli := nts.getPartitionCli(ctx)

	type args struct {
		ctxService   *ctxSrv
		profileCli   *profileV1.ProfileClient
		partitionCli *partitionV1.PartitionClient
	}
	tests := []struct {
		name      string
		args      args
		want      business.NotificationBusiness
		expectErr bool
	}{

		{name: "NewNotificationBusiness",
			args: args{
				ctxService: &ctxSrv{
					ctx: ctx,
					srv: service,
				},
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
			if got, err := business.NewNotificationBusiness(tt.args.ctxService.ctx, tt.args.ctxService.srv, tt.args.profileCli, tt.args.partitionCli); !tt.expectErr && (err != nil || got == nil) {
				t.Errorf("NewNotificationBusiness() = could not get a valid notificationBusiness at %s", tt.name)
			}
		})
	}
}

func (nts *NotificationTestSuite) Test_notificationBusiness_QueueIn() {

	t := nts.T()
	ctx := nts.ctx
	service := nts.service

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
				ctxService: &ctxSrv{
					ctx: ctx,
					srv: service,
				},
				profileCli:  nts.getProfileCli(ctx),
				partitionCl: nts.getPartitionCli(ctx),
			},
			args: args{
				message: &notificationV1.Notification{
					Id:        "justtestingId",
					Language:  "en",
					Recipient: &commonv1.ContactLink{ContactId: "epochTesting"},
					OutBound:  true,
					Data:      "Hello we are just testing things out",
				},
			},
			wantErr: false,
			want: &commonv1.StatusResponse{
				Id:     "123456",
				State:  commonv1.STATE_CREATED,
				Status: commonv1.STATUS_UNKNOWN,
			},
		},
		{name: "NormalWithIDQueueIn",
			fields: fields{
				ctxService:  &ctxSrv{ctx: ctx, srv: service},
				profileCli:  nts.getProfileCli(ctx),
				partitionCl: nts.getPartitionCli(ctx),
			},
			args: args{
				message: &notificationV1.Notification{
					Id:        "c2f4j7au6s7f91uqnojg",
					Language:  "en",
					Recipient: &commonv1.ContactLink{ContactId: "epochTesting"},
					Data:      "Hello we are just testing things out",
				},
			},
			wantErr: false,
			want: &commonv1.StatusResponse{
				Id:     "c2f4j7au6s7f91uqnojg",
				State:  commonv1.STATE_CREATED,
				Status: commonv1.STATUS_UNKNOWN,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nb, err := business.NewNotificationBusiness(
				tt.fields.ctxService.ctx,
				tt.fields.ctxService.srv,
				tt.fields.profileCli,
				tt.fields.partitionCl)

			if err != nil {
				t.Errorf("QueueIn() error = %v, could not get notification business", err)
				return
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

func (nts *NotificationTestSuite) Test_notificationBusiness_QueueOut() {

	t := nts.T()
	ctx := nts.ctx
	service := nts.service

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
				ctxService:  &ctxSrv{ctx: ctx, srv: service},
				profileCli:  nts.getProfileCli(ctx),
				partitionCl: nts.getPartitionCli(ctx),
			},
			args: args{
				ctx: ctx,
				message: &notificationV1.Notification{
					Id:        "testingQueue_out",
					Language:  "en",
					Recipient: &commonv1.ContactLink{ContactId: "epochTesting"},
					Data:      "Hello we are just testing things out",
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
				ctxService:  &ctxSrv{ctx: ctx, srv: service},
				profileCli:  nts.getProfileCli(ctx),
				partitionCl: nts.getPartitionCli(ctx),
			},
			args: args{
				ctx: ctx,
				message: &notificationV1.Notification{
					Id:        "c2f4j7au6s7f91uqnojg",
					Language:  "en",
					Recipient: &commonv1.ContactLink{ContactId: "epochTesting"},
					Data:      "Hello we are just testing things out",
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
			nb, err := business.NewNotificationBusiness(
				tt.fields.ctxService.ctx,
				tt.fields.ctxService.srv,
				tt.fields.profileCli,
				tt.fields.partitionCl)
			if err != nil {
				t.Errorf("QueueOut() error = %v, could not get notification business", err)
				return
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

//
// func (nts *NotificationTestSuite) Test_notificationBusiness_Release() {
//
// 	t := nts.T()
// 	ctx := nts.ctx
// 	service := nts.service
//
// 	type fields struct {
// 		ctxService  *ctxSrv
// 		profileCli  *profileV1.ProfileClient
// 		partitionCl *partitionV1.PartitionClient
// 	}
// 	type args struct {
// 		releaseReq *notificationV1.ReleaseRequest
// 	}
// 	tests := []struct {
// 		name    string
// 		fields  fields
// 		args    args
// 		want    *commonv1.StatusResponse
// 		wantErr bool
// 	}{
// 		{name: "NormalRelease",
// 			fields: fields{
// 				ctxService:  &ctxSrv{ctx: ctx, srv: service},
// 				profileCli:  nts.getProfileCli(ctx),
// 				partitionCl: nts.getPartitionCli(ctx),
// 			},
// 			args: args{
// 				releaseReq: &notificationV1.ReleaseRequest{
// 					Id:      []string{"testingQueue_out"},
// 					Comment: "testing releasing messages",
// 				},
// 			},
// 			wantErr: false,
// 			want: &commonv1.StatusResponse{
// 				Id:         "c2f4j7au6s7f91uqnojg",
// 				State:      commonv1.STATE_ACTIVE,
// 				Status:     commonv1.STATUS_QUEUED,
// 				ExternalId: "total_externalisation",
// 			},
// 		},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			nb, err := business.NewNotificationBusiness(
// 				tt.fields.ctxService.ctx,
// 				tt.fields.ctxService.srv,
// 				tt.fields.profileCli,
// 				tt.fields.partitionCl)
// 			if err != nil {
// 				t.Errorf("Release() error = %v, could not get notification business", err)
// 				return
// 			}
//
// 			n := models.Notification{
// 				SenderContactID:  "epochTesting",
// 				Message:          "Hello we are just testing statuses out",
// 				NotificationType: "email",
// 				State:            int32(commonv1.STATE_ACTIVE.Number()),
// 				LanguageID:       "9bsv0s23l8og00vgjqa0",
// 			}
// 			n.AccessID = "testingAccessData"
// 			n.PartitionID = "test_partition-id"
// 			n.TenantID = "test_tenant-id"
//
// 			nRepo := repository.NewNotificationRepository(ctx, tt.fields.ctxService.srv)
// 			err = nRepo.Save(ctx, &n)
// 			if err != nil {
// 				t.Errorf("Status() error = %v could not store a notification for status checking", err)
// 				return
// 			}
// 			tt.args.releaseReq.Id = n.GetID()
//
// 			got, err := nb.Release(tt.fields.ctxService.ctx, tt.args.releaseReq)
// 			if (err != nil) != tt.wantErr {
// 				t.Errorf("Release() error = %v, wantErr %v", err, tt.wantErr)
// 				return
// 			}
//
// 			if got.GetStatus() != tt.want.GetStatus() || got.GetState() != tt.want.GetState() {
// 				t.Errorf("Release() got = %v, want %v", got, tt.want)
// 			}
//
// 			if got.GetId() != n.GetID() {
// 				t.Errorf("Release() expecting notification id to be reused")
// 			}
// 		})
// 	}
// }

// func (nts *NotificationTestSuite) Test_notificationBusiness_Search() {
//
//	t := nts.T()
//	ctx := nts.ctx
//	service := nts.service
//
//	ctrl := gomock.NewController(t)
//	defer ctrl.Finish()
//
//	// Mock the ServerStreamingClient[SearchResponse]
//
//	//nsSs.EXPECT().Context().Return(ctx).AnyTimes()
//
//	type fields struct {
//		ctxService *ctxSrv
//
//		profileCli  *profileV1.ProfileClient
//		partitionCl *partitionV1.PartitionClient
//		leastCount  int
//	}
//	type args struct {
//		search *commonv1.SearchRequest
//		stream grpc.ServerStreamingServer[notificationV1.SearchResponse]
//	}
//	tests := []struct {
//		name    string
//		fields  fields
//		args    args
//		wantErr bool
//	}{
//		{
//			name: "Normal Search",
//			fields: fields{
//				ctxService:  &ctxSrv{ctx: ctx, srv: service},
//				profileCli:  nts.getProfileCli(ctx),
//				partitionCl: nts.getPartitionCli(ctx),
//				leastCount:  1,
//			},
//			args: args{
//				search: &commonv1.SearchRequest{Query: ""},
//			},
//			wantErr: false,
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//
//			nsSs := notificationV1.NewMockNotificationServiceServer(ctrl)
//			nsSs.EXPECT().Send(gomock.Any()).MinTimes(tt.fields.leastCount)
//			tt.args.stream = nsSs
//
//			nb, err := business.NewNotificationBusiness(
//				tt.fields.ctxService.ctx,
//				tt.fields.ctxService.srv,
//				tt.fields.profileCli,
//				tt.fields.partitionCl)
//			if err != nil {
//				t.Errorf("Search() error = %v, could not get notification business", err)
//				return
//			}
//
//			nStatus := models.NotificationStatus{
//				State:  int32(commonv1.STATE_ACTIVE.Number()),
//				Status: int32(commonv1.STATUS_QUEUED.Number()),
//			}
//
//			nStatus.GenID(ctx)
//
//			n := models.Notification{
//				SenderContactID:  "epochTesting",
//				Message:          "Hello we are just testing statuses out",
//				NotificationType: "email",
//				State:            int32(commonv1.STATE_ACTIVE.Number()),
//				LanguageID:       "9bsv0s23l8og00vgjqa0",
//				StatusID:         nStatus.GetID(),
//			}
//			n.PartitionID = "test_partition-id"
//
//			nRepo := repository.NewNotificationRepository(ctx, tt.fields.ctxService.srv)
//			err = nRepo.Save(ctx, &n)
//			if err != nil {
//				t.Errorf("Search() error = %v could not store a notification", err)
//				return
//			}
//
//			nStatus.NotificationID = n.GetID()
//
//			nStatusRepo := repository.NewNotificationStatusRepository(ctx, tt.fields.ctxService.srv)
//			err = nStatusRepo.Save(ctx, &nStatus)
//			if err != nil {
//				t.Errorf("Search() error = %v could not store a notification status", err)
//				return
//			}
//
//			if err := nb.Search(tt.args.search, tt.args.stream); (err != nil) != tt.wantErr {
//				t.Errorf("Search() error = %v, wantErr %v", err, tt.wantErr)
//			}
//
//		})
//	}
// }

func (nts *NotificationTestSuite) Test_notificationBusiness_Status() {

	t := nts.T()
	ctx := nts.ctx
	service := nts.service

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
				ctxService:  &ctxSrv{ctx: ctx, srv: service},
				profileCli:  nts.getProfileCli(ctx),
				partitionCl: nts.getPartitionCli(ctx),
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

			nb, err := business.NewNotificationBusiness(
				tt.fields.ctxService.ctx,
				tt.fields.ctxService.srv,
				tt.fields.profileCli,
				tt.fields.partitionCl)
			if err != nil {
				t.Errorf("Status() error = %v, could not get notification business", err)
				return
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
				SenderContactID:  "epochTesting",
				Message:          "Hello we are just testing statuses out",
				NotificationType: "email",
				StatusID:         nStatus.GetID(),
				ReleasedAt:       &releaseDate,
				LanguageID:       "9bsv0s23l8og00vgjqa0",
			}

			n.ID = "c2f4j7au6s7f91uqnojg"
			n.AccessID = "testingAccessData"
			n.PartitionID = "test_partition-id"
			n.TenantID = "test_tenant-id"

			nRepo := repository2.NewNotificationRepository(ctx, tt.fields.ctxService.srv)
			err = nRepo.Save(ctx, &n)
			if err != nil {
				t.Errorf("Status() error = %v could not store a notification for status checking", err)
				return
			}

			nStatus.NotificationID = n.GetID()
			nSRepo := repository2.NewNotificationStatusRepository(ctx, tt.fields.ctxService.srv)
			err = nSRepo.Save(ctx, &nStatus)
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

func (nts *NotificationTestSuite) Test_notificationBusiness_StatusUpdate() {

	t := nts.T()
	ctx := nts.ctx
	service := nts.service

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
				ctxService:  &ctxSrv{ctx: ctx, srv: service},
				profileCli:  nts.getProfileCli(ctx),
				partitionCl: nts.getPartitionCli(ctx),
			},
			args: args{
				ctx: ctx,
				statusReq: &commonv1.StatusUpdateRequest{
					Id:         "testingQueue_out",
					State:      commonv1.STATE_INACTIVE,
					Status:     commonv1.STATUS_SUCCESSFUL,
					ExternalId: "total_externalisation",
				},
			},
			wantErr: false,
			want: &commonv1.StatusResponse{
				Id:         "c2f4j7au6s7f91uqnojg",
				State:      commonv1.STATE_INACTIVE,
				Status:     commonv1.STATUS_SUCCESSFUL,
				ExternalId: "total_externalisation",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nb, err := business.NewNotificationBusiness(
				tt.fields.ctxService.ctx,
				tt.fields.ctxService.srv,
				tt.fields.profileCli,
				tt.fields.partitionCl)
			if err != nil {
				t.Errorf("Status() error = %v, could not get notification business", err)
				return
			}

			releaseDate := time.Now()
			n := models.Notification{
				SenderContactID:  "epochTesting",
				Message:          "Hello we are just testing statuses out",
				NotificationType: "email",
				State:            int32(commonv1.STATE_ACTIVE.Number()),
				ReleasedAt:       &releaseDate,
				LanguageID:       "9bsv0s23l8og00vgjqa0",
			}
			n.AccessID = "testingAccessData"
			n.PartitionID = "test_partition-id"

			nRepo := repository2.NewNotificationRepository(ctx, tt.fields.ctxService.srv)
			err = nRepo.Save(ctx, &n)
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

// func (nts *NotificationTestSuite) Test_notificationBusiness_TemplateSearch() {
//
//	t := nts.T()
//	ctx := nts.ctx
//	service := nts.service
//
//	ctrl := gomock.NewController(t)
//	defer ctrl.Finish()
//
//	type fields struct {
//		ctxService *ctxSrv
//
//		profileCli  *profileV1.ProfileClient
//		partitionCl *partitionV1.PartitionClient
//		resultCount int
//	}
//	type args struct {
//		search *notificationV1.TemplateSearchRequest
//	}
//	tests := []struct {
//		name    string
//		fields  fields
//		args    args
//		wantErr bool
//	}{
//
//		{
//			name: "Normal Search",
//			fields: fields{
//				ctxService:  &ctxSrv{ctx: ctx, srv: service},
//				profileCli:  nts.getProfileCli(ctx),
//				partitionCl: nts.getPartitionCli(ctx),
//				resultCount: 1,
//			},
//			args: args{
//				search: &notificationV1.TemplateSearchRequest{Query: "Normal Search"},
//			},
//			wantErr: false,
//		},
//		{
//			name: "Non existent Search",
//			fields: fields{
//				ctxService:  &ctxSrv{ctx: ctx, srv: service},
//				profileCli:  nts.getProfileCli(ctx),
//				partitionCl: nts.getPartitionCli(ctx),
//				resultCount: 0,
//			},
//			args: args{
//				search: &notificationV1.TemplateSearchRequest{Query: "alien cryptic template"},
//			},
//			wantErr: false,
//		},
//		{
//			name: "Empty Search",
//			fields: fields{
//				ctxService:  &ctxSrv{ctx: ctx, srv: service},
//				profileCli:  nts.getProfileCli(ctx),
//				partitionCl: nts.getPartitionCli(ctx),
//				resultCount: 3,
//			},
//			args: args{
//				search: &notificationV1.TemplateSearchRequest{
//					Query: "",
//					Page:  0,
//					Count: 3,
//				},
//			},
//			wantErr: false,
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//
//			nsSs := notificationV1.NewMockServerStream(ctrl)
//			nsSs.EXPECT().Context().Return(tt.fields.ctxService.ctx).AnyTimes()
//			nsSs.EXPECT().Send(gomock.Any()).MinTimes(1).DoAndReturn(
//				func(arg *notificationV1.TemplateSearchResponse) any {
//
//					if len(arg.Data) != tt.fields.resultCount {
//						t.Errorf("TemplateSearch() expected result items %v don't match %v", tt.fields.resultCount, len(arg.Data))
//					}
//
//					return nil
//				},
//			)
//
//			nb, err := business.NewNotificationBusiness(
//				tt.fields.ctxService.ctx,
//				tt.fields.ctxService.srv,
//				tt.fields.profileCli,
//				tt.fields.partitionCl)
//			if err != nil {
//				t.Errorf("TemplateSearch() error = %v, could not get notification business", err)
//				return
//			}
//
//			template := models.Template{
//				Name: fmt.Sprintf("%s-test template", tt.name),
//			}
//			template.PartitionID = "test_partition-id"
//
//			templateRepository := repository.NewTemplateRepository(tt.fields.ctxService.ctx, tt.fields.ctxService.srv)
//			err = templateRepository.Save(tt.fields.ctxService.ctx, &template)
//			if err != nil {
//				t.Errorf("TemplateSearch() error = %v could not store a template", err)
//				return
//			}
//
//			templateData := models.TemplateData{
//				TemplateID: template.GetID(),
//				LanguageID: "9bsv0s23l8og00vgjqa0",
//				Type:       models.RouteTypeShortForm,
//				Detail:     fmt.Sprintf("testing short message - %s", tt.name),
//			}
//
//			templateDataRepository := repository.NewTemplateDataRepository(tt.fields.ctxService.ctx, tt.fields.ctxService.srv)
//
//			err = templateDataRepository.Save(tt.fields.ctxService.ctx, &templateData)
//			if err != nil {
//				t.Errorf("TemplateSearch() error = %v could not store a template data", err)
//				return
//			}
//
//			if err = nb.TemplateSearch(tt.args.search, nsSs); (err != nil) != tt.wantErr {
//				t.Errorf("TemplateSearch() error = %v, wantErr %v", err, tt.wantErr)
//			}
//
//		})
//	}
// }
