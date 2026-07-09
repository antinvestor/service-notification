package main

import (
	"context"

	"buf.build/gen/go/antinvestor/notification/connectrpc/go/notification/v1/notificationv1connect"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	"buf.build/gen/go/antinvestor/settingz/connectrpc/go/settings/v1/settingsv1connect"
	apis "github.com/antinvestor/common/v2"
	"github.com/antinvestor/common/v2/connection"
	"github.com/antinvestor/common/v2/servicecatalog"
	aconfig "github.com/antinvestor/service-notification/apps/integrations/emailsmtp/config"
	"github.com/antinvestor/service-notification/apps/integrations/emailsmtp/service/client"
	"github.com/antinvestor/service-notification/apps/integrations/emailsmtp/service/handlers"
	"github.com/antinvestor/service-notification/apps/integrations/emailsmtp/service/queues"
	"github.com/antinvestor/service-notification/pkg/events"
	"github.com/pitabwire/frame/v2"
	"github.com/pitabwire/frame/v2/config"
	"github.com/pitabwire/util"
)

func main() {

	ctx := context.Background()

	cfg, err := config.LoadWithOIDC[aconfig.EmailSMTPConfig](ctx)
	if err != nil {
		util.Log(ctx).With("err", err).Error("could not process configs")
		return
	}

	if cfg.Name() == "" {
		cfg.ServiceName = "integration_notification_emailsmtp"
	}

	ctx, svc := frame.NewServiceWithContext(ctx, frame.WithConfig(&cfg))
	defer svc.Stop(ctx)

	logger := svc.Log(ctx)

	eventsMan := svc.EventsManager()

	notificationCli, err := setupNotificationClient(ctx, cfg)
	if err != nil {
		logger.WithError(err).Fatal("could not setup notification client")
	}

	profileCli, err := setupProfileClient(ctx, cfg)
	if err != nil {
		logger.WithError(err).Fatal("could not setup profile client")
	}

	settingsCli, err := setupSettingsClient(ctx, cfg)
	if err != nil {
		logger.WithError(err).Fatal("could not setup profile client")
	}

	emailSMTPCli, err := client.NewClient(logger, &cfg, profileCli, settingsCli)
	if err != nil {
		logger.WithError(err).Fatal("could not setup email smtp client")
	}

	// Create handlers with injected dependencies
	implementation := handlers.NewSMTPServer(profileCli, notificationCli, emailSMTPCli)
	messageHandler := queues.NewMessageToSend(eventsMan, profileCli, notificationCli, emailSMTPCli)

	serviceOptions := []frame.Option{
		frame.WithHTTPHandler(implementation.NewRouterV1()),
		frame.WithRegisterEvents(events.NewNotificationStatusUpdate(ctx, notificationCli)),
		frame.WithRegisterSubscriber(cfg.QueueEmailSMTPDequeueName, cfg.QueueEmailSMTPDequeueURI, messageHandler),
	}

	svc.Init(ctx, serviceOptions...)

	logger.Info("Initiating Email SMTP integration server operations")
	err = svc.Run(ctx, "")
	if err != nil {
		logger.WithError(err).Error("could not run Server")
	}
}

// setupProfileClient creates and configures the profile client.
func setupProfileClient(
	ctx context.Context,
	cfg aconfig.EmailSMTPConfig) (profilev1connect.ProfileServiceClient, error) {
	return connection.NewServiceClient(ctx, &cfg, apis.ServiceTarget{
		Endpoint:              cfg.ProfileServiceURI,
		WorkloadAPITargetPath: cfg.ProfileServiceWorkloadAPITargetPath,
		ServiceID:             servicecatalog.ServiceProfile,
	}, profilev1connect.NewProfileServiceClient)
}

// setupNotificationClient creates and configures the notification client.
func setupNotificationClient(
	ctx context.Context,
	cfg aconfig.EmailSMTPConfig) (notificationv1connect.NotificationServiceClient, error) {
	return connection.NewServiceClient(ctx, &cfg, apis.ServiceTarget{
		Endpoint:              cfg.NotificationServiceURI,
		WorkloadAPITargetPath: cfg.NotificationServiceWorkloadAPITargetPath,
		ServiceID:             servicecatalog.ServiceNotification,
	}, notificationv1connect.NewNotificationServiceClient)
}

// setupSettingsClient creates and configures the settings client.
func setupSettingsClient(
	ctx context.Context,
	cfg aconfig.EmailSMTPConfig) (settingsv1connect.SettingsServiceClient, error) {
	return connection.NewServiceClient(ctx, &cfg, apis.ServiceTarget{
		Endpoint:              cfg.SettingsServiceURI,
		WorkloadAPITargetPath: cfg.SettingsServiceWorkloadAPITargetPath,
		ServiceID:             servicecatalog.ServiceSettings,
	}, settingsv1connect.NewSettingsServiceClient)
}
