import 'package:antinvestor_api_notification/antinvestor_api_notification.dart';
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../providers/template_providers.dart';

/// Screen listing notification templates using AdminEntityListPage with
/// DataTable, CSV export, and audit callback.
class TemplateListScreen extends ConsumerStatefulWidget {
  const TemplateListScreen({super.key});

  @override
  ConsumerState<TemplateListScreen> createState() => _TemplateListScreenState();
}

class _TemplateListScreenState extends ConsumerState<TemplateListScreen> {
  String _searchQuery = '';

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final asyncTemplates =
        ref.watch(templateSearchProvider(TemplateSearchParams(query: _searchQuery)));

    return asyncTemplates.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (error, _) => Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.error_outline,
                size: 48, color: theme.colorScheme.error),
            const SizedBox(height: 16),
            Text('$error', style: theme.textTheme.bodyLarge),
            const SizedBox(height: 16),
            FilledButton.tonal(
              onPressed: () =>
                  ref.invalidate(templateSearchProvider(TemplateSearchParams(query: _searchQuery))),
              child: const Text('Retry'),
            ),
          ],
        ),
      ),
      data: (templates) => AdminEntityListPage<Template>(
        title: 'Templates',
        breadcrumbs: const ['Home', 'Notifications', 'Templates'],
        columns: const [
          DataColumn(label: Text('Name')),
          DataColumn(label: Text('ID')),
          DataColumn(label: Text('Data entries count'), numeric: true),
        ],
        items: templates,
        onSearch: (query) {
          setState(() => _searchQuery = query.trim());
        },
        searchHint: 'Search templates...',
        onAdd: () => context.go('/notifications/templates/edit'),
        addLabel: 'New Template',
        onRowNavigate: (template) {
          context.go(
            '/notifications/templates/edit/${template.id}',
            extra: template,
          );
        },
        rowBuilder: (template, selected, onSelect) {
          return DataRow(
            selected: selected,
            onSelectChanged: (_) => onSelect(),
            cells: [
              DataCell(Text(template.name)),
              DataCell(Text(template.id)),
              DataCell(Text('${template.data.length}')),
            ],
          );
        },
        exportRow: (template) => [
          template.name,
          template.id,
          '${template.data.length}',
        ],
        onExport: (format, count) {
          debugPrint('[AUDIT] Exported $count Templates as $format');
        },
      ),
    );
  }
}
