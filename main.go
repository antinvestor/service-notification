package main

import (
	"context"
	"fmt"
	"github.com/antinvestor/apis"
	napi "github.com/antinvestor/service-notification-api"
	"github.com/antinvestor/service-notification/config"
	"github.com/antinvestor/service-notification/service"
	"github.com/antinvestor/service-notification/service/handlers"
	"github.com/antinvestor/service-notification/service/repository/models"
	papi "github.com/antinvestor/service-profile-api"
	"github.com/pitabwire/frame"
	"gocloud.dev/server"
	"google.golang.org/grpc"
	"log"
	"os"
	"strconv"
	"strings"
)

func main() {

	serviceName := "Notification"

	ctx := context.Background()

	var serviceOptions []frame.Option

	datasource := frame.GetEnv(config.EnvDatabaseUrl, "postgres://ant:@nt@localhost/service_notification")
	mainDb := frame.Datastore(ctx, datasource, false)
	serviceOptions = append(serviceOptions, mainDb)

	readOnlydatasource := frame.GetEnv(config.EnvReplicaDatabaseUrl, datasource)
	readDb := frame.Datastore(ctx, readOnlydatasource, true)
	serviceOptions = append(serviceOptions, readDb)



	messageInLoggedHandler := &handlers.MessageInLoggedQueueHandler{}
	//Setup queue subscribers
	messageInLoggedQueueUrl := frame.GetEnv(config.EnvQueueMessageInLogged, fmt.Sprintf("mem://%s", config.ConfigQueueMessageInLoggedName))
	messageInLoggedQueue := frame.RegisterSubscriber(config.ConfigQueueMessageInLoggedName, messageInLoggedQueueUrl, 5, messageInLoggedHandler)
	messageInLoggedQueueP := frame.RegisterPublisher(config.ConfigQueueMessageInLoggedName, messageInLoggedQueueUrl)
	serviceOptions = append(serviceOptions, messageInLoggedQueue, messageInLoggedQueueP)

	messageOutLoggedHandler := &handlers.MessageOutLoggedQueueHandler{}
	messageOutLoggedQueueUrl := frame.GetEnv(config.EnvQueueMessageOutLogged, fmt.Sprintf("mem://%s", config.ConfigQueueMessageOutLoggedName))
	messageOutLoggedQueue := frame.RegisterSubscriber(config.ConfigQueueMessageOutLoggedName, messageOutLoggedQueueUrl, 5, messageOutLoggedHandler)
	messageOutLoggedQueueP := frame.RegisterPublisher(config.ConfigQueueMessageOutLoggedName, messageOutLoggedQueueUrl)
	serviceOptions = append(serviceOptions, messageOutLoggedQueue, messageOutLoggedQueueP)

	messageInRouteHandler := &handlers.MessageInRoutedQueueHandler{}
	messageInRoutedIds := frame.GetEnv(config.EnvQueueMessageInRouteIds, "")
	for _, routeId := range strings.Split(messageInRoutedIds, ",") {

		messageInRouteQueueUrl := frame.GetEnv(config.EnvQueueMessageOutLogged,
			fmt.Sprintf("mem://%s",
				fmt.Sprintf(config.ConfigQueueMessageInRoutedName, routeId)))

		messageInRoutedQueueSub := frame.RegisterSubscriber(
			fmt.Sprintf(config.ConfigQueueMessageInRoutedName, routeId),
			messageInRouteQueueUrl, 5, messageInRouteHandler)

		messageInRoutedQueue := frame.RegisterPublisher(
			fmt.Sprintf(config.ConfigQueueMessageInRoutedName, routeId), messageInRouteQueueUrl)

		serviceOptions = append(serviceOptions, messageInRoutedQueueSub, messageInRoutedQueue)
	}

	messageOutRouteHandler := &handlers.MessageOutRouteQueueHandler{}
	messageOutRoutedIds := frame.GetEnv(config.EnvQueueMessageOutRouteIds,
		"9bsv0s23l8og00vgjq7g,9bsv0s23l8og00vgjq1g")
	for _, routeId := range strings.Split(messageOutRoutedIds, ",") {

		messageOutRouteQueueUrl := frame.GetEnv(config.EnvQueueMessageOutRouteIds,
			fmt.Sprintf("mem://%s",
				fmt.Sprintf(config.ConfigQueueMessageOutRoutedName, routeId)))

		messageOutRoutedQueueSub := frame.RegisterSubscriber(
			fmt.Sprintf(config.ConfigQueueMessageOutRoutedName, routeId),
			messageOutRouteQueueUrl, 5, messageOutRouteHandler)

		messageOutRoutedQueuePub := frame.RegisterPublisher(
			fmt.Sprintf(config.ConfigQueueMessageOutRoutedName, routeId), messageOutRouteQueueUrl)

		serviceOptions = append(serviceOptions, messageOutRoutedQueueSub, messageOutRoutedQueuePub)

	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(service.AuthInterceptor),
	)

	implementation := &handlers.Notificationserver{}

	napi.RegisterNotificationServiceServer(grpcServer, implementation)

	httpOptions := &server.Options{}

	defaultServer := frame.GrpcServer(grpcServer, httpOptions)
	serviceOptions = append(serviceOptions, defaultServer)

	sysService := frame.NewService(serviceName, serviceOptions...)


	profileServiceUrl := frame.GetEnv(config.EnvProfileServiceUri, "profile.api.antinvestor.com:443")
	profileCli, err := papi.NewProfileClient(ctx, apis.WithEndpoint(profileServiceUrl))
	if err != nil {
		log.Printf("main -- Could not setup profile server : %v", err)
	}

	implementation.Service = sysService
	implementation.ProfileCli = profileCli

	messageInLoggedHandler.Service = sysService
	messageInLoggedHandler.ProfileCli = profileCli

	messageInRouteHandler.Service = sysService
	messageInRouteHandler.ProfileCli = profileCli

	messageOutLoggedHandler.Service = sysService
	messageOutLoggedHandler.ProfileCli = profileCli

	messageOutRouteHandler.Service = sysService
	messageOutRouteHandler.ProfileCli = profileCli


	isMigration, err := strconv.ParseBool(frame.GetEnv(config.EnvMigrate, "false"))
	if err != nil {
		isMigration = false
	}

	stdArgs := os.Args[1:]
	if (len(stdArgs) > 0 && stdArgs[0] == "migrate") || isMigration {

		migrationPath := frame.GetEnv(config.EnvMigrationPath, "./migrations/0001")
		err := sysService.MigrateDatastore(ctx, migrationPath,
			models.Channel{}, models.Language{}, models.Templete{},
			models.TempleteData{}, models.Notification{})
		if err != nil {
			log.Printf("main -- Could not migrate successfully because : %v", err)
		}

	} else {

		serverPort := frame.GetEnv(config.EnvServerPort, "7020")

		log.Printf(" main -- Initiating server operations on : %s", serverPort)
		err := sysService.Run(ctx, fmt.Sprintf(":%v", serverPort))
		if err != nil {
			log.Printf("main -- Could not run Server : %v", err)
		}

	}

}
