package authz

const (
	NamespaceNotifications = "service_notifications"
	NamespaceTenancyAccess = "tenancy_access"
	NamespaceProfile       = "profile/user"
)

const (
	PermissionSendNotification         = "send_notification"
	PermissionReleaseNotification      = "release_notification"
	PermissionSearchNotifications      = "search_notifications"
	PermissionViewNotificationStatus   = "view_notification_status"
	PermissionUpdateNotificationStatus = "update_notification_status"
	PermissionManageTemplate           = "manage_template"
	PermissionViewTemplate             = "view_template"
)

const (
	RoleOwner    = "owner"
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleViewer   = "viewer"
	RoleMember   = "member"
	RoleService  = "service"
)

// RolePermissions maps each role to the permissions it grants.
// These are materialised as direct tuples (Keto v1alpha2 gRPC Check API
// does not evaluate OPL permits).
var RolePermissions = map[string][]string{
	RoleOwner: {
		PermissionSendNotification, PermissionReleaseNotification,
		PermissionSearchNotifications, PermissionViewNotificationStatus,
		PermissionUpdateNotificationStatus, PermissionManageTemplate, PermissionViewTemplate,
	},
	RoleAdmin: {
		PermissionSendNotification, PermissionReleaseNotification,
		PermissionSearchNotifications, PermissionViewNotificationStatus,
		PermissionUpdateNotificationStatus, PermissionManageTemplate, PermissionViewTemplate,
	},
	RoleOperator: {
		PermissionSendNotification, PermissionReleaseNotification,
		PermissionSearchNotifications, PermissionViewNotificationStatus,
		PermissionViewTemplate,
	},
	RoleViewer: {
		PermissionSearchNotifications, PermissionViewNotificationStatus,
		PermissionViewTemplate,
	},
	RoleMember: {
		PermissionSearchNotifications, PermissionViewNotificationStatus,
	},
	RoleService: {
		PermissionSendNotification, PermissionReleaseNotification,
		PermissionSearchNotifications, PermissionViewNotificationStatus,
		PermissionUpdateNotificationStatus, PermissionManageTemplate, PermissionViewTemplate,
	},
}
