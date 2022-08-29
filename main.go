package main

import (
	"context"
	"fmt"
	partitionV1 "github.com/antinvestor/service-partition-api"
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

	datasource := frame.GetEnv(config.EnvDatabaseURL, "postgres://ant:@nt@localhost/service_notification")
	mainDB := frame.Datastore(ctx, datasource, false)

	readOnlydatasource := frame.GetEnv(config.EnvReplicaDatabaseURL, datasource)
	readDB := frame.Datastore(ctx, readOnlydatasource, true)

	service := frame.NewService(serviceName, mainDB, readDB)
	log := service.L()

	isMigration, err := strconv.ParseBool(frame.GetEnv(config.EnvMigrate, "false"))
	if err != nil {
		isMigration = false
	}

	stdArgs := os.Args[1:]
	if (len(stdArgs) > 0 && stdArgs[0] == "migrate") || isMigration {
		migrationPath := frame.GetEnv(config.EnvMigrationPath, "./migrations/0001")
		err := service.MigrateDatastore(ctx, migrationPath,
			models.Route{}, models.Language{}, models.Templete{},
			models.TempleteData{}, models.Notification{}, models.NotificationStatus{})

		if err != nil {
			log.WithError(err).Fatal("could not migrate successfully")
		}
		return
	}

	oauth2ServiceHost := frame.GetEnv(config.EnvOauth2ServiceURI, "")
	oauth2ServiceURL := fmt.Sprintf("%s/oauth2/token", oauth2ServiceHost)
	oauth2ServiceSecret := frame.GetEnv(config.EnvOauth2ServiceClientSecret, "")

	audienceList := make([]string, 0)
	oauth2ServiceAudience := frame.GetEnv(config.EnvOauth2ServiceAudience, "")
	if oauth2ServiceAudience != "" {
		audienceList = strings.Split(oauth2ServiceAudience, ",")
	}

	profileServiceURL := frame.GetEnv(config.EnvProfileServiceURI, "127.0.0.1:7005")
	profileCli, err := profileV1.NewProfileClient(ctx,
		apis.WithEndpoint(profileServiceURL),
		apis.WithTokenEndpoint(oauth2ServiceURL),
		apis.WithTokenUsername(serviceName),
		apis.WithTokenPassword(oauth2ServiceSecret),
		apis.WithAudiences(audienceList...))
	if err != nil {
		log.WithError(err).Fatal("could not setup profile client")
	}

	partitionServiceURL := frame.GetEnv(config.EnvPartitionServiceURI, "127.0.0.1:7003")
	partitionCli, err := partitionV1.NewPartitionsClient(
		ctx,
		apis.WithEndpoint(partitionServiceURL),
		apis.WithTokenEndpoint(oauth2ServiceURL),
		apis.WithTokenUsername(serviceName),
		apis.WithTokenPassword(oauth2ServiceSecret),
		apis.WithAudiences(audienceList...))
	if err != nil {
		log.WithError(err).Fatal("could not setup partition client")
	}

	var serviceOptions []frame.Option

	jwtAudience := frame.GetEnv(config.EnvOauth2JwtVerifyAudience, serviceName)
	jwtIssuer := frame.GetEnv(config.EnvOauth2JwtVerifyIssuer, "")

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(grpcMiddleware.ChainUnaryServer(
			grpcctxtags.UnaryServerInterceptor(),
			grpcrecovery.UnaryServerInterceptor(),
			frame.UnaryAuthInterceptor(jwtAudience, jwtIssuer),
		)),
		grpc.StreamInterceptor(frame.StreamAuthInterceptor(jwtAudience, jwtIssuer)),
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

	serverPort := frame.GetEnv(config.EnvServerPort, "7020")
	log.WithField("port", serverPort).Info(" initiating server operations")
	defer implementation.Service.Stop(ctx)
	err = implementation.Service.Run(ctx, fmt.Sprintf(":%v", serverPort))
	if err != nil {
		log.WithError(err).Fatal("could not run Server ")
	}
}
