package handlers

import (
	"antinvestor.com/service/notification/grpc/notification"
	"antinvestor.com/service/notification/service/business"
	"antinvestor.com/service/notification/utils"
	"context"
	"github.com/opentracing/opentracing-go"
)

type Notificationserver struct {
	Env    *utils.Env
}

// Out method act after income request let out notification
func (server *Notificationserver) Out(ctxt context.Context, req *notification.MessageOut) (*notification.StatusResponse, error) {

	span, ctx := opentracing.StartSpanFromContext(ctxt, "Message Send")
	defer span.Finish()

	notificationBusiness := business.NewNotificationBusiness(ctx, server.Env)
	return notificationBusiness.QueueOut(ctx, "#", req)

}

// Status
func (server *Notificationserver) Status(ctxt context.Context, req *notification.StatusRequest) (*notification.StatusResponse, error) {

	span, ctx := opentracing.StartSpanFromContext(ctxt, "Message Status")
	defer span.Finish()

	notificationBusiness := business.NewNotificationBusiness(ctx, server.Env)
	return notificationBusiness.Status(ctx, "", req)

}

//Release method that is called for messages queued for release
func (server *Notificationserver) Release(ctxt context.Context, req *notification.ReleaseRequest) (*notification.StatusResponse, error) {

	span, ctx := opentracing.StartSpanFromContext(ctxt, "Message Release")
	defer span.Finish()

	notificationBusiness := business.NewNotificationBusiness(ctx, server.Env)
	return notificationBusiness.Release(ctx, "", req)
}

//In method call for income rquest of any notification
func (server *Notificationserver) In(ctxt context.Context, req *notification.MessageIn) (*notification.StatusResponse, error) {

	span, ctx := opentracing.StartSpanFromContext(ctxt, "Message Receive")
	defer span.Finish()

	notificationBusiness := business.NewNotificationBusiness(ctx, server.Env)
	return notificationBusiness.QueueIn(ctx, req)
}

func (server *Notificationserver) Search(req *notification.SearchRequest, stream notification.NotificationService_SearchServer) error {

	span, ctx := opentracing.StartSpanFromContext(stream.Context(), "Message Search")
	defer span.Finish()

	notificationBusiness := business.NewNotificationBusiness(ctx, server.Env)
	return notificationBusiness.Search(ctx, "", req, stream)

}

