import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../providers/notification_providers.dart';
import '../widgets/notification_status_badge.dart';
import '../widgets/priority_badge.dart';

/// Screen displaying the full details of a single notification.
class NotificationDetailScreen extends ConsumerStatefulWidget {
  const NotificationDetailScreen({
    super.key,
    required this.notificationId,
    this.initialNotification,
  });

  final String notificationId;
  final notif.Notification? initialNotification;

  @override
  ConsumerState<NotificationDetailScreen> createState() =>
      _NotificationDetailScreenState();
}

class _NotificationDetailScreenState
    extends ConsumerState<NotificationDetailScreen> {
  bool _releasing = false;
  String? _error;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final initial = widget.initialNotification;
    if (initial != null) {
      return _buildScreen(theme, initial);
    }

    // Deep-link path: fetch by ID.
    final asyncN = ref.watch(notificationByIdProvider(widget.notificationId));
    return asyncN.when(
      loading: () =>
          const Scaffold(body: Center(child: CircularProgressIndicator())),
      error: (e, _) => _buildNotFound(theme),
      data: (n) => n == null ? _buildNotFound(theme) : _buildScreen(theme, n),
    );
  }

  Widget _buildScreen(ThemeData theme, notif.Notification notification) {
    return Scaffold(
      appBar: AppBar(
        leading: IconButton(
          icon: const Icon(Icons.arrow_back),
          onPressed: () => context.canPop()
              ? context.pop()
              : context.go('/notifications'),
        ),
        title: Text(
          notification.template.isNotEmpty
              ? notification.template
              : 'Notification',
          style: theme.textTheme.titleMedium?.copyWith(
            fontWeight: FontWeight.w600,
          ),
        ),
        actions: [
          FilledButton.icon(
            key: const Key('detail-retry-button'),
            onPressed: _releasing ? null : () => _release(notification),
            icon: _releasing
                ? const SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : const Icon(Icons.refresh, size: 18),
            label: Text(_releasing ? 'Retrying...' : 'Retry / Release'),
          ),
          const SizedBox(width: 8),
        ],
      ),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(24),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Status and priority row
            Row(
              children: [
                NotificationStatusBadge(status: notification.status.state.name),
                const SizedBox(width: 8),
                PriorityBadge(priority: notification.priority),
                const Spacer(),
                if (notification.outBound)
                  Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 8,
                      vertical: 4,
                    ),
                    decoration: BoxDecoration(
                      color: theme.colorScheme.tertiaryContainer,
                      borderRadius: BorderRadius.circular(6),
                    ),
                    child: Text(
                      'OUTBOUND',
                      style: theme.textTheme.labelSmall?.copyWith(
                        fontWeight: FontWeight.w600,
                        color: theme.colorScheme.onTertiaryContainer,
                      ),
                    ),
                  ),
              ],
            ),
            const SizedBox(height: 24),

            // Lifecycle card
            _buildLifecycleCard(theme, notification),
            const SizedBox(height: 16),

            // Metadata card
            _buildMetadataCard(theme, notification),
            const SizedBox(height: 16),

            // Routing card
            _buildRoutingCard(theme, notification),
            const SizedBox(height: 16),

            // Payload card
            if (notification.hasPayload())
              _buildPayloadCard(theme, notification),

            // Data card
            if (notification.data.isNotEmpty) ...[
              const SizedBox(height: 16),
              _buildDataCard(theme, notification),
            ],

            // Extras card
            if (notification.hasExtras()) ...[
              const SizedBox(height: 16),
              _buildExtrasCard(theme, notification),
            ],

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
    );
  }

  Widget _buildMetadataCard(ThemeData theme, notif.Notification notification) {
    return Card(
      elevation: 0,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: BorderSide(color: theme.colorScheme.outlineVariant),
      ),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              'Details',
              style: theme.textTheme.titleSmall?.copyWith(
                fontWeight: FontWeight.w600,
                color: theme.colorScheme.primary,
              ),
            ),
            const SizedBox(height: 12),
            MetadataRow(label: 'ID', value: notification.id, copiable: true),
            if (notification.parentId.isNotEmpty)
              MetadataRow(
                label: 'Parent ID',
                value: notification.parentId,
                copiable: true,
              ),
            MetadataRow(label: 'Type', value: notification.type),
            MetadataRow(label: 'Template', value: notification.template),
            MetadataRow(label: 'Language', value: notification.language),
            MetadataRow(
              label: 'Auto Release',
              value: notification.autoRelease ? 'Yes' : 'No',
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildRoutingCard(ThemeData theme, notif.Notification notification) {
    return Card(
      elevation: 0,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: BorderSide(color: theme.colorScheme.outlineVariant),
      ),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              'Routing',
              style: theme.textTheme.titleSmall?.copyWith(
                fontWeight: FontWeight.w600,
                color: theme.colorScheme.primary,
              ),
            ),
            const SizedBox(height: 12),
            MetadataRow(label: 'Source', value: notification.source.detail),
            MetadataRow(
              label: 'Recipient',
              value: notification.recipient.detail,
            ),
            if (notification.routeId.isNotEmpty)
              MetadataRow(label: 'Route ID', value: notification.routeId),
          ],
        ),
      ),
    );
  }

  Widget _buildPayloadCard(ThemeData theme, notif.Notification notification) {
    return Card(
      elevation: 0,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: BorderSide(color: theme.colorScheme.outlineVariant),
      ),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              'Payload',
              style: theme.textTheme.titleSmall?.copyWith(
                fontWeight: FontWeight.w600,
                color: theme.colorScheme.primary,
              ),
            ),
            const SizedBox(height: 12),
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: theme.colorScheme.surfaceContainerLow,
                borderRadius: BorderRadius.circular(8),
              ),
              child: SelectableText(
                notification.payload.toString(),
                style: theme.textTheme.bodySmall?.copyWith(
                  fontFamily: 'monospace',
                  fontSize: 12,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildDataCard(ThemeData theme, notif.Notification notification) {
    return Card(
      elevation: 0,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: BorderSide(color: theme.colorScheme.outlineVariant),
      ),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              'Data',
              style: theme.textTheme.titleSmall?.copyWith(
                fontWeight: FontWeight.w600,
                color: theme.colorScheme.primary,
              ),
            ),
            const SizedBox(height: 12),
            SelectableText(
              notification.data,
              style: theme.textTheme.bodySmall?.copyWith(
                fontFamily: 'monospace',
                fontSize: 12,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildExtrasCard(ThemeData theme, notif.Notification notification) {
    return Card(
      elevation: 0,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: BorderSide(color: theme.colorScheme.outlineVariant),
      ),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              'Extras',
              style: theme.textTheme.titleSmall?.copyWith(
                fontWeight: FontWeight.w600,
                color: theme.colorScheme.primary,
              ),
            ),
            const SizedBox(height: 12),
            for (final entry in notification.extras.fields.entries)
              MetadataRow(label: entry.key, value: entry.value.toString()),
          ],
        ),
      ),
    );
  }

  Widget _buildLifecycleCard(ThemeData theme, notif.Notification n) {
    final state = n.status.state.name;
    final (action, color, icon) = switch (state) {
      'ACTIVE' => ('Delivered', Colors.green, Icons.check_circle_outline),
      'INACTIVE' => ('Failed', Colors.red, Icons.error_outline),
      'CREATED' || 'CHECKED' => ('Queued', Colors.blue, Icons.schedule),
      _ => ('Status: $state', Colors.grey, Icons.info_outline),
    };
    return Card(
      elevation: 0,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: BorderSide(color: theme.colorScheme.outlineVariant),
      ),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              'Lifecycle',
              style: theme.textTheme.titleSmall?.copyWith(
                fontWeight: FontWeight.w600,
                color: theme.colorScheme.primary,
              ),
            ),
            const SizedBox(height: 12),
            AuditTrailEntry(
              action: action,
              timestamp: n.status.id,
              performedBy: 'system',
              icon: icon,
              color: color,
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _release(notif.Notification notification) async {
    setState(() {
      _releasing = true;
      _error = null;
    });

    try {
      final notifier = ref.read(notificationNotifierProvider.notifier);
      final request = notif.ReleaseRequest()..id.add(notification.id);
      await notifier.release(request);

      if (mounted) {
        setState(() => _releasing = false);
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Notification released successfully'),
            behavior: SnackBarBehavior.floating,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _releasing = false;
          _error = friendlyError(e);
        });
      }
    }
  }

  Widget _buildNotFound(ThemeData theme) {
    return Scaffold(
      appBar: AppBar(
        leading: IconButton(
          icon: const Icon(Icons.arrow_back),
          onPressed: () => context.canPop()
              ? context.pop()
              : context.go('/notifications'),
        ),
        title: const Text('Notification Not Found'),
      ),
      body: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              Icons.search_off,
              size: 48,
              color: theme.colorScheme.onSurfaceVariant,
            ),
            const SizedBox(height: 16),
            Text(
              'Notification "${widget.notificationId}" was not found.',
              style: theme.textTheme.titleMedium?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
            const SizedBox(height: 12),
            FilledButton.tonal(
              onPressed: () => context.go('/notifications'),
              child: const Text('Back to Inbox'),
            ),
          ],
        ),
      ),
    );
  }
}
