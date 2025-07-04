package main

import (
	"fmt"
	"log/slog"
	"strings"

	"buf.build/go/protovalidate"
	apis "github.com/antinvestor/apis/go/common"
	notificationV1 "github.com/antinvestor/apis/go/notification/v1"
	partitionV1 "github.com/antinvestor/apis/go/partition/v1"
	profileV1 "github.com/antinvestor/apis/go/profile/v1"
	"github.com/antinvestor/service-notification/apps/default/config"
	events2 "github.com/antinvestor/service-notification/apps/default/service/events"
	"github.com/antinvestor/service-notification/apps/default/service/handlers"
	"github.com/antinvestor/service-notification/apps/default/service/models"
	protovalidateinterceptor "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/protovalidate"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/pitabwire/frame"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {

	serviceName := "service_notifications"

	notificationConfig, err := frame.ConfigFromEnv[config.NotificationConfig]()
	if err != nil {
		slog.With("err", err).Error("could not process configs")
		return
	}

	ctx, service := frame.NewService(serviceName, frame.WithConfig(&notificationConfig))

	log := service.Log(ctx)

	serviceOptions := []frame.Option{frame.WithDatastore()}

	if notificationConfig.DoDatabaseMigrate() {

		service.Init(ctx, serviceOptions...)

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
			recovery.UnaryServerInterceptor(recovery.WithRecoveryHandlerContext(frame.RecoveryHandlerFun)),
			service.UnaryAuthInterceptor(jwtAudience, notificationConfig.Oauth2JwtVerifyIssuer),
			protovalidateinterceptor.UnaryServerInterceptor(validator),
		),
		grpc.ChainStreamInterceptor(
			recovery.StreamServerInterceptor(recovery.WithRecoveryHandlerContext(frame.RecoveryHandlerFun)),
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

	grpcServerOpt := frame.WithGRPCServer(grpcServer)
	serviceOptions = append(serviceOptions, grpcServerOpt)

	proxyOptions := apis.ProxyOptions{
		GrpcServerEndpoint: fmt.Sprintf("localhost:%s", notificationConfig.GrpcServerPort),
		GrpcServerDialOpts: []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
	}

	proxyMux, err := notificationV1.CreateProxyHandler(ctx, proxyOptions)
	if err != nil {
		log.WithError(err).Fatal("could not create proxy handler")
		return
	}

	proxyServerOpt := frame.WithHTTPHandler(proxyMux)
	serviceOptions = append(serviceOptions, proxyServerOpt)

	serviceOptions = append(serviceOptions,
		frame.WithRegisterEvents(
			&events2.NotificationSave{Service: service},
			&events2.NotificationStatusSave{Service: service},
			&events2.NotificationInRoute{Service: service},
			&events2.NotificationInQueue{Service: service, ProfileCli: profileCli},
			&events2.NotificationOutRoute{Service: service, ProfileCli: profileCli},
			&events2.NotificationOutQueue{Service: service, ProfileCli: profileCli}))

	service.Init(ctx, serviceOptions...)

	log.WithField("server http port", notificationConfig.HTTPServerPort).
		WithField("server grpc port", notificationConfig.GrpcServerPort).
		Info(" Initiating server operations")

	defer implementation.Service.Stop(ctx)
	err = implementation.Service.Run(ctx, "")
	if err != nil {
		log.WithError(err).Fatal("could not run Server ")
	}
}
