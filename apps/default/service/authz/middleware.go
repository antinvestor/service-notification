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
	checker *authorizer.FunctionChecker
}

func NewMiddleware(service security.Authorizer) Middleware {
	return &middleware{
		checker: authorizer.NewFunctionChecker(service, NamespaceNotifications),
	}
}

func (m *middleware) CanSendNotification(ctx context.Context) error {
	return m.checker.Check(ctx, PermissionSendNotification)
}

func (m *middleware) CanReleaseNotification(ctx context.Context) error {
	return m.checker.Check(ctx, PermissionReleaseNotification)
}

func (m *middleware) CanSearchNotifications(ctx context.Context) error {
	return m.checker.Check(ctx, PermissionSearchNotifications)
}

func (m *middleware) CanViewNotificationStatus(ctx context.Context) error {
	return m.checker.Check(ctx, PermissionViewNotificationStatus)
}

func (m *middleware) CanUpdateNotificationStatus(ctx context.Context) error {
	return m.checker.Check(ctx, PermissionUpdateNotificationStatus)
}

func (m *middleware) CanManageTemplate(ctx context.Context) error {
	return m.checker.Check(ctx, PermissionManageTemplate)
}

func (m *middleware) CanViewTemplate(ctx context.Context) error {
	return m.checker.Check(ctx, PermissionViewTemplate)
}
