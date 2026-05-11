import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

class _RebuildHost extends StatefulWidget {
  const _RebuildHost({super.key});
  @override
  State<_RebuildHost> createState() => _RebuildHostState();
}

class _RebuildHostState extends State<_RebuildHost> {
  int _tick = 0;
  void bump() => setState(() => _tick++);
  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Text('tick:$_tick', style: const TextStyle(fontSize: 1)),
        TemplateVariantMatrix(
          variants: const [],
          onChanged: (_) {},
          availableChannels: const ['SMS'],
          availableLanguages: const ['en'],
        ),
      ],
    );
  }
}

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

  testWidgets('editing an existing filled variant updates without mutating original',
      (tester) async {
    final original = notif.TemplateData()
      ..type = 'SMS'
      ..detail = 'old text'
      ..language = (notif.Language()..code = 'en');
    final variants = [original];
    List<notif.TemplateData>? lastEmitted;

    await tester.pumpWidget(MaterialApp(
      home: Scaffold(
        body: TemplateVariantMatrix(
          variants: variants,
          onChanged: (v) => lastEmitted = v,
          availableChannels: const ['SMS'],
          availableLanguages: const ['en'],
        ),
      ),
    ));
    await tester.tap(find.byKey(const Key('cell-SMS-en')));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(const Key('cell-editor-content')),
      'new text',
    );
    await tester.tap(find.byKey(const Key('cell-editor-save')));
    await tester.pumpAndSettle();

    expect(lastEmitted, isNotNull);
    expect(lastEmitted!.single.detail, 'new text');
    // Crucially: the original proto must not be mutated.
    expect(original.detail, 'old text');
  });

  testWidgets('cancel closes the editor without emitting', (tester) async {
    bool emitted = false;
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(
        body: TemplateVariantMatrix(
          variants: const [],
          onChanged: (_) => emitted = true,
          availableChannels: const ['SMS'],
          availableLanguages: const ['en'],
        ),
      ),
    ));
    await tester.tap(find.byKey(const Key('cell-SMS-en')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('cell-editor-content')), findsOneWidget);

    await tester.enterText(
      find.byKey(const Key('cell-editor-content')),
      'unsaved',
    );
    await tester.tap(find.text('Cancel'));
    await tester.pumpAndSettle();

    expect(emitted, isFalse);
    expect(find.byKey(const Key('cell-editor-content')), findsNothing);
  });

  testWidgets('controller survives parent rebuild while editor is open',
      (tester) async {
    // The widget tree wraps the matrix in a Builder so we can trigger an
    // extraneous rebuild via setState elsewhere. Without the state-owned
    // controller, the typed text would be lost on rebuild.
    final key = GlobalKey<_RebuildHostState>();
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(
        body: _RebuildHost(key: key),
      ),
    ));
    await tester.tap(find.byKey(const Key('cell-SMS-en')));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(const Key('cell-editor-content')),
      'will it survive',
    );
    // Force a parent rebuild while the editor is open.
    key.currentState!.bump();
    await tester.pumpAndSettle();
    // Text must still be there.
    expect(find.text('will it survive'), findsOneWidget);
  });
}
