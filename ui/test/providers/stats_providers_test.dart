import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../_helpers/fake_notification_client.dart';
import '../_helpers/test_harness.dart';

notif.Notification _withState(
    String id, notif.STATE state, String type, String template) {
  return makeNotification(id: id, type: type, template: template)
    ..status = (notif.StatusResponse()..state = state);
}

void main() {
  test('NotificationStats.fromList tallies counts correctly', () {
    final stats = NotificationStats.fromList([
      _withState('1', notif.STATE.ACTIVE, 'SMS', 'welcome'),
      _withState('2', notif.STATE.ACTIVE, 'EMAIL', 'welcome'),
      _withState('3', notif.STATE.INACTIVE, 'SMS', 'reset'),
      _withState('4', notif.STATE.INACTIVE, 'SMS', 'reset'),
      _withState('5', notif.STATE.CREATED, 'PUSH', 'reset'),
    ]);
    expect(stats.sent, 5);
    expect(stats.delivered, 2);
    expect(stats.failed, 2);
    expect(stats.queued, 1);
    expect(stats.channelMix, {'SMS': 3, 'EMAIL': 1, 'PUSH': 1});
    expect(stats.topFailing.first.template, 'reset');
    expect(stats.topFailing.first.failures, 2);
  });

  testWidgets('notificationStatsProvider derives from search snapshot',
      (tester) async {
    final fake = FakeNotificationClient()
      ..nextSearchResults = [
        _withState('1', notif.STATE.ACTIVE, 'SMS', 'welcome'),
        _withState('2', notif.STATE.INACTIVE, 'SMS', 'reset'),
      ];
    NotificationStats? observed;
    await tester.pumpWidget(TestHarness(
      client: fake,
      child: Consumer(builder: (_, ref, __) {
        ref.watch(notificationSearchProvider(
            const NotificationSearchParams()));
        observed = ref.watch(notificationStatsProvider);
        return const SizedBox();
      }),
    ));
    await tester.pumpAndSettle();
    expect(observed!.sent, 2);
    expect(observed!.delivered, 1);
    expect(observed!.failed, 1);
  });
}
