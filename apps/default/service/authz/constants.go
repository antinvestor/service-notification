package authz

const (
	NamespaceNotifications = "service_notifications"
	NamespaceTenancyAccess = "tenancy_access"
	NamespaceProfile       = "profile_user"
)

// Permission constants for notification operations.
// These names match the OPL permits functions and are used with Keto's Check API.
// Named as noun_verb (e.g. notification_send) so related permissions group together.
const (
	PermissionNotificationSend         = "notification_send"
	PermissionNotificationRelease      = "notification_release"
	PermissionNotificationSearch       = "notification_search"
	PermissionNotificationStatusView   = "notification_status_view"
	PermissionNotificationStatusUpdate = "notification_status_update"
	PermissionTemplateManage           = "template_manage"
	PermissionTemplateView             = "template_view"
)

// Granted relation constants for direct permission grants in the OPL.
// These are prefixed with "granted_" to avoid name conflicts with the OPL
// permits functions -- Keto skips permit evaluation when a relation with
// the same name exists.
const (
	GrantedNotificationSend         = "granted_notification_send"
	GrantedNotificationRelease      = "granted_notification_release"
	GrantedNotificationSearch       = "granted_notification_search"
	GrantedNotificationStatusView   = "granted_notification_status_view"
	GrantedNotificationStatusUpdate = "granted_notification_status_update"
	GrantedTemplateManage           = "granted_template_manage"
	GrantedTemplateView             = "granted_template_view"
)

// Role constants.
const (
	RoleOwner    = "owner"
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleViewer   = "viewer"
	RoleMember   = "member"
	RoleService  = "service"
)

// RolePermissions documents the permission model defined in the OPL namespace config.
// Keto's Check API evaluates OPL permits, so only role tuples need to be written;
// permission resolution happens automatically through the OPL model.
var RolePermissions = map[string][]string{ //nolint:gochecknoglobals // permission model registry
	RoleOwner: {
		PermissionNotificationSend, PermissionNotificationRelease,
		PermissionNotificationSearch, PermissionNotificationStatusView,
		PermissionNotificationStatusUpdate, PermissionTemplateManage, PermissionTemplateView,
	},
	RoleAdmin: {
		PermissionNotificationSend, PermissionNotificationRelease,
		PermissionNotificationSearch, PermissionNotificationStatusView,
		PermissionNotificationStatusUpdate, PermissionTemplateManage, PermissionTemplateView,
	},
	RoleOperator: {
		PermissionNotificationSend, PermissionNotificationRelease,
		PermissionNotificationSearch, PermissionNotificationStatusView,
		PermissionTemplateView,
	},
	RoleViewer: {
		PermissionNotificationSearch, PermissionNotificationStatusView,
		PermissionTemplateView,
	},
	RoleMember: {
		PermissionNotificationSearch, PermissionNotificationStatusView,
	},
	RoleService: {
		PermissionNotificationSend, PermissionNotificationRelease,
		PermissionNotificationSearch, PermissionNotificationStatusView,
		PermissionNotificationStatusUpdate, PermissionTemplateManage, PermissionTemplateView,
	},
}
