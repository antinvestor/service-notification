package business

import (
	"context"

	napi "github.com/antinvestor/service-notification-api"
	papi "github.com/antinvestor/service-profile-api"
	"github.com/pitabwire/frame"
)

type NotificationBusiness interface {
	QueueOut(ctx context.Context, out *napi.Notification) (*napi.StatusResponse, error)
	QueueIn(ctx context.Context, in *napi.Notification) (*napi.StatusResponse, error)
	Status(ctx context.Context, status *napi.StatusRequest) (*napi.StatusResponse, error)
	StatusUpdate(ctx context.Context, req *napi.StatusUpdateRequest) (*napi.StatusResponse, error)
	Release(ctx context.Context, status *napi.ReleaseRequest) (*napi.StatusResponse, error)
	Search(search *napi.SearchRequest, stream napi.NotificationService_SearchServer) error
}

func NewNotificationBusiness(ctx context.Context, service *frame.Service, profileCli *papi.ProfileClient) NotificationBusiness {
	return &notificationBusiness{
		service:    service,
		profileCli: profileCli,
	}
}
