package handlers

import (
	"context"

	napi "github.com/antinvestor/service-notification-api"
	"github.com/antinvestor/service-notification/service/business"
	papi "github.com/antinvestor/service-profile-api"
	"github.com/pitabwire/frame"
)

type NotificationServer struct {
	Service    *frame.Service
	ProfileCli *papi.ProfileClient

	napi.UnimplementedNotificationServiceServer
}

func (server *NotificationServer) newNotificationBusiness(ctx context.Context) business.NotificationBusiness {
	return business.NewNotificationBusiness(ctx, server.Service, server.ProfileCli)
}

//Send method for queueing massages as requested
func (server *NotificationServer) Send(ctx context.Context, req *napi.Notification) (*napi.StatusResponse, error) {

	notificationBusiness := server.newNotificationBusiness(ctx)
	return notificationBusiness.QueueOut(ctx, req)

}

//Status request to determine if notification is prepared or released
func (server *NotificationServer) Status(ctx context.Context, req *napi.StatusRequest) (*napi.StatusResponse, error) {

	notificationBusiness := server.newNotificationBusiness(ctx)
	return notificationBusiness.Status(ctx, req)

}

//StatusUpdate request to allow continuation of notification processing
func (server *NotificationServer) StatusUpdate(ctx context.Context, req *napi.StatusUpdateRequest) (*napi.StatusResponse, error) {

	notificationBusiness := server.newNotificationBusiness(ctx)
	return notificationBusiness.StatusUpdate(ctx, req)

}

//Release method for releasing queued massages and returns if notification status if released
func (server *NotificationServer) Release(ctx context.Context, req *napi.ReleaseRequest) (*napi.StatusResponse, error) {

	notificationBusiness := server.newNotificationBusiness(ctx)
	return notificationBusiness.Release(ctx, req)
}

//Receive method is for client request for particular notification responses from system
func (server *NotificationServer) Receive(ctx context.Context, req *napi.Notification) (*napi.StatusResponse, error) {

	notificationBusiness := server.newNotificationBusiness(ctx)
	return notificationBusiness.QueueIn(ctx, req)
}

//Search method is for client request for particular notification details from system
func (server *NotificationServer) Search(req *napi.SearchRequest, stream napi.NotificationService_SearchServer) error {

	notificationBusiness := server.newNotificationBusiness(stream.Context())
	return notificationBusiness.Search(req, stream)

}
