import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'notification_providers.dart';

/// Derived KPIs for the notification dashboard.
///
/// `sent` counts every notification in the snapshot. `delivered`,
/// `failed`, and `queued` are bucketed by `status.state` mapping to common
/// `STATE`. `channelMix` tallies by `type`. `topFailing` is the top 5
/// templates by failure count, descending.
class NotificationStats {
  const NotificationStats({
    required this.sent,
    required this.delivered,
    required this.failed,
    required this.queued,
    required this.channelMix,
    required this.topFailing,
  });

  final int sent;
  final int delivered;
  final int failed;
  final int queued;
  final Map<String, int> channelMix;
  final List<({String template, int failures})> topFailing;

  factory NotificationStats.empty() => const NotificationStats(
        sent: 0,
        delivered: 0,
        failed: 0,
        queued: 0,
        channelMix: <String, int>{},
        topFailing: <({String template, int failures})>[],
      );

  factory NotificationStats.fromList(List<notif.Notification> ns) {
    var delivered = 0;
    var failed = 0;
    var queued = 0;
    final channelMix = <String, int>{};
    final failuresByTemplate = <String, int>{};
    for (final n in ns) {
      switch (n.status.state) {
        case notif.STATE.ACTIVE:
          delivered++;
          break;
        case notif.STATE.INACTIVE:
          failed++;
          if (n.template.isNotEmpty) {
            failuresByTemplate.update(
              n.template,
              (v) => v + 1,
              ifAbsent: () => 1,
            );
          }
          break;
        case notif.STATE.CREATED:
        case notif.STATE.CHECKED:
          queued++;
          break;
        default:
          break;
      }
      if (n.type.isNotEmpty) {
        channelMix.update(n.type, (v) => v + 1, ifAbsent: () => 1);
      }
    }
    final topFailing = failuresByTemplate.entries
        .map((e) => (template: e.key, failures: e.value))
        .toList()
      ..sort((a, b) => b.failures.compareTo(a.failures));
    return NotificationStats(
      sent: ns.length,
      delivered: delivered,
      failed: failed,
      queued: queued,
      channelMix: channelMix,
      topFailing: topFailing.take(5).toList(),
    );
  }
}

/// Computes stats from the current scope's full notification snapshot.
final notificationStatsProvider = Provider.autoDispose<NotificationStats>(
    (ref) {
  final asyncNotifs = ref.watch(
    notificationSearchProvider(const NotificationSearchParams()),
  );
  return asyncNotifs.maybeWhen(
    data: NotificationStats.fromList,
    orElse: NotificationStats.empty,
  );
});
