package events

import (
	"context"
	notificationV1 "github.com/antinvestor/apis/notification/v1"
	"github.com/antinvestor/service-notification/config"
	"github.com/antinvestor/service-notification/service/models"
	"github.com/antinvestor/service-notification/service/repository"
	"github.com/pitabwire/frame"
	"testing"
)

func getService(serviceName string) fields {
	dbURL := frame.GetEnv("TEST_DATABASE_URL", "postgres://ant:secret@localhost:5436/service_notification?sslmode=disable")
	testDb := frame.DatastoreCon(dbURL, false)

	var ncfg config.NotificationConfig
	_ = frame.ConfigProcess("", &ncfg)

	ctx, service := frame.NewService(serviceName, testDb, frame.Config(&ncfg), frame.NoopDriver())

	eventList := frame.RegisterEvents(
		&NotificationSave{Service: service},
		&NotificationStatusSave{Service: service})
	service.Init(eventList)
	_ = service.Run(ctx, "")
	return fields{
		Ctx:     ctx,
		Service: service,
	}
}

type fields struct {
	Ctx     context.Context
	Service *frame.Service
}

func TestNotificationSave_Execute(t *testing.T) {

	ctx := context.Background()

	type args struct {
		ctx     context.Context
		payload interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name:   "Successful save",
			fields: getService("NotificationSaveTest"),
			args: args{
				ctx: ctx,
				payload: &models.Notification{
					BaseModel: frame.BaseModel{
						ID:          "testingSaveId",
						TenantID:    "tenantData",
						PartitionID: "partitionData",
						AccessID:    "testingAccessData",
					},
					ContactID: "epochTesting",
					OutBound:  true,
					Message:   "Hello we are just testing things out",
					Priority:  int32(notificationV1.PRIORITY_LOW),
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &NotificationSave{
				Service: tt.fields.Service,
			}
			if err := e.Execute(tt.args.ctx, tt.args.payload); (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}

			nRepo := repository.NewNotificationRepository(ctx, tt.fields.Service)
			n, err := nRepo.GetByID("testingSaveId")
			if err != nil {
				t.Errorf("Search() error = %v could not retrieve saved notification", err)
				return
			}

			if n == nil {
				t.Errorf("Matching notification could not be found")
				return
			}

		})
	}
}
