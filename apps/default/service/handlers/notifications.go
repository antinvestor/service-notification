package handlers

import (
	"context"
	"github.com/antinvestor/service-notification/apps/default/service/business"

	commonv1 "github.com/antinvestor/apis/go/common/v1"
	notificationV1 "github.com/antinvestor/apis/go/notification/v1"
	partitionv1 "github.com/antinvestor/apis/go/partition/v1"
	profileV1 "github.com/antinvestor/apis/go/profile/v1"
	"github.com/pitabwire/frame"
	"google.golang.org/grpc"
)

type NotificationServer struct {
	Service      *frame.Service
	ProfileCli   *profileV1.ProfileClient
	PartitionCli *partitionv1.PartitionClient

	notificationV1.UnimplementedNotificationServiceServer
}

func (ns *NotificationServer) newNotificationBusiness(ctx context.Context) (business.NotificationBusiness, error) {
	return business.NewNotificationBusiness(ctx, ns.Service, ns.ProfileCli, ns.PartitionCli)
}

// Send method for queueing massages as requested
func (ns *NotificationServer) Send(req *notificationV1.SendRequest, stream grpc.ServerStreamingServer[notificationV1.SendResponse]) error {
	ctx := stream.Context()
	notificationBusiness, err := ns.newNotificationBusiness(ctx)
	if err != nil {
		return err
	}

	jobResultChannelList := make(chan any, len(req.GetData()))

	for _, data := range req.GetData() {

		job := frame.NewJob(func(ctx context.Context, result frame.JobResultPipe) error {
			resp, jobErr := notificationBusiness.QueueOut(ctx, data)
			if jobErr != nil {

				jobResultChannelList <- jobErr
				return nil
			}

			jobResultChannelList <- resp
			return nil
		})

		err = frame.SubmitJob(ctx, ns.Service, job)
		if err != nil {
			return err
		}
	}

	var responses []*commonv1.StatusResponse
	for range len(req.GetData()) {
		resp := <-jobResultChannelList
		switch v := resp.(type) {
		case error:
			err = v
		case *commonv1.StatusResponse:
			responses = append(responses, v)

		}
	}

	if err != nil {
		return err
	}

	err = stream.Send(&notificationV1.SendResponse{Data: responses})
	if err != nil {
		return err
	}
	return nil

}

// Status request to determine if notification is prepared or released
func (ns *NotificationServer) Status(ctx context.Context, req *commonv1.StatusRequest) (*commonv1.StatusResponse, error) {

	notificationBusiness, err := ns.newNotificationBusiness(ctx)
	if err != nil {
		return nil, err
	}
	return notificationBusiness.Status(ctx, req)
}

// StatusUpdate request to allow continuation of notification processing
func (ns *NotificationServer) StatusUpdate(ctx context.Context, req *commonv1.StatusUpdateRequest) (*commonv1.StatusUpdateResponse, error) {

	notificationBusiness, err := ns.newNotificationBusiness(ctx)
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
func (ns *NotificationServer) Release(req *notificationV1.ReleaseRequest, stream grpc.ServerStreamingServer[notificationV1.ReleaseResponse]) error {
	ctx := stream.Context()
	notificationBusiness, err := ns.newNotificationBusiness(ctx)
	if err != nil {
		return err
	}
	err = notificationBusiness.Release(ctx, req, stream)
	if err != nil {
		return err
	}

	return nil
}

// Receive method is for client request for particular notification responses from system
func (ns *NotificationServer) Receive(req *notificationV1.ReceiveRequest, stream grpc.ServerStreamingServer[notificationV1.ReceiveResponse]) error {

	ctx := stream.Context()
	notificationBusiness, err := ns.newNotificationBusiness(ctx)
	if err != nil {
		return err
	}

	jobResultChannelList := make(chan any, len(req.GetData()))

	for _, data := range req.GetData() {

		job := frame.NewJob(func(ctx context.Context, result frame.JobResultPipe) error {
			resp, jobErr := notificationBusiness.QueueIn(ctx, data)
			if jobErr != nil {

				jobResultChannelList <- jobErr
				return nil
			}

			jobResultChannelList <- resp
			return nil
		})

		err = frame.SubmitJob(ctx, ns.Service, job)
		if err != nil {
			return err
		}
	}

	var responses []*commonv1.StatusResponse
	for range len(req.GetData()) {
		resp := <-jobResultChannelList
		switch v := resp.(type) {
		case error:
			err = v
		case *commonv1.StatusResponse:
			responses = append(responses, v)

		}
	}

	if err != nil {
		return err
	}

	err = stream.Send(&notificationV1.ReceiveResponse{Data: responses})
	if err != nil {
		return err
	}
	return nil

}

// Search method is for client request for particular notification details from system
func (ns *NotificationServer) Search(req *commonv1.SearchRequest, stream grpc.ServerStreamingServer[notificationV1.SearchResponse]) error {

	notificationBusiness, err := ns.newNotificationBusiness(stream.Context())
	if err != nil {
		return err
	}
	return notificationBusiness.Search(req, stream)

}

// TemplateSearch method is for client request for templates matching criteria from system
func (ns *NotificationServer) TemplateSearch(req *notificationV1.TemplateSearchRequest, stream grpc.ServerStreamingServer[notificationV1.TemplateSearchResponse]) error {

	notificationBusiness, err := ns.newNotificationBusiness(stream.Context())
	if err != nil {
		return err
	}
	return notificationBusiness.TemplateSearch(req, stream)

}

func (ns *NotificationServer) TemplateSave(ctx context.Context, req *notificationV1.TemplateSaveRequest) (*notificationV1.TemplateSaveResponse, error) {
	notificationBusiness, err := ns.newNotificationBusiness(ctx)
	if err != nil {
		return nil, err
	}
	response, err := notificationBusiness.TemplateSave(ctx, req)

	if err != nil {
		return nil, err
	}

	return &notificationV1.TemplateSaveResponse{Data: response}, nil
}
