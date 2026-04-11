import 'package:antinvestor_api_notification/antinvestor_api_notification.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../providers/notification_providers.dart';

/// Counts unread notifications for a recipient.
///
/// Uses [notificationReceiveProvider] to fetch notifications and counts those
/// whose status indicates they are unread/pending.
final unreadCountProvider =
    FutureProvider.family<int, String>((ref, recipientId) async {
  if (recipientId.isEmpty) return 0;
  final notifications =
      await ref.watch(notificationReceiveProvider(recipientId).future);
  return notifications
      .where((n) =>
          n.status == NotificationStatus.NOTIFICATION_STATUS_PENDING ||
          n.status == NotificationStatus.NOTIFICATION_STATUS_SENT)
      .length;
});

/// A badge that displays the unread notification count for a given recipient.
///
/// Renders a small red badge over a [child] (typically a bell icon).
/// Hides when there are zero unread notifications.
///
/// ```dart
/// NotificationBadge(
///   recipientId: currentUserId,
///   child: Icon(Icons.notifications_outlined),
/// )
/// ```
class NotificationBadge extends ConsumerWidget {
  const NotificationBadge({
    super.key,
    required this.recipientId,
    this.child,
    this.offset = const Offset(8, -4),
    this.backgroundColor,
    this.textColor,
  });

  final String recipientId;

  /// Widget to overlay the badge on. Defaults to a notifications icon.
  final Widget? child;

  /// Offset of the badge relative to the child.
  final Offset offset;

  final Color? backgroundColor;
  final Color? textColor;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final countAsync = ref.watch(unreadCountProvider(recipientId));

    final count = countAsync.when(
      data: (c) => c,
      loading: () => 0,
      error: (_, __) => 0,
    );

    final icon = child ?? const Icon(Icons.notifications_outlined);

    if (count <= 0) return icon;

    final bgColor = backgroundColor ?? theme.colorScheme.error;
    final fgColor = textColor ?? theme.colorScheme.onError;
    final label = count > 99 ? '99+' : '$count';

    return Stack(
      clipBehavior: Clip.none,
      children: [
        icon,
        Positioned(
          right: offset.dx * -1,
          top: offset.dy,
          child: Container(
            padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 1),
            constraints: const BoxConstraints(minWidth: 18, minHeight: 18),
            decoration: BoxDecoration(
              color: bgColor,
              borderRadius: BorderRadius.circular(10),
            ),
            alignment: Alignment.center,
            child: Text(
              label,
              style: theme.textTheme.labelSmall?.copyWith(
                color: fgColor,
                fontWeight: FontWeight.w700,
                fontSize: 10,
                height: 1,
              ),
            ),
          ),
        ),
      ],
    );
  }
}

/// An inline list of recent notifications for a recipient.
///
/// Shows the last [maxItems] notifications as compact tiles. Useful for
/// embedding in dashboards or sidebars.
///
/// ```dart
/// InlineNotificationList(recipientId: userId, maxItems: 5)
/// ```
class InlineNotificationList extends ConsumerWidget {
  const InlineNotificationList({
    super.key,
    required this.recipientId,
    this.maxItems = 5,
    this.onTap,
    this.emptyMessage = 'No notifications',
  });

  final String recipientId;
  final int maxItems;
  final ValueChanged<Notification>? onTap;
  final String emptyMessage;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final notificationsAsync =
        ref.watch(notificationReceiveProvider(recipientId));

    return notificationsAsync.when(
      loading: () => const Center(
        child: Padding(
          padding: EdgeInsets.all(16),
          child: CircularProgressIndicator(strokeWidth: 2),
        ),
      ),
      error: (e, _) => Padding(
        padding: const EdgeInsets.all(16),
        child: Text(
          'Failed to load notifications',
          style: TextStyle(color: theme.colorScheme.error),
        ),
      ),
      data: (notifications) {
        if (notifications.isEmpty) {
          return Padding(
            padding: const EdgeInsets.all(16),
            child: Text(
              emptyMessage,
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          );
        }

        final items = notifications.take(maxItems).toList();
        return Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            for (final notification in items)
              _CompactNotificationTile(
                notification: notification,
                onTap: onTap != null ? () => onTap!(notification) : null,
              ),
          ],
        );
      },
    );
  }
}

class _CompactNotificationTile extends StatelessWidget {
  const _CompactNotificationTile({
    required this.notification,
    this.onTap,
  });

  final Notification notification;
  final VoidCallback? onTap;

  IconData _typeIcon(String type) {
    final lower = type.toLowerCase();
    if (lower.contains('sms')) return Icons.sms_outlined;
    if (lower.contains('email') || lower.contains('mail')) {
      return Icons.email_outlined;
    }
    if (lower.contains('push')) return Icons.notifications_outlined;
    return Icons.campaign_outlined;
  }

  String _title(Notification n) {
    if (n.template.isNotEmpty) return n.template;
    if (n.type.isNotEmpty) return n.type;
    return 'Notification';
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isUnread =
        notification.status == NotificationStatus.NOTIFICATION_STATUS_PENDING ||
        notification.status == NotificationStatus.NOTIFICATION_STATUS_SENT;

    return InkWell(
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
        child: Row(
          children: [
            if (isUnread)
              Container(
                width: 6,
                height: 6,
                margin: const EdgeInsets.only(right: 8),
                decoration: BoxDecoration(
                  color: theme.colorScheme.primary,
                  shape: BoxShape.circle,
                ),
              )
            else
              const SizedBox(width: 14),
            Icon(
              _typeIcon(notification.type),
              size: 16,
              color: theme.colorScheme.onSurfaceVariant,
            ),
            const SizedBox(width: 8),
            Expanded(
              child: Text(
                _title(notification),
                style: theme.textTheme.bodySmall?.copyWith(
                  fontWeight: isUnread ? FontWeight.w600 : FontWeight.w400,
                ),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
