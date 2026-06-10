import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../analytics/notification_analytics.dart';
import '../providers/notification_providers.dart';
import '../providers/stats_providers.dart';

/// Top-level dashboard for the partition's notification activity.
///
/// KPIs, the sent trend, and the channel mix come from the thesa analytics
/// gate ([analyticsDataSourceProvider]) using the notification service's
/// business metrics. The "Top failing templates" panel stays entity-derived
/// from the current search snapshot. Tenant scoping is injected server-side;
/// this screen never sends tenant or partition filters.
class NotificationDashboardScreen extends ConsumerStatefulWidget {
  const NotificationDashboardScreen({super.key});

  @override
  ConsumerState<NotificationDashboardScreen> createState() =>
      _NotificationDashboardScreenState();
}

class _NotificationDashboardScreenState
    extends ConsumerState<NotificationDashboardScreen> {
  AnalyticsTimeRange _timeRange = AnalyticsTimeRange.last30Days();

  static const String _service = 'notification';

  ServiceMetricsParams get _metricsParams =>
      ServiceMetricsParams(_service, timeRange: _timeRange);

  ServiceTimeSeriesParams get _trendParams => ServiceTimeSeriesParams(
    _service,
    notificationsSentMetric,
    timeRange: _timeRange,
  );

  ServiceDistributionParams get _channelMixParams => ServiceDistributionParams(
    _service,
    notificationsSentMetric,
    'channel',
    timeRange: _timeRange,
  );

  void _refresh() {
    ref
      ..invalidate(serviceMetricsProvider(_metricsParams))
      ..invalidate(serviceTimeSeriesProvider(_trendParams))
      ..invalidate(serviceDistributionProvider(_channelMixParams));
  }

  @override
  Widget build(BuildContext context) {
    // Trigger the underlying search so the entity-derived panel has data.
    ref.watch(notificationSearchProvider(const NotificationSearchParams()));
    final stats = ref.watch(notificationStatsProvider);
    final tenancy = ref.watch(tenancyContextProvider);

    final metricsAsync = ref.watch(serviceMetricsProvider(_metricsParams));
    final trendAsync = ref.watch(serviceTimeSeriesProvider(_trendParams));
    final mixAsync = ref.watch(serviceDistributionProvider(_channelMixParams));

    final crumbs = <String>['Home', ...tenancy.breadcrumbs, 'Notifications'];
    final isDesktop = AppBreakpoints.isDesktop(
      MediaQuery.sizeOf(context).width,
    );

    final trendCard = _ChartCard(
      title: 'Notifications sent',
      subtitle: 'Dispatch volume over time',
      child: trendAsync.when(
        data: (series) => TimeSeriesChart(
          series: series,
          granularity: _timeRange.granularity,
        ),
        loading: () => const _ChartLoading(),
        error: (e, _) => AnalyticsGateErrorCard(
          message: analyticsGateMessage(e),
          onRetry: _refresh,
        ),
      ),
    );

    final mixCard = _ChartCard(
      title: 'Channel mix',
      subtitle: 'Notifications sent by channel',
      child: mixAsync.when(
        data: (segments) => DistributionChart(segments: segments),
        loading: () => const _ChartLoading(),
        error: (e, _) => AnalyticsGateErrorCard(
          message: analyticsGateMessage(e),
          onRetry: _refresh,
        ),
      ),
    );

    return SingleChildScrollView(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          PageHeader(
            title: 'Notifications',
            breadcrumbs: crumbs,
            actions: [
              IconButton(
                icon: const Icon(Icons.refresh),
                tooltip: 'Refresh',
                onPressed: _refresh,
              ),
            ],
          ),
          const SizedBox(height: 16),
          SingleChildScrollView(
            scrollDirection: Axis.horizontal,
            child: TimeRangeSelector(
              value: _timeRange,
              onChanged: (range) => setState(() => _timeRange = range),
            ),
          ),
          const SizedBox(height: 20),
          metricsAsync.when(
            data: (metrics) => metrics.isEmpty
                ? const AnalyticsGateErrorCard(
                    message:
                        'Analytics are not configured for the '
                        'notification service in this app.',
                  )
                : MetricsRow(metrics: metrics),
            loading: () => MetricsRow(
              metrics: const [],
              isLoading: true,
              skeletonCount: notificationAnalyticsSpec.kpis.length,
            ),
            error: (e, _) => AnalyticsGateErrorCard(
              message: analyticsGateMessage(e),
              onRetry: _refresh,
            ),
          ),
          const SizedBox(height: 20),
          if (isDesktop)
            IntrinsicHeight(
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Expanded(child: trendCard),
                  const SizedBox(width: 16),
                  Expanded(child: mixCard),
                ],
              ),
            )
          else ...[
            trendCard,
            const SizedBox(height: 16),
            mixCard,
          ],
          const SizedBox(height: 20),
          _TopFailingCard(topFailing: stats.topFailing),
        ],
      ),
    );
  }
}

/// Friendly inline error/empty card for analytics gate failures.
class AnalyticsGateErrorCard extends StatelessWidget {
  const AnalyticsGateErrorCard({
    super.key,
    required this.message,
    this.onRetry,
  });

  final String message;
  final VoidCallback? onRetry;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: cs.errorContainer.withValues(alpha: 0.25),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        children: [
          Icon(Icons.insights_outlined, color: cs.error, size: 20),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              message,
              style: TextStyle(color: cs.error, fontSize: 13),
            ),
          ),
          if (onRetry != null) ...[
            const SizedBox(width: 8),
            TextButton(onPressed: onRetry, child: const Text('Retry')),
          ],
        ],
      ),
    );
  }
}

class _ChartCard extends StatelessWidget {
  const _ChartCard({
    required this.title,
    required this.subtitle,
    required this.child,
  });

  final String title;
  final String subtitle;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;
    return Container(
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: cs.surface,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: cs.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            title,
            style: theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            subtitle,
            style: theme.textTheme.bodySmall?.copyWith(
              color: cs.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 16),
          child,
        ],
      ),
    );
  }
}

class _ChartLoading extends StatelessWidget {
  const _ChartLoading();

  @override
  Widget build(BuildContext context) {
    return const SizedBox(
      height: 240,
      child: Center(child: CircularProgressIndicator()),
    );
  }
}

/// Entity-derived panel: top failing templates from the search snapshot.
class _TopFailingCard extends StatelessWidget {
  const _TopFailingCard({required this.topFailing});

  final List<({String template, int failures})> topFailing;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;
    return Container(
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: cs.surface,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: cs.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Top failing templates',
            style: theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 12),
          if (topFailing.isEmpty)
            Text(
              'No failures in the current snapshot',
              style: theme.textTheme.bodyMedium?.copyWith(
                color: cs.onSurfaceVariant,
              ),
            )
          else
            for (final entry in topFailing)
              Padding(
                padding: const EdgeInsets.symmetric(vertical: 4),
                child: Row(
                  children: [
                    Icon(Icons.error_outline, size: 16, color: cs.error),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        entry.template,
                        style: theme.textTheme.bodyMedium,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                    Text(
                      '${entry.failures} failure(s)',
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: cs.onSurfaceVariant,
                      ),
                    ),
                  ],
                ),
              ),
        ],
      ),
    );
  }
}
