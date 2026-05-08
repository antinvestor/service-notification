import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:go_router/go_router.dart';

import '../_helpers/fake_notification_client.dart';

void main() {
  testWidgets('lists languages derived from template variants',
      (tester) async {
    final fake = FakeNotificationClient()
      ..nextTemplateResults = [
        notif.Template()
          ..name = 'welcome'
          ..data.addAll([
            notif.TemplateData()
              ..type = 'SMS'
              ..detail = '...'
              ..language =
                  (notif.Language()
                    ..code = 'en'
                    ..name = 'English'),
            notif.TemplateData()
              ..type = 'SMS'
              ..detail = '...'
              ..language =
                  (notif.Language()
                    ..code = 'sw'
                    ..name = 'Swahili'),
          ]),
      ];
    final tenancy = TenancyContext()
      ..initializeFromLogin(LoginLevel.root, partitionId: 'p1');

    final router = GoRouter(routes: [
      GoRoute(
        path: '/',
        builder: (_, __) =>
            const Scaffold(body: LanguageListScreen()),
      ),
      GoRoute(
        path: '/notifications/languages/edit',
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

    expect(find.text('en'), findsOneWidget);
    expect(find.text('sw'), findsOneWidget);
    expect(find.text('English'), findsOneWidget);
    expect(find.text('Swahili'), findsOneWidget);
  });
}
