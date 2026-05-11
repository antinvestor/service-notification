import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import '../_helpers/fake_notification_client.dart';

void main() {
  testWidgets('saves a language by writing a placeholder template',
      (tester) async {
    final fake = FakeNotificationClient();
    final tenancy = TenancyContext()
      ..initializeFromLogin(LoginLevel.root, partitionId: 'p1');

    final router = GoRouter(routes: [
      GoRoute(path: '/', builder: (_, _) => const LanguageEditScreen()),
      GoRoute(
        path: '/notifications/languages',
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

    await tester.enterText(find.byKey(const Key('lang-code-field')), 'pt');
    await tester.enterText(
        find.byKey(const Key('lang-name-field')), 'Portuguese');

    await tester.tap(find.byKey(const Key('lang-save-button')));
    await tester.pumpAndSettle();

    expect(fake.templateSaveRequests, hasLength(1));
    expect(fake.templateSaveRequests.single.name, '_lang_pt');
    final variants = fake.templateSaveRequests.single.data
        .fields['variants']?.listValue.values;
    expect(variants?.single.structValue.fields['language']?.stringValue,
        'pt');
    expect(variants?.single.structValue.fields['languageName']?.stringValue,
        'Portuguese');
  });
}
