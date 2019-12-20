package business

import (
	"antinvestor.com/service/notification/grpc/notification"
	"antinvestor.com/service/notification/service/repository"
	"antinvestor.com/service/notification/utils"
	"context"
)

type NotificationBusiness interface {
	QueueOut(ctx context.Context, productID string, out *notification.MessageOut) (*notification.StatusResponse, error)
	QueueIn(ctx context.Context, in *notification.MessageIn) (*notification.StatusResponse, error)
	Status(ctx context.Context, productID string, status *notification.StatusRequest) (*notification.StatusResponse, error)
	Release(ctx context.Context, productID string, status *notification.ReleaseRequest) (*notification.StatusResponse, error)
	Search(ctx context.Context, productID string, search *notification.SearchRequest, stream notification.NotificationService_SearchServer) error
}

func NewNotificationBusiness(ctx context.Context, env *utils.Env) NotificationBusiness {
	notificationRepository := repository.NewRepository(ctx, env)
	return &notificationBusiness{env:env, notificationRepository: notificationRepository}
}
