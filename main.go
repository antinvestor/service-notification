package main

import (
	"context"
	"fmt"
	partitionV1 "github.com/antinvestor/service-partition-api"

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

	datasource := frame.GetEnv(config.EnvDatabaseUrl, "postgres://ant:@nt@localhost/service_notification")
	mainDb := frame.Datastore(ctx, datasource, false)

	readOnlydatasource := frame.GetEnv(config.EnvReplicaDatabaseUrl, datasource)
	readDb := frame.Datastore(ctx, readOnlydatasource, true)

	service := frame.NewService(serviceName, mainDb, readDb)
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
			models.TempleteData{}, models.Notification{})

		if err != nil {
			log.Fatal("main -- Could not migrate successfully because : %v", err)
		}
		return

	}

	profileServiceUrl := frame.GetEnv(config.EnvProfileServiceUri, "127.0.0.1:7005")
	profileCli, err := profileV1.NewProfileClient(ctx, apis.WithEndpoint(profileServiceUrl))
	if err != nil {
		log.Fatal("main -- Could not setup profile client : %v", err)
	}

	partitionServiceUrl := frame.GetEnv(config.EnvPartitionServiceUri, "127.0.0.1:7003")
	partitionCli, err := partitionV1.NewPartitionsClient(ctx, apis.WithEndpoint(partitionServiceUrl))
	if err != nil {
		log.Fatal("main -- Could not setup partition client : %v", err)
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
		frame.RegisterEvents(&events.NotificationSave{Service: service},
		&events.NotificationInRoute{Service: service},
		&events.NotificationOutRoute{Service: service, ProfileCli: profileCli},
		&events.NotificationOutQueue{Service: service, ProfileCli: profileCli}))

	service.Init(serviceOptions...)

	serverPort := frame.GetEnv(config.EnvServerPort, "7020")

	log.Info(" main -- Initiating server operations on : %s", serverPort)
	err = implementation.Service.Run(ctx, fmt.Sprintf(":%v", serverPort))
	if err != nil {
		log.Fatal("main -- Could not run Server : %v", err)
	}

}
