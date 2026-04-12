import 'package:antinvestor_api_notification/antinvestor_api_notification.dart';
import 'package:antinvestor_ui_core/api/stream_helpers.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'notification_transport_provider.dart';

/// Search templates by query.
final templateSearchProvider =
    FutureProvider.family<List<Template>, String>((ref, query) async {
  final client = ref.watch(notificationServiceClientProvider);
  final request = TemplateSearchRequest()..query = query;
  final stream = client.templateSearch(request);
  return collectStream<TemplateSearchResponse, Template>(
    stream,
    extract: (r) => r.data,
  );
});

/// Notifier for template mutations.
class TemplateNotifier extends Notifier<AsyncValue<void>> {
  @override
  AsyncValue<void> build() => const AsyncValue.data(null);

  NotificationServiceClient get _client =>
      ref.read(notificationServiceClientProvider);

  Future<Template> save(TemplateSaveRequest request) async {
    state = const AsyncValue.loading();
    try {
      final response = await _client.templateSave(request);
      state = const AsyncValue.data(null);
      return response.data;
    } catch (e, st) {
      state = AsyncValue.error(e, st);
      rethrow;
    }
  }
}

final templateNotifierProvider =
    NotifierProvider<TemplateNotifier, AsyncValue<void>>(
        TemplateNotifier.new);
