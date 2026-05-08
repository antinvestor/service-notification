import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../providers/language_providers.dart';

/// Lists every language used across templates in the active partition.
class LanguageListScreen extends ConsumerStatefulWidget {
  const LanguageListScreen({super.key});

  @override
  ConsumerState<LanguageListScreen> createState() =>
      _LanguageListScreenState();
}

class _LanguageListScreenState extends ConsumerState<LanguageListScreen> {
  String _query = '';

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final asyncLangs = ref.watch(languageSearchProvider(_query));

    return asyncLangs.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (e, _) => Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Text(
            '$e',
            style: theme.textTheme.bodyMedium?.copyWith(
              color: theme.colorScheme.error,
            ),
          ),
        ),
      ),
      data: (languages) => AdminEntityListPage<notif.Language>(
        title: 'Languages',
        breadcrumbs: const ['Home', 'Notifications', 'Languages'],
        columns: const [
          DataColumn(label: Text('Code')),
          DataColumn(label: Text('Name')),
        ],
        items: languages,
        onSearch: (q) => setState(() => _query = q.trim()),
        searchHint: 'Search languages...',
        onAdd: () => context.go('/notifications/languages/edit'),
        addLabel: 'New Language',
        onRowNavigate: (lang) {
          context.go('/notifications/languages/edit/${lang.code}',
              extra: lang);
        },
        rowBuilder: (lang, selected, onSelect) {
          return DataRow(
            selected: selected,
            onSelectChanged: (_) => onSelect(),
            cells: [
              DataCell(Text(lang.code)),
              DataCell(Text(lang.name)),
            ],
          );
        },
        exportRow: (lang) => [lang.code, lang.name],
        onExport: (format, count) {
          debugPrint('[AUDIT] Exported $count Languages as $format');
        },
      ),
    );
  }
}
