package handlers

import (
	"context"
	n_api "github.com/antinvestor/service-notification-api"
	"github.com/antinvestor/service-notification/service/business"
	"github.com/opentracing/opentracing-go"
	"github.com/pitabwire/frame"
)

type Notificationserver struct {
	Service *frame.Service
	n_api.NotificationServiceServer
}


// Out method act after income request let out notification
func (server *Notificationserver) Out(ctxt context.Context, req *n_api.MessageOut) (*n_api.StatusResponse, error) {

	span, ctx := opentracing.StartSpanFromContext(ctxt, "Message Send")
	defer span.Finish()

	notificationBusiness := business.NewNotificationBusiness(ctx, server.Service)
	return notificationBusiness.QueueOut(ctx, "#", req)

}

// Status
func (server *Notificationserver) Status(ctxt context.Context, req *n_api.StatusRequest) (*n_api.StatusResponse, error) {

	span, ctx := opentracing.StartSpanFromContext(ctxt, "Message Status")
	defer span.Finish()

	notificationBusiness := business.NewNotificationBusiness(ctx, server.Service)
	return notificationBusiness.Status(ctx, "", req)

}

//Release method that is called for messages queued for release
func (server *Notificationserver) Release(ctxt context.Context, req *n_api.ReleaseRequest) (*n_api.StatusResponse, error) {

	span, ctx := opentracing.StartSpanFromContext(ctxt, "Message Release")
	defer span.Finish()

	notificationBusiness := business.NewNotificationBusiness(ctx, server.Service)
	return notificationBusiness.Release(ctx, "", req)
}

//In method call for income rquest of any notification
func (server *Notificationserver) In(ctxt context.Context, req *n_api.MessageIn) (*n_api.StatusResponse, error) {

	span, ctx := opentracing.StartSpanFromContext(ctxt, "Message Receive")
	defer span.Finish()

	notificationBusiness := business.NewNotificationBusiness(ctx, server.Service)
	return notificationBusiness.QueueIn(ctx, req)
}

func (server *Notificationserver) Search(req *n_api.SearchRequest, stream n_api.NotificationService_SearchServer) error {

	span, ctx := opentracing.StartSpanFromContext(stream.Context(), "Message Search")
	defer span.Finish()

	notificationBusiness := business.NewNotificationBusiness(ctx, server.Service)
	return notificationBusiness.Search(ctx, "", req, stream)

}
