package authz_test

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"github.com/antinvestor/service-notification/apps/default/service/authz"
	"github.com/antinvestor/service-notification/apps/default/tests/testketo"
	"github.com/pitabwire/frame/config"
	"github.com/pitabwire/frame/frametests"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/pitabwire/frame/frametests/deps/testpostgres"
	"github.com/pitabwire/frame/security"
	"github.com/pitabwire/frame/security/authorizer"
	"github.com/stretchr/testify/suite"
)

const (
	testTenantID    = "tenant1"
	testPartitionID = "partition1"
)

var testTenancyPath = fmt.Sprintf("%s/%s", testTenantID, testPartitionID)

// ---------------------------------------------------------------------------
// Test suite with real Keto
// ---------------------------------------------------------------------------

type MiddlewareTestSuite struct {
	frametests.FrameBaseTestSuite
	ketoReadURI  string
	ketoWriteURI string
}

func initMiddlewareResources(_ context.Context) []definition.TestResource {
	pg := testpostgres.NewWithOpts("authz_middleware_test",
		definition.WithUserName("ant"),
		definition.WithCredential("s3cr3t"),
		definition.WithEnableLogging(false),
		definition.WithUseHostMode(false),
	)
	keto := testketo.NewWithOpts(
		definition.WithDependancies(pg),
		definition.WithEnableLogging(false),
	)
	return []definition.TestResource{pg, keto}
}

func (s *MiddlewareTestSuite) SetupSuite() {
	s.InitResourceFunc = initMiddlewareResources
	s.FrameBaseTestSuite.SetupSuite()

	ctx := s.T().Context()
	var ketoDep definition.DependancyConn
	for _, res := range s.Resources() {
		if res.Name() == testketo.ImageName {
			ketoDep = res
			break
		}
	}
	s.Require().NotNil(ketoDep, "keto dependency should be available")

	writeURL, err := url.Parse(string(ketoDep.GetDS(ctx)))
	s.Require().NoError(err)
	s.ketoWriteURI = writeURL.Host

	readPort, err := ketoDep.PortMapping(ctx, "4466/tcp")
	s.Require().NoError(err)
	s.ketoReadURI = fmt.Sprintf("%s:%s", writeURL.Hostname(), readPort)
}

func (s *MiddlewareTestSuite) newAuthorizer() security.Authorizer {
	cfg := &config.ConfigurationDefault{
		AuthorizationServiceReadURI:  s.ketoReadURI,
		AuthorizationServiceWriteURI: s.ketoWriteURI,
	}
	return authorizer.NewKetoAdapter(cfg, nil)
}

func (s *MiddlewareTestSuite) ctxWithClaims(subjectID string) context.Context {
	claims := &security.AuthenticationClaims{
		TenantID:    testTenantID,
		PartitionID: testPartitionID,
	}
	claims.Subject = subjectID
	return claims.ClaimsToContext(context.Background())
}

func (s *MiddlewareTestSuite) ctxWithSystemInternalClaims(subjectID string) context.Context {
	claims := &security.AuthenticationClaims{
		TenantID:    testTenantID,
		PartitionID: testPartitionID,
		Roles:       []string{"system_internal"},
	}
	claims.Subject = subjectID
	return claims.ClaimsToContext(context.Background())
}

// seedRole writes functional permission tuples in service_notifications namespace.
func (s *MiddlewareTestSuite) seedRole(auth security.Authorizer, tenancyPath, profileID, role string) {
	permissions := authz.RolePermissions[role]
	tuples := make([]security.RelationTuple, 0, 1+len(permissions))

	tuples = append(tuples, security.RelationTuple{
		Object:   security.ObjectRef{Namespace: authz.NamespaceNotifications, ID: tenancyPath},
		Relation: role,
		Subject:  security.SubjectRef{Namespace: authz.NamespaceProfile, ID: profileID},
	})

	for _, perm := range permissions {
		tuples = append(tuples, security.RelationTuple{
			Object:   security.ObjectRef{Namespace: authz.NamespaceNotifications, ID: tenancyPath},
			Relation: perm,
			Subject:  security.SubjectRef{Namespace: authz.NamespaceProfile, ID: profileID},
		})
	}

	err := auth.WriteTuples(s.T().Context(), tuples)
	s.Require().NoError(err)
}

