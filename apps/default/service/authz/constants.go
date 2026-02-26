package authz

const (
	NamespaceTenant  = "notification_tenant"
	NamespaceProfile = "profile"
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
)
