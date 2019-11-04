package main

import (
	"log"
	"testing"

	"github.com/rs/xid"
	"github.com/stretchr/testify/assert"

	"context"
	"io"

	//"math/rand"
	"net"
	"os"
	"os/exec"

	//"testing"
	"time"

	"google.golang.org/grpc"
	//"google.golang.org/grpc/grpclog"

	pb "bitbucket.org/antinvestor/service-notification/notification"
)

// Test started when the test binary is started. Only calls main.
func TestSystem(t *testing.T) {

	// assert equality
	assert.Equal(t, 123, 123, "Just ensuring we are testing something")

}

var client pb.NotificationServiceClient
var conn *grpc.ClientConn
var serverCmd *exec.Cmd

func TestMain(m *testing.M) {
	log.Printf("TestMain()")
	//startServer()
	startServer()
	client, conn = startClient()
	returnCode := m.Run()
	stopClient()
	stopServer() 
	os.Exit(returnCode)
}

func startServer() {
	go main()
	if !serverUp() {
		log.Fatal("Server failed to open port")
	}
}

func serverUp() bool {
	// wait for port to open
	for i := 0; i < 100; i++ {
		if checkServerUp() {
			log.Println("server up!")
			return true
		}
		log.Println("server not up yet...trying again!")
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func checkServerUp() bool {
	// Check if server port is in use

	// Try to create a server with the port
	server, err := net.Listen("tcp", ":7000")

	// if it fails then the port is likely taken
	if err != nil {
		return true
	}

	err = server.Close()
	if err != nil {
		return true
	}

	// we successfully used and closed the port
	// so it's now available to be used again
	return false

}

func stopServer() {
	log.Printf("stopServer()")

	con := grpc.NewServer()

	con.Stop()
}

func startClient() (pb.NotificationServiceClient, *grpc.ClientConn) {
	log.Printf("startClient()")
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())
	//conn, err := grpc.Dial("127.0.0.1:"+string(*port), opts...)
	conn, err := grpc.Dial("0.0.0.0:7000", opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	client := pb.NewNotificationServiceClient(conn)

	return client, conn
}

func stopClient() {
	conn.Close()
}

///////////////////////////////////////

func TestSearch(t *testing.T) {

	req := &pb.SearchRequest{

		NotificationID: "req.GetNotificationID()",
	}

	env2, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()
	stream, err := client.Search(env2, req)

	if err != nil {
		log.Fatalf("%v.Search(_) = _, %v", client, err)
	}

	for {
		id, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("%v.Search(_) = _, %v", client, err)
		}
		log.Printf("Response from sender: %s", id)
	}

}

func TestRelease(t *testing.T) {

	req := &pb.ReleaseRequest{

		ReleaseMessage: "send",
	}

	env2, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()
	res, err := client.Release(env2, req)

	if err != nil {
		log.Fatalf("error while call send RPC %v", err)
	}
	log.Printf("Response from sender: %s", res.GetNotificationID())
}

func TestStatus(t *testing.T) {

	req := &pb.StatusRequest{

		NotificationID: "req.GetNotificationID()",
	}

	env2, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()
	res, err := client.Status(env2, req)

	if err != nil {
		log.Fatalf("error while call send RPC %v", err)
	}
	log.Printf("Response from sender: %s", res.GetMessageStatus())
}

func TestMessageOut(t *testing.T) {

	req := &pb.MessageOut{
		NotificationID:  xid.New().String(),
		Language:        "English",
		Channel:         "Email",
		MessageTemplete: "Receveid_templete",
		Autosend:        "false",
		ProfileID:       "001isaac",
		MessageVariables: map[string]string{
			"name":    "isa",
			"Account": "AC100000",
			"Amount":  "100000",
		},
	}
	env2, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()
	res, err := client.Out(env2, req)

	if err != nil {
		log.Fatalf("error while call send RPC %v", err)
	}
	log.Printf("Response from sender: %s", res.GetNotificationID())
}

func TestMessageIn(t *testing.T) {

	req := &pb.MessageIn{
		NotificationID: xid.New().String(),
		RequestStatus:  "send",     //req.Requeststatus,
		Language:       "English",  //req.Language,
		ProductID:      "Funds",    //req.Product,
		MessageType:    "Recieved", //req.Massagetype,
		ProfileID:      "001isaac",
		PayLoad: map[string]string{
			"id":          "1",
			"name":        "test entity 1",
			"description": "Testing email service on the notification service part",
		},
	}
	env2, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()
	res, err := client.In(env2, req)

	if err != nil {
		log.Fatalf("error while call send RPC %v", err)
	}
	log.Printf("Response from sender: %s", res.GetNotificationID())

}
