package main

import (
	"context"
	"fmt"

	"buf.build/go/protovalidate"
	apis "github.com/antinvestor/apis/go/common"
	notificationv1 "github.com/antinvestor/apis/go/notification/v1"
	partitionV1 "github.com/antinvestor/apis/go/partition/v1"
	profilev1 "github.com/antinvestor/apis/go/profile/v1"
	"github.com/antinvestor/service-notification/apps/default/config"
	events2 "github.com/antinvestor/service-notification/apps/default/service/events"
	"github.com/antinvestor/service-notification/apps/default/service/handlers"
	"github.com/antinvestor/service-notification/apps/default/service/repository"
	protovalidateinterceptor "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/protovalidate"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {

	serviceName := "service_notifications"

	ctx := context.Background()

	cfg, err := frame.ConfigLoadWithOIDC[config.NotificationConfig](ctx)
	if err != nil {
		util.Log(ctx).With("err", err).Error("could not process configs")
		return
	}

	ctx, svc := frame.NewServiceWithContext(ctx, serviceName, frame.WithConfig(&cfg))

	log := svc.Log(ctx)

	serviceOptions := []frame.Option{frame.WithDatastore()}

	// Handle database migration if requested
	if handleDatabaseMigration(ctx, svc, cfg, log) {
		return
	}

	err = svc.RegisterForJwt(ctx)
	if err != nil {
		log.WithError(err).Fatal("main -- could not register fo jwt")
	}

	profileCli, err := profilev1.NewProfileClient(ctx,
		apis.WithEndpoint(cfg.ProfileServiceURI),
		apis.WithTokenEndpoint(cfg.GetOauth2TokenEndpoint()),
		apis.WithTokenUsername(svc.JwtClientID()),
		apis.WithTokenPassword(svc.JwtClientSecret()),
		apis.WithAudiences("service_profile"))
	if err != nil {
		log.WithError(err).Fatal("could not setup profile client")
	}

	partitionCli, err := partitionV1.NewPartitionsClient(
		ctx,
		apis.WithEndpoint(cfg.PartitionServiceURI),
		apis.WithTokenEndpoint(cfg.GetOauth2TokenEndpoint()),
		apis.WithTokenUsername(svc.JwtClientID()),
		apis.WithTokenPassword(svc.JwtClientSecret()),
		apis.WithAudiences("service_partition"))
	if err != nil {
		log.WithError(err).Fatal("could not setup partition client")
	}

	jwtAudience := cfg.Oauth2JwtVerifyAudience
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
			svc.UnaryAuthInterceptor(jwtAudience, cfg.GetOauth2Issuer()),
			protovalidateinterceptor.UnaryServerInterceptor(validator),
		),
		grpc.ChainStreamInterceptor(
			recovery.StreamServerInterceptor(recovery.WithRecoveryHandlerContext(frame.RecoveryHandlerFun)),
			svc.StreamAuthInterceptor(jwtAudience, cfg.GetOauth2Issuer()),
			protovalidateinterceptor.StreamServerInterceptor(validator),
		),
	)

	implementation := &handlers.NotificationServer{

		Service:      svc,
		ProfileCli:   profileCli,
		PartitionCli: partitionCli,
	}

	notificationv1.RegisterNotificationServiceServer(grpcServer, implementation)

	grpcServerOpt := frame.WithGRPCServer(grpcServer)
	serviceOptions = append(serviceOptions, grpcServerOpt)

	proxyOptions := apis.ProxyOptions{
		GrpcServerEndpoint: fmt.Sprintf("localhost:%s", cfg.GrpcServerPort),
		GrpcServerDialOpts: []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
	}

	proxyMux, err := notificationv1.CreateProxyHandler(ctx, proxyOptions)
	if err != nil {
		log.WithError(err).Fatal("could not create proxy handler")
		return
	}

	proxyServerOpt := frame.WithHTTPHandler(proxyMux)
	serviceOptions = append(serviceOptions, proxyServerOpt)

	serviceOptions = append(serviceOptions,
		frame.WithRegisterEvents(
			&events2.NotificationSave{Service: svc},
			&events2.NotificationStatusSave{Service: svc},
			&events2.NotificationInRoute{Service: svc},
			&events2.NotificationInQueue{Service: svc, ProfileCli: profileCli},
			&events2.NotificationOutRoute{Service: svc, ProfileCli: profileCli},
			&events2.NotificationOutQueue{Service: svc, ProfileCli: profileCli}))

	svc.Init(ctx, serviceOptions...)

	log.WithField("server http port", cfg.HTTPServerPort).
		WithField("server grpc port", cfg.GrpcServerPort).
		Info(" Initiating server operations")

	defer implementation.Service.Stop(ctx)
	err = implementation.Service.Run(ctx, "")
	if err != nil {
		log.WithError(err).Fatal("could not run Server ")
	}
}

// handleDatabaseMigration performs database migration if configured to do so.
func handleDatabaseMigration(
	ctx context.Context,
	svc *frame.Service,
	cfg config.NotificationConfig,
	log *util.LogEntry,
) bool {
	serviceOptions := []frame.Option{frame.WithDatastore()}

	if cfg.DoDatabaseMigrate() {
		svc.Init(ctx, serviceOptions...)

		err := repository.Migrate(ctx, svc, cfg.GetDatabaseMigrationPath())
		if err != nil {
			log.WithError(err).Fatal("main -- Could not migrate successfully")
		}
		return true
	}
	return false
}
