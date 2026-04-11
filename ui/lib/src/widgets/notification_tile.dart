import 'package:antinvestor_api_notification/antinvestor_api_notification.dart';
import 'package:flutter/material.dart';

import 'notification_status_badge.dart';
import 'priority_badge.dart';

/// An inbox tile for displaying a single notification.
///
/// Shows a type icon, title (from template or type), timestamp,
/// unread indicator dot, and priority badge.
class NotificationTile extends StatelessWidget {
  const NotificationTile({
    super.key,
    required this.notification,
    this.onTap,
    this.isUnread = false,
  });

  final Notification notification;
  final VoidCallback? onTap;
  final bool isUnread;

  IconData _typeIcon(String type) {
    final lower = type.toLowerCase();
    if (lower.contains('sms')) return Icons.sms_outlined;
    if (lower.contains('email') || lower.contains('mail')) {
      return Icons.email_outlined;
    }
    if (lower.contains('push')) return Icons.notifications_outlined;
    if (lower.contains('whatsapp') || lower.contains('wa')) {
      return Icons.chat_outlined;
    }
    if (lower.contains('voice') || lower.contains('call')) {
      return Icons.phone_outlined;
    }
    return Icons.campaign_outlined;
  }

  String _formatTimestamp(Notification n) {
    if (!n.hasCreated()) return '';
    final ts = n.created.toDateTime();
    final now = DateTime.now();
    final diff = now.difference(ts);
    if (diff.inMinutes < 1) return 'just now';
    if (diff.inHours < 1) return '${diff.inMinutes}m ago';
    if (diff.inDays < 1) return '${diff.inHours}h ago';
    if (diff.inDays < 30) return '${diff.inDays}d ago';
    return '${ts.year}-${ts.month.toString().padLeft(2, '0')}-'
        '${ts.day.toString().padLeft(2, '0')}';
  }

  String _title(Notification n) {
    if (n.template.isNotEmpty) return n.template;
    if (n.type.isNotEmpty) return n.type;
    return 'Notification';
  }

  String _subtitle(Notification n) {
    if (n.hasRecipient() && n.recipient.detail.isNotEmpty) {
      return 'To: ${n.recipient.detail}';
    }
    if (n.hasSource() && n.source.detail.isNotEmpty) {
      return 'From: ${n.source.detail}';
    }
    return '';
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final timestamp = _formatTimestamp(notification);

    return Card(
      elevation: 0,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: BorderSide(color: theme.colorScheme.outlineVariant),
      ),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(12),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          child: Row(
            children: [
              // Unread dot
              if (isUnread)
                Container(
                  width: 8,
                  height: 8,
                  margin: const EdgeInsets.only(right: 8),
                  decoration: BoxDecoration(
                    color: theme.colorScheme.primary,
                    shape: BoxShape.circle,
                  ),
                ),

              // Type icon
              Container(
                width: 40,
                height: 40,
                decoration: BoxDecoration(
                  color: theme.colorScheme.primaryContainer,
                  borderRadius: BorderRadius.circular(10),
                ),
                child: Icon(
                  _typeIcon(notification.type),
                  size: 20,
                  color: theme.colorScheme.onPrimaryContainer,
                ),
              ),
              const SizedBox(width: 12),

              // Content
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      _title(notification),
                      style: theme.textTheme.titleSmall?.copyWith(
                        fontWeight:
                            isUnread ? FontWeight.w700 : FontWeight.w600,
                      ),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    const SizedBox(height: 2),
                    Text(
                      _subtitle(notification),
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: theme.colorScheme.onSurfaceVariant,
                      ),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ],
                ),
              ),

              // Priority + status + timestamp
              Column(
                crossAxisAlignment: CrossAxisAlignment.end,
                children: [
                  Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      PriorityBadge(priority: notification.priority),
                      const SizedBox(width: 4),
                      NotificationStatusBadge(status: notification.status.state.name),
                    ],
                  ),
                  if (timestamp.isNotEmpty) ...[
                    const SizedBox(height: 4),
                    Text(
                      timestamp,
                      style: theme.textTheme.labelSmall?.copyWith(
                        color: theme.colorScheme.onSurfaceVariant,
                      ),
                    ),
                  ],
                ],
              ),
              const SizedBox(width: 4),
              Icon(
                Icons.chevron_right,
                size: 20,
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ],
          ),
        ),
      ),
    );
  }
}
