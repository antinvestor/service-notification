package main

import (
	"fmt"
	"log/slog"
	"strings"

	apis "github.com/antinvestor/apis/go/common"
	notificationv1 "github.com/antinvestor/apis/go/notification/v1"
	profilev1 "github.com/antinvestor/apis/go/profile/v1"
	"github.com/antinvestor/service-notification/apps/integrations/matrix/config"
	"github.com/antinvestor/service-notification/apps/integrations/matrix/service/client"
	"github.com/antinvestor/service-notification/apps/integrations/matrix/service/events"
	"github.com/pitabwire/frame"
)

func main() {

	serviceName := "service_matrix_integration"

	cfg, err := frame.ConfigFromEnv[config.NotificationMatrixConfig]()
	if err != nil {
		slog.With("err", err).Error("could not process configs")
		return
	}

	ctx, srv := frame.NewService(serviceName, frame.WithConfig(&cfg))
	defer srv.Stop(ctx)

	logger := srv.Log(ctx)

	err = srv.RegisterForJwt(ctx)
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
		apis.WithTokenUsername(srv.JwtClientID()),
		apis.WithTokenPassword(srv.JwtClientSecret()),
		apis.WithAudiences(audienceList...))
	if err != nil {
		logger.WithError(err).Fatal("could not setup notification client")
	}

	profileCli, err := profilev1.NewProfileClient(ctx,
		apis.WithEndpoint(cfg.ProfileServiceURI),
		apis.WithTokenEndpoint(oauth2ServiceURL),
		apis.WithTokenUsername(srv.JwtClientID()),
		apis.WithTokenPassword(srv.JwtClientSecret()),
		apis.WithAudiences(audienceList...))
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
