import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_core/widgets/error_helpers.dart';
import 'package:antinvestor_ui_core/widgets/form_field_card.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../providers/language_providers.dart';

class LanguageEditScreen extends ConsumerStatefulWidget {
  const LanguageEditScreen({
    super.key,
    this.languageCode,
    this.initialLanguage,
  });

  final String? languageCode;
  final notif.Language? initialLanguage;

  @override
  ConsumerState<LanguageEditScreen> createState() =>
      _LanguageEditScreenState();
}

class _LanguageEditScreenState extends ConsumerState<LanguageEditScreen> {
  final _formKey = GlobalKey<FormState>();
  late final TextEditingController _codeController;
  late final TextEditingController _nameController;
  bool _saving = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _codeController = TextEditingController(
        text: widget.initialLanguage?.code ?? widget.languageCode ?? '');
    _nameController =
        TextEditingController(text: widget.initialLanguage?.name ?? '');
  }

  @override
  void dispose() {
    _codeController.dispose();
    _nameController.dispose();
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
              : context.go('/notifications/languages'),
        ),
        title: Text(
          widget.languageCode == null ? 'New Language' : 'Edit Language',
          style: theme.textTheme.titleMedium
              ?.copyWith(fontWeight: FontWeight.w600),
        ),
        actions: [
          FilledButton.icon(
            key: const Key('lang-save-button'),
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
                label: 'Code',
                description: 'ISO 639-1 code (e.g., en, sw, fr).',
                isRequired: true,
                child: TextFormField(
                  key: const Key('lang-code-field'),
                  controller: _codeController,
                  decoration: InputDecoration(
                    hintText: 'en',
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(12),
                    ),
                  ),
                  validator: (v) =>
                      (v == null || v.trim().isEmpty) ? 'Required' : null,
                ),
              ),
              FormFieldCard(
                label: 'Name',
                description: 'Human-readable language name.',
                isRequired: true,
                child: TextFormField(
                  key: const Key('lang-name-field'),
                  controller: _nameController,
                  decoration: InputDecoration(
                    hintText: 'English',
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(12),
                    ),
                  ),
                  validator: (v) =>
                      (v == null || v.trim().isEmpty) ? 'Required' : null,
                ),
              ),
              if (_error != null) ...[
                const SizedBox(height: 16),
                Text(
                  _error!,
                  style: theme.textTheme.bodySmall
                      ?.copyWith(color: theme.colorScheme.error),
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
      await ref.read(languageNotifierProvider.notifier).save(
            code: _codeController.text.trim(),
            name: _nameController.text.trim(),
          );
      if (mounted) {
        setState(() => _saving = false);
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Language saved'),
            behavior: SnackBarBehavior.floating,
          ),
        );
        context.go('/notifications/languages');
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
