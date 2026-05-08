import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import '../_helpers/fake_notification_client.dart';
import '../_helpers/test_harness.dart';

GoRouter _routerFor(Widget screen) {
  return GoRouter(routes: [
    GoRoute(path: '/', builder: (_, __) => screen),
    GoRoute(
      path: '/notifications',
      builder: (_, __) => const Scaffold(body: SizedBox.shrink()),
    ),
  ]);
}

void main() {
  testWidgets('uses MetadataRow and renders an AuditTrailEntry timeline',
      (tester) async {
    final n = makeNotification(id: 'n1')
      ..status = (notif.StatusResponse()..state = notif.STATE.ACTIVE);
    final tenancy = TenancyContext()
      ..initializeFromLogin(LoginLevel.root, partitionId: 'p1');

    await tester.pumpWidget(ProviderScope(
      overrides: [
        tenancyContextProvider.overrideWithValue(tenancy),
      ],
      child: MaterialApp.router(
        routerConfig: _routerFor(NotificationDetailScreen(
          notificationId: 'n1',
          initialNotification: n,
        )),
      ),
    ));
    await tester.pumpAndSettle();

    // MetadataRow renders the ID label.
    expect(find.byType(MetadataRow), findsWidgets);
    expect(find.text('ID'), findsOneWidget);
    expect(find.text('n1'), findsOneWidget);
    // Lifecycle uses AuditTrailEntry.
    expect(find.byType(AuditTrailEntry), findsWidgets);
  });

  testWidgets('retry calls Release with the notification id', (tester) async {
    final fake = FakeNotificationClient();
    final n = makeNotification(id: 'n1')
      ..status = (notif.StatusResponse()..state = notif.STATE.INACTIVE);
    final tenancy = TenancyContext()
      ..initializeFromLogin(LoginLevel.root, partitionId: 'p1');

    await tester.pumpWidget(ProviderScope(
      overrides: [
        tenancyContextProvider.overrideWithValue(tenancy),
        notificationServiceClientProvider.overrideWithValue(fake.client),
      ],
      child: MaterialApp.router(
        routerConfig: _routerFor(NotificationDetailScreen(
          notificationId: 'n1',
          initialNotification: n,
        )),
      ),
    ));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('detail-retry-button')));
    await tester.pumpAndSettle();

    expect(fake.releaseRequests, hasLength(1));
    expect(fake.releaseRequests.single.id, contains('n1'));
  });
}
