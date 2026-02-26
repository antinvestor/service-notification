package authz

import (
	"context"

	"github.com/pitabwire/frame/security"
	"github.com/pitabwire/frame/security/authorizer"
)

type Middleware interface {
	CanSendNotification(ctx context.Context) error
	CanReleaseNotification(ctx context.Context) error
	CanSearchNotifications(ctx context.Context) error
	CanViewNotificationStatus(ctx context.Context) error
	CanUpdateNotificationStatus(ctx context.Context) error
	CanManageTemplate(ctx context.Context) error
	CanViewTemplate(ctx context.Context) error
}

type middleware struct {
	authorizer security.Authorizer
}

func NewMiddleware(authorizer security.Authorizer) Middleware {
	return &middleware{authorizer: authorizer}
}

func (m *middleware) CanSendNotification(ctx context.Context) error {
	return m.check(ctx, PermissionSendNotification)
}

func (m *middleware) CanReleaseNotification(ctx context.Context) error {
	return m.check(ctx, PermissionReleaseNotification)
}

func (m *middleware) CanSearchNotifications(ctx context.Context) error {
	return m.check(ctx, PermissionSearchNotifications)
}

func (m *middleware) CanViewNotificationStatus(ctx context.Context) error {
	return m.check(ctx, PermissionViewNotificationStatus)
}

func (m *middleware) CanUpdateNotificationStatus(ctx context.Context) error {
	return m.check(ctx, PermissionUpdateNotificationStatus)
}

func (m *middleware) CanManageTemplate(ctx context.Context) error {
	return m.check(ctx, PermissionManageTemplate)
}

func (m *middleware) CanViewTemplate(ctx context.Context) error {
	return m.check(ctx, PermissionViewTemplate)
}

func (m *middleware) check(ctx context.Context, permission string) error {
	claims := security.ClaimsFromContext(ctx)
	if claims == nil {
		return authorizer.ErrInvalidSubject
	}

	subjectID, err := claims.GetSubject()
	if err != nil || subjectID == "" {
		return authorizer.ErrInvalidSubject
	}

	tenantID := claims.GetTenantID()
	if tenantID == "" {
		return authorizer.ErrInvalidObject
	}

	req := security.CheckRequest{
		Object:     security.ObjectRef{Namespace: NamespaceTenant, ID: tenantID},
		Permission: permission,
		Subject:    security.SubjectRef{Namespace: NamespaceProfile, ID: subjectID},
	}

	result, err := m.authorizer.Check(ctx, req)
	if err != nil {
		return err
	}
	if !result.Allowed {
		return authorizer.NewPermissionDeniedError(req.Object, permission, req.Subject, result.Reason)
	}

	return nil
}
