package business_test

import (
	"testing"
	"time"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	notificationv1 "buf.build/gen/go/antinvestor/notification/protocolbuffers/go/notification/v1"
	"connectrpc.com/connect"
	"github.com/antinvestor/service-notification/apps/default/service/models"
	"github.com/antinvestor/service-notification/apps/default/tests"
	"github.com/pitabwire/frame/v2/frametests/definition"
	"github.com/pitabwire/frame/v2/workerpool"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// RobustnessTestSuite covers the pipeline defects fixed alongside the WhatsApp channel.
type RobustnessTestSuite struct {
	tests.BaseTestSuite
}

func TestRobustnessSuite(t *testing.T) {
	suite.Run(t, new(RobustnessTestSuite))
}

func storedNotification(t *testing.T, resources *tests.ServiceResources, mutate func(n *models.Notification)) *models.Notification {
	t.Helper()
	n := &models.Notification{
		RecipientContactID: "contact-1",
		Message:            "body",
		NotificationType:   models.RouteTypeSMSForm,
		OutBound:           true,
		LanguageID:         "9bsv0s23l8og00vgjqa0",
	}
	n.AccessID = "testingAccessData"
	n.PartitionID = "test_partition-id"
	n.TenantID = "test_tenant-id"
	if mutate != nil {
		mutate(n)
	}
	require.NoError(t, resources.NotificationRepo.Create(t.Context(), n))
	return n
}

// Release must persist released_at on the row itself: the previous implementation
// re-emitted a save event that the duplicate guard silently dropped.
func (s *RobustnessTestSuite) TestReleasePersistsReleaseDate() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		_, ctx, resources := s.CreateService(t, dep)

		n := storedNotification(t, resources, nil)
		require.False(t, n.IsReleased())

		pipe, err := resources.NotificationBusiness.Release(ctx, &notificationv1.ReleaseRequest{Id: []string{n.GetID()}, Comment: "test"})
		require.NoError(t, err)

		var got []*commonv1.StatusResponse
		require.NoError(t, workerpool.ConsumeResultStream(ctx, pipe, func(res *notificationv1.ReleaseResponse) error {
			got = append(got, res.GetData()...)
			return nil
		}))
		require.Len(t, got, 1)
		require.Equal(t, commonv1.STATUS_QUEUED, got[0].GetStatus())

		stored, err := resources.NotificationRepo.GetByID(ctx, n.GetID())
		require.NoError(t, err)
		require.True(t, stored.IsReleased(), "released_at must be persisted")

		// Releasing again reports the existing status instead of re-dispatching.
		pipe, err = resources.NotificationBusiness.Release(ctx, &notificationv1.ReleaseRequest{Id: []string{n.GetID()}, Comment: "test"})
		require.NoError(t, err)
		got = nil
		require.NoError(t, workerpool.ConsumeResultStream(ctx, pipe, func(res *notificationv1.ReleaseResponse) error {
			got = append(got, res.GetData()...)
			return nil
		}))
		require.Len(t, got, 1)
	})
}

// Delivery reports only know the provider's message id.
func (s *RobustnessTestSuite) TestStatusUpdateByExternalID() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		_, ctx, resources := s.CreateService(t, dep)

		n := storedNotification(t, resources, func(n *models.Notification) { n.ExternalID = "wamid.EXT-1" })

		got, err := resources.NotificationBusiness.StatusUpdate(ctx, &commonv1.StatusUpdateRequest{
			ExternalId: "wamid.EXT-1",
			State:      commonv1.STATE_INACTIVE,
			Status:     commonv1.STATUS_SUCCESSFUL,
		})
		require.NoError(t, err)
		require.Equal(t, n.GetID(), got.GetId())
		require.Equal(t, commonv1.STATUS_SUCCESSFUL, got.GetStatus())

		_, err = resources.NotificationBusiness.StatusUpdate(ctx, &commonv1.StatusUpdateRequest{ExternalId: "wamid.NOPE"})
		require.Error(t, err)

		_, err = resources.NotificationBusiness.StatusUpdate(ctx, &commonv1.StatusUpdateRequest{})
		require.Error(t, err)
		require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})
}

