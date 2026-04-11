import 'package:antinvestor_api_notification/antinvestor_api_notification.dart';
import 'package:antinvestor_ui_core/widgets/entity_list_page.dart';
import 'package:antinvestor_ui_core/widgets/error_helpers.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../providers/notification_providers.dart';
import '../widgets/notification_tile.dart';

/// Screen displaying the notification inbox using EntityListPage.
///
/// Supports search, filtering, and navigating to notification details.
class NotificationInboxScreen extends ConsumerStatefulWidget {
  const NotificationInboxScreen({super.key});

  @override
  ConsumerState<NotificationInboxScreen> createState() =>
      _NotificationInboxScreenState();
}

class _NotificationInboxScreenState
    extends ConsumerState<NotificationInboxScreen> {
  String _searchQuery = '';
  String _typeFilter = '';

  NotificationSearchParams get _searchParams => NotificationSearchParams(
        query: _searchQuery,
        type: _typeFilter,
      );

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final asyncNotifications =
        ref.watch(notificationSearchProvider(_searchParams));

    return asyncNotifications.when(
      loading: () => _buildShell(theme, isLoading: true, items: const []),
      error: (error, _) => _buildShell(
        theme,
        error: friendlyError(error),
        items: const [],
      ),
      data: (notifications) => _buildShell(theme, items: notifications),
    );
  }

  Widget _buildShell(
    ThemeData theme, {
    required List<Notification> items,
    bool isLoading = false,
    String? error,
  }) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        // Type filter chips
        Padding(
          padding: const EdgeInsets.fromLTRB(24, 16, 24, 0),
          child: SingleChildScrollView(
            scrollDirection: Axis.horizontal,
            child: Row(
              children: [
                _filterChip(theme, '', 'All'),
                const SizedBox(width: 8),
                _filterChip(theme, 'SMS', 'SMS'),
                const SizedBox(width: 8),
                _filterChip(theme, 'EMAIL', 'Email'),
                const SizedBox(width: 8),
                _filterChip(theme, 'PUSH', 'Push'),
                const SizedBox(width: 8),
                _filterChip(theme, 'WHATSAPP', 'WhatsApp'),
              ],
            ),
          ),
        ),

        // Main list
        Expanded(
          child: EntityListPage<Notification>(
            title: 'Notifications',
            icon: Icons.notifications,
            items: items,
            isLoading: isLoading,
            error: error,
            onRetry: _refresh,
            searchHint: 'Search notifications...',
            onSearchChanged: (query) {
              setState(() => _searchQuery = query.trim());
            },
            actionLabel: 'Compose',
            onAction: () => context.go('/notifications/send'),
            itemBuilder: (context, notification) {
              return NotificationTile(
                notification: notification,
                isUnread: notification.status.state.name == 'PENDING' ||
                    notification.status.state.name == 'QUEUED',
                onTap: () {
                  context.go(
                    '/notifications/detail/${notification.id}',
                    extra: notification,
                  );
                },
              );
            },
          ),
        ),
      ],
    );
  }

  Widget _filterChip(ThemeData theme, String value, String label) {
    final isSelected = _typeFilter == value;
    return FilterChip(
      selected: isSelected,
      label: Text(label),
      selectedColor: theme.colorScheme.secondaryContainer,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(8),
      ),
      onSelected: (_) {
        setState(() => _typeFilter = value);
      },
    );
  }

  void _refresh() {
    ref.invalidate(notificationSearchProvider(_searchParams));
  }
}
