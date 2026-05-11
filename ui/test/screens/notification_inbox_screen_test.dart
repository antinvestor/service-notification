import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import '../_helpers/fake_notification_client.dart';

void main() {
  testWidgets('language filter chip pushes language: into search',
      (tester) async {
    // Seed a template with an English variant so languageSearchProvider
    // resolves the 'en' chip.
    final fake = FakeNotificationClient()
      ..nextSearchResults = []
      ..nextTemplateResults = [
        notif.Template()
          ..name = 'welcome'
          ..data.add(notif.TemplateData()
            ..type = 'SMS'
            ..detail = '...'
            ..language = (notif.Language()
              ..code = 'en'
              ..name = 'English')),
      ];
    final tenancy = TenancyContext()
      ..initializeFromLogin(LoginLevel.root, partitionId: 'p1');

    final router = GoRouter(routes: [
      GoRoute(
        path: '/',
        builder: (_, _) =>
            const Scaffold(body: NotificationInboxScreen()),
      ),
      GoRoute(
        path: '/notifications/send',
        builder: (_, _) => const Scaffold(body: SizedBox.shrink()),
      ),
      GoRoute(
        path: '/notifications/detail/:id',
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
    // AdminEntityListPage uses a DropdownButton in its pagination footer that
    // keeps the animation system busy; pump a fixed duration instead.
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 500));

    await tester.tap(find.byKey(const Key('inbox-lang-en')));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 500));

    expect(
      fake.searchRequests.last.properties,
      contains('language:en'),
    );
  });
}
