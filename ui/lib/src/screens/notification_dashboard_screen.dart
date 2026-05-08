import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../providers/notification_providers.dart';
import '../providers/stats_providers.dart';

/// Top-level dashboard for the partition's notification activity.
///
/// Composes `ServiceAnalyticsPage` from ui_core. Data comes from
/// `notificationStatsProvider` (derived from the current search snapshot).
class NotificationDashboardScreen extends ConsumerWidget {
  const NotificationDashboardScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // Trigger the underlying search so stats has data.
    ref.watch(notificationSearchProvider(const NotificationSearchParams()));
    final stats = ref.watch(notificationStatsProvider);
    final tenancy = ref.watch(tenancyContextProvider);

    final crumbs = <String>['Home', ...tenancy.breadcrumbs, 'Notifications'];

    final kpis = <ServiceKpi>[
      ServiceKpi(
        label: 'Sent',
        value: '${stats.sent}',
        icon: Icons.send_outlined,
      ),
      ServiceKpi(
        label: 'Delivered',
        value: '${stats.delivered}',
        icon: Icons.check_circle_outline,
        changePositive: true,
      ),
      ServiceKpi(
        label: 'Failed',
        value: '${stats.failed}',
        icon: Icons.error_outline,
        changePositive: false,
      ),
      ServiceKpi(
        label: 'Queued',
        value: '${stats.queued}',
        icon: Icons.schedule_outlined,
      ),
    ];

    final events = stats.topFailing
        .map((e) => ServiceEvent(
              title: '${e.template} — ${e.failures} failure(s)',
              timeAgo: '',
              severity: EventSeverity.error,
              icon: Icons.error_outline,
            ))
        .toList();

    return ServiceAnalyticsPage(
      title: 'Notifications',
      breadcrumbs: crumbs,
      kpis: kpis,
      chartTitle: 'Channel mix',
      chartSubtitle: 'Distribution of recent notifications by channel',
      chartWidget: _ChannelMixBars(channelMix: stats.channelMix),
      events: events,
    );
  }
}

/// Horizontal-bar visualization of channel mix. Avoids pulling in a
/// chart dep; uses LinearProgressIndicator per channel.
class _ChannelMixBars extends StatelessWidget {
  const _ChannelMixBars({required this.channelMix});
  final Map<String, int> channelMix;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    if (channelMix.isEmpty) {
      return SizedBox(
        height: 200,
        child: Center(
          child: Text(
            'No channel data',
            style: theme.textTheme.bodyMedium?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
        ),
      );
    }
    final total = channelMix.values.fold<int>(0, (a, b) => a + b);
    final entries = channelMix.entries.toList()
      ..sort((a, b) => b.value.compareTo(a.value));
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        for (final e in entries)
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 4),
            child: Row(
              children: [
                SizedBox(
                  width: 80,
                  child: Text(
                    e.key,
                    style: theme.textTheme.bodyMedium?.copyWith(
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
                Expanded(
                  child: ClipRRect(
                    borderRadius: BorderRadius.circular(4),
                    child: LinearProgressIndicator(
                      value: e.value / total,
                      minHeight: 10,
                      backgroundColor:
                          theme.colorScheme.surfaceContainerHighest,
                    ),
                  ),
                ),
                const SizedBox(width: 12),
                Text(
                  '${e.value}',
                  style: theme.textTheme.bodyMedium,
                ),
              ],
            ),
          ),
      ],
    );
  }
}
