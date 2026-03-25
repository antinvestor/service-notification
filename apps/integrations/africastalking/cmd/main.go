package main

import (
	"context"

	"buf.build/gen/go/antinvestor/notification/connectrpc/go/notification/v1/notificationv1connect"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	"buf.build/gen/go/antinvestor/settingz/connectrpc/go/settings/v1/settingsv1connect"
	apis "github.com/antinvestor/common"
	"github.com/antinvestor/common/connection"
	aconfig "github.com/antinvestor/service-notification/apps/integrations/africastalking/config"
	"github.com/antinvestor/service-notification/apps/integrations/africastalking/service/client"
	"github.com/antinvestor/service-notification/apps/integrations/africastalking/service/handlers"
	"github.com/antinvestor/service-notification/apps/integrations/africastalking/service/queue"
	"github.com/antinvestor/service-notification/internal/events"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/config"
	"github.com/pitabwire/util"
)

func main() {

	ctx := context.Background()

	cfg, err := config.LoadWithOIDC[aconfig.AfricasTalkingConfig](ctx)
	if err != nil {
		util.Log(ctx).With("err", err).Error("could not process configs")
		return
	}

	if cfg.Name() == "" {
		cfg.ServiceName = "integration_notification_africastalking"
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

	africasTalkingCli, err := client.NewClient(&cfg, profileCli, settingsCli)
	if err != nil {
		logger.WithError(err).Fatal("could not setup africa's Talking client")
	}

	// Create handlers with injected dependencies
	implementation := handlers.NewATServer(profileCli, notificationCli, africasTalkingCli)
	messageHandler := queue.NewMessageToSend(eventsMan, profileCli, notificationCli, africasTalkingCli)

	serviceOptions := []frame.Option{
		frame.WithHTTPHandler(implementation.NewRouterV1()),
		frame.WithRegisterEvents(events.NewNotificationStatusUpdate(ctx, notificationCli)),
		frame.WithRegisterSubscriber(cfg.QueueATDequeueName, cfg.QueueATDequeueURI, messageHandler),
	}

	svc.Init(ctx, serviceOptions...)

	logger.Info("Initiating Africa's Talking integration server operations")
	err = svc.Run(ctx, "")
	if err != nil {
		logger.WithError(err).Error("could not run Server")
	}
}

// setupProfileClient creates and configures the profile client.
func setupProfileClient(
	ctx context.Context,
	cfg aconfig.AfricasTalkingConfig) (profilev1connect.ProfileServiceClient, error) {
	return connection.NewServiceClient(ctx, &cfg, apis.ServiceTarget{
		Endpoint:              cfg.ProfileServiceURI,
		WorkloadAPITargetPath: cfg.ProfileServiceWorkloadAPITargetPath,
		Audiences:             []string{"service_profile"},
	}, profilev1connect.NewProfileServiceClient)
}

// setupNotificationClient creates and configures the notification client.
func setupNotificationClient(
	ctx context.Context,
	cfg aconfig.AfricasTalkingConfig) (notificationv1connect.NotificationServiceClient, error) {
	return connection.NewServiceClient(ctx, &cfg, apis.ServiceTarget{
		Endpoint:              cfg.NotificationServiceURI,
		WorkloadAPITargetPath: cfg.NotificationServiceWorkloadAPITargetPath,
		Audiences:             []string{"service_notification"},
	}, notificationv1connect.NewNotificationServiceClient)
}

// setupSettingsClient creates and configures the settings client.
func setupSettingsClient(
	ctx context.Context,
	cfg aconfig.AfricasTalkingConfig) (settingsv1connect.SettingsServiceClient, error) {
	return connection.NewServiceClient(ctx, &cfg, apis.ServiceTarget{
		Endpoint:              cfg.SettingsServiceURI,
		WorkloadAPITargetPath: cfg.SettingsServiceWorkloadAPITargetPath,
		Audiences:             []string{"service_setting"},
	}, settingsv1connect.NewSettingsServiceClient)
}
