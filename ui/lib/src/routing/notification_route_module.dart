import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_core/navigation/nav_items.dart';
import 'package:antinvestor_ui_core/permissions/permission_manifest.dart';
import 'package:antinvestor_ui_core/routing/route_module.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../screens/end_user_inbox_screen.dart';
import '../screens/language_edit_screen.dart';
import '../screens/language_list_screen.dart';
import '../screens/notification_dashboard_screen.dart';
import '../screens/notification_detail_screen.dart';
import '../screens/notification_inbox_screen.dart';
import '../screens/notification_send_screen.dart';
import '../screens/template_edit_screen.dart';
import '../screens/template_list_screen.dart';

/// Route module for notification management.
///
/// Registers the following routes:
/// - `/notifications` - notification dashboard (landing)
/// - `/notifications/inbox` - notification inbox
/// - `/notifications/detail/:id` - notification detail view
/// - `/notifications/send` - compose and send a notification
/// - `/notifications/templates` - template list
/// - `/notifications/templates/edit` - create new template
/// - `/notifications/templates/edit/:id` - edit existing template
/// - `/notifications/languages` - language list
/// - `/notifications/languages/edit` - create new language
/// - `/notifications/languages/edit/:id` - edit existing language
/// - `/me/notifications` - end-user notification inbox
class NotificationRouteModule extends RouteModule {
  @override
  String get moduleId => 'notification';

  @override
  List<RouteBase> buildRoutes() {
    return [
      GoRoute(
        path: '/notifications',
        builder: (context, state) => const NotificationDashboardScreen(),
        routes: [
          GoRoute(
            path: 'inbox',
            builder: (context, state) => const NotificationInboxScreen(),
          ),
          GoRoute(
            path: 'detail/:id',
            builder: (context, state) {
              final id = state.pathParameters['id'] ?? '';
              final extra = state.extra;
              final notification =
                  extra is notif.Notification ? extra : null;
              return NotificationDetailScreen(
                notificationId: id,
                initialNotification: notification,
              );
            },
          ),
          GoRoute(
            path: 'send',
            builder: (context, state) => const NotificationSendScreen(),
          ),
          GoRoute(
            path: 'templates',
            builder: (context, state) => const TemplateListScreen(),
            routes: [
              GoRoute(
                path: 'edit',
                builder: (context, state) => const TemplateEditScreen(),
              ),
              GoRoute(
                path: 'edit/:id',
                builder: (context, state) {
                  final id = state.pathParameters['id'] ?? '';
                  final extra = state.extra;
                  final template =
                      extra is notif.Template ? extra : null;
                  return TemplateEditScreen(
                    templateId: id,
                    initialTemplate: template,
                  );
                },
              ),
            ],
          ),
          GoRoute(
            path: 'languages',
            builder: (context, state) => const LanguageListScreen(),
            routes: [
              GoRoute(
                path: 'edit',
                builder: (context, state) => const LanguageEditScreen(),
              ),
              GoRoute(
                path: 'edit/:id',
                builder: (context, state) {
                  final id = state.pathParameters['id'] ?? '';
                  final extra = state.extra;
                  final language =
                      extra is notif.Language ? extra : null;
                  return LanguageEditScreen(
                    languageCode: id,
                    initialLanguage: language,
                  );
                },
              ),
            ],
          ),
        ],
      ),
      GoRoute(
        path: '/me/notifications',
        builder: (context, state) {
          final extra = state.extra;
          final profileId = extra is String ? extra : 'me';
          return EndUserInboxScreen(profileId: profileId);
        },
      ),
    ];
  }

  @override
  List<NavItem> buildNavItems() {
    return [
      const NavItem(
        id: 'notifications',
        label: 'Notifications',
        icon: Icons.notifications_outlined,
        activeIcon: Icons.notifications,
        route: '/notifications',
        requiredPermissions: {'notification_search'},
        children: [
          NavItem(
            id: 'notification-dashboard',
            label: 'Dashboard',
            icon: Icons.dashboard_outlined,
            route: '/notifications',
            requiredPermissions: {'notification_search'},
          ),
          NavItem(
            id: 'notification-inbox',
            label: 'Inbox',
            icon: Icons.inbox,
            route: '/notifications/inbox',
            requiredPermissions: {'notification_search'},
          ),
          NavItem(
            id: 'notification-send',
            label: 'Compose',
            icon: Icons.send,
            route: '/notifications/send',
            requiredPermissions: {'notification_send'},
          ),
          NavItem(
            id: 'notification-templates',
            label: 'Templates',
            icon: Icons.description,
            route: '/notifications/templates',
            requiredPermissions: {'template_manage'},
          ),
          NavItem(
            id: 'notification-languages',
            label: 'Languages',
            icon: Icons.language,
            route: '/notifications/languages',
            requiredPermissions: {'template_manage'},
          ),
        ],
      ),
    ];
  }

  @override
  Map<String, Set<String>> get routePermissions => {
        '/notifications': {'notification_search'},
        '/notifications/inbox': {'notification_search'},
        '/notifications/detail': {'notification_status_view'},
        '/notifications/send': {'notification_send'},
        '/notifications/templates': {'template_view'},
        '/notifications/templates/edit': {'template_manage'},
        '/notifications/languages': {'template_view'},
        '/notifications/languages/edit': {'template_manage'},
        '/me/notifications': {'notification_search'},
      };

  @override
  PermissionManifest get permissionManifest => const PermissionManifest(
        namespace: 'service_notification',
        permissions: [
          PermissionEntry(
            key: 'notification_send',
            label: 'Send Notifications',
            scope: PermissionScope.action,
          ),
          PermissionEntry(
            key: 'notification_search',
            label: 'Search Notifications',
            scope: PermissionScope.service,
          ),
          PermissionEntry(
            key: 'notification_status_view',
            label: 'View Notification Status',
            scope: PermissionScope.feature,
          ),
          PermissionEntry(
            key: 'template_manage',
            label: 'Manage Templates and Languages',
            scope: PermissionScope.feature,
          ),
          PermissionEntry(
            key: 'template_view',
            label: 'View Templates and Languages',
            scope: PermissionScope.feature,
          ),
        ],
      );
}
