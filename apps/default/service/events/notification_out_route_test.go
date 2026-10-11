package events

import (
	"context"
	"errors"
	"sync"
	"testing"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	"buf.build/gen/go/antinvestor/profile/connectrpc/go/profile/v1/profilev1connect"
	profilev1 "buf.build/gen/go/antinvestor/profile/protocolbuffers/go/profile/v1"
	"connectrpc.com/connect"
	aconfig "github.com/antinvestor/service-notification/apps/default/config"
	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/antinvestor/service-notification/apps/default/service/repository"
	internaltests "github.com/antinvestor/service-notification/pkg/tests"
	"github.com/pitabwire/frame/v2"
	"github.com/pitabwire/frame/v2/config"
	"github.com/pitabwire/frame/v2/datastore"
	"github.com/pitabwire/frame/v2/events"
	"github.com/pitabwire/frame/v2/frametests"
	"github.com/pitabwire/frame/v2/frametests/definition"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// Seeded by migrations/0001/20191224_initialization.sql: partition with an sms and an email route.
const (
	seededPartitionID = "c2f4j7au6s7f91uqnokg"
	seededTenantID    = "c2f4j7au6s7f91uqnojg"
	seededSMSRouteID  = "9bsv0s23l8og00vgjq7g"
	seededMailRouteID = "9bsv0s23l8og00vgjq1g"
)

// recordingEvents captures emitted events instead of dispatching them.
type recordingEvents struct {
	events.Manager
	mu      sync.Mutex
	emitted map[string][]any
}

func (r *recordingEvents) Emit(_ context.Context, name string, payload any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.emitted == nil {
		r.emitted = map[string][]any{}
	}
	r.emitted[name] = append(r.emitted[name], payload)
	return nil
}

func (r *recordingEvents) statuses() []*models.NotificationStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*models.NotificationStatus
	for _, p := range r.emitted[NotificationStatusSaveEvent] {
		out = append(out, p.(*models.NotificationStatus))
	}
	return out
}

// profilesByID serves GetById from a fixed map.
type profilesByID struct {
	profilev1connect.ProfileServiceClient
	profiles map[string]*profilev1.ProfileObject
}

func (p *profilesByID) GetById(_ context.Context, req *connect.Request[profilev1.GetByIdRequest]) (*connect.Response[profilev1.GetByIdResponse], error) {
	obj, ok := p.profiles[req.Msg.GetId()]
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("profile not found"))
	}
	return connect.NewResponse(&profilev1.GetByIdResponse{Data: obj}), nil
}

type NotificationOutRouteTestSuite struct {
	internaltests.BaseTestSuite
}

func TestNotificationOutRouteSuite(t *testing.T) {
	suite.Run(t, new(NotificationOutRouteTestSuite))
}

func (s *NotificationOutRouteTestSuite) createRepos(t *testing.T, depOpts *definition.DependencyOption) (context.Context, repository.NotificationRepository, repository.RouteRepository) {
	ctx := t.Context()
	cfg, err := config.FromEnv[aconfig.NotificationConfig]()
	require.NoError(t, err)
	cfg.DatabaseMigrate = true
	cfg.RunServiceSecurely = false
	cfg.ServerPort = ""

	res := depOpts.ByIsDatabase(ctx)
	testDS, cleanup, err := res.GetRandomisedDS(ctx, depOpts.Prefix())
	require.NoError(t, err)
	t.Cleanup(func() { cleanup(ctx) })

	cfg.DatabasePrimaryURL = []string{testDS.String()}
	cfg.DatabaseReplicaURL = []string{testDS.String()}

	ctx, svc := frame.NewServiceWithContext(ctx, frame.WithConfig(&cfg), frame.WithDatastore(), frametests.WithNoopDriver())
	svc.Init(ctx)
	dbPool := svc.DatastoreManager().GetPool(ctx, datastore.DefaultPoolName)

	require.NoError(t, repository.Migrate(ctx, svc.DatastoreManager(), "../../migrations/0001"))
	require.NoError(t, svc.Run(ctx, ""))

	return ctx, repository.NewNotificationRepository(ctx, dbPool, svc.WorkManager()),
		repository.NewRouteRepository(ctx, dbPool, svc.WorkManager())
}

func (s *NotificationOutRouteTestSuite) Test_ProfileOnlyRecipientRoutesByProfileContact() {
	profiles := map[string]*profilev1.ProfileObject{
		"p-phone": {Id: "p-phone", Contacts: []*profilev1.ContactObject{
			{Id: "c-mail-unverified", Type: profilev1.ContactType_EMAIL, Detail: "a@example.com"},
			{Id: "c-phone", Type: profilev1.ContactType_MSISDN, Detail: "+254700000001", Verified: true},
		}},
		"p-mail": {Id: "p-mail", Contacts: []*profilev1.ContactObject{
			{Id: "c-phone-optout", Type: profilev1.ContactType_MSISDN, Detail: "+254700000002", Verified: true,
				CommunicationLevel: profilev1.CommunicationLevel_NO_CONTACT},
			{Id: "c-mail", Type: profilev1.ContactType_EMAIL, Detail: "b@example.com", Verified: true},
		}},
		"p-none": {Id: "p-none"},
	}

	tests := []struct {
		name        string
		profileID   string
		wantRoute   string
		wantContact string
		wantChannel string
		wantStep    string
		wantFailed  bool
	}{
		{name: "verified phone routes by sms", profileID: "p-phone",
			wantRoute: seededSMSRouteID, wantContact: "c-phone", wantChannel: models.RouteTypeSMSForm, wantStep: "routed_for_queue"},
		{name: "opted-out phone is skipped for verified email", profileID: "p-mail",
			wantRoute: seededMailRouteID, wantContact: "c-mail", wantChannel: models.RouteTypeEmailForm, wantStep: "routed_for_queue"},
		{name: "profile without contacts fails with clear status", profileID: "p-none",
			wantFailed: true, wantStep: "resolve_recipient_contact"},
	}

	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, notificationRepo, routeRepo := s.createRepos(t, dep)

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				n := &models.Notification{RecipientProfileID: tt.profileID, OutBound: true}
				n.GenID(ctx)
				n.TenantID = seededTenantID
				n.PartitionID = seededPartitionID
				require.NoError(t, notificationRepo.Create(ctx, n))

				rec := &recordingEvents{}
				handler := NewNotificationOutRoute(ctx, rec, &profilesByID{profiles: profiles}, notificationRepo, routeRepo)
				id := n.GetID()
				require.NoError(t, handler.Execute(ctx, &id))

				statuses := rec.statuses()
				require.Len(t, statuses, 1)
				require.Equal(t, tt.wantStep, statuses[0].Extra["step"])

				saved, err := notificationRepo.GetByID(ctx, id)
				require.NoError(t, err)

				if tt.wantFailed {
					require.Equal(t, int32(commonv1.STATUS_FAILED), statuses[0].Status)
					require.Equal(t, "recipient profile has no reachable contact", statuses[0].Extra["error"])
					require.Empty(t, saved.RouteID)
					require.Empty(t, rec.emitted[NotificationOutQueueEvent])
					return
				}

				require.Equal(t, int32(commonv1.STATUS_QUEUED), statuses[0].Status)
				require.Equal(t, tt.wantRoute, saved.RouteID)
				require.Equal(t, tt.wantContact, saved.RecipientContactID)
				require.Equal(t, tt.wantChannel, saved.NotificationType)
				require.Len(t, rec.emitted[NotificationOutQueueEvent], 1)
			})
		}
	})
}
