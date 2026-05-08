import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

void main() {
  test('exposes dashboard, inbox, languages, end-user inbox routes', () {
    final module = NotificationRouteModule();
    final paths = _flattenPaths(module.buildRoutes());
    expect(paths, containsAll(<String>[
      '/notifications',
      '/notifications/inbox',
      '/notifications/detail/:id',
      '/notifications/send',
      '/notifications/templates',
      '/notifications/templates/edit',
      '/notifications/templates/edit/:id',
      '/notifications/languages',
      '/notifications/languages/edit',
      '/notifications/languages/edit/:id',
      '/me/notifications',
    ]));
  });

  test('nav items include dashboard and languages but not /me', () {
    final module = NotificationRouteModule();
    final ids = _flattenNavIds(module.buildNavItems());
    expect(ids,
        containsAll(<String>[
          'notifications',
          'notification-inbox',
          'notification-send',
          'notification-templates',
          'notification-languages',
          'notification-dashboard',
        ]));
    expect(ids, isNot(contains('end-user-inbox')));
  });
}

List<String> _flattenPaths(List<RouteBase> routes, [String prefix = '']) {
  final out = <String>[];
  for (final r in routes) {
    if (r is GoRoute) {
      final full = prefix.isEmpty
          ? r.path
          : (r.path.startsWith('/') ? r.path : '$prefix/${r.path}');
      out.add(full);
      out.addAll(_flattenPaths(r.routes, full));
    }
  }
  return out;
}

List<String> _flattenNavIds(List<NavItem> items) {
  final out = <String>[];
  for (final n in items) {
    out.add(n.id);
    out.addAll(_flattenNavIds(n.children));
  }
  return out;
}
