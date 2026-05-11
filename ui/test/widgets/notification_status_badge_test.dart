import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('renders a StatusBadge with the right label', (tester) async {
    await tester.pumpWidget(const MaterialApp(
      home: Scaffold(
        body: NotificationStatusBadge(status: 'ACTIVE'),
      ),
    ));
    expect(find.byType(StatusBadge), findsOneWidget);
    expect(find.text('Active'), findsOneWidget);
  });

  testWidgets('falls back to the raw status string for unknown values',
      (tester) async {
    await tester.pumpWidget(const MaterialApp(
      home: Scaffold(
        body: NotificationStatusBadge(status: 'UNHEARD_OF'),
      ),
    ));
    expect(find.text('UNHEARD_OF'), findsOneWidget);
  });
}
