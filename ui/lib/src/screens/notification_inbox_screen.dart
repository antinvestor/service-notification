import 'package:antinvestor_api_notification/antinvestor_api_notification.dart';
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../providers/notification_providers.dart';
import '../widgets/notification_status_badge.dart';
import '../widgets/priority_badge.dart';

/// Screen displaying the notification inbox using AdminEntityListPage with
/// DataTable, CSV export, and audit callback.
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
        const SizedBox(height: 8),
        // Main list
        Expanded(
          child: asyncNotifications.when(
            loading: () => const Center(child: CircularProgressIndicator()),
            error: (error, _) => Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(Icons.error_outline,
                      size: 48, color: theme.colorScheme.error),
                  const SizedBox(height: 16),
                  Text('$error', style: theme.textTheme.bodyLarge),
                  const SizedBox(height: 16),
                  FilledButton.tonal(
                    onPressed: _refresh,
                    child: const Text('Retry'),
                  ),
                ],
              ),
            ),
            data: (notifications) => AdminEntityListPage<Notification>(
              title: 'Notifications',
              breadcrumbs: const ['Home', 'Notifications'],
              columns: const [
                DataColumn(label: Text('Type')),
                DataColumn(label: Text('Template')),
                DataColumn(label: Text('Source')),
                DataColumn(label: Text('Recipient')),
                DataColumn(label: Text('Priority')),
                DataColumn(label: Text('Status')),
                DataColumn(label: Text('Date')),
              ],
              items: notifications,
              onSearch: (query) {
                setState(() => _searchQuery = query.trim());
              },
              searchHint: 'Search notifications...',
              onAdd: () => context.go('/notifications/send'),
              addLabel: 'Compose',
              onRowNavigate: (notification) {
                context.go(
                  '/notifications/detail/${notification.id}',
                  extra: notification,
                );
              },
              rowBuilder: (notification, selected, onSelect) {
                return DataRow(
                  selected: selected,
                  onSelectChanged: (_) => onSelect(),
                  cells: [
                    DataCell(Text(notification.type)),
                    DataCell(Text(notification.template)),
                    DataCell(Text(notification.source.detail)),
                    DataCell(Text(notification.recipient.detail)),
                    DataCell(PriorityBadge(priority: notification.priority)),
                    DataCell(NotificationStatusBadge(
                      status: notification.status.state.name,
                    )),
                    DataCell(Text(notification.status.id)),
                  ],
                );
              },
              exportRow: (notification) => [
                notification.type,
                notification.template,
                notification.source.detail,
                notification.recipient.detail,
                notification.priority.name,
                notification.status.state.name,
                notification.status.id,
              ],
              onExport: (format, count) {
                debugPrint(
                    '[AUDIT] Exported $count Notifications as $format');
              },
            ),
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
