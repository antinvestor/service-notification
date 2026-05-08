import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:go_router/go_router.dart';

import '../_helpers/fake_notification_client.dart';
import '../_helpers/test_harness.dart';

void main() {
  testWidgets('renders tiles and acks receipt on first paint', (tester) async {
    final fake = FakeNotificationClient()
      ..nextSearchResults = [
        makeNotification(id: 'a', template: 'welcome'),
        makeNotification(id: 'b', template: 'reset'),
      ];
    final tenancy = TenancyContext()
      ..initializeFromLogin(LoginLevel.root, partitionId: 'p1');

    final router = GoRouter(routes: [
      GoRoute(
        path: '/',
        builder: (_, __) =>
            const Scaffold(body: EndUserInboxScreen(profileId: 'profile-1')),
      ),
      GoRoute(
        path: '/notifications/detail/:id',
        builder: (_, __) => const Scaffold(body: SizedBox.shrink()),
      ),
    ]);

    await tester.pumpWidget(ProviderScope(
      overrides: [
        tenancyContextProvider.overrideWithValue(tenancy),
        notificationServiceClientProvider.overrideWithValue(fake.client),
      ],
      child: MaterialApp.router(routerConfig: router),
    ));
    await tester.pumpAndSettle();

    expect(find.text('welcome'), findsOneWidget);
    expect(find.text('reset'), findsOneWidget);

    expect(fake.receiveRequests, hasLength(1));
    expect(fake.receiveRequests.single.data.map((n) => n.id),
        unorderedEquals(['a', 'b']));
  });
}
