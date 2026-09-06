package events

import (
	"context"
	"testing"

	aconfig "github.com/antinvestor/service-notification/apps/default/config"
	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/antinvestor/service-notification/apps/default/service/repository"
	internaltests "github.com/antinvestor/service-notification/pkg/tests"
	"github.com/pitabwire/frame/v2"
	"github.com/pitabwire/frame/v2/config"
	"github.com/pitabwire/frame/v2/data"
	"github.com/pitabwire/frame/v2/datastore"
	"github.com/pitabwire/frame/v2/frametests"
	"github.com/pitabwire/frame/v2/frametests/definition"
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
				"code":       "1234",
				"expiryDate": "tomorrow",
			},
		}

		event := &NotificationOutQueue{
			templateDataRepo: templateDataRepo,
		}

		messageMap, err := event.formatOutboundNotification(ctx, util.Log(ctx), n, make(map[string]string))
		require.NoError(t, err)
		require.NotEmpty(t, messageMap)
		require.Equal(t, "Your contact verification code is : 1234 and will expire at tomorrow", messageMap["text"])
	})
}

func (s *NotificationOutQueueTestSuite) Test_formatOutboundNotification_WhatsAppTemplateExtra() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, templateDataRepo, templateRepo := s.createServiceWithTemplates(t, dep)

		tmpl := &models.Template{Name: "template.test.whatsapp.otp", Extra: data.JSONMap{
			"whatsapp": map[string]any{"name": "otp_code", "language": "en_US", "params": []any{"code"}},
		}}
		require.NoError(t, templateRepo.Create(ctx, tmpl))
		require.NoError(t, templateDataRepo.Create(ctx, &models.TemplateData{
			TemplateID: tmpl.GetID(), LanguageID: "9bsv0s23l8og00vgjqa0", Type: models.RouteTypeWhatsAppForm,
			Detail: "Code {{.code}}", Subject: "Code for {{.code}}",
		}))

		event := &NotificationOutQueue{templateDataRepo: templateDataRepo, templateRepo: templateRepo}

		n := &models.Notification{TemplateID: tmpl.GetID(), LanguageID: "9bsv0s23l8og00vgjqa0",
			NotificationType: models.RouteTypeWhatsAppForm, Payload: data.JSONMap{"code": "9876"}}
		messageMap, err := event.formatOutboundNotification(ctx, util.Log(ctx), n, nil)
		require.NoError(t, err)
		require.Equal(t, "Code 9876", messageMap[models.RouteTypeWhatsAppForm])
		require.Equal(t, "Code for 9876", messageMap[models.ExtraKeySubject])
		require.JSONEq(t, `{"name":"otp_code","language":"en_US","params":["code"]}`, messageMap[models.ExtraKeyWhatsAppTemplate])

		// SMS on the same template carries no WhatsApp definition.
		n.NotificationType = models.RouteTypeSMSForm
		messageMap, err = event.formatOutboundNotification(ctx, util.Log(ctx), n, nil)
		require.NoError(t, err)
		require.NotContains(t, messageMap, models.ExtraKeyWhatsAppTemplate)

		// A pre-rendered message short-circuits templating.
		n.Message = "prerendered"
		messageMap, err = event.formatOutboundNotification(ctx, util.Log(ctx), n, nil)
		require.NoError(t, err)
		require.Equal(t, "prerendered", messageMap[models.MessageBodyDefaultKey])

		// Missing template content is an error, not an empty message.
		n.Message = ""
		n.LanguageID = "no-such-language"
		_, err = event.formatOutboundNotification(ctx, util.Log(ctx), n, nil)
		require.ErrorContains(t, err, "no content")
	})
}

func (s *NotificationOutQueueTestSuite) createServiceWithTemplates(t *testing.T, depOpts *definition.DependencyOption) (context.Context, repository.TemplateDataRepository, repository.TemplateRepository) {
	ctx, templateDataRepo := s.createService(t, depOpts)
	svc := frame.FromContext(ctx)
	dbPool := svc.DatastoreManager().GetPool(ctx, datastore.DefaultPoolName)
	return ctx, templateDataRepo, repository.NewTemplateRepository(ctx, dbPool, svc.WorkManager())
}
