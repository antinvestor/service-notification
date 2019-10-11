package service

import(
	"bitbucket.org/antinvestor/service-notification/notification"
	"google.golang.org/grpc"
	"fmt"
	"log"
	//"github.com/rs/xid"
	"context"
	"time"
	"github.com/jinzhu/gorm"
)
	
//Runclient runs client server
func Runclient(db *gorm.DB){


	
	fmt.Println("hello I'm a client")
	cc, err := grpc.Dial("0.0.0.0:7000",grpc.WithInsecure())
if err != nil{
	log.Fatalf("could not connect : %v" ,err)
}
defer cc.Close()

c := notification.NewNotificationServiceClient(cc)


//client methods called on particular occasion/functionality

status(c)
//dosend(c)
//income(c)

		}

// income notification requests
func income(c notification.NotificationServiceClient){

			req :=&notification.IncomeRequest{
				Requeststatus:		"send",//req.Requeststatus,
				Language:			"English",//req.Language,
				Product:			"Funds",//req.Product,
				Massagetype:		"Recieved",//req.Massagetype,
				ProfileID :			map[string] string{
											"name":"isa",
				 
									},

}
env2, cancel := context.WithTimeout(context.Background(), time.Second * 15)
	defer cancel()
res ,err := c.In(env2,req)

if err != nil {
	log.Fatalf("error while call send RPC %v",err)
}
log.Printf("Response from sender: %s",res.GetNotificationID())
}


//dosend creates notification for outgoing
func dosend(c notification.NotificationServiceClient){

	req :=&notification.QueueRequest{
			
			
			Language:					"English",
			Channel:					"Email",
			Massagetemplete:			"Receveid_templete",
			Autosend:	    			"false",
			ProfileID :			map[string] string{
									"name":"isa",
		 
							},
			Massagevariables:			map[string] string{
								"name":"isa",
								"Account":"AC100000",
								"Amount": "100000",
	 
						},


}
env2, cancel := context.WithTimeout(context.Background(), time.Second * 15)
defer cancel()
res ,err := c.Out(env2,req)

if err != nil {
log.Fatalf("error while call send RPC %v",err)
}
log.Printf("Response from sender: %s",res.GetNotificationID())
}

//status looks for notification status with notification id
func status(c notification.NotificationServiceClient){

	req :=&notification.StatusRequest{
			
			
			NotificationID:					"bmg4apdq29bieu2s9640",
			
}

env2, cancel := context.WithTimeout(context.Background(), time.Second * 15)
defer cancel()
res ,err := c.Status(env2,req)

if err != nil {
log.Fatalf("error while call send RPC %v",err)
}
log.Printf("Response from sender: %s",res.GetMessagestatus())
}
