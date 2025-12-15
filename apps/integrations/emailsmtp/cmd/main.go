package main

import (
	"context"

	"buf.build/gen/go/antinvestor/notification/connectrpc/go/notification/v1/notificationv1connect"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	"buf.build/gen/go/antinvestor/settingz/connectrpc/go/settings/v1/settingsv1connect"
	apis "github.com/antinvestor/apis/go/common"
	"github.com/antinvestor/apis/go/notification"
	"github.com/antinvestor/apis/go/profile"
	"github.com/antinvestor/apis/go/settings"
	aconfig "github.com/antinvestor/service-notification/apps/integrations/emailsmtp/config"
	"github.com/antinvestor/service-notification/apps/integrations/emailsmtp/service/client"
	"github.com/antinvestor/service-notification/apps/integrations/emailsmtp/service/handlers"
	"github.com/antinvestor/service-notification/apps/integrations/emailsmtp/service/queues"
	"github.com/antinvestor/service-notification/internal/events"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/config"
	"github.com/pitabwire/frame/security"
	"github.com/pitabwire/frame/security/openid"
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

	ctx, svc := frame.NewServiceWithContext(ctx, frame.WithConfig(&cfg), frame.WithRegisterServerOauth2Client())
	defer svc.Stop(ctx)

	logger := svc.Log(ctx)

	sm := svc.SecurityManager()
	eventsMan := svc.EventsManager()

	notificationCli, err := setupNotificationClient(ctx, sm, cfg)
	if err != nil {
		logger.WithError(err).Fatal("could not setup notification client")
	}

	profileCli, err := setupProfileClient(ctx, sm, cfg)
	if err != nil {
		logger.WithError(err).Fatal("could not setup profile client")
	}

	settingsCli, err := setupSettingsClient(ctx, sm, cfg)
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
		frame.WithRegisterSubscriber(cfg.QueueATDequeueName, cfg.QueueATDequeueURI, messageHandler),
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
	clHolder security.InternalOauth2ClientHolder,
	cfg aconfig.EmailSMTPConfig) (profilev1connect.ProfileServiceClient, error) {
	return profile.NewClient(ctx,
		apis.WithEndpoint(cfg.ProfileServiceURI),
		apis.WithTokenEndpoint(cfg.GetOauth2TokenEndpoint()),
		apis.WithTokenUsername(clHolder.JwtClientID()),
		apis.WithTokenPassword(clHolder.JwtClientSecret()),
		apis.WithScopes(openid.ConstSystemScopeInternal),
		apis.WithAudiences("service_profile"))
}

// setupNotificationClient creates and configures the notification client.
func setupNotificationClient(
	ctx context.Context,
	clHolder security.InternalOauth2ClientHolder,
	cfg aconfig.EmailSMTPConfig) (notificationv1connect.NotificationServiceClient, error) {
	return notification.NewClient(ctx,
		apis.WithEndpoint(cfg.NotificationServiceURI),
		apis.WithTokenEndpoint(cfg.GetOauth2TokenEndpoint()),
		apis.WithTokenUsername(clHolder.JwtClientID()),
		apis.WithTokenPassword(clHolder.JwtClientSecret()),
		apis.WithScopes(openid.ConstSystemScopeInternal),
		apis.WithAudiences("service_notifications"))
}

// setupSettingsClient creates and configures the settings client.
func setupSettingsClient(
	ctx context.Context,
	clHolder security.InternalOauth2ClientHolder,
	cfg aconfig.EmailSMTPConfig) (settingsv1connect.SettingsServiceClient, error) {
	return settings.NewClient(ctx,
		apis.WithEndpoint(cfg.SettingsServiceURI),
		apis.WithTokenEndpoint(cfg.GetOauth2TokenEndpoint()),
		apis.WithTokenUsername(clHolder.JwtClientID()),
		apis.WithTokenPassword(clHolder.JwtClientSecret()),
		apis.WithScopes(openid.ConstSystemScopeInternal),
		apis.WithAudiences("service_settings"))
}
