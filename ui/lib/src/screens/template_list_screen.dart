import 'package:antinvestor_api_notification/antinvestor_api_notification.dart';
import 'package:antinvestor_ui_core/widgets/entity_list_page.dart';
import 'package:antinvestor_ui_core/widgets/error_helpers.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../providers/template_providers.dart';
import '../widgets/template_preview.dart';

/// Screen listing notification templates using EntityListPage.
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
    final asyncTemplates = ref.watch(templateSearchProvider(_searchQuery));

    return asyncTemplates.when(
      loading: () => _buildShell(theme, isLoading: true, items: const []),
      error: (error, _) => _buildShell(
        theme,
        error: friendlyError(error),
        items: const [],
      ),
      data: (templates) => _buildShell(theme, items: templates),
    );
  }

  Widget _buildShell(
    ThemeData theme, {
    required List<Template> items,
    bool isLoading = false,
    String? error,
  }) {
    return EntityListPage<Template>(
      title: 'Templates',
      icon: Icons.description,
      items: items,
      isLoading: isLoading,
      error: error,
      onRetry: () => ref.invalidate(templateSearchProvider(_searchQuery)),
      searchHint: 'Search templates...',
      onSearchChanged: (query) {
        setState(() => _searchQuery = query.trim());
      },
      actionLabel: 'New Template',
      onAction: () => context.go('/notifications/templates/edit'),
      itemBuilder: (context, template) {
        return InkWell(
          onTap: () {
            context.go(
              '/notifications/templates/edit/${template.id}',
              extra: template,
            );
          },
          borderRadius: BorderRadius.circular(12),
          child: TemplatePreview(template: template),
        );
      },
    );
  }
}
