import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter_test/flutter_test.dart';

import '../_helpers/fake_notification_client.dart';
import '../_helpers/test_harness.dart';

void main() {
  testWidgets('renders KPI tiles from search snapshot', (tester) async {
    final fake = FakeNotificationClient()
      ..nextSearchResults = [
        makeNotification(id: '1')
          ..status = (notif.StatusResponse()..state = notif.STATE.ACTIVE),
        makeNotification(id: '2')
          ..status = (notif.StatusResponse()..state = notif.STATE.INACTIVE),
        makeNotification(id: '3')
          ..status = (notif.StatusResponse()..state = notif.STATE.CREATED),
      ];

    await tester.pumpWidget(TestHarness(
      client: fake,
      child: const NotificationDashboardScreen(),
    ));
    await tester.pumpAndSettle();

    expect(find.text('Sent'), findsOneWidget);
    expect(find.text('3'), findsAtLeastNWidgets(1)); // sent (may also appear in channel mix)
    expect(find.text('Delivered'), findsOneWidget);
    expect(find.text('Failed'), findsOneWidget);
    expect(find.text('Queued'), findsOneWidget);
  });
}
