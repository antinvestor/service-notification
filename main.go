package main

import (
	"context"
	"fmt"
	partitionV1 "github.com/antinvestor/service-partition-api"
	"github.com/sirupsen/logrus"
	"strings"

	"github.com/antinvestor/apis"
	notificationV1 "github.com/antinvestor/service-notification-api"
	"github.com/antinvestor/service-notification/config"
	"github.com/antinvestor/service-notification/service/events"

	"github.com/antinvestor/service-notification/service/handlers"
	"github.com/antinvestor/service-notification/service/models"

	"os"
	"strconv"

	profileV1 "github.com/antinvestor/service-profile-api"
	grpcMiddleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpcrecovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	grpcctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"github.com/pitabwire/frame"
	"google.golang.org/grpc"
)

func main() {

	serviceName := "service_notification"

	ctx := context.Background()
	var notificationConfig config.NotificationConfig
	err := frame.ConfigProcess("", &notificationConfig)
	if err != nil {
		logrus.WithError(err).Fatal("could not process configs")
		return
	}

	mainDB := frame.Datastore(ctx, notificationConfig.DatabaseURL, false)
	readOnlyDatasource := notificationConfig.ReplicaDatabaseURL
	if notificationConfig.ReplicaDatabaseURL == "" {
		readOnlyDatasource = notificationConfig.DatabaseURL
	}
	readDB := frame.Datastore(ctx, readOnlyDatasource, true)

	service := frame.NewService(serviceName, frame.Config(notificationConfig), mainDB, readDB)

	log := service.L()

	isMigration, err := strconv.ParseBool(notificationConfig.Migrate)
	if err != nil {
		isMigration = false
	}

	stdArgs := os.Args[1:]
	if (len(stdArgs) > 0 && stdArgs[0] == "migrate") || isMigration {
		err = service.MigrateDatastore(ctx, notificationConfig.MigrationPath,
			models.Route{}, models.Language{}, models.Templete{},
			models.TempleteData{}, models.Notification{}, models.NotificationStatus{})

		if err != nil {
			log.WithError(err).Fatal("could not migrate successfully")
		}
		return
	}

	oauth2ServiceHost := notificationConfig.Oauth2ServiceURI
	oauth2ServiceURL := fmt.Sprintf("%s/oauth2/token", oauth2ServiceHost)
	oauth2ServiceSecret := notificationConfig.Oauth2ServiceClientSecret

	audienceList := make([]string, 0)

	if notificationConfig.Oauth2ServiceAudience != "" {
		audienceList = strings.Split(notificationConfig.Oauth2ServiceAudience, ",")
	}

	profileCli, err := profileV1.NewProfileClient(ctx,
		apis.WithEndpoint(notificationConfig.ProfileServiceURI),
		apis.WithTokenEndpoint(oauth2ServiceURL),
		apis.WithTokenUsername(serviceName),
		apis.WithTokenPassword(oauth2ServiceSecret),
		apis.WithAudiences(audienceList...))
	if err != nil {
		log.WithError(err).Fatal("could not setup profile client")
	}

	partitionCli, err := partitionV1.NewPartitionsClient(
		ctx,
		apis.WithEndpoint(notificationConfig.PartitionServiceURI),
		apis.WithTokenEndpoint(oauth2ServiceURL),
		apis.WithTokenUsername(serviceName),
		apis.WithTokenPassword(oauth2ServiceSecret),
		apis.WithAudiences(audienceList...))
	if err != nil {
		log.WithError(err).Fatal("could not setup partition client")
	}

	var serviceOptions []frame.Option

	jwtAudience := notificationConfig.Oauth2JwtVerifyAudience
	if jwtAudience == "" {
		jwtAudience = serviceName
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(grpcMiddleware.ChainUnaryServer(
			grpcctxtags.UnaryServerInterceptor(),
			grpcrecovery.UnaryServerInterceptor(),
			service.UnaryAuthInterceptor(jwtAudience, notificationConfig.Oauth2JwtVerifyIssuer),
		)),
		grpc.StreamInterceptor(service.StreamAuthInterceptor(jwtAudience, notificationConfig.Oauth2JwtVerifyIssuer)),
	)

	implementation := &handlers.NotificationServer{
		Service:      service,
		ProfileCli:   profileCli,
		PartitionCli: partitionCli,
	}

	notificationV1.RegisterNotificationServiceServer(grpcServer, implementation)

	grpcServerOpt := frame.GrpcServer(grpcServer)
	serviceOptions = append(serviceOptions, grpcServerOpt)

	serviceOptions = append(serviceOptions,
		frame.RegisterEvents(
			&events.NotificationSave{Service: service},
			&events.NotificationStatusSave{Service: service},
			&events.NotificationInRoute{Service: service},
			&events.NotificationOutRoute{Service: service, ProfileCli: profileCli},
			&events.NotificationOutQueue{Service: service, ProfileCli: profileCli}))

	service.Init(serviceOptions...)

	serverPort := notificationConfig.ServerPort
	if serverPort == "" {
		serverPort = "7020"
	}

	log.WithField("port", serverPort).Info(" initiating server operations")
	defer implementation.Service.Stop(ctx)
	err = implementation.Service.Run(ctx, fmt.Sprintf(":%v", serverPort))
	if err != nil {
		log.WithError(err).Fatal("could not run Server ")
	}
}
