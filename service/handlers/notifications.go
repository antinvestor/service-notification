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

	napi.NotificationServiceServer
}


// Out method act after income request let out notification
func (server *NotificationServer) Out(ctx context.Context, req *napi.MessageOut) (*napi.StatusResponse, error) {

	notificationBusiness := business.NewNotificationBusiness(ctx, server.Service, server.ProfileCli)
	return notificationBusiness.QueueOut(ctx, "#", req)

}

// Status
func (server *NotificationServer) Status(ctx context.Context, req *napi.StatusRequest) (*napi.StatusResponse, error) {

	notificationBusiness := business.NewNotificationBusiness(ctx, server.Service, server.ProfileCli)
	return notificationBusiness.Status(ctx, "", req)

}

//Release method that is called for messages queued for release
func (server *NotificationServer) Release(ctx context.Context, req *napi.ReleaseRequest) (*napi.StatusResponse, error) {

	notificationBusiness := business.NewNotificationBusiness(ctx, server.Service, server.ProfileCli)
	return notificationBusiness.Release(ctx, "", req)
}

//In method call for income rquest of any notification
func (server *NotificationServer) In(ctx context.Context, req *napi.MessageIn) (*napi.StatusResponse, error) {

	notificationBusiness := business.NewNotificationBusiness(ctx, server.Service, server.ProfileCli)
	return notificationBusiness.QueueIn(ctx, req)
}

func (server *NotificationServer) Search(req *napi.SearchRequest, stream napi.NotificationService_SearchServer) error {

	ctx := stream.Context()
	notificationBusiness := business.NewNotificationBusiness(ctx, server.Service, server.ProfileCli)
	return notificationBusiness.Search(ctx, "", req, stream)

}
