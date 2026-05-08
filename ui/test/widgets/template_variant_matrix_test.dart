import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('renders a row per channel and a column per language',
      (tester) async {
    final variants = [
      notif.TemplateData()
        ..type = 'SMS'
        ..detail = 'Hi'
        ..language = (notif.Language()..code = 'en'),
    ];
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(
        body: TemplateVariantMatrix(
          variants: variants,
          onChanged: (_) {},
          availableChannels: const ['SMS', 'EMAIL'],
          availableLanguages: const ['en', 'sw'],
        ),
      ),
    ));
    expect(find.text('SMS'), findsOneWidget);
    expect(find.text('EMAIL'), findsOneWidget);
    expect(find.text('en'), findsOneWidget);
    expect(find.text('sw'), findsOneWidget);
  });

  testWidgets('clicking an empty cell opens the editor and onChanged fires',
      (tester) async {
    List<notif.TemplateData>? lastEmitted;
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(
        body: TemplateVariantMatrix(
          variants: const [],
          onChanged: (v) => lastEmitted = v,
          availableChannels: const ['SMS'],
          availableLanguages: const ['en'],
        ),
      ),
    ));
    // Tap the (SMS, en) cell.
    await tester.tap(find.byKey(const Key('cell-SMS-en')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('cell-editor-content')), findsOneWidget);

    await tester.enterText(
      find.byKey(const Key('cell-editor-content')),
      'Hello {{name}}',
    );
    await tester.tap(find.byKey(const Key('cell-editor-save')));
    await tester.pumpAndSettle();

    expect(lastEmitted, isNotNull);
    expect(lastEmitted!.single.type, 'SMS');
    expect(lastEmitted!.single.language.code, 'en');
    expect(lastEmitted!.single.detail, 'Hello {{name}}');
  });
}
