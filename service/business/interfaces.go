package business

import (
	"context"
	n_api "github.com/antinvestor/service-notification-api"
	"github.com/antinvestor/service-notification/service/repository"
	"github.com/pitabwire/frame"
)

type NotificationBusiness interface {
	QueueOut(ctx context.Context, productID string, out *n_api.MessageOut) (*n_api.StatusResponse, error)
	QueueIn(ctx context.Context, in *n_api.MessageIn) (*n_api.StatusResponse, error)
	Status(ctx context.Context, productID string, status *n_api.StatusRequest) (*n_api.StatusResponse, error)
	Release(ctx context.Context, productID string, status *n_api.ReleaseRequest) (*n_api.StatusResponse, error)
	Search(ctx context.Context, productID string, search *n_api.SearchRequest, stream n_api.NotificationService_SearchServer) error
}

func NewNotificationBusiness(ctx context.Context, service *frame.Service) NotificationBusiness {
	notificationRepository := repository.NewNotificationRepository(ctx, service)
	languageRepository := repository.NewLanguageRepository(ctx, service)
	templateRepository := repository.NewTemplateRepository(ctx, service)
	return &notificationBusiness{service: service,
		notificationRepository: notificationRepository,
		languageRepository:     languageRepository,
		templateRepository:     templateRepository}
}
