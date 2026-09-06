package main

import (
	"context"

	"buf.build/gen/go/antinvestor/notification/connectrpc/go/notification/v1/notificationv1connect"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	"buf.build/gen/go/antinvestor/settingz/connectrpc/go/settings/v1/settingsv1connect"
	apis "github.com/antinvestor/common/v2"
	"github.com/antinvestor/common/v2/connection"
	"github.com/antinvestor/common/v2/servicecatalog"
	aconfig "github.com/antinvestor/service-notification/apps/integrations/whatsapp/config"
	"github.com/antinvestor/service-notification/apps/integrations/whatsapp/service/client"
	"github.com/antinvestor/service-notification/apps/integrations/whatsapp/service/handlers"
	"github.com/antinvestor/service-notification/apps/integrations/whatsapp/service/queue"
	"github.com/antinvestor/service-notification/pkg/events"
	"github.com/pitabwire/frame/v2"
	"github.com/pitabwire/frame/v2/config"
	"github.com/pitabwire/util"
)

func main() {
	ctx := context.Background()

	cfg, err := config.LoadWithOIDC[aconfig.WhatsAppConfig](ctx)
	if err != nil {
		util.Log(ctx).WithError(err).Error("could not process configs")
		return
	}

	if cfg.Name() == "" {
		cfg.ServiceName = "integration_notification_whatsapp"
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
		logger.WithError(err).Fatal("could not setup settings client")
	}

	whatsAppCli := client.NewClient(&cfg, profileCli, settingsCli)

	webhook := handlers.NewWebhookServer(&cfg, notificationCli, whatsAppCli)
	messageHandler := queue.NewMessageToSend(eventsMan, whatsAppCli)

	serviceOptions := []frame.Option{
		frame.WithHTTPHandler(webhook.NewRouterV1()),
		frame.WithRegisterEvents(events.NewNotificationStatusUpdate(ctx, notificationCli)),
		frame.WithRegisterSubscriber(cfg.QueueWhatsAppDequeueName, cfg.QueueWhatsAppDequeueURI, messageHandler),
	}

	svc.Init(ctx, serviceOptions...)

	logger.Info("Initiating WhatsApp integration server operations")
	if err = svc.Run(ctx, ""); err != nil {
		logger.WithError(err).Error("could not run Server")
	}
}

func setupProfileClient(ctx context.Context, cfg aconfig.WhatsAppConfig) (profilev1connect.ProfileServiceClient, error) {
	return connection.NewServiceClient(ctx, &cfg, apis.ServiceTarget{
		Endpoint:              cfg.ProfileServiceURI,
		WorkloadAPITargetPath: cfg.ProfileServiceWorkloadAPITargetPath,
		ServiceID:             servicecatalog.ServiceProfile,
	}, profilev1connect.NewProfileServiceClient)
}

func setupNotificationClient(ctx context.Context, cfg aconfig.WhatsAppConfig) (notificationv1connect.NotificationServiceClient, error) {
	return connection.NewServiceClient(ctx, &cfg, apis.ServiceTarget{
		Endpoint:              cfg.NotificationServiceURI,
		WorkloadAPITargetPath: cfg.NotificationServiceWorkloadAPITargetPath,
		ServiceID:             servicecatalog.ServiceNotification,
	}, notificationv1connect.NewNotificationServiceClient)
}

func setupSettingsClient(ctx context.Context, cfg aconfig.WhatsAppConfig) (settingsv1connect.SettingsServiceClient, error) {
	return connection.NewServiceClient(ctx, &cfg, apis.ServiceTarget{
		Endpoint:              cfg.SettingsServiceURI,
		WorkloadAPITargetPath: cfg.SettingsServiceWorkloadAPITargetPath,
		ServiceID:             servicecatalog.ServiceSettings,
	}, settingsv1connect.NewSettingsServiceClient)
}
