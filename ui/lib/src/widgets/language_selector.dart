import 'package:flutter/material.dart';

/// A dropdown selector for choosing a notification language.
class LanguageSelector extends StatelessWidget {
  const LanguageSelector({
    super.key,
    required this.selectedLanguage,
    required this.onChanged,
    this.availableLanguages = _defaultLanguages,
  });

  final String selectedLanguage;
  final ValueChanged<String> onChanged;
  final List<LanguageOption> availableLanguages;

  static const _defaultLanguages = [
    LanguageOption(code: 'en', label: 'English'),
    LanguageOption(code: 'sw', label: 'Swahili'),
    LanguageOption(code: 'fr', label: 'French'),
    LanguageOption(code: 'pt', label: 'Portuguese'),
    LanguageOption(code: 'ar', label: 'Arabic'),
  ];

  @override
  Widget build(BuildContext context) {
    return DropdownButtonFormField<String>(
      value: selectedLanguage.isEmpty
          ? availableLanguages.first.code
          : selectedLanguage,
      decoration: InputDecoration(
        labelText: 'Language',
        prefixIcon: const Icon(Icons.language, size: 20),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
        ),
      ),
      items: availableLanguages
          .map((lang) => DropdownMenuItem(
                value: lang.code,
                child: Text(lang.label),
              ))
          .toList(),
      onChanged: (value) {
        if (value != null) onChanged(value);
      },
    );
  }
}

/// Represents a selectable language option.
class LanguageOption {
  const LanguageOption({required this.code, required this.label});
  final String code;
  final String label;
}