// A status carrying an external id must stamp it on the notification so later reports resolve.
func (s *RobustnessTestSuite) TestStatusUpdateRecordsExternalID() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		_, ctx, resources := s.CreateService(t, dep)

		n := storedNotification(t, resources, nil)

		_, err := resources.NotificationBusiness.StatusUpdate(ctx, &commonv1.StatusUpdateRequest{
			Id:         n.GetID(),
			ExternalId: "wamid.SENT-1",
			State:      commonv1.STATE_ACTIVE,
			Status:     commonv1.STATUS_QUEUED,
		})
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			stored, getErr := resources.NotificationRepo.GetByID(ctx, n.GetID())
			return getErr == nil && stored.ExternalID == "wamid.SENT-1"
		}, 10*time.Second, 100*time.Millisecond, "status save must persist the external id")

		resolved, err := resources.NotificationRepo.GetByExternalID(ctx, "wamid.SENT-1")
		require.NoError(t, err)
		require.Equal(t, n.GetID(), resolved.GetID())
	})
}

// A FAILED status naming a fallback channel spawns exactly one child on that channel.
func (s *RobustnessTestSuite) TestFailedStatusWithFallbackSpawnsChild() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		_, ctx, resources := s.CreateService(t, dep)

		parent := storedNotification(t, resources, func(n *models.Notification) {
			n.NotificationType = models.RouteTypeWhatsAppForm
			n.RouteID = "route-wa"
		})

		fail := func(id string) {
			extras := map[string]any{models.StatusExtraFallbackChannel: models.RouteTypeSMSForm, "error": "131026 not on whatsapp"}
			_, err := resources.NotificationBusiness.StatusUpdate(ctx, &commonv1.StatusUpdateRequest{
				Id:     id,
				State:  commonv1.STATE_INACTIVE,
				Status: commonv1.STATUS_FAILED,
				Extras: mustStruct(t, extras),
			})
			require.NoError(t, err)
		}

		fail(parent.GetID())

		var child *models.Notification
		require.Eventually(t, func() bool {
			child = findChild(t, resources, parent.GetID())
			return child != nil
		}, 10*time.Second, 100*time.Millisecond, "fallback child must be created")

		require.Equal(t, models.RouteTypeSMSForm, child.NotificationType)
		require.Equal(t, parent.RecipientContactID, child.RecipientContactID)
		require.Equal(t, parent.Message, child.Message)
		require.Empty(t, child.RouteID)
		require.True(t, child.IsReleased())
		require.True(t, child.OutBound)

		// A duplicate provider webhook reports the same failure again: still one child.
		fail(parent.GetID())
		time.Sleep(time.Second)
		require.Equal(t, child.GetID(), findChild(t, resources, parent.GetID()).GetID())
		require.Equal(t, 1, countChildren(t, resources, parent.GetID()), "duplicate failures must not spawn a second child")

		// The child failing again with a fallback request must not create a grandchild.
		fail(child.GetID())
		time.Sleep(time.Second)
		require.Nil(t, findChild(t, resources, child.GetID()), "fallback is one hop only")
	})
}

// A notification whose release was persisted but whose routing emit failed must be routed
// by the next Release call rather than reported as already handled.
func (s *RobustnessTestSuite) TestReleaseReroutesStuckNotification() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		_, ctx, resources := s.CreateService(t, dep)

		released := time.Now()
		n := storedNotification(t, resources, func(n *models.Notification) {
			n.ReleasedAt = &released
			n.State = int32(commonv1.STATE_CHECKED)
		})

		pipe, err := resources.NotificationBusiness.Release(ctx, &notificationv1.ReleaseRequest{Id: []string{n.GetID()}})
		require.NoError(t, err)
		require.NoError(t, workerpool.ConsumeResultStream(ctx, pipe, func(*notificationv1.ReleaseResponse) error { return nil }))

		// The test partition has no routes, so routing runs and terminates the notification.
		require.Eventually(t, func() bool {
			stored, getErr := resources.NotificationRepo.GetByID(ctx, n.GetID())
			return getErr == nil && commonv1.STATE(stored.State) == commonv1.STATE_INACTIVE
		}, 10*time.Second, 100*time.Millisecond, "stuck released notification must be re-routed")
	})
}
