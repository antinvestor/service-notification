package business

import (
	"context"
	partitionv1 "github.com/antinvestor/apis/partition/v1"

	notificationV1 "github.com/antinvestor/apis/notification/v1"
	profileV1 "github.com/antinvestor/apis/profile/v1"
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

func NewNotificationBusiness(ctx context.Context, service *frame.Service, profileCli *profileV1.ProfileClient, partitionCli *partitionv1.PartitionClient) (NotificationBusiness, error) {

	if service == nil || profileCli == nil || partitionCli == nil {
		return nil, ErrorInitializationFail
	}

	return &notificationBusiness{
		service:      service,
		profileCli:   profileCli,
		partitionCli: partitionCli,
	}, nil
}
