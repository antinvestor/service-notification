import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_core/widgets/error_helpers.dart';
import 'package:antinvestor_ui_core/widgets/form_field_card.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../providers/language_providers.dart';
import '../providers/template_providers.dart';
import '../widgets/template_preview.dart';
import '../widgets/template_variant_matrix.dart';

/// Create or edit a notification template using the variants matrix.
class TemplateEditScreen extends ConsumerStatefulWidget {
  const TemplateEditScreen({
    super.key,
    this.templateId,
    this.initialTemplate,
  });

  final String? templateId;
  final notif.Template? initialTemplate;

  @override
  ConsumerState<TemplateEditScreen> createState() => _TemplateEditScreenState();
}

class _TemplateEditScreenState extends ConsumerState<TemplateEditScreen> {
  final _formKey = GlobalKey<FormState>();
  late final TextEditingController _nameController;
  late List<notif.TemplateData> _variants;
  bool _saving = false;
  String? _error;

  bool get _isEditing => widget.templateId != null;

  @override
  void initState() {
    super.initState();
    final t = widget.initialTemplate;
    _nameController = TextEditingController(text: t?.name ?? '');
    _variants = t == null ? <notif.TemplateData>[] : decodeTemplateVariants(t);
  }

  @override
  void dispose() {
    _nameController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final asyncLanguages = ref.watch(languageSearchProvider(''));

    final availableLanguages = asyncLanguages.maybeWhen(
      data: (langs) => langs.isEmpty
          ? const ['en']
          : langs.map((l) => l.code).toList(),
      orElse: () => const ['en'],
    );

    return Scaffold(
      appBar: AppBar(
        leading: IconButton(
          icon: const Icon(Icons.arrow_back),
          onPressed: () => context.canPop()
              ? context.pop()
              : context.go('/notifications/templates'),
        ),
        title: Text(
          _isEditing ? 'Edit Template' : 'New Template',
          style: theme.textTheme.titleMedium?.copyWith(
            fontWeight: FontWeight.w600,
          ),
        ),
        actions: [
          FilledButton.icon(
            key: const Key('template-save-button'),
            onPressed: _saving ? null : _save,
            icon: _saving
                ? const SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : const Icon(Icons.save, size: 18),
            label: Text(_saving ? 'Saving...' : 'Save'),
          ),
          const SizedBox(width: 16),
        ],
      ),
      body: Form(
        key: _formKey,
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              FormFieldCard(
                label: 'Template Name',
                description: 'A unique identifier for this template.',
                isRequired: true,
                child: TextFormField(
                  key: const Key('template-name-field'),
                  controller: _nameController,
                  decoration: InputDecoration(
                    hintText: 'e.g., welcome_sms',
                    prefixIcon: const Icon(Icons.label_outline),
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(12),
                    ),
                  ),
                  validator: (v) =>
                      (v == null || v.trim().isEmpty) ? 'Required' : null,
                ),
              ),
              Text(
                'Variants',
                style: theme.textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.w700,
                  color: theme.colorScheme.primary,
                ),
              ),
              const SizedBox(height: 4),
              Text(
                'Click a cell to edit content for that channel + language.',
                style: theme.textTheme.bodySmall?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                ),
              ),
              const SizedBox(height: 16),
              TemplateVariantMatrix(
                variants: _variants,
                onChanged: (next) => setState(() => _variants = next),
                availableLanguages: availableLanguages,
              ),
              const SizedBox(height: 24),
              if (_variants.isNotEmpty)
                TemplatePreview(
                  template: notif.Template()
                    ..name = _nameController.text
                    ..data.addAll(_variants),
                ),
              if (_error != null) ...[
                const SizedBox(height: 16),
                Container(
                  padding: const EdgeInsets.all(12),
                  decoration: BoxDecoration(
                    color: theme.colorScheme.errorContainer,
                    borderRadius: BorderRadius.circular(10),
                  ),
                  child: Row(
                    children: [
                      Icon(Icons.error_outline,
                          size: 20,
                          color: theme.colorScheme.onErrorContainer),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Text(
                          _error!,
                          style: theme.textTheme.bodySmall?.copyWith(
                            color: theme.colorScheme.onErrorContainer,
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }

  Future<void> _save() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() {
      _saving = true;
      _error = null;
    });
    try {
      await ref.read(templateNotifierProvider.notifier).save(
            name: _nameController.text.trim(),
            variants: _variants,
          );
      if (mounted) {
        setState(() => _saving = false);
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              _isEditing
                  ? 'Template updated successfully'
                  : 'Template created successfully',
            ),
            behavior: SnackBarBehavior.floating,
          ),
        );
        context.go('/notifications/templates');
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _saving = false;
          _error = friendlyError(e);
        });
      }
    }
  }
}
