package main

import (
	"fmt"
	apis "github.com/antinvestor/apis/go/common"
	partitionV1 "github.com/antinvestor/apis/go/partition/v1"
	"github.com/bufbuild/protovalidate-go"
	"github.com/sirupsen/logrus"
	"strings"

	notificationV1 "github.com/antinvestor/apis/go/notification/v1"
	"github.com/antinvestor/service-notification/config"
	"github.com/antinvestor/service-notification/service/events"

	"github.com/antinvestor/service-notification/service/handlers"
	"github.com/antinvestor/service-notification/service/models"

	profileV1 "github.com/antinvestor/apis/go/profile/v1"
	protovalidateinterceptor "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/protovalidate"
	"github.com/pitabwire/frame"
	"google.golang.org/grpc"
)

func main() {

	serviceName := "service_notifications"

	var notificationConfig config.NotificationConfig
	err := frame.ConfigProcess("", &notificationConfig)
	if err != nil {
		logrus.WithError(err).Fatal("could not process configs")
		return
	}

	ctx, service := frame.NewService(serviceName, frame.Config(&notificationConfig))

	log := service.L()

	serviceOptions := []frame.Option{frame.Datastore(ctx)}

	if notificationConfig.DoDatabaseMigrate() {

		service.Init(serviceOptions...)

		err = service.MigrateDatastore(ctx, notificationConfig.GetDatabaseMigrationPath(),
			&models.Route{}, &models.Language{}, &models.Template{},
			&models.TemplateData{}, &models.Notification{}, &models.NotificationStatus{})

		if err != nil {
			log.WithError(err).Fatal("could not migrate successfully")
		}
		return
	}

	err = service.RegisterForJwt(ctx)
	if err != nil {
		log.WithError(err).Fatal("main -- could not register fo jwt")
	}

	oauth2ServiceHost := notificationConfig.GetOauth2ServiceURI()
	oauth2ServiceURL := fmt.Sprintf("%s/oauth2/token", oauth2ServiceHost)
	oauth2ServiceSecret := notificationConfig.Oauth2ServiceClientSecret

	audienceList := make([]string, 0)

	if notificationConfig.Oauth2ServiceAudience != "" {
		audienceList = strings.Split(notificationConfig.Oauth2ServiceAudience, ",")
	}

	profileCli, err := profileV1.NewProfileClient(ctx,
		apis.WithEndpoint(notificationConfig.ProfileServiceURI),
		apis.WithTokenEndpoint(oauth2ServiceURL),
		apis.WithTokenUsername(service.JwtClientID()),
		apis.WithTokenPassword(oauth2ServiceSecret),
		apis.WithAudiences(audienceList...))
	if err != nil {
		log.WithError(err).Fatal("could not setup profile client")
	}

	partitionCli, err := partitionV1.NewPartitionsClient(
		ctx,
		apis.WithEndpoint(notificationConfig.PartitionServiceURI),
		apis.WithTokenEndpoint(oauth2ServiceURL),
		apis.WithTokenUsername(service.JwtClientID()),
		apis.WithTokenPassword(oauth2ServiceSecret),
		apis.WithAudiences(audienceList...))
	if err != nil {
		log.WithError(err).Fatal("could not setup partition client")
	}

	jwtAudience := notificationConfig.Oauth2JwtVerifyAudience
	if jwtAudience == "" {
		jwtAudience = serviceName
	}

	validator, err := protovalidate.New()
	if err != nil {
		log.WithError(err).Fatal("could not load validator for proto messages")
	}

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			service.UnaryAuthInterceptor(jwtAudience, notificationConfig.Oauth2JwtVerifyIssuer),
			protovalidateinterceptor.UnaryServerInterceptor(validator),
		),
		grpc.ChainStreamInterceptor(
			service.StreamAuthInterceptor(jwtAudience, notificationConfig.Oauth2JwtVerifyIssuer),
			protovalidateinterceptor.StreamServerInterceptor(validator),
		),
	)

	implementation := &handlers.NotificationServer{
		Service:      service,
		ProfileCli:   profileCli,
		PartitionCli: partitionCli,
	}

	notificationV1.RegisterNotificationServiceServer(grpcServer, implementation)

	grpcServerOpt := frame.GrpcServer(grpcServer)
	serviceOptions = append(serviceOptions, grpcServerOpt, frame.EnableGrpcServerReflection())

	serviceOptions = append(serviceOptions,
		frame.RegisterEvents(
			&events.NotificationSave{Service: service},
			&events.NotificationStatusSave{Service: service},
			&events.NotificationInRoute{Service: service},
			&events.NotificationInQueue{Service: service, ProfileCli: profileCli},
			&events.NotificationOutRoute{Service: service, ProfileCli: profileCli},
			&events.NotificationOutQueue{Service: service, ProfileCli: profileCli}))

	service.Init(serviceOptions...)

	log.WithField("server http port", notificationConfig.HttpServerPort).
		WithField("server grpc port", notificationConfig.GrpcServerPort).
		Info(" Initiating server operations")

	defer implementation.Service.Stop(ctx)
	err = implementation.Service.Run(ctx, "")
	if err != nil {
		log.WithError(err).Fatal("could not run Server ")
	}
}
