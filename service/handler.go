package service

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"bitbucket.org/antinvestor/service-notification/notification"

	"github.com/rs/xid"

	stan "github.com/nats-io/stan.go"
)

const (
	port      = ":50051"
	clusterID = "test-cluster"
	clientID  = "event-store"
)

type notificationserver struct {
	Env    *Env
	stream *notification.NotificationService_SearchServer
}

//out method act after income request let out notification
func (server *notificationserver) Out(ctxt context.Context, req *notification.QueueRequest) (*notification.StatusResponse, error) {

	Massagevariables, _ := json.Marshal(req.GetMessageVariables())

	
	var id []string

	//checks if profileID is valid
	server.Env.GeWtDb(ctxt).Where("profile_id = ?", req.GetProfileID()).Find(&Notification{}).Pluck("profile_id", &id)
	if len(id) != 0 {

		
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
			ProductID:"3900Rent",
			Payload: string(Massagevariables),
		}

		server.Env.GeWtDb(ctxt).Create(in)
		
		//nats stream subscribation
		go subscribetion()
		
	} else {
		return nil, errors.New("ProfileID doesnot exit/match any value")

	}

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


	//checks if profileID  contact field is not null in table
	if len(req.GetProfileID()) == 0 || req.ProductID == ""{

			
			return nil, errors.New("ProfileID or ProductID is invalid")

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
			Channel:		"Email",
		}
			//create notification
			server.Env.GeWtDb(ctxt).Create(in)

			go publishEvent(in)
			

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

// publishEvent publish an event via NATS Streaming server
func publishEvent(model *Notification) error {
	// Connect to NATS Streaming server
	sc, err := stan.Connect(
		clusterID,
		clientID,
		stan.NatsURL(stan.DefaultNatsURL),
	)
	if err != nil {
		return err
	}
	defer sc.Close()
	channel := model.Channel
	eventMsg := []byte(model.NotificationID+model.ProductID)
	// Publish message on subject (channel)
	sc.Publish(channel, eventMsg)
	log.Println("Published message on channel: " + channel)

	return nil
}

//subscribetion event
func subscribetion() {

	const (
		clusterID = "test-cluster"
		clientID  = "restaurant-service"
		channel   = "Email"
		durableID = "restaurant-service-durable"
	)

	sc, err := stan.Connect(
		clusterID,
		clientID,
		stan.NatsURL(stan.DefaultNatsURL),
	)

	if err != nil {
		log.Fatal(err)
	}
	// Subscribe with manual ack mode, and set AckWait to 60 seconds
	aw, _ := time.ParseDuration("60s")
	sc.Subscribe(channel, func(msg *stan.Msg) {
		msg.Ack() // Manual ACK
		//order := &Notification{}
		// Unmarshal JSON that represents the Order data
		// err := json.Unmarshal(msg.Data, &order)
		// if err != nil {
		// 	log.Print("ll",err)
		// 	return
		// }
		// Handle the message
		log.Printf("Subscribed message from clientID - %s for Order: %+v\n", clientID,string(msg.Data))

	}, stan.DurableName(durableID),
		stan.MaxInflight(25),
		stan.SetManualAckMode(),
		stan.AckWait(aw),
	)
}
