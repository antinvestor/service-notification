import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:flutter/material.dart';

/// Metric names emitted by service-notification's business instrumentation
/// (`notifications_*` counters in apps/default/service/events/metrics.go).
const String notificationsSentMetric = 'notifications_sent_total';
const String notificationsDeliveredMetric = 'notifications_delivered_total';
const String notificationsFailedMetric = 'notifications_failed_total';
const String notificationsQueuedMetric = 'notifications_queued_total';
const String notificationsSendDurationMetric = 'notifications_send_duration_ms';

/// Analytics catalog for the notification service, consumed by the
/// thesa-gated metrics pipeline.
///
/// Host apps register this spec on their [ThesaAnalyticsDataSource]:
///
/// ```dart
/// analyticsDataSourceProvider.overrideWith(
///   (ref) => ThesaAnalyticsDataSource(
///     transport,
///     specs: [notificationAnalyticsSpec],
///   ),
/// );
/// ```
///
/// Tenant scoping is injected server-side from the caller's JWT; no
/// tenant/partition filters are declared (or ever sent) here.
const ServiceAnalyticsSpec notificationAnalyticsSpec = ServiceAnalyticsSpec(
  service: 'notification',
  kpis: [
    KpiSpec(
      'sent',
      label: 'Sent',
      metric: notificationsSentMetric,
      unit: 'count',
      icon: Icons.send_outlined,
    ),
    KpiSpec(
      'delivered',
      label: 'Delivered',
      metric: notificationsDeliveredMetric,
      unit: 'count',
      icon: Icons.check_circle_outline,
    ),
    KpiSpec(
      'failed',
      label: 'Failed',
      metric: notificationsFailedMetric,
      unit: 'count',
      icon: Icons.error_outline,
    ),
    KpiSpec(
      'queued',
      label: 'Queued',
      metric: notificationsQueuedMetric,
      unit: 'count',
      icon: Icons.schedule_outlined,
    ),
    KpiSpec(
      'avg_send_ms',
      label: 'Avg send time (ms)',
      metric: notificationsSendDurationMetric,
      aggregation: AnalyticsAggregation.avg,
      icon: Icons.speed_outlined,
    ),
  ],
  charts: [
    ChartConfig.timeSeries(
      notificationsSentMetric,
      label: 'Notifications sent',
    ),
    ChartConfig.distribution(
      notificationsSentMetric,
      groupBy: 'channel',
      label: 'Channel mix',
    ),
  ],
);

/// Maps analytics gate failures to short, user-facing messages.
///
/// The gate's error contract: 400 -> metric rejected by the server-side
/// allowlist, 403 -> caller's JWT carries no tenant scope, 5xx -> metrics
/// backend unreachable.
String analyticsGateMessage(Object error) {
  if (error is AnalyticsQueryException) {
    return switch (error.statusCode) {
      400 => 'This metric is not available from the analytics gate.',
      403 => 'Analytics are not available for your current sign-in scope.',
      >= 500 =>
        'The analytics backend is temporarily unavailable. '
            'Please try again shortly.',
      _ => 'Could not load analytics (HTTP ${error.statusCode}).',
    };
  }
  return 'Could not load analytics.';
}
