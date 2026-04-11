import 'package:antinvestor_api_notification/antinvestor_api_notification.dart';
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
    List<Notification>, NotificationSearchParams>((ref, params) async {
  final client = ref.watch(notificationServiceClientProvider);
  final request = SearchRequest()
    ..query = params.query;
  if (params.type.isNotEmpty) {
    request.properties.add('type:${params.type}');
  }
  if (params.recipient.isNotEmpty) {
    request.properties.add('recipient:${params.recipient}');
  }
  final stream = client.search(request);
  return collectStream<SearchResponse, Notification>(
    stream,
    extract: (r) => r.data,
  );
});

/// Acknowledge receipt of notifications.
final notificationReceiveProvider =
    FutureProvider.family<List<StatusResponse>, List<Notification>>((ref, notifications) async {
  final client = ref.watch(notificationServiceClientProvider);
  final request = ReceiveRequest();
  request.data.addAll(notifications);
  final stream = client.receive(request);
  return collectStream<ReceiveResponse, StatusResponse>(
    stream,
    extract: (r) => r.data,
  );
});

/// Get notification status by ID.
final notificationStatusProvider =
    FutureProvider.family<StatusResponse, String>((ref, notificationId) async {
  final client = ref.watch(notificationServiceClientProvider);
  final request = StatusRequest()..id = notificationId;
  return client.status(request);
});

/// Notifier for notification mutations (send, release, status update).
class NotificationNotifier extends StateNotifier<AsyncValue<void>> {
  NotificationNotifier(this._client) : super(const AsyncValue.data(null));
  final NotificationServiceClient _client;

  Future<void> send(SendRequest request) async {
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

  Future<void> release(ReleaseRequest request) async {
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

  Future<void> statusUpdate(StatusUpdateRequest request) async {
    state = const AsyncValue.loading();
    try {
      final response = await _client.statusUpdate(request);
      state = const AsyncValue.data(null);
    } catch (e, st) {
      state = AsyncValue.error(e, st);
      rethrow;
    }
  }
}

final notificationNotifierProvider =
    StateNotifierProvider<NotificationNotifier, AsyncValue<void>>((ref) {
  final client = ref.watch(notificationServiceClientProvider);
  return NotificationNotifier(client);
});
