import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:flutter/material.dart';
import 'package:protobuf/protobuf.dart';

/// Editable channel × language matrix for a Template's variants.
///
/// The host owns the variants list (controlled component). Each cell click
/// opens an editor pane on desktop or a bottom sheet on compact widths.
/// On save, the widget rebuilds the variants list and emits via `onChanged`.
class TemplateVariantMatrix extends StatefulWidget {
  const TemplateVariantMatrix({
    super.key,
    required this.variants,
    required this.onChanged,
    this.availableLanguages = const ['en'],
    this.availableChannels = const ['SMS', 'EMAIL', 'PUSH', 'WHATSAPP'],
  });

  final List<notif.TemplateData> variants;
  final ValueChanged<List<notif.TemplateData>> onChanged;
  final List<String> availableLanguages;
  final List<String> availableChannels;

  @override
  State<TemplateVariantMatrix> createState() => _TemplateVariantMatrixState();
}

class _TemplateVariantMatrixState extends State<TemplateVariantMatrix> {
  ({String channel, String language})? _editing;
  TextEditingController? _editingController;

  notif.TemplateData? _findVariant(String channel, String language) {
    for (final v in widget.variants) {
      if (v.type == channel && v.language.code == language) return v;
    }
    return null;
  }

  void _openCell(String channel, String language) {
    final variant = _findVariant(channel, language);
    _editingController?.dispose();
    _editingController = TextEditingController(text: variant?.detail ?? '');
    setState(() => _editing = (channel: channel, language: language));
  }

  void _closeEditor() {
    _editingController?.dispose();
    _editingController = null;
    setState(() => _editing = null);
  }

  @override
  void dispose() {
    _editingController?.dispose();
    super.dispose();
  }

  void _saveCell({
    required String channel,
    required String language,
    required String detail,
  }) {
    final next = List<notif.TemplateData>.of(widget.variants);
    final i = next.indexWhere(
        (v) => v.type == channel && v.language.code == language);
    if (i >= 0) {
      next[i] = next[i].deepCopy()..detail = detail;
    } else {
      next.add(notif.TemplateData()
        ..type = channel
        ..detail = detail
        ..language = (notif.Language()..code = language));
    }
    widget.onChanged(next);
    _closeEditor();
  }

  @override
  Widget build(BuildContext context) {
    final width = MediaQuery.sizeOf(context).width;
    final compact = !AppBreakpoints.isDesktop(width);

    final grid = _buildGrid(context);
    if (compact) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          grid,
          if (_editing != null) _buildEditor(context, sheet: true),
        ],
      );
    }
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Expanded(flex: 3, child: grid),
        if (_editing != null) ...[
          const SizedBox(width: 16),
          Expanded(flex: 2, child: _buildEditor(context)),
        ],
      ],
    );
  }

  Widget _buildGrid(BuildContext context) {
    final theme = Theme.of(context);
    return Card(
      elevation: 0,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: BorderSide(color: theme.colorScheme.outlineVariant),
      ),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: SingleChildScrollView(
          scrollDirection: Axis.horizontal,
          child: DataTable(
            headingRowHeight: 36,
            columns: [
              const DataColumn(label: Text('Channel \\ Lang')),
              for (final lang in widget.availableLanguages)
                DataColumn(label: Text(lang)),
            ],
            rows: [
              for (final channel in widget.availableChannels)
                DataRow(cells: [
                  DataCell(Text(channel)),
                  for (final lang in widget.availableLanguages)
                    DataCell(_cellChip(channel, lang)),
                ]),
            ],
          ),
        ),
      ),
    );
  }

  Widget _cellChip(String channel, String language) {
    final theme = Theme.of(context);
    final variant = _findVariant(channel, language);
    final filled = variant != null && variant.detail.isNotEmpty;
    return InkWell(
      key: Key('cell-$channel-$language'),
      onTap: () => _openCell(channel, language),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
        decoration: BoxDecoration(
          color: filled
              ? theme.colorScheme.primaryContainer
              : theme.colorScheme.surfaceContainerLow,
          borderRadius: BorderRadius.circular(6),
        ),
        child: Icon(
          filled ? Icons.check_circle : Icons.add_circle_outline,
          size: 16,
          color: filled
              ? theme.colorScheme.onPrimaryContainer
              : theme.colorScheme.onSurfaceVariant,
        ),
      ),
    );
  }

  Widget _buildEditor(BuildContext context, {bool sheet = false}) {
    final cell = _editing!;
    final theme = Theme.of(context);
    return FormFieldCard(
      label: '${cell.channel} · ${cell.language}',
      description: 'Edit the content for this channel and language.',
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          TextField(
            key: const Key('cell-editor-content'),
            controller: _editingController!,
            minLines: 3,
            maxLines: 8,
            decoration: InputDecoration(
              border: OutlineInputBorder(
                borderRadius: BorderRadius.circular(12),
              ),
              hintText: 'Hello {{name}}',
            ),
          ),
          const SizedBox(height: 8),
          Row(
            mainAxisAlignment: MainAxisAlignment.end,
            children: [
              TextButton(
                onPressed: _closeEditor,
                child: const Text('Cancel'),
              ),
              const SizedBox(width: 8),
              FilledButton(
                key: const Key('cell-editor-save'),
                onPressed: () => _saveCell(
                  channel: cell.channel,
                  language: cell.language,
                  detail: _editingController!.text,
                ),
                child: const Text('Save'),
              ),
            ],
          ),
          if (sheet)
            Padding(
              padding: const EdgeInsets.only(top: 8),
              child: Text(
                'Tip: on wider screens this opens to the right of the matrix.',
                style: theme.textTheme.bodySmall?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                ),
              ),
            ),
        ],
      ),
    );
  }
}
