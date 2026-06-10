import 'dart:convert';

import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

import '../_helpers/fake_analytics_transport.dart';

void main() {
  final timeRange = AnalyticsTimeRange(
    start: DateTime.utc(2026, 1, 1),
    end: DateTime.utc(2026, 1, 31),
    granularity: TimeGranularity.day,
  );
  const wireTimeRange = {
    'start': '2026-01-01T00:00:00.000Z',
    'end': '2026-01-31T00:00:00.000Z',
  };

  late FakeAnalyticsTransport transport;
  late ThesaAnalyticsDataSource dataSource;

  setUp(() {
    transport = FakeAnalyticsTransport();
    dataSource = ThesaAnalyticsDataSource(
      transport.call,
      specs: const [notificationAnalyticsSpec],
    );
  });

  test('KPI fetch posts one exact scalar body per declared metric', () async {
    await dataSource.getMetrics('notification', timeRange: timeRange);

    expect(
      transport.calls.map((c) => c.path),
      everyElement('/api/analytics/query/scalar'),
    );
    expect(transport.calls.map((c) => c.body), [
      {
        'metric': 'notifications_sent_total',
        'aggregation': 'sum',
        'time_range': wireTimeRange,
      },
      {
        'metric': 'notifications_delivered_total',
        'aggregation': 'sum',
        'time_range': wireTimeRange,
      },
      {
        'metric': 'notifications_failed_total',
        'aggregation': 'sum',
        'time_range': wireTimeRange,
      },
      {
        'metric': 'notifications_queued_total',
        'aggregation': 'sum',
        'time_range': wireTimeRange,
      },
      {
        'metric': 'notifications_send_duration_ms',
        'aggregation': 'avg',
        'time_range': wireTimeRange,
      },
    ]);
  });

  test('sent trend posts an exact timeseries body with step', () async {
    await dataSource.getTimeSeries(
      'notification',
      notificationsSentMetric,
      timeRange: timeRange,
    );

    expect(transport.calls, hasLength(1));
    expect(transport.calls.single.path, '/api/analytics/query/timeseries');
    expect(transport.calls.single.body, {
      'metric': 'notifications_sent_total',
      'aggregation': 'sum',
      'time_range': wireTimeRange,
      'step': 'day',
    });
  });

  test('channel mix posts an exact grouped body', () async {
    await dataSource.getDistribution(
      'notification',
      notificationsSentMetric,
      'channel',
      timeRange: timeRange,
    );

    expect(transport.calls, hasLength(1));
    expect(transport.calls.single.path, '/api/analytics/query/grouped');
    expect(transport.calls.single.body, {
      'metric': 'notifications_sent_total',
      'aggregation': 'sum',
      'group_by': 'channel',
      'time_range': wireTimeRange,
    });
  });

  test('no request ever carries tenant or partition filters', () async {
    await dataSource.getMetrics('notification', timeRange: timeRange);
    await dataSource.getTimeSeries(
      'notification',
      notificationsSentMetric,
      timeRange: timeRange,
    );
    await dataSource.getDistribution(
      'notification',
      notificationsSentMetric,
      'channel',
      timeRange: timeRange,
    );

    for (final call in transport.calls) {
      final filters =
          (call.body['filters'] as Map<String, dynamic>?) ?? const {};
      expect(filters.keys, isNot(contains('tenant_id')));
      expect(filters.keys, isNot(contains('partition_id')));
    }
  });

  test('gate errors surface status code and server message', () async {
    transport.handler = (path, body) =>
        http.Response(json.encode({'error': 'metric not allowed'}), 400);

    await expectLater(
      dataSource.getMetrics('notification', timeRange: timeRange),
      throwsA(
        isA<AnalyticsQueryException>()
            .having((e) => e.statusCode, 'statusCode', 400)
            .having((e) => e.message, 'message', 'metric not allowed'),
      ),
    );
  });

  test('analyticsGateMessage maps gate statuses to friendly text', () {
    const path = '/api/analytics/query/scalar';
    expect(
      analyticsGateMessage(
        const AnalyticsQueryException(
          statusCode: 400,
          message: 'metric not allowed',
          path: path,
        ),
      ),
      'This metric is not available from the analytics gate.',
    );
    expect(
      analyticsGateMessage(
        const AnalyticsQueryException(
          statusCode: 403,
          message: 'no tenant scope',
          path: path,
        ),
      ),
      'Analytics are not available for your current sign-in scope.',
    );
    expect(
      analyticsGateMessage(
        const AnalyticsQueryException(
          statusCode: 503,
          message: 'backend down',
          path: path,
        ),
      ),
      contains('temporarily unavailable'),
    );
    expect(
      analyticsGateMessage(StateError('boom')),
      'Could not load analytics.',
    );
  });
}
