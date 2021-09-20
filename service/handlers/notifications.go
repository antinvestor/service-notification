package handlers

import (
	"context"
	partapi "github.com/antinvestor/service-partition-api"

	notificationV1 "github.com/antinvestor/service-notification-api"
	"github.com/antinvestor/service-notification/service/business"
	profileV1 "github.com/antinvestor/service-profile-api"
	"github.com/pitabwire/frame"
)

type NotificationServer struct {
	Service    *frame.Service
	ProfileCli *profileV1.ProfileClient
	PartitionCli *partapi.PartitionClient

	notificationV1.UnimplementedNotificationServiceServer
}

func (server *NotificationServer) newNotificationBusiness(ctx context.Context) (business.NotificationBusiness,error) {
	return business.NewNotificationBusiness(ctx, server.Service, server.ProfileCli, server.PartitionCli)
}

//Send method for queueing massages as requested
func (server *NotificationServer) Send(ctx context.Context, req *notificationV1.Notification) (*notificationV1.StatusResponse, error) {

	notificationBusiness, err := server.newNotificationBusiness(ctx)
	if err != nil {
		return nil, err
	}
	return notificationBusiness.QueueOut(ctx, req)

}

//Status request to determine if notification is prepared or released
func (server *NotificationServer) Status(ctx context.Context, req *notificationV1.StatusRequest) (*notificationV1.StatusResponse, error) {

	notificationBusiness, err := server.newNotificationBusiness(ctx)
	if err != nil {
		return nil, err
	}
	return notificationBusiness.Status(ctx, req)

}

//StatusUpdate request to allow continuation of notification processing
func (server *NotificationServer) StatusUpdate(ctx context.Context, req *notificationV1.StatusUpdateRequest) (*notificationV1.StatusResponse, error) {

	notificationBusiness, err := server.newNotificationBusiness(ctx)
	if err != nil {
		return nil, err
	}
	return notificationBusiness.StatusUpdate(ctx, req)

}

//Release method for releasing queued massages and returns if notification status if released
func (server *NotificationServer) Release(ctx context.Context, req *notificationV1.ReleaseRequest) (*notificationV1.StatusResponse, error) {

	notificationBusiness, err := server.newNotificationBusiness(ctx)
	if err != nil {
		return nil, err
	}
	return notificationBusiness.Release(ctx, req)
}

//Receive method is for client request for particular notification responses from system
func (server *NotificationServer) Receive(ctx context.Context, req *notificationV1.Notification) (*notificationV1.StatusResponse, error) {

	notificationBusiness, err := server.newNotificationBusiness(ctx)
	if err != nil {
		return nil, err
	}
	return notificationBusiness.QueueIn(ctx, req)
}

//Search method is for client request for particular notification details from system
func (server *NotificationServer) Search(req *notificationV1.SearchRequest, stream notificationV1.NotificationService_SearchServer) error {

	notificationBusiness, err := server.newNotificationBusiness(stream.Context())
	if err != nil {
		return err
	}
	return notificationBusiness.Search(req, stream)

}
