import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:flutter/material.dart';

/// Thin wrapper around `StatusBadge.fromEnum` that maps notification status
/// strings to a label/color/icon. Centralizes the mapping in one place so
/// tiles, lists, and detail screens all render badges identically.
class NotificationStatusBadge extends StatelessWidget {
  const NotificationStatusBadge({super.key, required this.status});

  final String status;

  @override
  Widget build(BuildContext context) {
    return StatusBadge.fromEnum<String>(
      value: status,
      mapper: (s) => switch (s) {
        'ACTIVE' || 'DELIVERED' => ('Active', Colors.green, null),
        'CREATED' || 'QUEUED' => ('Queued', Colors.blue, Icons.schedule),
        'CHECKED' => ('Checked', Colors.orange, null),
        'INACTIVE' || 'FAILED' =>
          ('Failed', Colors.red, Icons.error_outline),
        'DELETED' => ('Deleted', Colors.grey, null),
        '' => ('Unknown', Colors.grey, null),
        _ => (s, Colors.grey, null),
      },
    );
  }
}
