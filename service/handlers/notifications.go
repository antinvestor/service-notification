package handlers

import (
	"context"
	commonv1 "github.com/antinvestor/apis/go/common/v1"
	partitionv1 "github.com/antinvestor/apis/go/partition/v1"

	notificationV1 "github.com/antinvestor/apis/go/notification/v1"
	profileV1 "github.com/antinvestor/apis/go/profile/v1"
	"github.com/antinvestor/service-notification/service/business"
	"github.com/pitabwire/frame"
)

type NotificationServer struct {
	Service      *frame.Service
	ProfileCli   *profileV1.ProfileClient
	PartitionCli *partitionv1.PartitionClient

	notificationV1.UnimplementedNotificationServiceServer
}

func (server *NotificationServer) newNotificationBusiness(ctx context.Context) (business.NotificationBusiness, error) {
	return business.NewNotificationBusiness(ctx, server.Service, server.ProfileCli, server.PartitionCli)
}

// Send method for queueing massages as requested
func (server *NotificationServer) Send(ctx context.Context, req *notificationV1.SendRequest) (*notificationV1.SendResponse, error) {

	notificationBusiness, err := server.newNotificationBusiness(ctx)
	if err != nil {
		return nil, err
	}
	response, err := notificationBusiness.QueueOut(ctx, req.GetData())
	if err != nil {
		return nil, err
	}

	return &notificationV1.SendResponse{Data: response}, nil

}

// Status request to determine if notification is prepared or released
func (server *NotificationServer) Status(ctx context.Context, req *commonv1.StatusRequest) (*commonv1.StatusResponse, error) {

	notificationBusiness, err := server.newNotificationBusiness(ctx)
	if err != nil {
		return nil, err
	}
	return notificationBusiness.Status(ctx, req)
}

// StatusUpdate request to allow continuation of notification processing
func (server *NotificationServer) StatusUpdate(ctx context.Context, req *commonv1.StatusUpdateRequest) (*commonv1.StatusUpdateResponse, error) {

	notificationBusiness, err := server.newNotificationBusiness(ctx)
	if err != nil {
		return nil, err
	}
	response, err := notificationBusiness.StatusUpdate(ctx, req)
	if err != nil {
		return nil, err
	}

	return &commonv1.StatusUpdateResponse{Data: response}, nil
}

// Release method for releasing queued massages and returns if notification status if released
func (server *NotificationServer) Release(ctx context.Context, req *notificationV1.ReleaseRequest) (*notificationV1.ReleaseResponse, error) {

	notificationBusiness, err := server.newNotificationBusiness(ctx)
	if err != nil {
		return nil, err
	}
	response, err := notificationBusiness.Release(ctx, req)

	if err != nil {
		return nil, err
	}

	return &notificationV1.ReleaseResponse{Data: response}, nil
}

// Receive method is for client request for particular notification responses from system
func (server *NotificationServer) Receive(ctx context.Context, req *notificationV1.ReceiveRequest) (*notificationV1.ReceiveResponse, error) {

	notificationBusiness, err := server.newNotificationBusiness(ctx)
	if err != nil {
		return nil, err
	}
	response, err := notificationBusiness.QueueIn(ctx, req.GetData())

	if err != nil {
		return nil, err
	}

	return &notificationV1.ReceiveResponse{Data: response}, nil
}

// Search method is for client request for particular notification details from system
func (server *NotificationServer) Search(req *commonv1.SearchRequest, stream notificationV1.NotificationService_SearchServer) error {

	notificationBusiness, err := server.newNotificationBusiness(stream.Context())
	if err != nil {
		return err
	}
	return notificationBusiness.Search(req, stream)

}
