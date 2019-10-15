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
	Env *Env
}

func (server *notificationserver) Out(ctxt context.Context, req *notification.QueueRequest) (*notification.QueueResponse, error){
	
	uProfile, _ := json.Marshal(req.GetProfileID())
	Massagevariables, _ := json.Marshal(req.GetMassagevariables())
	
	in := &Notification{
			NotificationID:				xid.New().String(),
			ProfileID : 				string(uProfile),
			Messagevariables:			string(Massagevariables),
			Language:					req.Language,
			Channel:					req.Channel,
			Messagetype:				req.Massagetemplete,
			Autosend:	    			req.Autosend,
			}

			server.Env.GeWtDb(ctxt).Create(in)

	return &notification.QueueResponse{NotificationID: xid.New().String(),}, nil

}	

//Status
func (server *notificationserver) Status(ctxt context.Context, req *notification.StatusRequest) (*notification.StatusResponse, error){

	var StatusInfos [] string
	var status string

	//server.Env.GeWtDb(ctxt).Debug().Raw("select status from notifications where notification_id = ?", req.GetNotificationID()).Pluck("status", &StatusInfos)
	server.Env.GeWtDb(ctxt).Debug().Table("notifications").Select("status").Where("notification_id = ?", req.GetNotificationID()).Pluck("status", &StatusInfos)

	for _,status := range StatusInfos {
	
	return &notification.StatusResponse{Messagestatus: status,}, nil
}

return &notification.StatusResponse{Messagestatus: status,}, nil

}

func (server *notificationserver) Release(ctxt context.Context, req *notification.ReleaseRequest) (*notification.StatusResponse, error){

	return &notification.StatusResponse{}, nil
}

func (server *notificationserver) In(ctxt context.Context, req *notification.IncomeRequest) (*notification.QueueResponse, error){
	
	uProfile, _ := json.Marshal(req.GetProfileID())
	
	in := &Notification{
			NotificationID: xid.New().String(),
			ProfileID : 	string(uProfile),
			Status:			req.Requeststatus,
			Language:		req.Language,
			Product:		req.Product,
			Messagetype:	req.Massagetype,
			}

			server.Env.GeWtDb(ctxt).Create(in)
			
	return &notification.QueueResponse{NotificationID: xid.New().String(),}, nil

}