func TestMiddlewareSuite(t *testing.T) {
	suite.Run(t, new(MiddlewareTestSuite))
}

// ---------------------------------------------------------------------------
// FunctionChecker (middleware) tests — only checks service_notifications permissions
// ---------------------------------------------------------------------------

func (s *MiddlewareTestSuite) TestOwnerHasAllPermissions() {
	auth := s.newAuthorizer()
	s.seedRole(auth, testTenancyPath, "user1", authz.RoleOwner)

	mw := authz.NewMiddleware(auth)
	ctx := s.ctxWithClaims("user1")

	s.NoError(mw.CanSendNotification(ctx))
	s.NoError(mw.CanReleaseNotification(ctx))
	s.NoError(mw.CanSearchNotifications(ctx))
	s.NoError(mw.CanViewNotificationStatus(ctx))
	s.NoError(mw.CanUpdateNotificationStatus(ctx))
	s.NoError(mw.CanManageTemplate(ctx))
	s.NoError(mw.CanViewTemplate(ctx))
}

func (s *MiddlewareTestSuite) TestOperatorPermissions() {
	auth := s.newAuthorizer()
	s.seedRole(auth, testTenancyPath, "user2", authz.RoleOperator)

	mw := authz.NewMiddleware(auth)
	ctx := s.ctxWithClaims("user2")

	// Operator can send, release, search, view status, view template
	s.NoError(mw.CanSendNotification(ctx))
	s.NoError(mw.CanReleaseNotification(ctx))
	s.NoError(mw.CanSearchNotifications(ctx))
	s.NoError(mw.CanViewNotificationStatus(ctx))
	s.NoError(mw.CanViewTemplate(ctx))

	// Operator cannot update status or manage templates
	s.Error(mw.CanUpdateNotificationStatus(ctx))
	s.Error(mw.CanManageTemplate(ctx))
}

func (s *MiddlewareTestSuite) TestViewerPermissions() {
	auth := s.newAuthorizer()
	s.seedRole(auth, testTenancyPath, "user3", authz.RoleViewer)

	mw := authz.NewMiddleware(auth)
	ctx := s.ctxWithClaims("user3")

	// Viewer can search, view status, view template
	s.NoError(mw.CanSearchNotifications(ctx))
	s.NoError(mw.CanViewNotificationStatus(ctx))
	s.NoError(mw.CanViewTemplate(ctx))

	// Viewer cannot send, release, update status, manage template
	s.Error(mw.CanSendNotification(ctx))
	s.Error(mw.CanReleaseNotification(ctx))
	s.Error(mw.CanUpdateNotificationStatus(ctx))
	s.Error(mw.CanManageTemplate(ctx))
}

func (s *MiddlewareTestSuite) TestMemberPermissions() {
	auth := s.newAuthorizer()
	s.seedRole(auth, testTenancyPath, "user4", authz.RoleMember)

	mw := authz.NewMiddleware(auth)
	ctx := s.ctxWithClaims("user4")

	// Member can search, view status
	s.NoError(mw.CanSearchNotifications(ctx))
	s.NoError(mw.CanViewNotificationStatus(ctx))

	// Member cannot send, release, update status, manage/view template
	s.Error(mw.CanSendNotification(ctx))
	s.Error(mw.CanReleaseNotification(ctx))
	s.Error(mw.CanUpdateNotificationStatus(ctx))
	s.Error(mw.CanManageTemplate(ctx))
	s.Error(mw.CanViewTemplate(ctx))
}

func (s *MiddlewareTestSuite) TestNoClaims() {
	auth := s.newAuthorizer()
	mw := authz.NewMiddleware(auth)

	err := mw.CanSearchNotifications(context.Background())
	s.ErrorIs(err, authorizer.ErrInvalidSubject)
}

