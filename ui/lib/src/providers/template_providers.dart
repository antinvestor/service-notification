import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_core/api/stream_helpers.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'notification_transport_provider.dart';
import 'tenancy_aware_providers.dart';

/// Parameters for searching templates.
class TemplateSearchParams {
  const TemplateSearchParams({this.query = '', this.languageCode = ''});

  final String query;
  final String languageCode;

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is TemplateSearchParams &&
          query == other.query &&
          languageCode == other.languageCode;

  @override
  int get hashCode => Object.hash(query, languageCode);

  @override
  String toString() =>
      'TemplateSearchParams(query: $query, languageCode: $languageCode)';
}

/// Search templates scoped to the current tenancy.
///
/// Tenancy scope is enforced server-side via auth context; we still
/// ref.watch it so a partition switch invalidates this cache and a
/// fresh search runs under the new scope.
final templateSearchProvider = FutureProvider.autoDispose
    .family<List<notif.Template>, TemplateSearchParams>((ref, params) async {
  ref.watch(tenancyScopeProvider);

  final client = ref.watch(notificationServiceClientProvider);
  final request = notif.TemplateSearchRequest()
    ..query = params.query
    ..languageCode = params.languageCode;
  final stream = client.templateSearch(request);
  return collectStream<notif.TemplateSearchResponse, notif.Template>(
    stream,
    extract: (r) => r.data,
  );
});

/// Notifier for template mutations.
///
/// `save` encodes the variants list into the proto `data Struct` under a
/// `variants` key. Each variant is a Struct with `type`, `language`, `detail`
/// string fields. On success invalidates `templateSearchProvider`.
class TemplateNotifier extends Notifier<AsyncValue<void>> {
  @override
  AsyncValue<void> build() => const AsyncValue.data(null);

  notif.NotificationServiceClient get _client =>
      ref.read(notificationServiceClientProvider);

  Future<notif.Template> save({
    required String name,
    required List<notif.TemplateData> variants,
  }) async {
    state = const AsyncValue.loading();
    try {
      final variantValues = variants.map((td) {
        final variantStruct = notif.Struct();
        variantStruct.fields['type'] = notif.Value()..stringValue = td.type;
        variantStruct.fields['language'] = notif.Value()
          ..stringValue = td.language.code;
        variantStruct.fields['detail'] = notif.Value()..stringValue = td.detail;
        return notif.Value()..structValue = variantStruct;
      }).toList();

      final dataStruct = notif.Struct();
      dataStruct.fields['variants'] = notif.Value()
        ..listValue = (notif.ListValue()..values.addAll(variantValues));

      // Default the top-level language_code to the first variant's language
      // so older backends that ignore data.variants still record something.
      final defaultLang = variants.isEmpty ? '' : variants.first.language.code;

      final request = notif.TemplateSaveRequest()
        ..name = name
        ..languageCode = defaultLang
        ..data = dataStruct
        ..extra = dataStruct;

      final response = await _client.templateSave(request);
      ref.invalidate(templateSearchProvider);
      state = const AsyncValue.data(null);
      return response.data;
    } catch (e, st) {
      state = AsyncValue.error(e, st);
      rethrow;
    }
  }
}

final templateNotifierProvider =
    NotifierProvider<TemplateNotifier, AsyncValue<void>>(TemplateNotifier.new);

/// Decodes a Template's variants. Prefers the proto's typed
/// `repeated TemplateData data` field if populated; otherwise falls back
/// to the Struct contract written by [TemplateNotifier.save] (variants
/// list stored under `extra.fields['variants']`).
List<notif.TemplateData> decodeTemplateVariants(notif.Template template) {
  if (template.data.isNotEmpty) return List.of(template.data);

  if (!template.hasExtra()) return const [];
  final variantsField = template.extra.fields['variants'];
  if (variantsField == null) return const [];
  final list = variantsField.listValue.values;
  return list.map((v) {
    final s = v.structValue;
    final type = s.fields['type']?.stringValue ?? '';
    final language = s.fields['language']?.stringValue ?? '';
    final detail = s.fields['detail']?.stringValue ?? '';
    return notif.TemplateData()
      ..type = type
      ..detail = detail
      ..language = (notif.Language()..code = language);
  }).toList();
}
