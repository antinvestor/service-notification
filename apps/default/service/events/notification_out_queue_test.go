package events

import (
	"context"
	"testing"

	aconfig "github.com/antinvestor/service-notification/apps/default/config"
	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/antinvestor/service-notification/apps/default/service/repository"
	internaltests "github.com/antinvestor/service-notification/internal/tests"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/data"
	"github.com/pitabwire/frame/config"
	"github.com/pitabwire/frame/datastore"
	"github.com/pitabwire/frame/frametests"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/pitabwire/util"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type NotificationOutQueueTestSuite struct {
	internaltests.BaseTestSuite
}

func (s *NotificationOutQueueTestSuite) SetupSuite() {
	s.BaseTestSuite.SetupSuite()
}

func TestNotificationOutQueueSuite(t *testing.T) {
	suite.Run(t, new(NotificationOutQueueTestSuite))
}

func (s *NotificationOutQueueTestSuite) createService(t *testing.T, depOpts *definition.DependencyOption) (context.Context, repository.TemplateDataRepository) {
	ctx := t.Context()
	cfg, err := config.FromEnv[aconfig.NotificationConfig]()
	require.NoError(t, err)

	cfg.LogLevel = "debug"
	cfg.DatabaseMigrate = true
	cfg.DatabaseTraceQueries = false
	cfg.RunServiceSecurely = false
	cfg.ServerPort = ""

	res := depOpts.ByIsDatabase(ctx)
	testDS, cleanup, err0 := res.GetRandomisedDS(ctx, depOpts.Prefix())
	require.NoError(t, err0)

	t.Cleanup(func() {
		cleanup(ctx)
	})

	cfg.DatabasePrimaryURL = []string{testDS.String()}
	cfg.DatabaseReplicaURL = []string{testDS.String()}

	ctx, svc := frame.NewServiceWithContext(ctx,
		frame.WithConfig(&cfg),
		frame.WithDatastore(),
		frametests.WithNoopDriver())

	workMan := svc.WorkManager()
	dbPool := svc.DatastoreManager().GetPool(ctx, datastore.DefaultPoolName)

	svc.Init(ctx)

	templateDataRepo := repository.NewTemplateDataRepository(ctx, dbPool, workMan)

	err = repository.Migrate(ctx, svc.DatastoreManager(), "../../migrations/0001")
	require.NoError(t, err)

	err = svc.Run(ctx, "")
	require.NoError(t, err)

	return ctx, templateDataRepo
}

func (s *NotificationOutQueueTestSuite) Test_formatOutboundNotification_TemplateDataLookupAndRender() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, templateDataRepo := s.createService(t, dep)

		n := &models.Notification{
			TemplateID: "9bsv0s23l8og00vgjq90",
			LanguageID: "9bsv0s23l8og00vgjqa0",
			Payload: data.JSONMap{
				"pin":        "1234",
				"expiryDate": "tomorrow",
			},
		}

		event := &NotificationOutQueue{
			TemplateDataRepo: templateDataRepo,
		}

		messageMap, err := event.formatOutboundNotification(ctx, util.Log(ctx), n)
		require.NoError(t, err)
		require.NotEmpty(t, messageMap)
		require.Equal(t, "Your contact verification code is : 1234 and will expire at tomorrow", messageMap["text"])
	})
}