func (s *MiddlewareTestSuite) TestNoTenant() {
	auth := s.newAuthorizer()
	mw := authz.NewMiddleware(auth)

	claims := &security.AuthenticationClaims{}
	claims.Subject = "user1"
	ctx := claims.ClaimsToContext(context.Background())
	err := mw.CanSearchNotifications(ctx)
	s.ErrorIs(err, authorizer.ErrInvalidObject)
}

// ---------------------------------------------------------------------------
// TenancyAccessChecker tests — data access layer
// ---------------------------------------------------------------------------

func (s *MiddlewareTestSuite) TestAccessChecker_MemberAllowed() {
	auth := s.newAuthorizer()
	checker := authorizer.NewTenancyAccessChecker(auth, authz.NamespaceTenancyAccess)

	err := auth.WriteTuple(s.T().Context(), authz.BuildAccessTuple(testTenancyPath, "member-user"))
	s.Require().NoError(err)

	ctx := s.ctxWithClaims("member-user")
	s.NoError(checker.CheckAccess(ctx))
}

func (s *MiddlewareTestSuite) TestAccessChecker_ServiceBotAllowed() {
	auth := s.newAuthorizer()
	checker := authorizer.NewTenancyAccessChecker(auth, authz.NamespaceTenancyAccess)

	err := auth.WriteTuple(s.T().Context(), authz.BuildServiceAccessTuple(testTenancyPath, "bot-user"))
	s.Require().NoError(err)

	ctx := s.ctxWithSystemInternalClaims("bot-user")
	s.NoError(checker.CheckAccess(ctx))
}

func (s *MiddlewareTestSuite) TestAccessChecker_NoTupleDenied() {
	auth := s.newAuthorizer()
	checker := authorizer.NewTenancyAccessChecker(auth, authz.NamespaceTenancyAccess)

	ctx := s.ctxWithClaims("unknown-user")
	s.Error(checker.CheckAccess(ctx))
}

// ---------------------------------------------------------------------------
// Service bot via subject sets — full two-layer check
// ---------------------------------------------------------------------------

func (s *MiddlewareTestSuite) seedServiceBridgeTuples(auth security.Authorizer, tenancyPath string) {
	tuples := authz.BuildServiceInheritanceTuples(tenancyPath)
	err := auth.WriteTuples(s.T().Context(), tuples)
	s.Require().NoError(err)
}

func (s *MiddlewareTestSuite) TestServiceBotViaSubjectSets() {
	auth := s.newAuthorizer()
	mw := authz.NewMiddleware(auth)
	accessChecker := authorizer.NewTenancyAccessChecker(auth, authz.NamespaceTenancyAccess)

	// Step 1: Write bridge tuples (normally done at partition sync).
	s.seedServiceBridgeTuples(auth, testTenancyPath)

	// Step 2: Grant the bot service access in tenancy_access.
	err := auth.WriteTuple(s.T().Context(), authz.BuildServiceAccessTuple(testTenancyPath, "service-bot"))
	s.Require().NoError(err)

	botCtx := s.ctxWithSystemInternalClaims("service-bot")

	// Layer 1: Access check passes
	s.NoError(accessChecker.CheckAccess(botCtx))

	// Layer 2: Functional permissions resolved through subject sets
	s.NoError(mw.CanSendNotification(botCtx))
	s.NoError(mw.CanSearchNotifications(botCtx))
	s.NoError(mw.CanManageTemplate(botCtx))
}

func (s *MiddlewareTestSuite) TestDirectPermissionGrant() {
	auth := s.newAuthorizer()
	mw := authz.NewMiddleware(auth)

	// User has a direct permission grant for send_notification only
	err := auth.WriteTuple(s.T().Context(), security.RelationTuple{
		Object:   security.ObjectRef{Namespace: authz.NamespaceNotifications, ID: testTenancyPath},
		Relation: authz.PermissionSendNotification,
		Subject:  security.SubjectRef{Namespace: authz.NamespaceProfile, ID: "user5"},
	})
	s.Require().NoError(err)

	ctx := s.ctxWithClaims("user5")

	// Direct grant works
	s.NoError(mw.CanSendNotification(ctx))

	// Other permissions require their own tuples
	s.Error(mw.CanSearchNotifications(ctx))
	s.Error(mw.CanManageTemplate(ctx))
}
