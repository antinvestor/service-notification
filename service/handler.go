package service

import (
	"context"
	"encoding/json"

	//"time"
	"bitbucket.org/antinvestor/service-notification/notification"
	//"fmt"
	//"github.com/jinzhu/gorm"
	//"log"
	"github.com/rs/xid"
)

type notificationserver struct {
	Env    *Env
	stream *notification.NotificationService_SearchServer
}

//out method act after income request let out notification
func (server *notificationserver) Out(ctxt context.Context, req *notification.QueueRequest) (*notification.StatusResponse, error) {

	Massagevariables, _ := json.Marshal(req.GetMessageVariables())

	in := &Notification{
		NotificationID:   xid.New().String(),
		ProfileID:        req.GetProfileID(),
		Messagevariables: string(Massagevariables),
		Language:         req.Language,
		Channel:          req.Channel,
		Messagetype:      req.MessageTemplete,
		Autosend:         req.Autosend,
	}

	var cxt context.Context
	var id []string

	//checks if profileID is valid
	server.Env.GeWtDb(cxt).Where("profile_id = ?", req.GetProfileID()).Find(&Notification{}).Pluck("profile_id", &id)
	
	for _, d := range id {
		if d == req.GetProfileID() {
			server.Env.GeWtDb(ctxt).Create(in)
			//send notification

		}
	}

	return &notification.StatusResponse{NotificationID: xid.New().String()}, nil

}

//Status
func (server *notificationserver) Status(ctxt context.Context, req *notification.StatusRequest) (*notification.StatusResponse, error) {

	var StatusInfos []string
	var status string

	//server.Env.GeWtDb(ctxt).Debug().Raw("select status from notifications where notification_id = ?", req.GetNotificationID()).Pluck("status", &StatusInfos)
	server.Env.GeWtDb(ctxt).Debug().Table("notifications").Select("status").Where("notification_id = ?", req.GetNotificationID()).Pluck("status", &StatusInfos)

	for _, status := range StatusInfos {

		return &notification.StatusResponse{MessageStatus: status}, nil
	}

	return &notification.StatusResponse{MessageStatus: status}, nil

}

//Release method that is called for messages queued for release
func (server *notificationserver) Release(ctxt context.Context, req *notification.ReleaseRequest) (*notification.StatusResponse, error) {

	var Notificationid []string
	var notifications string

	server.Env.GeWtDb(ctxt).Debug().Table("notifications").Select("notification_id").Where("status = ?", req.GetReleaseMessage()).Pluck("notification_id", &Notificationid)

	for _, notifications := range Notificationid {

		return &notification.StatusResponse{NotificationID: notifications}, nil
	}

	return &notification.StatusResponse{NotificationID: notifications}, nil
}

//In method call for income rquest of any notification
func (server *notificationserver) In(ctxt context.Context, req *notification.IncomeRequest) (*notification.StatusResponse, error) {

	uPayload, _ := json.Marshal(req.GetPayLoad())
	in := &Notification{
		NotificationID: xid.New().String(),
		ProfileID:      req.GetProfileID(),
		Status:         req.RequestStatus,
		Language:       req.Language,
		ProductID:      req.ProductID,
		Messagetype:    req.MessageType,
		Payload:        string(uPayload),
	}

	server.Env.GeWtDb(ctxt).Create(in)

	return &notification.StatusResponse{NotificationID: xid.New().String()}, nil

}

func (server *notificationserver) Search(req *notification.SearchRequest, stream notification.NotificationService_SearchServer) error {

	var cxt context.Context
	var serch []string

	server.Env.GeWtDb(cxt).Where("notification_id = ?", req.GetNotificationID()).Find(&Notification{}).Pluck("status", &serch)
	for _, d := range serch {
		if err := stream.Send(&notification.SearchResponse{RequestStatus: d}); err != nil {
			return err
		}
	}

	return nil

}
