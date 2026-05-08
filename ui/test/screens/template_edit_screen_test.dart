import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import '../_helpers/fake_notification_client.dart';

/// Builds a [GoRouter]-backed [MaterialApp] wrapping [TemplateEditScreen],
/// with provider overrides for the given [FakeNotificationClient].
///
/// Using a router rather than TestHarness avoids:
///  1. Double-Scaffold nesting (TestHarness wraps in Scaffold).
///  2. Navigation crash when [_save] calls `context.go('/notifications/templates')`.
Widget _buildApp(FakeNotificationClient fake) {
  final tenancy = TenancyContext()
    ..initializeFromLogin(
      LoginLevel.root,
      partitionId: 'part-test',
      partitionName: 'Test Partition',
      orgId: null,
      orgName: 'Test Org',
      branchId: null,
      branchName: 'Test Branch',
    );

  final router = GoRouter(
    initialLocation: '/edit',
    routes: [
      GoRoute(
        path: '/edit',
        builder: (_, __) => const TemplateEditScreen(),
      ),
      GoRoute(
        path: '/notifications/templates',
        builder: (_, __) => const Scaffold(body: Text('Templates')),
      ),
    ],
  );

  return ProviderScope(
    overrides: [
      tenancyContextProvider.overrideWithValue(tenancy),
      notificationServiceClientProvider.overrideWithValue(fake.client),
    ],
    child: MaterialApp.router(routerConfig: router),
  );
}

void main() {
  testWidgets('saves variants entered via the matrix', (tester) async {
    final fake = FakeNotificationClient();

    await tester.pumpWidget(_buildApp(fake));
    await tester.pumpAndSettle();

    // Set the template name.
    await tester.enterText(
      find.byKey(const Key('template-name-field')),
      'welcome',
    );

    // Open the (SMS, en) cell, type content, save the cell.
    await tester.tap(find.byKey(const Key('cell-SMS-en')));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(const Key('cell-editor-content')),
      'Hi {{name}}',
    );
    // Scroll the editor save button into view before tapping (compact layout
    // stacks the editor below the matrix, which may push it off the 800×600
    // test viewport).
    await tester.ensureVisible(find.byKey(const Key('cell-editor-save')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('cell-editor-save')));
    await tester.pumpAndSettle();

    // Submit the form (top-right Save button in AppBar is always visible).
    await tester.tap(find.byKey(const Key('template-save-button')));
    await tester.pumpAndSettle();

    expect(fake.templateSaveRequests, hasLength(1));
    final req = fake.templateSaveRequests.single;
    expect(req.name, 'welcome');
    final list = req.data.fields['variants']?.listValue.values;
    expect(list, isNotNull);
    expect(list!.first.structValue.fields['type']?.stringValue, 'SMS');
    expect(
      list.first.structValue.fields['detail']?.stringValue,
      'Hi {{name}}',
    );
  });

  testWidgets('save button has correct key and validates name', (tester) async {
    final fake = FakeNotificationClient();

    await tester.pumpWidget(_buildApp(fake));
    await tester.pumpAndSettle();

    // Tap save without entering a name — should not submit.
    await tester.tap(find.byKey(const Key('template-save-button')));
    await tester.pumpAndSettle();

    expect(fake.templateSaveRequests, isEmpty);
    expect(find.text('Required'), findsOneWidget);
  });

  testWidgets('renders TemplateVariantMatrix', (tester) async {
    final fake = FakeNotificationClient();

    await tester.pumpWidget(_buildApp(fake));
    await tester.pumpAndSettle();

    // The matrix renders a data table with the channel × language grid.
    expect(find.byType(DataTable), findsOneWidget);
    // The SMS/en cell is present.
    expect(find.byKey(const Key('cell-SMS-en')), findsOneWidget);
  });

  testWidgets('decodes Struct fallback variants on init', (tester) async {
    // Build a template whose variants are stored only in extra.fields['variants']
    // (the Struct fallback written by TemplateNotifier.save when the backend
    // does not populate the typed data field).
    final variantStruct = notif.Struct()
      ..fields['type'] = (notif.Value()..stringValue = 'SMS')
      ..fields['language'] = (notif.Value()..stringValue = 'en')
      ..fields['languageName'] = (notif.Value()..stringValue = 'English')
      ..fields['detail'] = (notif.Value()..stringValue = 'Struct body');
    final extra = notif.Struct()
      ..fields['variants'] = (notif.Value()
        ..listValue = (notif.ListValue()
          ..values.add(notif.Value()..structValue = variantStruct)));
    final template = notif.Template()
      ..name = 'fallback'
      ..extra = extra;
    // Intentionally no `data` field set (simulates Struct-only backend echo).

    final fake = FakeNotificationClient();
    final tenancy = TenancyContext()
      ..initializeFromLogin(
        LoginLevel.root,
        partitionId: 'part-test',
        partitionName: 'Test Partition',
        orgId: null,
        orgName: 'Test Org',
        branchId: null,
        branchName: 'Test Branch',
      );

    final router = GoRouter(
      initialLocation: '/edit',
      routes: [
        GoRoute(
          path: '/edit',
          builder: (_, __) => TemplateEditScreen(
            templateId: 'tmpl-fallback',
            initialTemplate: template,
          ),
        ),
        GoRoute(
          path: '/notifications/templates',
          builder: (_, __) => const Scaffold(body: Text('Templates')),
        ),
      ],
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          tenancyContextProvider.overrideWithValue(tenancy),
          notificationServiceClientProvider.overrideWithValue(fake.client),
        ],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pumpAndSettle();

    // The matrix cell for (SMS, en) should already contain 'Struct body'
    // because decodeTemplateVariants decoded the Struct fallback.
    expect(find.byKey(const Key('cell-SMS-en')), findsOneWidget);
    // Tapping the cell opens the editor pre-filled with the decoded content.
    await tester.tap(find.byKey(const Key('cell-SMS-en')));
    await tester.pumpAndSettle();
    // The cell editor is a TextField (not TextFormField) keyed 'cell-editor-content'.
    expect(
      (tester.widget(find.byKey(const Key('cell-editor-content'))) as TextField)
          .controller
          ?.text,
      'Struct body',
    );
  });

  testWidgets('initialises from initialTemplate', (tester) async {
    final fake = FakeNotificationClient();
    final template = notif.Template()
      ..name = 'existing'
      ..data.add(
        notif.TemplateData()
          ..type = 'EMAIL'
          ..detail = 'Hello!'
          ..language = (notif.Language()..code = 'en'),
      );

    final tenancy = TenancyContext()
      ..initializeFromLogin(
        LoginLevel.root,
        partitionId: 'part-test',
        partitionName: 'Test Partition',
        orgId: null,
        orgName: 'Test Org',
        branchId: null,
        branchName: 'Test Branch',
      );

    final router = GoRouter(
      initialLocation: '/edit',
      routes: [
        GoRoute(
          path: '/edit',
          builder: (_, __) => TemplateEditScreen(
            templateId: 'tmpl-1',
            initialTemplate: template,
          ),
        ),
        GoRoute(
          path: '/notifications/templates',
          builder: (_, __) => const Scaffold(body: Text('Templates')),
        ),
      ],
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          tenancyContextProvider.overrideWithValue(tenancy),
          notificationServiceClientProvider.overrideWithValue(fake.client),
        ],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pumpAndSettle();

    // Name field should be pre-filled.
    expect(find.widgetWithText(TextFormField, 'existing'), findsOneWidget);
    // AppBar should say 'Edit Template'.
    expect(find.text('Edit Template'), findsOneWidget);
  });
}
