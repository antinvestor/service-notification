import 'package:antinvestor_ui_core/widgets/status_badge.dart';
import 'package:flutter/material.dart';

/// A badge that displays the current status of a notification.
///
/// Maps common notification status strings to appropriate colors:
/// - SENT / DELIVERED: green
/// - PENDING / QUEUED: orange
/// - FAILED / BOUNCED: red
/// - DRAFT: grey
class NotificationStatusBadge extends StatelessWidget {
  const NotificationStatusBadge({
    super.key,
    required this.status,
  });

  final String status;

  @override
  Widget build(BuildContext context) {
    final upper = status.toUpperCase();
    final (Color color, IconData? icon) = switch (upper) {
      'SENT' || 'DELIVERED' => (Colors.green, Icons.check_circle_outline),
      'PENDING' || 'QUEUED' => (Colors.orange, Icons.schedule),
      'FAILED' || 'BOUNCED' => (Colors.red, Icons.error_outline),
      'RELEASED' => (Colors.teal, Icons.send),
      'DRAFT' => (Colors.grey, Icons.edit_outlined),
      _ => (Colors.blueGrey, null),
    };

    return StatusBadge(
      label: upper,
      color: color,
      icon: icon,
    );
  }
}
