import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_core/widgets/error_helpers.dart';
import 'package:antinvestor_ui_core/widgets/form_field_card.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../providers/notification_providers.dart';
import '../widgets/channel_selector.dart';
import '../widgets/language_selector.dart';

/// Screen for composing and sending a new notification.
class NotificationSendScreen extends ConsumerStatefulWidget {
  const NotificationSendScreen({super.key});

  @override
  ConsumerState<NotificationSendScreen> createState() =>
      _NotificationSendScreenState();
}

class _NotificationSendScreenState
    extends ConsumerState<NotificationSendScreen> {
  final _formKey = GlobalKey<FormState>();
  final _recipientController = TextEditingController();
  final _sourceController = TextEditingController();
  final _payloadController = TextEditingController();
  final _templateController = TextEditingController();

  String _selectedLanguage = 'en';
  String _selectedChannel = 'SMS';
  notif.PRIORITY _selectedPriority = notif.PRIORITY.LOW;
  bool _autoRelease = true;
  bool _outBound = true;
  bool _sending = false;
  String? _error;

  // Data key-value pairs
  final List<MapEntry<String, String>> _dataEntries = [];

  @override
  void dispose() {
    _recipientController.dispose();
    _sourceController.dispose();
    _payloadController.dispose();
    _templateController.dispose();
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
              : context.go('/notifications'),
        ),
        title: Text(
          'Compose Notification',
          style: theme.textTheme.titleMedium?.copyWith(
            fontWeight: FontWeight.w600,
          ),
        ),
        actions: [
          FilledButton.icon(
            key: const Key('send-submit-button'),
            onPressed: _sending ? null : _send,
            icon: _sending
                ? const SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(
                      strokeWidth: 2,
                      color: Colors.white,
                    ),
                  )
                : const Icon(Icons.send, size: 18),
            label: Text(_sending ? 'Sending...' : 'Send'),
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
              // Recipient
              FormFieldCard(
                label: 'Recipient',
                description: 'The target address (phone number, email, etc).',
                isRequired: true,
                child: TextFormField(
                  key: const Key('send-recipient-field'),
                  controller: _recipientController,
                  decoration: InputDecoration(
                    hintText: 'e.g., +254700000000',
                    prefixIcon: const Icon(Icons.person_outline),
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(12),
                    ),
                  ),
                  validator: (v) =>
                      (v == null || v.trim().isEmpty) ? 'Required' : null,
                ),
              ),

              // Source
              FormFieldCard(
                label: 'Source',
                description: 'The sender identifier.',
                child: TextFormField(
                  controller: _sourceController,
                  decoration: InputDecoration(
                    hintText: 'e.g., MYAPP',
                    prefixIcon: const Icon(Icons.account_circle_outlined),
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(12),
                    ),
                  ),
                ),
              ),

              // Channel selector
              FormFieldCard(
                label: 'Channel',
                description: 'Select the delivery channel.',
                isRequired: true,
                child: ChannelSelector(
                  selectedChannels: {_selectedChannel},
                  onChanged: (channels) {
                    // Single-select semantics over a multi-select widget:
                    // the set still contains the previous channel, so pick
                    // the newly tapped one — `channels.first` would always
                    // return the old selection (insertion order) and the
                    // channel could never change.
                    final next = channels.firstWhere(
                      (c) => c != _selectedChannel,
                      orElse: () => _selectedChannel,
                    );
                    setState(() => _selectedChannel = next);
                  },
                ),
              ),

              // Template
              FormFieldCard(
                label: 'Template',
                description:
                    'Template name to use for rendering the notification.',
                child: TextFormField(
                  controller: _templateController,
                  decoration: InputDecoration(
                    hintText: 'e.g., welcome_message',
                    prefixIcon: const Icon(Icons.description_outlined),
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(12),
                    ),
                  ),
                ),
              ),

              // Language
              FormFieldCard(
                label: 'Language',
                description: 'Language for the notification content.',
                child: LanguageSelector(
                  selectedLanguage: _selectedLanguage,
                  onChanged: (lang) {
                    setState(() => _selectedLanguage = lang);
                  },
                ),
              ),

              // Priority
              FormFieldCard(
                label: 'Priority',
                description: 'Delivery priority level.',
                child: SegmentedButton<notif.PRIORITY>(
                  segments: [
                    ButtonSegment(
                      value: notif.PRIORITY.HIGH,
                      label: const Text('High'),
                      icon: const Icon(Icons.keyboard_double_arrow_up, size: 16),
                    ),
                    ButtonSegment(
                      value: notif.PRIORITY.LOW,
                      label: const Text('Low'),
                      icon: const Icon(Icons.keyboard_arrow_down, size: 16),
                    ),
                    ButtonSegment(
                      value: notif.PRIORITY.VERY_LOW,
                      label: const Text('Very Low'),
                      icon: const Icon(Icons.arrow_downward, size: 16),
                    ),
                  ],
                  selected: {_selectedPriority},
                  onSelectionChanged: (set) {
                    setState(() => _selectedPriority = set.first);
                  },
                  showSelectedIcon: false,
                ),
              ),

              // Payload
              FormFieldCard(
                label: 'Payload',
                description: 'Raw message content or body text.',
                child: TextFormField(
                  controller: _payloadController,
                  maxLines: 5,
                  minLines: 3,
                  decoration: InputDecoration(
                    hintText: 'Enter notification body...',
                    alignLabelWithHint: true,
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(12),
                    ),
                  ),
                ),
              ),

              // Data key-value pairs
              FormFieldCard(
                label: 'Data',
                description:
                    'Template variables as key-value pairs.',
                child: Column(
                  children: [
                    for (var i = 0; i < _dataEntries.length; i++)
                      Padding(
                        padding: const EdgeInsets.only(bottom: 8),
                        child: Row(
                          children: [
                            Expanded(
                              child: TextFormField(
                                key: Key('send-data-key-$i'),
                                initialValue: _dataEntries[i].key,
                                decoration: InputDecoration(
                                  hintText: 'Key',
                                  isDense: true,
                                  border: OutlineInputBorder(
                                    borderRadius: BorderRadius.circular(8),
                                  ),
                                ),
                                onChanged: (val) {
                                  _dataEntries[i] =
                                      MapEntry(val, _dataEntries[i].value);
                                },
                              ),
                            ),
                            const SizedBox(width: 8),
                            Expanded(
                              child: TextFormField(
                                key: Key('send-data-value-$i'),
                                initialValue: _dataEntries[i].value,
                                decoration: InputDecoration(
                                  hintText: 'Value',
                                  isDense: true,
                                  border: OutlineInputBorder(
                                    borderRadius: BorderRadius.circular(8),
                                  ),
                                ),
                                onChanged: (val) {
                                  _dataEntries[i] =
                                      MapEntry(_dataEntries[i].key, val);
                                },
                              ),
                            ),
                            IconButton(
                              icon: const Icon(Icons.remove_circle_outline,
                                  size: 20),
                              onPressed: () {
                                setState(() => _dataEntries.removeAt(i));
                              },
                            ),
                          ],
                        ),
                      ),
                    Align(
                      alignment: Alignment.centerLeft,
                      child: TextButton.icon(
                        key: const Key('send-data-add-button'),
                        onPressed: () {
                          setState(() {
                            _dataEntries.add(const MapEntry('', ''));
                          });
                        },
                        icon: const Icon(Icons.add, size: 18),
                        label: const Text('Add Entry'),
                      ),
                    ),
                  ],
                ),
              ),

              // Options row
              Row(
                children: [
                  Expanded(
                    child: CheckboxListTile(
                      title: const Text('Auto Release'),
                      value: _autoRelease,
                      onChanged: (v) {
                        setState(() => _autoRelease = v ?? true);
                      },
                      controlAffinity: ListTileControlAffinity.leading,
                      contentPadding: EdgeInsets.zero,
                    ),
                  ),
                  Expanded(
                    child: CheckboxListTile(
                      title: const Text('Outbound'),
                      value: _outBound,
                      onChanged: (v) {
                        setState(() => _outBound = v ?? true);
                      },
                      controlAffinity: ListTileControlAffinity.leading,
                      contentPadding: EdgeInsets.zero,
                    ),
                  ),
                ],
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

  Future<void> _send() async {
    if (!_formKey.currentState!.validate()) {
      // The invalid field may be scrolled out of view — without this the
      // Send button appears to do nothing.
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Please fix the highlighted fields before sending'),
          behavior: SnackBarBehavior.floating,
        ),
      );
      return;
    }

    setState(() {
      _sending = true;
      _error = null;
    });

    try {
      final notifier = ref.read(notificationNotifierProvider.notifier);

      final notification = notif.Notification()
        ..recipient = (notif.ContactLink()..detail = _recipientController.text.trim())
        ..source = (notif.ContactLink()..detail = _sourceController.text.trim())
        ..type = _selectedChannel
        ..template = _templateController.text.trim()
        ..language = _selectedLanguage
        ..priority = _selectedPriority
        ..autoRelease = _autoRelease
        ..outBound = _outBound;

      // Body: pre-rendered text goes into `data`.
      if (_payloadController.text.trim().isNotEmpty) {
        notification.data = _payloadController.text.trim();
      }

      // Template variables go into `payload` Struct.
      if (_dataEntries.isNotEmpty) {
        final payload = notif.Struct();
        for (final entry in _dataEntries) {
          if (entry.key.isNotEmpty) {
            payload.fields[entry.key] = notif.Value()..stringValue = entry.value;
          }
        }
        if (payload.fields.isNotEmpty) {
          notification.payload = payload;
        }
      }

      final request = notif.SendRequest();
      request.data.add(notification);

      await notifier.send(request);

      if (mounted) {
        setState(() => _sending = false);
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Notification sent successfully'),
            behavior: SnackBarBehavior.floating,
          ),
        );
        context.go('/notifications');
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _sending = false;
          _error = friendlyError(e);
        });
        // Mirror the inline banner with a snackbar so the failure is
        // visible even when the banner is scrolled out of view.
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(friendlyError(e)),
            behavior: SnackBarBehavior.floating,
          ),
        );
      }
    }
  }
}
