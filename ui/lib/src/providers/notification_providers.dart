import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_core/api/stream_helpers.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'notification_transport_provider.dart';

/// Parameters for searching notifications.
class NotificationSearchParams {
  const NotificationSearchParams({
    this.query = '',
    this.type = '',
    this.recipient = '',
  });

  final String query;
  final String type;
  final String recipient;

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is NotificationSearchParams &&
          query == other.query &&
          type == other.type &&
          recipient == other.recipient;

  @override
  int get hashCode => Object.hash(query, type, recipient);
}

/// Search notifications with optional filters.
final notificationSearchProvider = FutureProvider.family<
    List<notif.Notification>, NotificationSearchParams>((ref, params) async {
  final client = ref.watch(notificationServiceClientProvider);
  final request = notif.SearchRequest()
    ..query = params.query;
  if (params.type.isNotEmpty) {
    request.properties.add('type:${params.type}');
  }
  if (params.recipient.isNotEmpty) {
    request.properties.add('recipient:${params.recipient}');
  }
  final stream = client.search(request);
  return collectStream<notif.SearchResponse, notif.Notification>(
    stream,
    extract: (r) => r.data,
  );
});

/// Acknowledge receipt of notifications.
final notificationReceiveProvider =
    FutureProvider.family<List<notif.StatusResponse>, List<notif.Notification>>((ref, notifications) async {
  final client = ref.watch(notificationServiceClientProvider);
  final request = notif.ReceiveRequest();
  request.data.addAll(notifications);
  final stream = client.receive(request);
  return collectStream<notif.ReceiveResponse, notif.StatusResponse>(
    stream,
    extract: (r) => r.data,
  );
});

/// Get notification status by ID.
final notificationStatusProvider =
    FutureProvider.family<notif.StatusResponse, String>((ref, notificationId) async {
  final client = ref.watch(notificationServiceClientProvider);
  final request = notif.StatusRequest()..id = notificationId;
  return client.status(request);
});

/// Notifier for notification mutations (send, release, status update).
class NotificationNotifier extends Notifier<AsyncValue<void>> {
  @override
  AsyncValue<void> build() => const AsyncValue.data(null);

  notif.NotificationServiceClient get _client =>
      ref.read(notificationServiceClientProvider);

  Future<void> send(notif.SendRequest request) async {
    state = const AsyncValue.loading();
    try {
      final stream = _client.send(request);
      await for (final _ in stream) {
        // consume stream responses
      }
      state = const AsyncValue.data(null);
    } catch (e, st) {
      state = AsyncValue.error(e, st);
      rethrow;
    }
  }

  Future<void> release(notif.ReleaseRequest request) async {
    state = const AsyncValue.loading();
    try {
      final stream = _client.release(request);
      await for (final _ in stream) {
        // consume stream responses
      }
      state = const AsyncValue.data(null);
    } catch (e, st) {
      state = AsyncValue.error(e, st);
      rethrow;
    }
  }

  Future<void> statusUpdate(notif.StatusUpdateRequest request) async {
    state = const AsyncValue.loading();
    try {
      await _client.statusUpdate(request);
      state = const AsyncValue.data(null);
    } catch (e, st) {
      state = AsyncValue.error(e, st);
      rethrow;
    }
  }
}

final notificationNotifierProvider =
    NotifierProvider<NotificationNotifier, AsyncValue<void>>(
        NotificationNotifier.new);
