package service

import (
	"fmt"
	"net"
	"time"
	//"net/http"
	"os"
	"os/signal"

	"antinvestor.com/service/notification/utils"

	"antinvestor.com/service/notification/notification"
	"google.golang.org/grpc"
	"log"
)

// Error represents a handler error. It provides methods for a HTTP status
// code and embeds the built-in error interface.
type Error interface {
	error
	Status() int
}

// StatusError represents an error with an associated HTTP status code.
type StatusError struct {
	Code int
	Err  error
}

// Allows StatusError to satisfy the error interface.
func (se StatusError) Error() string {
	return se.Err.Error()
}

// Returns our HTTP status code.
func (se StatusError) Status() int {
	return se.Code
}

//RunServer Starts a server and waits on it
func RunServer(env *utils.Env) {

	//waitDuration := time.Second * 15
	serverPort := utils.GetEnv(utils.EnvServerPort, "7020")
	im := &notificationserver{Env: env}
	//instantiate grpc server not http server for api proccessing and registers
	srv := grpc.NewServer()
	notification.RegisterNotificationServiceServer(srv, im)

	//this comes in second telling server to listen on tcp address and on serverport as defined in os enviroment
	lis, err := net.Listen("tcp", fmt.Sprintf(":%v", serverPort))

	if err != nil {
		env.Logger.Fatalf("Could not start on supplied port %v %v ", serverPort, err)
	}

	// Run our server in a goroutine so that it doesn't block.
	go func() {

		env.Logger.Infof("Service running on port : %v", serverPort)

		// start the server continuously to be listening client request any tym in goroutine otherwise will exit with 1 soon it is starteds
		if err := srv.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %s", err)
		}

	}()

	c := make(chan os.Signal, 1)
	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
	signal.Notify(c, os.Interrupt)

	// Block until we receive our signal.
	<-c

	// Create a deadline to wait for.
	//env2, cancel := context.WithTimeout(context.Background(), waitDuration)
	//defer cancel()
	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline.
	srv.Stop()
	// Optionally, you could run srv.Shutdown in a goroutine and block on
	// <-env.Done() if your application should wait for other services
	// to finalize based on context cancellation.
	env.Logger.Infof("Service shutting down at : %v", time.Now())
}
