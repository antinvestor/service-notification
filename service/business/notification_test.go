package business

import (
	"context"
	"github.com/antinvestor/apis"
	notificationV1 "github.com/antinvestor/service-notification-api"
	"github.com/antinvestor/service-notification/service/repository"
	partitionV1 "github.com/antinvestor/service-partition-api"
	profileV1 "github.com/antinvestor/service-profile-api"
	"github.com/pitabwire/frame"
	"reflect"
	"testing"
)

func getService(ctx context.Context, service string) *frame.Service {
	dbUrl := frame.GetEnv("TEST_DATABASE_URL", "postgres://ant:secret@localhost:5432/service_notification?sslmode=disable")
	testDb := frame.Datastore(ctx, dbUrl, false)
	return frame.NewService(service, testDb)
}

func getProfileCli(ctx context.Context) (*profileV1.ProfileClient, error) {
	profileServiceUrl := frame.GetEnv("TEST_PROFILE_SERVICE_URI", "127.0.0.1:7005")
	return profileV1.NewProfileClient(ctx, apis.WithEndpoint(profileServiceUrl))
}

func getPartitionCli(ctx context.Context) (*partitionV1.PartitionClient, error) {
	partitionServiceUrl := frame.GetEnv("TEST_PARTITION_SERVICE_URI", "127.0.0.1:7003")
	return partitionV1.NewPartitionsClient(ctx, apis.WithEndpoint(partitionServiceUrl))
}

func TestNewNotificationBusiness(t *testing.T) {

	ctx := context.Background()

	profileCli, err := getProfileCli(ctx)
	if err != nil {
		t.Errorf("Could not setup profile client %v", err)
		return
	}

	partitionCli, err := getPartitionCli(ctx)
	if err != nil {
		t.Errorf("Could not setup profile client %v", err)
		return
	}

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
		// TODO: Add test cases.
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
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("QueueIn() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_notificationBusiness_QueueOut(t *testing.T) {
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
		// TODO: Add test cases.
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
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("QueueOut() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_notificationBusiness_Release(t *testing.T) {
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
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nb := &notificationBusiness{
				service:      tt.fields.service,
				profileCli:   tt.fields.profileCli,
				partitionCli: tt.fields.partitionCl,
			}
			got, err := nb.Release(tt.args.ctx, tt.args.releaseReq)
			if (err != nil) != tt.wantErr {
				t.Errorf("Release() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Release() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_notificationBusiness_Search(t *testing.T) {
	type fields struct {
		service     *frame.Service
		profileCli  *profileV1.ProfileClient
		partitionCl *partitionV1.PartitionClient
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
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nb := &notificationBusiness{
				service:      tt.fields.service,
				profileCli:   tt.fields.profileCli,
				partitionCli: tt.fields.partitionCl,
			}
			if err := nb.Search(tt.args.search, tt.args.stream); (err != nil) != tt.wantErr {
				t.Errorf("Search() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_notificationBusiness_Status(t *testing.T) {
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
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nb := &notificationBusiness{
				service:      tt.fields.service,
				profileCli:   tt.fields.profileCli,
				partitionCli: tt.fields.partitionCl,
			}
			got, err := nb.Status(tt.args.ctx, tt.args.statusReq)
			if (err != nil) != tt.wantErr {
				t.Errorf("Status() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Status() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_notificationBusiness_StatusUpdate(t *testing.T) {
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
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nb := &notificationBusiness{
				service:      tt.fields.service,
				profileCli:   tt.fields.profileCli,
				partitionCli: tt.fields.partitionCl,
			}
			got, err := nb.StatusUpdate(tt.args.ctx, tt.args.statusReq)
			if (err != nil) != tt.wantErr {
				t.Errorf("StatusUpdate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("StatusUpdate() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_notificationBusiness_getChannelRepo(t *testing.T) {
	type fields struct {
		service     *frame.Service
		profileCli  *profileV1.ProfileClient
		partitionCl *partitionV1.PartitionClient
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   repository.RouteRepository
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nb := &notificationBusiness{
				service:      tt.fields.service,
				profileCli:   tt.fields.profileCli,
				partitionCli: tt.fields.partitionCl,
			}
			if got := nb.getChannelRepo(tt.args.ctx); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getChannelRepo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_notificationBusiness_getLanguageRepo(t *testing.T) {
	type fields struct {
		service     *frame.Service
		profileCli  *profileV1.ProfileClient
		partitionCl *partitionV1.PartitionClient
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   repository.LanguageRepository
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nb := &notificationBusiness{
				service:      tt.fields.service,
				profileCli:   tt.fields.profileCli,
				partitionCli: tt.fields.partitionCl,
			}
			if got := nb.getLanguageRepo(tt.args.ctx); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getLanguageRepo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_notificationBusiness_getNotificationRepo(t *testing.T) {
	type fields struct {
		service     *frame.Service
		profileCli  *profileV1.ProfileClient
		partitionCl *partitionV1.PartitionClient
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   repository.NotificationRepository
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nb := &notificationBusiness{
				service:      tt.fields.service,
				profileCli:   tt.fields.profileCli,
				partitionCli: tt.fields.partitionCl,
			}
			if got := nb.getNotificationRepo(tt.args.ctx); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getNotificationRepo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_notificationBusiness_getPartitionData(t *testing.T) {
	type fields struct {
		service     *frame.Service
		profileCli  *profileV1.ProfileClient
		partitionCl *partitionV1.PartitionClient
	}
	type args struct {
		ctx      context.Context
		accessId string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    frame.BaseModel
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nb := &notificationBusiness{
				service:      tt.fields.service,
				profileCli:   tt.fields.profileCli,
				partitionCli: tt.fields.partitionCl,
			}
			got, err := nb.getPartitionData(tt.args.ctx, tt.args.accessId)
			if (err != nil) != tt.wantErr {
				t.Errorf("getPartitionData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getPartitionData() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_notificationBusiness_getTemplateRepo(t *testing.T) {
	type fields struct {
		service     *frame.Service
		profileCli  *profileV1.ProfileClient
		partitionCl *partitionV1.PartitionClient
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   repository.TemplateRepository
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nb := &notificationBusiness{
				service:      tt.fields.service,
				profileCli:   tt.fields.profileCli,
				partitionCli: tt.fields.partitionCl,
			}
			if got := nb.getTemplateRepo(tt.args.ctx); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getTemplateRepo() = %v, want %v", got, tt.want)
			}
		})
	}
}
