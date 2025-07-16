package main

import (
	"fmt"
	"log/slog"
	"strings"

	apis "github.com/antinvestor/apis/go/common"
	notificationv1 "github.com/antinvestor/apis/go/notification/v1"
	profilev1 "github.com/antinvestor/apis/go/profile/v1"
	settingsv1 "github.com/antinvestor/apis/go/settings/v1"
	"github.com/antinvestor/service-notification/apps/integrations/emailsmtp/config"
	"github.com/antinvestor/service-notification/apps/integrations/emailsmtp/service/client"
	"github.com/antinvestor/service-notification/apps/integrations/emailsmtp/service/events"
	"github.com/antinvestor/service-notification/apps/integrations/emailsmtp/service/handlers"
	"github.com/pitabwire/frame"
)

func main() {

	serviceName := "integration_notification_emailsmtp"

	cfg, err := frame.ConfigFromEnv[config.EmailSMTPConfig]()
	if err != nil {
		slog.With("err", err).Error("could not process configs")
		return
	}

	ctx, svc := frame.NewService(serviceName, frame.WithConfig(&cfg))
	defer svc.Stop(ctx)

	logger := svc.Log(ctx)

	err = svc.RegisterForJwt(ctx)
	if err != nil {
		logger.WithError(err).Fatal("could not register for jwt")
		return
	}

	oauth2ServiceHost := cfg.GetOauth2ServiceURI()
	oauth2ServiceURL := fmt.Sprintf("%s/oauth2/token", oauth2ServiceHost)

	audienceList := make([]string, 0)

	if cfg.Oauth2ServiceAudience != "" {
		audienceList = strings.Split(cfg.Oauth2ServiceAudience, ",")
	}

	notificationCli, err := notificationv1.NewNotificationClient(ctx,
		apis.WithEndpoint(cfg.NotificationServiceURI),
		apis.WithTokenEndpoint(oauth2ServiceURL),
		apis.WithTokenUsername(svc.JwtClientID()),
		apis.WithTokenPassword(svc.JwtClientSecret()),
		apis.WithAudiences(audienceList...))
	if err != nil {
		logger.WithError(err).Fatal("could not setup notification client")
	}

	profileCli, err := profilev1.NewProfileClient(ctx,
		apis.WithEndpoint(cfg.ProfileServiceURI),
		apis.WithTokenEndpoint(oauth2ServiceURL),
		apis.WithTokenUsername(svc.JwtClientID()),
		apis.WithTokenPassword(svc.JwtClientSecret()),
		apis.WithAudiences(audienceList...))
	if err != nil {
		logger.WithError(err).Fatal("could not setup profile client")
	}

	settingsCli, err := settingsv1.NewsettingsClient(ctx,
		apis.WithEndpoint(cfg.SettingsServiceURI),
		apis.WithTokenEndpoint(oauth2ServiceURL),
		apis.WithTokenUsername(svc.JwtClientID()),
		apis.WithTokenPassword(svc.JwtClientSecret()),
		apis.WithAudiences(audienceList...))
	if err != nil {
		logger.WithError(err).Fatal("could not setup profile client")
	}

	emailSMTPCli, err := client.NewClient(logger, &cfg, profileCli, settingsCli)
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
