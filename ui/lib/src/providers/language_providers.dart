import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'template_providers.dart';

/// Returns the union of all `Language` records found across templates,
/// deduped by `code`. Optionally filtered by a case-insensitive substring
/// match against either `code` or `name`.
final languageSearchProvider = FutureProvider.autoDispose
    .family<List<notif.Language>, String>((ref, query) async {
  final templates = await ref.watch(
      templateSearchProvider(const TemplateSearchParams()).future);
  final byCode = <String, notif.Language>{};
  for (final t in templates) {
    for (final td in t.data) {
      final code = td.language.code;
      if (code.isEmpty) continue;
      byCode.putIfAbsent(code, () => td.language);
    }
  }

  final q = query.toLowerCase().trim();
  final all = byCode.values.toList()
    ..sort((a, b) => a.code.compareTo(b.code));
  if (q.isEmpty) return all;
  return all
      .where((l) =>
          l.code.toLowerCase().contains(q) ||
          l.name.toLowerCase().contains(q))
      .toList();
});

/// Saves a Language by upserting it onto a placeholder template variant.
///
/// Because the proto has no dedicated LanguageSave RPC, we encode the
/// "language exists" fact into a placeholder template variant. Hosts that
/// later add a real LanguageSave can switch this notifier without touching
/// the screens.
class LanguageNotifier extends Notifier<AsyncValue<void>> {
  @override
  AsyncValue<void> build() => const AsyncValue.data(null);

  Future<notif.Language> save({
    required String code,
    required String name,
  }) async {
    state = const AsyncValue.loading();
    try {
      final language = notif.Language()
        ..code = code
        ..name = name;
      final placeholder = notif.TemplateData()
        ..type = 'SMS'
        ..detail = '(language registration placeholder)'
        ..language = language;
      final templateNotifier = ref.read(templateNotifierProvider.notifier);
      await templateNotifier.save(
        name: '_lang_$code',
        variants: [placeholder],
      );
      ref.invalidate(languageSearchProvider);
      state = const AsyncValue.data(null);
      return language;
    } catch (e, st) {
      state = AsyncValue.error(e, st);
      rethrow;
    }
  }
}

final languageNotifierProvider =
    NotifierProvider<LanguageNotifier, AsyncValue<void>>(
        LanguageNotifier.new);
