package tests

import (
	"context"
	"testing"

	"github.com/antinvestor/service-notification/apps/default/config"
	"github.com/antinvestor/service-notification/apps/default/service/events"
	"github.com/antinvestor/service-notification/apps/default/service/repository"
	internaltests "github.com/antinvestor/service-notification/internal/tests"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/stretchr/testify/require"
)

type BaseTestSuite struct {
	internaltests.BaseTestSuite
}

func (bs *BaseTestSuite) CreateService(
	t *testing.T,
	depOpts *definition.DependancyOption,
) (*frame.Service, context.Context) {

	ctx := t.Context()
	profileConfig, err := frame.ConfigFromEnv[config.NotificationConfig]()
	require.NoError(t, err)

	profileConfig.LogLevel = "debug"
	profileConfig.RunServiceSecurely = false
	profileConfig.ServerPort = ""

	for _, res := range depOpts.Database(ctx) {
		testDS, cleanup, err0 := res.GetRandomisedDS(ctx, depOpts.Prefix())
		require.NoError(t, err0)

		t.Cleanup(func() {
			cleanup(ctx)
		})

		profileConfig.DatabasePrimaryURL = []string{testDS.String()}
		profileConfig.DatabaseReplicaURL = []string{testDS.String()}
	}

	ctx, svc := frame.NewServiceWithContext(ctx, "profile tests",
		frame.WithConfig(&profileConfig),
		frame.WithDatastore(),
		frame.WithNoopDriver())

	profileCli := bs.GetProfileCli(ctx)

	svc.Init(ctx, frame.WithRegisterEvents(
		events.NewNotificationSave(ctx, svc),
		events.NewNotificationStatusSave(ctx, svc),
		events.NewNotificationInRoute(ctx, svc),
		events.NewNotificationInQueue(ctx, svc, profileCli),
		events.NewNotificationOutRoute(ctx, svc, profileCli),
		events.NewNotificationOutQueue(ctx, svc, profileCli)))

	err = repository.Migrate(ctx, svc, "../../migrations/0001")
	require.NoError(t, err)

	err = svc.Run(ctx, "")
	require.NoError(t, err)

	return svc, ctx
}
