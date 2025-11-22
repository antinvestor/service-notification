package handlers

import (
	"context"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	"buf.build/gen/go/antinvestor/notification/connectrpc/go/notification/v1/notificationv1connect"
	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"connectrpc.com/connect"
	"github.com/antinvestor/service-notification/apps/default/service/business"
	"github.com/antinvestor/service-notification/internal/apperrors"
	"github.com/pitabwire/frame/workerpool"
)

type NotificationServer struct {
	workMan              workerpool.Manager
	notificationBusiness business.NotificationBusiness

	notificationv1connect.UnimplementedNotificationServiceHandler
}

// NewNotificationServer creates a new NotificationServer.
func NewNotificationServer(
	workMan workerpool.Manager,
	notificationBusiness business.NotificationBusiness,
) notificationv1connect.NotificationServiceHandler {
	return &NotificationServer{
		workMan:              workMan,
		notificationBusiness: notificationBusiness,
	}
}

// Send method for queueing massages as requested
func (ns *NotificationServer) Send(ctx context.Context, req *connect.Request[notificationv1.SendRequest], stream *connect.ServerStream[notificationv1.SendResponse]) error {
	jobResultChannelList := make(chan any, len(req.Msg.GetData()))

	for _, data := range req.Msg.GetData() {

		job := workerpool.NewJob(func(ctx context.Context, result workerpool.JobResultPipe[*commonv1.StatusResponse]) error {
			resp, jobErr := ns.notificationBusiness.QueueOut(ctx, data)
			if jobErr != nil {
				jobResultChannelList <- jobErr
				return nil
			}

			jobResultChannelList <- resp
			return nil
		})

		err := workerpool.SubmitJob(ctx, ns.workMan, job)
		if err != nil {
			return apperrors.CleanErr(err)
		}
	}

	var responses []*commonv1.StatusResponse
	for range len(req.Msg.GetData()) {
		resp := <-jobResultChannelList
		switch v := resp.(type) {
		case error:
			err := v
			if err != nil {
				return err
			}
		case *commonv1.StatusResponse:
			responses = append(responses, v)

		}
	}

	return stream.Send(&notificationv1.SendResponse{Data: responses})
}

// Status request to determine if notification is prepared or released
func (ns *NotificationServer) Status(ctx context.Context, req *connect.Request[commonv1.StatusRequest]) (*connect.Response[commonv1.StatusResponse], error) {
	resp, err := ns.notificationBusiness.Status(ctx, req.Msg)
	if err != nil {
		return nil, apperrors.CleanErr(err)
	}
	return connect.NewResponse(resp), nil
}

// StatusUpdate request to allow continuation of notification processing
func (ns *NotificationServer) StatusUpdate(ctx context.Context, req *connect.Request[commonv1.StatusUpdateRequest]) (*connect.Response[commonv1.StatusUpdateResponse], error) {
	response, err := ns.notificationBusiness.StatusUpdate(ctx, req.Msg)
	if err != nil {
		return nil, apperrors.CleanErr(err)
	}

	return connect.NewResponse(&commonv1.StatusUpdateResponse{Data: response}), nil
}

// Release method for releasing queued massages and returns if notification status if released
func (ns *NotificationServer) Release(ctx context.Context, req *connect.Request[notificationv1.ReleaseRequest], stream *connect.ServerStream[notificationv1.ReleaseResponse]) error {
	result, err := ns.notificationBusiness.Release(ctx, req.Msg)
	if err != nil {
		return apperrors.CleanErr(err)
	}

	for {

		res, ok := result.ReadResult(ctx)
		if !ok {
			return nil
		}

		if res.IsError() {
			return apperrors.CleanErr(res.Error())
		}

		err = stream.Send(res.Item())
		if err != nil {
			return apperrors.CleanErr(err)
		}
	}

}

// Receive method is for client request for particular notification responses from system
func (ns *NotificationServer) Receive(ctx context.Context, req *connect.Request[notificationv1.ReceiveRequest], stream *connect.ServerStream[notificationv1.ReceiveResponse]) error {
	jobResultChannelList := make(chan any, len(req.Msg.GetData()))

	for _, data := range req.Msg.GetData() {

		job := workerpool.NewJob(func(ctx context.Context, result workerpool.JobResultPipe[*commonv1.StatusResponse]) error {
			resp, jobErr := ns.notificationBusiness.QueueIn(ctx, data)
			if jobErr != nil {

				jobResultChannelList <- jobErr
				return nil
			}

			jobResultChannelList <- resp
			return nil
		})

		err := workerpool.SubmitJob(ctx, ns.workMan, job)
		if err != nil {
			return apperrors.CleanErr(err)
		}
	}

	var responses []*commonv1.StatusResponse
	for range len(req.Msg.GetData()) {
		resp := <-jobResultChannelList
		switch v := resp.(type) {
		case error:
			err := v
			if err != nil {
				return err
			}
		case *commonv1.StatusResponse:
			responses = append(responses, v)

		}
	}

	return stream.Send(&notificationv1.ReceiveResponse{Data: responses})
}

// Search method is for client request for particular notification details from system
func (ns *NotificationServer) Search(ctx context.Context, req *connect.Request[commonv1.SearchRequest], stream *connect.ServerStream[notificationv1.SearchResponse]) error {
	result, err := ns.notificationBusiness.Search(ctx, req.Msg)
	if err != nil {
		return apperrors.CleanErr(err)
	}

	for {
		res, ok := result.ReadResult(ctx)
		if !ok {
			return nil
		}

		if res.IsError() {
			return apperrors.CleanErr(res.Error())
		}

		err = stream.Send(&notificationv1.SearchResponse{Data: res.Item()})
		if err != nil {
			return apperrors.CleanErr(err)
		}
	}
}

// TemplateSearch method is for client request for templates matching criteria from system
func (ns *NotificationServer) TemplateSearch(ctx context.Context, req *connect.Request[notificationv1.TemplateSearchRequest], stream *connect.ServerStream[notificationv1.TemplateSearchResponse]) error {
	result, err := ns.notificationBusiness.TemplateSearch(ctx, req.Msg)
	if err != nil {
		return apperrors.CleanErr(err)
	}

	for {
		res, ok := result.ReadResult(ctx)
		if !ok {
			return nil
		}

		if res.IsError() {
			return apperrors.CleanErr(res.Error())
		}

		err = stream.Send(&notificationv1.TemplateSearchResponse{Data: res.Item()})
		if err != nil {
			return apperrors.CleanErr(err)
		}
	}
}

func (ns *NotificationServer) TemplateSave(ctx context.Context, req *connect.Request[notificationv1.TemplateSaveRequest]) (*connect.Response[notificationv1.TemplateSaveResponse], error) {
	response, err := ns.notificationBusiness.TemplateSave(ctx, req.Msg)

	if err != nil {
		return nil, apperrors.CleanErr(err)
	}

	return connect.NewResponse(&notificationv1.TemplateSaveResponse{Data: response}), nil
}
