package authz

import (
	"context"

	"github.com/pitabwire/frame/security"
	"github.com/pitabwire/frame/security/authorizer"
)

type Middleware interface {
	CanNotificationSend(ctx context.Context) error
	CanNotificationRelease(ctx context.Context) error
	CanNotificationSearch(ctx context.Context) error
	CanNotificationStatusView(ctx context.Context) error
	CanNotificationStatusUpdate(ctx context.Context) error
	CanTemplateManage(ctx context.Context) error
	CanTemplateView(ctx context.Context) error
}

type middleware struct {
	checker *authorizer.FunctionChecker
}

func NewMiddleware(service security.Authorizer) Middleware {
	return &middleware{
		checker: authorizer.NewFunctionChecker(service, NamespaceNotifications),
	}
}

func (m *middleware) CanNotificationSend(ctx context.Context) error {
	return m.checker.Check(ctx, PermissionNotificationSend)
}

func (m *middleware) CanNotificationRelease(ctx context.Context) error {
	return m.checker.Check(ctx, PermissionNotificationRelease)
}

func (m *middleware) CanNotificationSearch(ctx context.Context) error {
	return m.checker.Check(ctx, PermissionNotificationSearch)
}

func (m *middleware) CanNotificationStatusView(ctx context.Context) error {
	return m.checker.Check(ctx, PermissionNotificationStatusView)
}

func (m *middleware) CanNotificationStatusUpdate(ctx context.Context) error {
	return m.checker.Check(ctx, PermissionNotificationStatusUpdate)
}

func (m *middleware) CanTemplateManage(ctx context.Context) error {
	return m.checker.Check(ctx, PermissionTemplateManage)
}

func (m *middleware) CanTemplateView(ctx context.Context) error {
	return m.checker.Check(ctx, PermissionTemplateView)
}
