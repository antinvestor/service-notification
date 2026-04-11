import 'package:flutter/material.dart';

/// A multi-select chip widget for choosing notification channels
/// (e.g., SMS, Email, Push, WhatsApp).
class ChannelSelector extends StatelessWidget {
  const ChannelSelector({
    super.key,
    required this.selectedChannels,
    required this.onChanged,
    this.availableChannels = _defaultChannels,
  });

  final Set<String> selectedChannels;
  final ValueChanged<Set<String>> onChanged;
  final List<String> availableChannels;

  static const _defaultChannels = [
    'SMS',
    'EMAIL',
    'PUSH',
    'WHATSAPP',
    'VOICE',
  ];

  IconData _channelIcon(String channel) {
    return switch (channel.toUpperCase()) {
      'SMS' => Icons.sms_outlined,
      'EMAIL' => Icons.email_outlined,
      'PUSH' => Icons.notifications_outlined,
      'WHATSAPP' => Icons.chat_outlined,
      'VOICE' => Icons.phone_outlined,
      _ => Icons.send_outlined,
    };
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Wrap(
      spacing: 8,
      runSpacing: 8,
      children: availableChannels.map((channel) {
        final isSelected = selectedChannels.contains(channel);
        return FilterChip(
          selected: isSelected,
          label: Text(channel),
          avatar: Icon(
            _channelIcon(channel),
            size: 18,
            color: isSelected
                ? theme.colorScheme.onSecondaryContainer
                : theme.colorScheme.onSurfaceVariant,
          ),
          selectedColor: theme.colorScheme.secondaryContainer,
          checkmarkColor: theme.colorScheme.onSecondaryContainer,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(8),
          ),
          onSelected: (selected) {
            final updated = Set<String>.from(selectedChannels);
            if (selected) {
              updated.add(channel);
            } else {
              updated.remove(channel);
            }
            onChanged(updated);
          },
        );
      }).toList(),
    );
  }
}
