import 'package:antinvestor_api_notification/antinvestor_api_notification.dart';
import 'package:antinvestor_ui_core/widgets/error_helpers.dart';
import 'package:antinvestor_ui_core/widgets/form_field_card.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../providers/template_providers.dart';
import '../widgets/language_selector.dart';

/// Screen for creating or editing a notification template.
class TemplateEditScreen extends ConsumerStatefulWidget {
  const TemplateEditScreen({
    super.key,
    this.templateId,
    this.initialTemplate,
  });

  final String? templateId;
  final Template? initialTemplate;

  @override
  ConsumerState<TemplateEditScreen> createState() => _TemplateEditScreenState();
}

class _TemplateEditScreenState extends ConsumerState<TemplateEditScreen> {
  final _formKey = GlobalKey<FormState>();
  late final TextEditingController _nameController;
  late List<_TemplateDataEntry> _dataEntries;

  bool _saving = false;
  String? _error;

  bool get _isEditing => widget.templateId != null;

  @override
  void initState() {
    super.initState();
    final t = widget.initialTemplate;
    _nameController = TextEditingController(text: t?.name ?? '');
    _dataEntries = t?.data.map((td) {
          return _TemplateDataEntry(
            typeController: TextEditingController(text: td.type),
            detailController: TextEditingController(text: td.detail),
            language: td.language.code.isEmpty ? 'en' : td.language.code,
          );
        }).toList() ??
        [
          _TemplateDataEntry(
            typeController: TextEditingController(),
            detailController: TextEditingController(),
            language: 'en',
          ),
        ];
  }

  @override
  void dispose() {
    _nameController.dispose();
    for (final entry in _dataEntries) {
      entry.typeController.dispose();
      entry.detailController.dispose();
    }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

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
            onPressed: _saving ? null : _save,
            icon: _saving
                ? const SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(
                      strokeWidth: 2,
                      color: Colors.white,
                    ),
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
              // Template name
              FormFieldCard(
                label: 'Template Name',
                description: 'A unique identifier for this template.',
                isRequired: true,
                child: TextFormField(
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

              // Template data variants
              Text(
                'Template Variants',
                style: theme.textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.w700,
                  color: theme.colorScheme.primary,
                ),
              ),
              const SizedBox(height: 4),
              Text(
                'Add content variants for different channels and languages.',
                style: theme.textTheme.bodySmall?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                ),
              ),
              const SizedBox(height: 16),

              for (var i = 0; i < _dataEntries.length; i++) ...[
                Card(
                  elevation: 0,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(12),
                    side: BorderSide(color: theme.colorScheme.outlineVariant),
                  ),
                  child: Padding(
                    padding: const EdgeInsets.all(16),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          children: [
                            Text(
                              'Variant ${i + 1}',
                              style: theme.textTheme.titleSmall?.copyWith(
                                fontWeight: FontWeight.w600,
                              ),
                            ),
                            const Spacer(),
                            if (_dataEntries.length > 1)
                              IconButton(
                                icon: Icon(
                                  Icons.delete_outline,
                                  size: 20,
                                  color: theme.colorScheme.error,
                                ),
                                onPressed: () {
                                  setState(() {
                                    _dataEntries[i]
                                        .typeController
                                        .dispose();
                                    _dataEntries[i]
                                        .detailController
                                        .dispose();
                                    _dataEntries.removeAt(i);
                                  });
                                },
                              ),
                          ],
                        ),
                        const SizedBox(height: 12),

                        // Type (channel type)
                        TextFormField(
                          controller: _dataEntries[i].typeController,
                          decoration: InputDecoration(
                            labelText: 'Type (e.g., SMS, EMAIL)',
                            prefixIcon: const Icon(Icons.category_outlined),
                            border: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(12),
                            ),
                          ),
                          validator: (v) =>
                              (v == null || v.trim().isEmpty)
                                  ? 'Required'
                                  : null,
                        ),
                        const SizedBox(height: 12),

                        // Language
                        LanguageSelector(
                          selectedLanguage: _dataEntries[i].language,
                          onChanged: (lang) {
                            setState(() {
                              _dataEntries[i].language = lang;
                            });
                          },
                        ),
                        const SizedBox(height: 12),

                        // Detail (content)
                        TextFormField(
                          controller: _dataEntries[i].detailController,
                          maxLines: 6,
                          minLines: 3,
                          decoration: InputDecoration(
                            labelText: 'Content',
                            alignLabelWithHint: true,
                            hintText:
                                'Hello {{name}}, your code is {{code}}.',
                            border: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(12),
                            ),
                          ),
                          validator: (v) =>
                              (v == null || v.trim().isEmpty)
                                  ? 'Required'
                                  : null,
                        ),
                      ],
                    ),
                  ),
                ),
                const SizedBox(height: 12),
              ],

              // Add variant button
              Align(
                alignment: Alignment.centerLeft,
                child: OutlinedButton.icon(
                  onPressed: () {
                    setState(() {
                      _dataEntries.add(_TemplateDataEntry(
                        typeController: TextEditingController(),
                        detailController: TextEditingController(),
                        language: 'en',
                      ));
                    });
                  },
                  icon: const Icon(Icons.add, size: 18),
                  label: const Text('Add Variant'),
                ),
              ),

              // Error display
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
                      Icon(
                        Icons.error_outline,
                        size: 20,
                        color: theme.colorScheme.onErrorContainer,
                      ),
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
      final notifier = ref.read(templateNotifierProvider.notifier);

      final request = TemplateSaveRequest()
        ..name = _nameController.text.trim()
        ..languageCode = _dataEntries.isNotEmpty ? _dataEntries.first.language : 'en';
      await notifier.save(request);

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

/// Internal helper for template data entries in the form.
class _TemplateDataEntry {
  _TemplateDataEntry({
    required this.typeController,
    required this.detailController,
    required this.language,
  });

  final TextEditingController typeController;
  final TextEditingController detailController;
  String language;
}
