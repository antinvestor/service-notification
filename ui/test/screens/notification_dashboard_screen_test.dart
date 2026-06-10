import 'dart:convert';

import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

import '../_helpers/fake_analytics_transport.dart';
import '../_helpers/fake_notification_client.dart';
import '../_helpers/test_harness.dart';

void main() {
  late FakeAnalyticsTransport transport;
  late FakeNotificationClient client;

  setUp(() {
    transport = FakeAnalyticsTransport();
    client = FakeNotificationClient();
  });

  Future<void> pumpDashboard(WidgetTester tester) async {
    await tester.pumpWidget(
      TestHarness(
        client: client,
        analytics: ThesaAnalyticsDataSource(
          transport.call,
          specs: const [notificationAnalyticsSpec],
        ),
        child: const NotificationDashboardScreen(),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('renders gate-backed KPI cards from scalar queries', (
    tester,
  ) async {
    transport.handler = (path, body) {
      if (!path.endsWith('/scalar')) {
        return http.Response(
          json.encode({'points': <Object>[], 'segments': <Object>[]}),
          200,
        );
      }
      final value = switch (body['metric']) {
        'notifications_sent_total' => 70,
        'notifications_delivered_total' => 64,
        'notifications_failed_total' => 6,
        'notifications_queued_total' => 9,
        'notifications_send_duration_ms' => 12.5,
        _ => 0,
      };
      return http.Response(json.encode({'value': value}), 200);
    };

    await pumpDashboard(tester);

    expect(find.text('Sent'), findsOneWidget);
    expect(find.text('70'), findsOneWidget);
    expect(find.text('Delivered'), findsOneWidget);
    expect(find.text('64'), findsOneWidget);
    expect(find.text('Failed'), findsOneWidget);
    expect(find.text('6'), findsOneWidget);
    expect(find.text('Queued'), findsOneWidget);
    expect(find.text('9'), findsOneWidget);
    expect(find.text('Avg send time (ms)'), findsOneWidget);
    expect(find.text('12.5'), findsOneWidget);
  });

  testWidgets('renders channel mix legend from the grouped query', (
    tester,
  ) async {
    transport.handler = (path, body) {
      if (path.endsWith('/grouped')) {
        expect(body['metric'], 'notifications_sent_total');
        expect(body['group_by'], 'channel');
        return http.Response(
          json.encode({
            'segments': [
              {'label': 'sms', 'value': 60},
              {'label': 'email', 'value': 40},
            ],
          }),
          200,
        );
      }
      if (path.endsWith('/timeseries')) {
        return http.Response(json.encode({'points': <Object>[]}), 200);
      }
      return http.Response(json.encode({'value': 0}), 200);
    };

    await pumpDashboard(tester);

    expect(find.text('Channel mix'), findsOneWidget);
    expect(find.text('sms'), findsOneWidget);
    expect(find.text('email'), findsOneWidget);
  });

  testWidgets('shows empty chart states when the gate has no data', (
    tester,
  ) async {
    await pumpDashboard(tester);

    // Sent trend and channel mix both report no data.
    expect(find.text('No data'), findsNWidgets(2));
  });

  for (final (status, fragment) in [
    (400, 'not available from the analytics gate'),
    (403, 'not available for your current sign-in scope'),
    (503, 'temporarily unavailable'),
  ]) {
    testWidgets('renders friendly state for gate HTTP $status', (tester) async {
      transport.handler = (path, body) =>
          http.Response(json.encode({'error': 'gate says no'}), status);

      await pumpDashboard(tester);

      // KPI row plus both charts surface the same friendly message.
      expect(find.textContaining(fragment), findsNWidgets(3));
      expect(find.textContaining('gate says no'), findsNothing);
      expect(find.text('Retry'), findsNWidgets(3));
    });
  }

  testWidgets('keeps entity-derived top failing templates panel', (
    tester,
  ) async {
    client.nextSearchResults = [
      makeNotification(id: '1', template: 'welcome')
        ..status = (notif.StatusResponse()..state = notif.STATE.INACTIVE),
      makeNotification(id: '2', template: 'welcome')
        ..status = (notif.StatusResponse()..state = notif.STATE.INACTIVE),
      makeNotification(id: '3', template: 'otp')
        ..status = (notif.StatusResponse()..state = notif.STATE.ACTIVE),
    ];

    await pumpDashboard(tester);

    expect(find.text('Top failing templates'), findsOneWidget);
    expect(find.text('welcome'), findsOneWidget);
    expect(find.text('2 failure(s)'), findsOneWidget);
  });
}
