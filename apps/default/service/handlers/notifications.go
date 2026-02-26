package handlers

import (
	"context"
	"errors"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	"buf.build/gen/go/antinvestor/notification/connectrpc/go/notification/v1/notificationv1connect"
	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"connectrpc.com/connect"
	"github.com/antinvestor/service-notification/apps/default/service/authz"
	"github.com/antinvestor/service-notification/apps/default/service/business"
	"github.com/antinvestor/service-notification/internal/apperrors"
	"github.com/pitabwire/frame/security/authorizer"
	"github.com/pitabwire/frame/workerpool"
	"github.com/pitabwire/util"
)

type NotificationServer struct {
	authz                authz.Middleware
	workMan              workerpool.Manager
	notificationBusiness business.NotificationBusiness

	notificationv1connect.UnimplementedNotificationServiceHandler
}

// NewNotificationServer creates a new NotificationServer.
func NewNotificationServer(
	workMan workerpool.Manager,
	notificationBusiness business.NotificationBusiness,
	authzMiddleware authz.Middleware,
) notificationv1connect.NotificationServiceHandler {
	return &NotificationServer{
		authz:                authzMiddleware,
		workMan:              workMan,
		notificationBusiness: notificationBusiness,
	}
}

func toConnectError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, authorizer.ErrInvalidSubject) || errors.Is(err, authorizer.ErrInvalidObject) {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	var permErr *authorizer.PermissionDeniedError
	if errors.As(err, &permErr) {
		return connect.NewError(connect.CodePermissionDenied, err)
	}

	return connect.NewError(connect.CodeInternal, err)
}

// Send method for queueing massages as requested
func (ns *NotificationServer) Send(ctx context.Context, req *connect.Request[notificationv1.SendRequest], stream *connect.ServerStream[notificationv1.SendResponse]) error {
	if err := ns.authz.CanSendNotification(ctx); err != nil {
		return toConnectError(err)
	}

	logger := util.Log(ctx).WithField("method", "Send")

	logger.Info("starting notification send request processing")

	var responses []*commonv1.StatusResponse
	data := req.Msg.GetData()

	logger.WithField("item_count", len(data)).Info("processing notification send request")

	for i, notification := range data {
		logger.WithFields(map[string]interface{}{
			"item_index":      i,
			"notification_id": notification.GetId(),
		}).Debug("processing notification item")

		resp, err := ns.notificationBusiness.QueueOut(ctx, notification)
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"item_index":      i,
				"notification_id": notification.GetId(),
				"error":           err,
			}).Error("failed to queue notification")
			return apperrors.CleanErr(err)
		}

		logger.WithFields(map[string]interface{}{
			"item_index":      i,
			"notification_id": notification.GetId(),
			"response_id":     resp.GetId(),
		}).Debug("successfully queued notification")

		responses = append(responses, resp)
	}

	logger.WithField("response_count", len(responses)).Info("sending notification send response")

	err := stream.Send(&notificationv1.SendResponse{Data: responses})
	if err != nil {
		logger.WithError(err).Error("failed to send notification response")
		return apperrors.CleanErr(err)
	}

	logger.Info("completed notification send request processing")
	return nil
}

// Status request to determine if notification is prepared or released
func (ns *NotificationServer) Status(ctx context.Context, req *connect.Request[commonv1.StatusRequest]) (*connect.Response[commonv1.StatusResponse], error) {
	if err := ns.authz.CanViewNotificationStatus(ctx); err != nil {
		return nil, toConnectError(err)
	}

	resp, err := ns.notificationBusiness.Status(ctx, req.Msg)
	if err != nil {
		return nil, apperrors.CleanErr(err)
	}
	return connect.NewResponse(resp), nil
}

// StatusUpdate request to allow continuation of notification processing
func (ns *NotificationServer) StatusUpdate(ctx context.Context, req *connect.Request[commonv1.StatusUpdateRequest]) (*connect.Response[commonv1.StatusUpdateResponse], error) {
	if err := ns.authz.CanUpdateNotificationStatus(ctx); err != nil {
		return nil, toConnectError(err)
	}

	response, err := ns.notificationBusiness.StatusUpdate(ctx, req.Msg)
	if err != nil {
		return nil, apperrors.CleanErr(err)
	}

	return connect.NewResponse(&commonv1.StatusUpdateResponse{Data: response}), nil
}

// Release method for releasing queued massages and returns if notification status if released
func (ns *NotificationServer) Release(ctx context.Context, req *connect.Request[notificationv1.ReleaseRequest], stream *connect.ServerStream[notificationv1.ReleaseResponse]) error {
	if err := ns.authz.CanReleaseNotification(ctx); err != nil {
		return toConnectError(err)
	}

	result, err := ns.notificationBusiness.Release(ctx, req.Msg)
	if err != nil {
		return apperrors.CleanErr(err)
	}

	err = workerpool.ConsumeResultStream(ctx, result, func(res *notificationv1.ReleaseResponse) error {
		err = stream.Send(res)
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return apperrors.CleanErr(err)
	}

	return nil

}

// Receive method is for client request for particular notification responses from system
func (ns *NotificationServer) Receive(ctx context.Context, req *connect.Request[notificationv1.ReceiveRequest], stream *connect.ServerStream[notificationv1.ReceiveResponse]) error {
	if err := ns.authz.CanReleaseNotification(ctx); err != nil {
		return toConnectError(err)
	}

	var responses []*commonv1.StatusResponse
	for _, data := range req.Msg.GetData() {

		resp, err := ns.notificationBusiness.QueueIn(ctx, data)
		if err != nil {
			return apperrors.CleanErr(err)
		}

		responses = append(responses, resp)
	}

	return stream.Send(&notificationv1.ReceiveResponse{Data: responses})
}

// Search method is for client request for particular notification details from system
func (ns *NotificationServer) Search(ctx context.Context, req *connect.Request[commonv1.SearchRequest], stream *connect.ServerStream[notificationv1.SearchResponse]) error {
	if err := ns.authz.CanSearchNotifications(ctx); err != nil {
		return toConnectError(err)
	}

	err := ns.notificationBusiness.Search(ctx, req.Msg,
		func(_ context.Context, batch []*notificationv1.Notification) error {
			return stream.Send(&notificationv1.SearchResponse{Data: batch})
		})
	if err != nil {
		return apperrors.CleanErr(err)
	}
	return nil
}

// TemplateSearch method is for client request for templates matching criteria from system
func (ns *NotificationServer) TemplateSearch(ctx context.Context, req *connect.Request[notificationv1.TemplateSearchRequest], stream *connect.ServerStream[notificationv1.TemplateSearchResponse]) error {
	if err := ns.authz.CanViewTemplate(ctx); err != nil {
		return toConnectError(err)
	}

	err := ns.notificationBusiness.TemplateSearch(ctx, req.Msg,
		func(_ context.Context, batch []*notificationv1.Template) error {
			return stream.Send(&notificationv1.TemplateSearchResponse{Data: batch})
		})
	if err != nil {
		return apperrors.CleanErr(err)
	}

	return nil
}

func (ns *NotificationServer) TemplateSave(ctx context.Context, req *connect.Request[notificationv1.TemplateSaveRequest]) (*connect.Response[notificationv1.TemplateSaveResponse], error) {
	if err := ns.authz.CanManageTemplate(ctx); err != nil {
		return nil, toConnectError(err)
	}

	response, err := ns.notificationBusiness.TemplateSave(ctx, req.Msg)

	if err != nil {
		return nil, apperrors.CleanErr(err)
	}

	return connect.NewResponse(&notificationv1.TemplateSaveResponse{Data: response}), nil
}
