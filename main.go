package main

import (
	"context"
	"fmt"
	napi "github.com/antinvestor/service-notification-api"
	"github.com/antinvestor/service-notification/config"
	"github.com/antinvestor/service-notification/service"
	"github.com/antinvestor/service-notification/service/handlers"
	"github.com/pitabwire/frame"
	"gocloud.dev/server"
	"google.golang.org/grpc"
	"log"
	"os"
	"strings"
)

func main() {

	serviceName := "Notification"

	ctx := context.Background()

	datasource := frame.GetEnv(config.EnvDatabaseUrl, "")
	mainDb := frame.Datastore(ctx, datasource, false)

	readOnlydatasource := frame.GetEnv(config.EnvReplicaDatabaseUrl, datasource)
	readDb := frame.Datastore(ctx, readOnlydatasource, true)

	messageInLoggedHandler := &handlers.MessageInLoggedQueueHandler{}
	//Setup queue subscribers
	messageInLoggedQueueUrl := frame.GetEnv(config.EnvQueueMessageInLogged, fmt.Sprintf("memt+://%s", config.ConfigQueueMessageInLoggedName))
	messageInLoggedQueue := frame.RegisterSubscriber(config.ConfigQueueMessageInLoggedName, messageInLoggedQueueUrl, 5, messageInLoggedHandler)
	messageInLoggedQueueP := frame.RegisterPublisher(config.ConfigQueueMessageInLoggedName, messageInLoggedQueueUrl)

	messageOutLoggedHandler := &handlers.MessageOutLoggedQueueHandler{}
	messageOutLoggedQueueUrl := frame.GetEnv(config.EnvQueueMessageOutLogged, fmt.Sprintf("memt+://%s", config.ConfigQueueMessageOutLoggedName))
	messageOutLoggedQueue := frame.RegisterSubscriber(config.ConfigQueueMessageOutLoggedName, messageOutLoggedQueueUrl, 5, messageOutLoggedHandler)
	messageOutLoggedQueueP := frame.RegisterPublisher(config.ConfigQueueMessageOutLoggedName, messageOutLoggedQueueUrl)

	dynamicPublishQueues := make([]frame.Option, 1)

	messageInRoutedIds := frame.GetEnv(config.EnvQueueMessageInRouteIds, "")
	for _, routeId := range strings.Split(messageInRoutedIds, ",") {

		messageInRouteQueueUrl := frame.GetEnv(config.EnvQueueMessageOutLogged,
			fmt.Sprintf("memt+://%s",
				fmt.Sprintf(config.ConfigQueueMessageInRoutedName, routeId)))

		messageInRoutedQueue := frame.RegisterPublisher(
			fmt.Sprintf(config.ConfigQueueMessageInRoutedName, routeId), messageInRouteQueueUrl)

		dynamicPublishQueues = append(dynamicPublishQueues, messageInRoutedQueue)
	}

	messageOutRouteHandler := &handlers.MessageOutRouteQueueHandler{}
	messageOutRoutedIds := frame.GetEnv(config.EnvQueueMessageOutRouteIds, "9bsv0s23l8og00vgjq7g,9bsv0s23l8og00vgjq1g")
	for _, routeId := range strings.Split(messageOutRoutedIds, ",") {

		messageOutRouteQueueUrl := frame.GetEnv(config.EnvQueueMessageOutRouteIds,
			fmt.Sprintf("memt+://%s",
				fmt.Sprintf(config.ConfigQueueMessageOutRoutedName, routeId)))

		messageOutRoutedQueueSub := frame.RegisterSubscriber(
			fmt.Sprintf(config.ConfigQueueMessageOutRoutedName, routeId), messageOutRouteQueueUrl, 5, messageOutRouteHandler)

		messageOutRoutedQueuePub := frame.RegisterPublisher(
			fmt.Sprintf(config.ConfigQueueMessageOutRoutedName, routeId), messageOutRouteQueueUrl)

		dynamicPublishQueues = append(dynamicPublishQueues, messageOutRoutedQueueSub, messageOutRoutedQueuePub)

	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(service.AuthInterceptor),
	)

	implementation := &handlers.Notificationserver{}

	napi.RegisterNotificationServiceServer(grpcServer, implementation)

	httpOptions := &server.Options{}

	defaultServer := frame.GrpcServer(grpcServer, httpOptions)

	serviceOptions := []frame.Option{
		mainDb, readDb, defaultServer,
		messageInLoggedQueue, messageInLoggedQueueP, messageOutLoggedQueue, messageOutLoggedQueueP,
	}
	serviceOptions = append(serviceOptions, dynamicPublishQueues...)

	sysService := frame.NewService(serviceName, serviceOptions...)

	isMigration := frame.GetEnv(config.EnvMigrate, "")
	stdArgs := os.Args[1:]
	if (len(stdArgs) > 0 && stdArgs[0] == "migrate") || isMigration == "true" {

		migrationPath := frame.GetEnv(config.EnvMigrationPath, "./migrations/0001")
		err := sysService.MigrateDatastore(ctx, migrationPath)
		if err != nil {
			log.Printf("main -- Could not migrate successfully because : %v", err)
		}

	} else {

		serverPort := frame.GetEnv(config.EnvServerPort, "7020")
		err := sysService.Run(ctx, fmt.Sprintf(":%v", serverPort))
		if err != nil {
			log.Printf("main -- Could not run Server : %v", err)
		}

	}

}
