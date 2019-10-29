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

	"net/smtp"

	"github.com/subosito/twilio"
)



type notificationserver struct {
	Env    *Env
	stream *notification.NotificationService_SearchServer
}

//out method act after income request let out notification
func (server *notificationserver) Out(ctxt context.Context, req *notification.MessageOut) (*notification.StatusResponse, error) {

	Massagevariables, _ := json.Marshal(req.GetMessageVariables())

	NOTID := xid.New().String()
	var id []string

	//checks if profileID is valid
	server.Env.GetRDb(ctxt).Where("profile_id = ?", req.GetProfileID()).Find(&Notification{}).Pluck("profile_id", &id)
	if len(id) != 0 {

		
		//send notification
		in := &Notification{
			NotificationID:   NOTID,
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
		// go QueueSubscribeGroup()
		// go QueueSubscribeGroup2()

		
	} else {
		return nil, errors.New("ProfileID doesnot exit/match any value")

	}

	return &notification.StatusResponse{NotificationID: NOTID}, nil

}

//Status
func (server *notificationserver) Status(ctxt context.Context, req *notification.StatusRequest) (*notification.StatusResponse, error) {

	var StatusInfos []string
	var status string

	//server.Env.GeWtDb(ctxt).Debug().Raw("select status from notifications where notification_id = ?", req.GetNotificationID()).Pluck("status", &StatusInfos)
	server.Env.GetRDb(ctxt).Table("notifications").Select("status").Where("notification_id = ?", req.GetNotificationID()).Pluck("status", &StatusInfos)
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

	server.Env.GetRDb(ctxt).Table("notifications").Select("notification_id").Where("status = ?", req.GetReleaseMessage()).Pluck("notification_id", &Notificationid)
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
func (server *notificationserver) In(ctxt context.Context, req *notification.MessageIn) (*notification.StatusResponse, error) {
	NOTID := xid.New().String()

	//checks if profileID  contact field is not null in table
	if len(req.GetProfileID()) == 0 || req.ProductID == ""{

			
			return nil, errors.New("ProfileID or ProductID is invalid")

	} 

		uPayload, _ := json.Marshal(req.GetPayLoad())
		
		in := &Notification{
			NotificationID: NOTID,
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
		 

	 

	return &notification.StatusResponse{NotificationID: NOTID}, nil

}

func (server *notificationserver) Search(req *notification.SearchRequest, stream notification.NotificationService_SearchServer) error {

	var cxt context.Context
	var serch []string

	server.Env.GetRDb(cxt).Where("notification_id = ?", req.GetNotificationID()).Find(&Notification{}).Pluck("status", &serch)
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

	const (
		clusterID = "test-cluster"
		clientID  = "event-store"
	)

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
	eventMsg := []byte(model.Payload)
	// Publish message on subject (channel)
	//for i:=0;i<=5;i++{
	sc.Publish(channel, eventMsg)
	//}
	log.Println("Published message on channel: " + channel)

	return nil
}

//subscribetion event
func subscribetion() {

	const (
		clusterID = "test-cluster"
		clientID  = "notification-service"
		channel   = "Email"
		durableID = "notification-service-durable"
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
		order := &Notification{}
		//Unmarshal JSON that represents the Order data
		err := json.Unmarshal(msg.Data, &order)
		if err != nil {
			log.Print("ll",err)
			return
		}
		//Handle the message
		log.Printf("Subscribed message from clientID - %s for Order: %+v\n", clientID,string(msg.Data))
		
		//send Email
		//HandleEmail(msg.Data)

		//send sms
		SmsHandler(msg.Data)

	}, stan.DurableName(durableID),
		stan.MaxInflight(25),
		stan.SetManualAckMode(),
		stan.AckWait(aw),
	)
}
//QueueSubscribeGroup duarable queue subscription with same durable Name
func QueueSubscribeGroup() {

	const (
		clusterID  = "test-cluster"
		clientID   = "notification-service-query"
		channel    = "Email"
		durableID  = "notification-durable"
		queueGroup = "notification-service-group"
	)

	// Connect to NATS Streaming server
	sc, err := stan.Connect(
		clusterID,
		clientID,
		stan.NatsURL(stan.DefaultNatsURL),
	)

	if err != nil {
		log.Fatal(err)
	}
	sc.QueueSubscribe(channel, queueGroup, func(msg *stan.Msg) {
		order := &Notification{}
		err := json.Unmarshal(msg.Data, &order)
		if err == nil {
			// Handle the message
			log.Printf("QueueSubscribed message from clientID - %s: %+v\n", clientID, string(msg.Data))
			 
			//HandleEmail(msg.Data)
			
		}
	}, stan.DurableName(durableID),
	)
	//runtime.Goexit()
}

//QueueSubscribeGroup2 duarable queue subscription with same durable Name
func QueueSubscribeGroup2() {

	const (
		clusterID  = "test-cluster"
		clientID   = "notification-service-query2"
		channel    = "Email"
		durableID  = "notification-durable"
		queueGroup = "notification-service-group"
	)

	// Connect to NATS Streaming server
	sc, err := stan.Connect(
		clusterID,
		clientID,
		stan.NatsURL(stan.DefaultNatsURL),
	)

	if err != nil {
		log.Fatal(err)
	}
	sc.QueueSubscribe(channel, queueGroup, func(msg *stan.Msg) {
		order := &Notification{}
		err := json.Unmarshal(msg.Data, &order)
		if err == nil {
			// Handle the message
			log.Printf("2QueueSubscribed message from clientID - %s: %+v\n", clientID, string(msg.Data))
			
		}
	}, stan.DurableName(durableID),
	)
	//runtime.Goexit()
}

// smtpServer data to smtp server
type smtpServer struct {
	host string
	port string
   }
   // serverName URI to smtp server
   func (s *smtpServer) serverName() string {
	return s.host + ":" + s.port
   }


//HandleEmail Handle Email
func HandleEmail( msg []byte) {
    // Sender data.
    from := "ochomisaac356@gmail.com"
    password := "#########"
    // Receiver email address.
    to := []string{
        "info@antinvestor.com",
        
    }
    // smtp server configuration.
    smtpServer := smtpServer{host: "smtp.gmail.com", port: "587"}
    // Message.
	message := []byte("To: info@antinvestor.com \r\n" +
	"Subject: Notification service \r\n " + 
	"\r\n" + 
	 "Welcome\r\n" + string(msg) )
    // Authentication.
    auth := smtp.PlainAuth("", from, password, smtpServer.host)
    // Sending email.
    err := smtp.SendMail(smtpServer.serverName(), auth, from, to, message)
    if err != nil {
        log.Println(err)
        return
    }
    log.Println("Email Sent!")
}

//SmsHandler SmsHandler
func SmsHandler(msg []byte) {
	var (
		AccountSid = "AC6322d5e60c2aadd30700b124b06e6dde"
		AuthToken  = "7411ae9453892d1a5ba10c1903376294"
		From       = "+15005550006"
		 To         = "+256783486428"
	)

    // Initialize twilio Client
    c := twilio.NewTwilio(AccountSid, AuthToken)

    // Send Message
    Body :=  msg

	 resp, err := c.SimpleSendSMS(From , To, string(Body))
	 
	 if err != nil{
		log.Println("Err:", err)
	 }
	
	log.Println("Response:", resp.Body)
	log.Println("Response:", resp.Status)
    
}