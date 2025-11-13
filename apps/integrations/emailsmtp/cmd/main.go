package main

import (
	"context"

	apis "github.com/antinvestor/apis/go/common"
	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	profilev1 "buf.build/gen/go/antinvestor/profile/protocolbuffers/go/profile/v1"
	"github.com/antinvestor/service-notification/apps/integrations/emailsmtp/config"
	"github.com/antinvestor/service-notification/apps/integrations/emailsmtp/service/client"
	"github.com/antinvestor/service-notification/apps/integrations/emailsmtp/service/events"
	"github.com/antinvestor/service-notification/apps/integrations/emailsmtp/service/handlers"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/util"
)

func main() {

	serviceName := "integration_notification_emailsmtp"
	ctx := context.Background()

	cfg, err := frame.ConfigLoadWithOIDC[config.EmailSMTPConfig](ctx)
	if err != nil {
		util.Log(ctx).With("err", err).Error("could not process configs")
		return
	}

	ctx, svc := frame.NewServiceWithContext(ctx, serviceName, frame.WithConfig(&cfg))
	defer svc.Stop(ctx)

	logger := svc.Log(ctx)

	err = svc.RegisterForJwt(ctx)
	if err != nil {
		logger.WithError(err).Fatal("could not register for jwt")
		return
	}

	notificationCli, err := notificationv1.NewNotificationClient(ctx,
		apis.WithEndpoint(cfg.NotificationServiceURI),
		apis.WithTokenEndpoint(cfg.GetOauth2TokenEndpoint()),
		apis.WithTokenUsername(svc.JwtClientID()),
		apis.WithTokenPassword(svc.JwtClientSecret()),
		apis.WithScopes(frame.ConstSystemScopeInternal),
		apis.WithAudiences("service_notifications"))
	if err != nil {
		logger.WithError(err).Fatal("could not setup notification client")
	}

	profileCli, err := profilev1.NewProfileClient(ctx,
		apis.WithEndpoint(cfg.ProfileServiceURI),
		apis.WithTokenEndpoint(cfg.GetOauth2TokenEndpoint()),
		apis.WithTokenUsername(svc.JwtClientID()),
		apis.WithTokenPassword(svc.JwtClientSecret()),
		apis.WithScopes(frame.ConstSystemScopeInternal),
		apis.WithAudiences("service_profile"))
	if err != nil {
		logger.WithError(err).Fatal("could not setup profile client")
	}

	emailSMTPCli, err := client.NewClient(logger, &cfg, profileCli)
	if err != nil {
		logger.WithError(err).Fatal("could not setup profile client")
	}

	var serviceOptions []frame.Option

	implementation := &handlers.SMTPServer{
		Service:         svc,
		ProfileCli:      profileCli,
		NotificationCli: notificationCli,
		EmailSMTPCli:    emailSMTPCli,
	}

	serviceOptions = append(serviceOptions,
		frame.WithHTTPHandler(implementation.NewRouterV1()),
		frame.WithRegisterSubscriber(cfg.QueueATDequeueName, cfg.QueueATDequeueURI,
			&events.MessageToSend{
				Service:         svc,
				NotificationCli: notificationCli,
				ProfileCli:      profileCli,
				EmailSMTPCli:    emailSMTPCli,
			}),
	)

	svc.Init(ctx, serviceOptions...)

	logger.Info("Initiating Matrix integration server operations")
	err = svc.Run(ctx, "")
	if err != nil {
		logger.WithError(err).Error("could not run Server")
	}
}
