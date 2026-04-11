import 'package:antinvestor_api_notification/antinvestor_api_notification.dart';
import 'package:antinvestor_ui_core/navigation/nav_items.dart';
import 'package:antinvestor_ui_core/routing/route_module.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../screens/notification_detail_screen.dart';
import '../screens/notification_inbox_screen.dart';
import '../screens/notification_send_screen.dart';
import '../screens/template_edit_screen.dart';
import '../screens/template_list_screen.dart';

/// Route module for notification management.
///
/// Registers the following routes:
/// - `/notifications` - notification inbox
/// - `/notifications/detail/:id` - notification detail view
/// - `/notifications/send` - compose and send a notification
/// - `/notifications/templates` - template list
/// - `/notifications/templates/edit` - create new template
/// - `/notifications/templates/edit/:id` - edit existing template
class NotificationRouteModule extends RouteModule {
  @override
  String get moduleId => 'notification';

  @override
  List<RouteBase> buildRoutes() {
    return [
      GoRoute(
        path: '/notifications',
        builder: (context, state) => const NotificationInboxScreen(),
        routes: [
          GoRoute(
            path: 'detail/:id',
            builder: (context, state) {
              final id = state.pathParameters['id'] ?? '';
              final extra = state.extra;
              final notification =
                  extra is Notification ? extra : null;
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
                      extra is Template ? extra : null;
                  return TemplateEditScreen(
                    templateId: id,
                    initialTemplate: template,
                  );
                },
              ),
            ],
          ),
        ],
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
        children: [
          NavItem(
            id: 'notification-inbox',
            label: 'Inbox',
            icon: Icons.inbox,
            route: '/notifications',
          ),
          NavItem(
            id: 'notification-send',
            label: 'Compose',
            icon: Icons.send,
            route: '/notifications/send',
          ),
          NavItem(
            id: 'notification-templates',
            label: 'Templates',
            icon: Icons.description,
            route: '/notifications/templates',
          ),
        ],
      ),
    ];
  }

  @override
  Map<String, Set<String>> get routePermissions => {
        '/notifications': {'notification:read', 'admin'},
        '/notifications/detail': {'notification:read', 'admin'},
        '/notifications/send': {'notification:write', 'admin'},
        '/notifications/templates': {'notification:read', 'admin'},
        '/notifications/templates/edit': {'notification:write', 'admin'},
      };
}
