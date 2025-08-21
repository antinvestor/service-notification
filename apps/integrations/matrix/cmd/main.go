package main

import (
	"context"

	apis "github.com/antinvestor/apis/go/common"
	notificationv1 "github.com/antinvestor/apis/go/notification/v1"
	profilev1 "github.com/antinvestor/apis/go/profile/v1"
	"github.com/antinvestor/service-notification/apps/integrations/matrix/config"
	"github.com/antinvestor/service-notification/apps/integrations/matrix/service/client"
	"github.com/antinvestor/service-notification/apps/integrations/matrix/service/events"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/util"
)

func main() {

	serviceName := "integration_notification_matrix"
	ctx := context.Background()

	cfg, err := frame.ConfigLoadWithOIDC[config.NotificationMatrixConfig](ctx)
	if err != nil {
		util.Log(ctx).With("err", err).Error("could not process configs")
		return
	}

	ctx, srv := frame.NewServiceWithContext(ctx, serviceName, frame.WithConfig(&cfg))
	defer srv.Stop(ctx)

	logger := srv.Log(ctx)

	err = srv.RegisterForJwt(ctx)
	if err != nil {
		logger.WithError(err).Fatal("could not register for jwt")
		return
	}

	notificationCli, err := notificationv1.NewNotificationClient(ctx,
		apis.WithEndpoint(cfg.NotificationServiceURI),
		apis.WithTokenEndpoint(cfg.GetOauth2TokenEndpoint()),
		apis.WithTokenUsername(srv.JwtClientID()),
		apis.WithTokenPassword(srv.JwtClientSecret()),
		apis.WithScopes(frame.ConstInternalSystemScope),
		apis.WithAudiences("service_notifications"))
	if err != nil {
		logger.WithError(err).Fatal("could not setup notification client")
	}

	profileCli, err := profilev1.NewProfileClient(ctx,
		apis.WithEndpoint(cfg.ProfileServiceURI),
		apis.WithTokenEndpoint(cfg.GetOauth2TokenEndpoint()),
		apis.WithTokenUsername(srv.JwtClientID()),
		apis.WithTokenPassword(srv.JwtClientSecret()),
		apis.WithScopes(frame.ConstInternalSystemScope),
		apis.WithAudiences("service_profile"))
	if err != nil {
		logger.WithError(err).Fatal("could not setup profile client")
	}

	matrixCl, err := client.NewClient(logger, &cfg)
	if err != nil {
		logger.WithError(err).Fatal("could not setup profile client")
	}

	var serviceOptions []frame.Option

	serviceOptions = append(serviceOptions,
		// Register Matrix subscriber
		frame.WithRegisterSubscriber(cfg.QueueMatrixDequeueName, cfg.QueueMatrixDequeueURI,
			&events.MessageToSend{
				Service:         srv,
				NotificationCli: notificationCli,
				ProfileCli:      profileCli,
				MatrixCli:       matrixCl,
			}),
	)

	srv.Init(ctx, serviceOptions...)

	logger.Info("Initiating Matrix integration server operations")
	err = srv.Run(ctx, "")
	if err != nil {
		logger.WithError(err).Error("could not run Server")
	}
}
