import 'package:antinvestor_api_notification/antinvestor_api_notification.dart';
import 'package:antinvestor_ui_core/widgets/status_badge.dart';
import 'package:flutter/material.dart';

/// A badge showing the notification priority level.
///
/// Color mapping:
/// - HIGH: red
/// - LOW: blue
/// - VERY_LOW: grey
class PriorityBadge extends StatelessWidget {
  const PriorityBadge({
    super.key,
    required this.priority,
  });

  final PRIORITY priority;

  @override
  Widget build(BuildContext context) {
    return StatusBadge.fromEnum(
      value: priority,
      mapper: (p) => switch (p) {
        PRIORITY.HIGH => ('HIGH', Colors.red, Icons.keyboard_double_arrow_up),
        PRIORITY.LOW => ('LOW', Colors.blue, Icons.keyboard_arrow_down),
        PRIORITY.VERY_LOW => ('VERY LOW', Colors.grey, Icons.arrow_downward),
        _ => ('NORMAL', Colors.blueGrey, null),
      },
    );
  }
}
