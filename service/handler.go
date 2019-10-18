package service

import (
	"context"
	"encoding/json"
	"errors"

	"bitbucket.org/antinvestor/service-notification/notification"

	"github.com/rs/xid"
)

type notificationserver struct {
	Env    *Env
	stream *notification.NotificationService_SearchServer
}

//out method act after income request let out notification
func (server *notificationserver) Out(ctxt context.Context, req *notification.QueueRequest) (*notification.StatusResponse, error) {

	Massagevariables, _ := json.Marshal(req.GetMessageVariables())

	var cxt context.Context
	var id []string
	var ProfileDetails []string
	var ProductDetails []string

	//checks if profileID is valid
	server.Env.GeWtDb(cxt).Where("profile_id = ?", req.GetProfileID()).Find(&Notification{}).Pluck("profile_id", &id)
	if len(id) != 0 {

		//query the profile service to get the profile data like name, contacts and any set of communication preferences
		server.Env.GeWtDb(cxt).Raw("select name,contact from profiles where profile_id = ?", req.GetProfileID()).Pluck("name", &ProfileDetails)
		if len(ProfileDetails) != 0 {

			//for _, profiledetails := range ProfileDetails {   to get contact, name and communication channels

			//service queries the product service to also get preferred communication preferences
			server.Env.GeWtDb(cxt).Raw("select channel,contact from product where product_id = ?", req.GetProfileID()).Pluck("channel", &ProductDetails)
			if len(ProductDetails) != 0 {

				//for _, productdetails := range ProductDetails {

				//send notification
				in := &Notification{
					NotificationID:   xid.New().String(),
					ProfileID:        req.GetProfileID(),
					Messagevariables: string(Massagevariables),
					Language:         req.Language,
					//channel will be determined by product preferences or profile client request
					//if the user has multiple phone numbers attached to their profile prefer the phone number that was last added and verified or last known to work
					Channel:     req.Channel,
					Messagetype: req.MessageTemplete,
					Autosend:    req.Autosend,
				}

				server.Env.GeWtDb(ctxt).Create(in)

			} else {
				return nil, errors.New("ProductID doesnot exit register and try again")
			}
			//}
		} else {
			return nil, errors.New("ProfileID doesnot exit register and try again")
		}
		//}
	} else {
		return nil, errors.New("ProfileID doesnot exit/match any value")

	}
	//}

	return &notification.StatusResponse{NotificationID: xid.New().String()}, nil

}

//Status
func (server *notificationserver) Status(ctxt context.Context, req *notification.StatusRequest) (*notification.StatusResponse, error) {

	var StatusInfos []string
	var status string

	//server.Env.GeWtDb(ctxt).Debug().Raw("select status from notifications where notification_id = ?", req.GetNotificationID()).Pluck("status", &StatusInfos)
	server.Env.GeWtDb(ctxt).Debug().Table("notifications").Select("status").Where("notification_id = ?", req.GetNotificationID()).Pluck("status", &StatusInfos)
	if len(StatusInfos) != 0 {

		for _, status := range StatusInfos {

			return &notification.StatusResponse{MessageStatus: status}, nil
		}

	} else {

		return nil, errors.New("Invalid status request")
	}

	return &notification.StatusResponse{MessageStatus: status}, nil

}

//Release method that is called for messages queued for release
func (server *notificationserver) Release(ctxt context.Context, req *notification.ReleaseRequest) (*notification.StatusResponse, error) {

	var Notificationid []string
	var notifications string

	server.Env.GeWtDb(ctxt).Debug().Table("notifications").Select("notification_id").Where("status = ?", req.GetReleaseMessage()).Pluck("notification_id", &Notificationid)
	if len(Notificationid) != 0 {

		for _, notifications := range Notificationid {

			return &notification.StatusResponse{NotificationID: notifications}, nil
		}
	} else {

		return nil, errors.New("Invalid status request")
	}

	return &notification.StatusResponse{NotificationID: notifications}, nil
}

//In method call for income rquest of any notification
func (server *notificationserver) In(ctxt context.Context, req *notification.IncomeRequest) (*notification.StatusResponse, error) {

	var Contact []string
	var Channel []string

	//checks if profileID  contact field is not null in table
	server.Env.GeWtDb(ctxt).Raw("select contact from profiles  where profile_id = ?", req.GetProfileID()).Pluck("contact", &Contact)
	if len(Contact) == 0 {

		//needs contact field added
		result := server.Env.GeWtDb(ctxt).Raw("insert into profiles (contact) values (?) where profile_id = ?", "---------", req.GetProfileID())

		if result.RowsAffected == 1 {

			uPayload, _ := json.Marshal(req.GetPayLoad())
			in := &Notification{
				NotificationID: xid.New().String(),
				ProfileID:      req.GetProfileID(),
				ProductID:      req.ProductID,
				Status:         req.RequestStatus,
				Language:       req.Language,
				Messagetype:    req.MessageType,
				Payload:        string(uPayload),
			}

			//checki if income notification channel route are mapped to particular product
			server.Env.GeWtDb(ctxt).Raw("select channel from product where channel = ?", req.GetProfileID()).Pluck("channel", &Channel)

			if len(Channel) != 0 {
				//create notification
				server.Env.GeWtDb(ctxt).Create(in)

			} else {
				return nil, errors.New("notification channel is invalid")

			}

			//fails to add to table
		} else {
			return nil, errors.New("ProfileID is invalid")

		}

	} else {

		uPayload, _ := json.Marshal(req.GetPayLoad())
		in := &Notification{
			NotificationID: xid.New().String(),
			ProfileID:      req.GetProfileID(),
			ProductID:      req.ProductID,
			Status:         req.RequestStatus,
			Language:       req.Language,
			Messagetype:    req.MessageType,
			Payload:        string(uPayload),
		}

		//checki if income notification channel route are mapped to particular product
		server.Env.GeWtDb(ctxt).Raw("select channel from product where channel = ?", req.GetProfileID()).Pluck("channel", &Channel)

		if len(Channel) != 0 {
			//create notification
			server.Env.GeWtDb(ctxt).Create(in)

		} else {
			return nil, errors.New("notification channel is invalid")

		}
	}

	return &notification.StatusResponse{NotificationID: xid.New().String()}, nil

}

func (server *notificationserver) Search(req *notification.SearchRequest, stream notification.NotificationService_SearchServer) error {

	var cxt context.Context
	var serch []string

	server.Env.GeWtDb(cxt).Where("notification_id = ?", req.GetNotificationID()).Find(&Notification{}).Pluck("status", &serch)
	//check if request input exist in database
	if len(serch) != 0 {

		for _, d := range serch {

			if err := stream.Send(&notification.SearchResponse{RequestStatus: d}); err != nil {
				return err
			}
		}

	} else {
		return errors.New("notificationID is invalid")

	}
	return nil

}
