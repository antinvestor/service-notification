package business_test

import (
	"context"
	"testing"
	"time"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/antinvestor/service-notification/apps/default/tests"
	"github.com/pitabwire/frame/frametests"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/structpb"
)

type NotificationTestSuite struct {
	tests.BaseTestSuite
}

func (nts *NotificationTestSuite) SetupSuite() {
	nts.BaseTestSuite.SetupSuite()

}

// Helper function removed - now using ServiceResources from CreateService

func TestNotificationSuite(t *testing.T) {
	suite.Run(t, new(NotificationTestSuite))
}

func (nts *NotificationTestSuite) TestNewNotificationBusiness() {

	nts.WithTestDependancies(nts.T(), func(t *testing.T, dep *definition.DependencyOption) {

		_, _, resources := nts.CreateService(t, dep)

		t.Run("NewNotificationBusiness", func(t *testing.T) {
			if resources.NotificationBusiness == nil {
				t.Errorf("NewNotificationBusiness() = could not get a valid notificationBusiness")
			}
		})

	})
}

func (nts *NotificationTestSuite) Test_notificationBusiness_QueueIn() {

	testCases := []struct {
		name    string
		message *notificationv1.Notification
		want    *commonv1.StatusResponse
		wantErr bool
	}{
		{name: "NormalPassingQueueIn",

			message: &notificationv1.Notification{
				Id:        "justtestingId",
				Language:  "en",
				Recipient: &commonv1.ContactLink{ContactId: "epochTesting"},
				OutBound:  true,
				Data:      "Hello we are just testing things out",
			},
			wantErr: false,
			want: &commonv1.StatusResponse{
				Id:     "123456",
				State:  commonv1.STATE_CREATED,
				Status: commonv1.STATUS_UNKNOWN,
			},
		},
		{name: "NormalWithIDQueueIn",
			message: &notificationv1.Notification{
				Id:        "c2f4j7au6s7f91uqnojg",
				Language:  "en",
				Recipient: &commonv1.ContactLink{ContactId: "epochTesting"},
				Data:      "Hello we are just testing things out",
			},
			wantErr: false,
			want: &commonv1.StatusResponse{
				Id:     "c2f4j7au6s7f91uqnojg",
				State:  commonv1.STATE_CREATED,
				Status: commonv1.STATUS_UNKNOWN,
			},
		},
	}

	nts.WithTestDependancies(nts.T(), func(t *testing.T, dep *definition.DependencyOption) {

		_, ctx, resources := nts.CreateService(t, dep)

		for _, tt := range testCases {
			t.Run(tt.name, func(t *testing.T) {
				got, err := resources.NotificationBusiness.QueueIn(ctx, tt.message)
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

	})
}

func (nts *NotificationTestSuite) Test_notificationBusiness_QueueOut() {

	testcases := []struct {
		name    string
		message *notificationv1.Notification
		want    *commonv1.StatusResponse
		wantErr bool
	}{
		{name: "NormalQueueOut",
			message: &notificationv1.Notification{
				Id:        "testingQueue_out",
				Language:  "en",
				Recipient: &commonv1.ContactLink{ContactId: "epochTesting"},
				Data:      "Hello we are just testing things out",
				Template:  "template.profilev1.contact.verification",
			},
			wantErr: false,
			want: &commonv1.StatusResponse{
				Id:     "c2f4j7au6s7f91uqnojg",
				State:  commonv1.STATE_CREATED,
				Status: commonv1.STATUS_QUEUED,
			},
		},

		{name: "NormalQueueOutWithXID",
			message: &notificationv1.Notification{
				Id:        "c2f4j7au6s7f91uqnojg",
				Language:  "en",
				Recipient: &commonv1.ContactLink{ContactId: "epochTesting"},
				Data:      "Hello we are just testing things out",
			},
			wantErr: false,
			want: &commonv1.StatusResponse{
				Id:     "c2f4j7au6s7f91uqnojg",
				State:  commonv1.STATE_CREATED,
				Status: commonv1.STATUS_QUEUED,
			},
		},
	}

	nts.WithTestDependancies(nts.T(), func(t *testing.T, dep *definition.DependencyOption) {

		_, ctx, resources := nts.CreateService(t, dep)

		for _, tt := range testcases {
			t.Run(tt.name, func(t *testing.T) {
				got, err := resources.NotificationBusiness.QueueOut(ctx, tt.message)
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

				notif, err := frametests.WaitForConditionWithResult[notificationv1.Notification](ctx, func() (*notificationv1.Notification, error) {

					var nSlice []*notificationv1.Notification

					err0 := resources.NotificationBusiness.Search(ctx, &commonv1.SearchRequest{
						Limits: &commonv1.Pagination{
							Count: 10,
							Page:  0,
						},
						IdQuery: got.GetId(),
					}, func(ctx context.Context, batch []*notificationv1.Notification) error {
						nSlice = append(nSlice, batch...)
						return nil
					})
					require.NoError(t, err0)

					if len(nSlice) == 0 {
						return nil, nil
					}

					return nSlice[0], nil
				}, 5*time.Second, 300*time.Millisecond)

				require.NoError(t, err)

				require.NotNil(t, notif)
				require.Equal(t, notif.GetId(), got.GetId())

			})
		}

	})
}

func (nts *NotificationTestSuite) Test_notificationBusiness_Release() {

	testcases := []struct {
		name       string
		releaseReq *notificationv1.ReleaseRequest
		want       *commonv1.StatusResponse
		wantErr    bool
	}{
		{name: "NormalRelease",

			releaseReq: &notificationv1.ReleaseRequest{
				Id:      []string{"testingQueue_out"},
				Comment: "testing releasing messages",
			},
			wantErr: false,
			want: &commonv1.StatusResponse{
				Id:         "c2f4j7au6s7f91uqnojg",
				State:      commonv1.STATE_ACTIVE,
				Status:     commonv1.STATUS_QUEUED,
				ExternalId: "total_externalisation",
			},
		},
	}

	nts.WithTestDependancies(nts.T(), func(t *testing.T, dep *definition.DependencyOption) {

		_, ctx, resources := nts.CreateService(t, dep)

		for _, tt := range testcases {
			t.Run(tt.name, func(t *testing.T) {
				n := models.Notification{
					SenderContactID:  "epochTesting",
					Message:          "Hello we are just testing statuses out",
					NotificationType: "email",
					State:            int32(commonv1.STATE_ACTIVE.Number()),
					LanguageID:       "9bsv0s23l8og00vgjqa0",
				}
				n.AccessID = "testingAccessData"
				n.PartitionID = "test_partition-id"
				n.TenantID = "test_tenant-id"

				err := resources.NotificationRepo.Create(ctx, &n)
				if err != nil {
					t.Errorf("Status() error = %v could not store a notification for status checking", err)
					return
				}

				// Verify notification was created and can be retrieved
				retrievedNotifications, getErr := resources.NotificationRepo.GetByIDList(ctx, n.GetID())
				if getErr != nil {
					t.Errorf("Could not retrieve created notification: %v", getErr)
					return
				}
				t.Logf("Retrieved %d notifications after creation", len(retrievedNotifications))

				tt.releaseReq.Id = []string{n.GetID()}

				t.Logf("Releasing notification with ID: %s, IsReleased: %v", n.GetID(), n.IsReleased())

				resultPipe, err := resources.NotificationBusiness.Release(ctx, tt.releaseReq)
				if (err != nil) != tt.wantErr {
					t.Errorf("Release() error = %v, wantErr %v", err, tt.wantErr)
					return
				}

				if resultPipe == nil {
					t.Errorf("Release() expected non-nil result pipe")
					return
				}

				// Consume the results from the JobResultPipe with timeout
				responseCount := 0
				var got *commonv1.StatusResponse

				// Create a context with timeout - give more time for async event processing
				timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				defer cancel()

				t.Logf("Starting to read results from pipe...")
				for {
					result, ok := resultPipe.ReadResult(timeoutCtx)
					if !ok {
						t.Logf("Result pipe closed, received %d responses", responseCount)
						break // Channel closed
					}
					if result.IsError() {
						t.Errorf("Release() unexpected error from result pipe: %v", result.Error())
						break
					}
					releaseResp := result.Item() // Returns *notificationv1.ReleaseResponse
					responseCount++
					t.Logf("Received response %d with %d status items", responseCount, len(releaseResp.Data))
					if len(releaseResp.Data) > 0 && got == nil {
						// Get the first status response for comparison
						got = releaseResp.Data[0]
					}
				}

				require.GreaterOrEqual(t, responseCount, 1, "Release() expected at least 1 response")

				if got.GetStatus() != tt.want.GetStatus() || got.GetState() != tt.want.GetState() {
					t.Errorf("Release() got = %v, want %v", got, tt.want)
				}

				if got.GetId() != n.GetID() {
					t.Errorf("Release() expecting notification id to be reused")
				}
			})
		}

	})
}

func (nts *NotificationTestSuite) Test_notificationBusiness_Search() {

	templateExtra, _ := structpb.NewStruct(map[string]any{"template_id": "9bsv0s23l8og00vemail"})

	testcases := []struct {
		name       string
		search     *commonv1.SearchRequest
		leastCount int
		wantErr    bool
	}{
		{
			name:       "Normal Search",
			leastCount: 1,
			search:     &commonv1.SearchRequest{Query: ""},
			wantErr:    false,
		},
		{
			name:       "Search by template",
			leastCount: 1,
			search:     &commonv1.SearchRequest{Query: "", Extras: templateExtra},
			wantErr:    false,
		},
	}

	nts.WithTestDependancies(nts.T(), func(t *testing.T, dep *definition.DependencyOption) {

		_, ctx, resources := nts.CreateService(t, dep)

		for _, tt := range testcases {
			t.Run(tt.name, func(t *testing.T) {

				nStatus := models.NotificationStatus{
					State:  int32(commonv1.STATE_ACTIVE.Number()),
					Status: int32(commonv1.STATUS_QUEUED.Number()),
				}

				nStatus.GenID(ctx)

				n := models.Notification{
					SenderContactID:  "epochTesting",
					Message:          "Hello we are just testing statuses out",
					NotificationType: "email",
					State:            int32(commonv1.STATE_ACTIVE.Number()),
					LanguageID:       "9bsv0s23l8og00vgjqa0",
					TemplateID:       "9bsv0s23l8og00vemail",
					StatusID:         nStatus.GetID(),
				}
				n.PartitionID = "test_partition-id"

				err := resources.NotificationRepo.Create(ctx, &n)
				if err != nil {
					t.Errorf("Search() error = %v could not store a notification", err)
					return
				}

				nStatus.NotificationID = n.GetID()

				err = resources.NotificationStatusRepo.Create(ctx, &nStatus)
				if err != nil {
					t.Errorf("Search() error = %v could not store a notification status", err)
					return
				}

				var notifications []*notificationv1.Notification
				err = resources.NotificationBusiness.Search(ctx, tt.search,
					func(_ context.Context, batch []*notificationv1.Notification) error {
						notifications = append(notifications, batch...)
						return nil
					})

				if (err != nil) != tt.wantErr {
					t.Errorf("Search() error = %v, wantErr %v", err, tt.wantErr)
					return
				}

				// Consume the results from the JobResultPipe with timeout
				resultCount := len(notifications)
				t.Logf("Received batch of %d notifications, total so far: %d", len(notifications), resultCount)

				require.GreaterOrEqual(t, resultCount, tt.leastCount, "Search() expected at least %d results", tt.leastCount)
			})
		}

	})
}

func (nts *NotificationTestSuite) Test_notificationBusiness_Status() {

	testcases := []struct {
		name      string
		statusReq *commonv1.StatusRequest
		want      *commonv1.StatusResponse
		wantErr   bool
	}{
		{name: "NormalStatus",
			statusReq: &commonv1.StatusRequest{
				Id: "testingQueue_out",
			},
			wantErr: false,
			want: &commonv1.StatusResponse{
				Id:     "c2f4j7au6s7f91uqnojg",
				State:  commonv1.STATE_DELETED,
				Status: commonv1.STATUS_FAILED,
			},
		},
	}

	nts.WithTestDependancies(nts.T(), func(t *testing.T, dep *definition.DependencyOption) {

		_, ctx, resources := nts.CreateService(t, dep)

		for _, tt := range testcases {

			t.Run(tt.name, func(t *testing.T) {

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

				err := resources.NotificationRepo.Create(ctx, &n)
				if err != nil {
					t.Errorf("Status() error = %v could not store a notification for status checking", err)
					return
				}

				nStatus.NotificationID = n.GetID()
				err = resources.NotificationStatusRepo.Create(ctx, &nStatus)
				if err != nil {
					t.Errorf("Status() error = %v could not store a notification Status for status checking", err)
					return
				}

				tt.statusReq.Id = n.GetID()

				got, err := resources.NotificationBusiness.Status(ctx, tt.statusReq)
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

	})
}

func (nts *NotificationTestSuite) Test_notificationBusiness_StatusUpdate() {

	testcases := []struct {
		name      string
		statusReq *commonv1.StatusUpdateRequest
		want      *commonv1.StatusResponse
		wantErr   bool
	}{
		{name: "NormalStatusUpdate",
			statusReq: &commonv1.StatusUpdateRequest{
				Id:         "testingQueue_out",
				State:      commonv1.STATE_INACTIVE,
				Status:     commonv1.STATUS_SUCCESSFUL,
				ExternalId: "total_externalisation",
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

	nts.WithTestDependancies(nts.T(), func(t *testing.T, dep *definition.DependencyOption) {

		_, ctx, resources := nts.CreateService(t, dep)

		for _, tt := range testcases {
			t.Run(tt.name, func(t *testing.T) {
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

				err := resources.NotificationRepo.Create(ctx, &n)
				if err != nil {
					t.Errorf("Status() error = %v could not store a notification for status checking", err)
					return
				}

				tt.statusReq.Id = n.GetID()

				got, err := resources.NotificationBusiness.StatusUpdate(ctx, tt.statusReq)
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

	})
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
//		profileCli  profilev1connect.ProfileServiceClient
//		partitionCl partitionv1connect.PartitionServiceClient
//		resultCount int
//	}
//	type args struct {
//		search *notificationv1.TemplateSearchRequest
//	}
//	testcases := []struct {
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
//				search: &notificationv1.TemplateSearchRequest{Query: "Normal Search"},
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
//				search: &notificationv1.TemplateSearchRequest{Query: "alien cryptic template"},
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
//				search: &notificationv1.TemplateSearchRequest{
//					Query: "",
//					Page:  0,
//					Count: 3,
//				},
//			},
//			wantErr: false,
//		},
//	}
//	for _, tt := range testcases {
//		t.Run(tt.name, func(t *testing.T) {
//
//			nsSs := notificationv1.NewMockServerStream(ctrl)
//			nsSs.EXPECT().Context().Return(ctx).AnyTimes()
//			nsSs.EXPECT().Send(gomock.Any()).MinTimes(1).DoAndReturn(
//				func(arg *notificationv1.TemplateSearchResponse) any {
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
//				ctx,
//				svc,
//				profileCli,
//				partitionCl)
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
//			templateRepository := repository.NewTemplateRepository(ctx, svc)
//			err = templateRepository.Save(ctx, &template)
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
//			templateDataRepository := repository.NewTemplateDataRepository(ctx, svc)
//
//			err = templateDataRepository.Save(ctx, &templateData)
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
