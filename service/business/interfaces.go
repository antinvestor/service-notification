package business

import (
	"context"
	partapi "github.com/antinvestor/service-partition-api"

	notificationV1 "github.com/antinvestor/service-notification-api"
	profileV1 "github.com/antinvestor/service-profile-api"
	"github.com/pitabwire/frame"
)

type NotificationBusiness interface {
	QueueOut(ctx context.Context, out *notificationV1.Notification) (*notificationV1.StatusResponse, error)
	QueueIn(ctx context.Context, in *notificationV1.Notification) (*notificationV1.StatusResponse, error)
	Status(ctx context.Context, status *notificationV1.StatusRequest) (*notificationV1.StatusResponse, error)
	StatusUpdate(ctx context.Context, req *notificationV1.StatusUpdateRequest) (*notificationV1.StatusResponse, error)
	Release(ctx context.Context, status *notificationV1.ReleaseRequest) (*notificationV1.StatusResponse, error)
	Search(search *notificationV1.SearchRequest, stream notificationV1.NotificationService_SearchServer) error
}

func NewNotificationBusiness(ctx context.Context, service *frame.Service, profileCli *profileV1.ProfileClient, partitionCli *partapi.PartitionClient) (NotificationBusiness, error) {

	if service == nil || profileCli == nil || partitionCli == nil {
		return nil, ErrorInitializationFail
	}

	return &notificationBusiness{
		service:      service,
		profileCli:   profileCli,
		partitionCli: partitionCli,
	}, nil
}
