import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:go_router/go_router.dart';

import '../_helpers/fake_notification_client.dart';

void main() {
  testWidgets('sends notification with payload as Struct, not concatenated string',
      (tester) async {
    // Use a taller viewport so the full form fits without needing to scroll.
    tester.view.physicalSize = const Size(800, 2000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);

    final fake = FakeNotificationClient();
    final tenancy = TenancyContext()
      ..initializeFromLogin(LoginLevel.root, partitionId: 'p1');

    final router = GoRouter(routes: [
      GoRoute(path: '/', builder: (_, _) => const NotificationSendScreen()),
      GoRoute(
        path: '/notifications',
        builder: (_, _) => const Scaffold(body: SizedBox.shrink()),
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

    await tester.enterText(
        find.byKey(const Key('send-recipient-field')), '+254700000000');

    // Add a data entry.
    await tester.tap(find.byKey(const Key('send-data-add-button')));
    await tester.pumpAndSettle();
    await tester.enterText(
        find.byKey(const Key('send-data-key-0')), 'name');
    await tester.enterText(
        find.byKey(const Key('send-data-value-0')), 'Alice');

    // Submit.
    await tester.ensureVisible(find.byKey(const Key('send-submit-button')));
    await tester.tap(find.byKey(const Key('send-submit-button')));
    await tester.pumpAndSettle();

    expect(fake.sendRequests, hasLength(1));
    final n = fake.sendRequests.single.data.single;
    expect(n.recipient.detail, '+254700000000');
    expect(n.payload.fields['name']?.stringValue, 'Alice');
    // The string `data` should NOT contain the "name=Alice" concat.
    expect(n.data, isNot(contains('name=Alice')));
  });
}
