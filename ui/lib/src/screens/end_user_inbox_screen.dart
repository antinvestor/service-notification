import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../providers/notification_providers.dart';
import '../widgets/notification_tile.dart';

/// Per-profile inbox surfaced for end users (e.g., embedded in a profile
/// drawer). Triggers `Receive` ack on first paint so the backend can mark
/// these notifications as delivered/read for this user.
class EndUserInboxScreen extends ConsumerStatefulWidget {
  const EndUserInboxScreen({super.key, required this.profileId});
  final String profileId;

  @override
  ConsumerState<EndUserInboxScreen> createState() =>
      _EndUserInboxScreenState();
}

class _EndUserInboxScreenState extends ConsumerState<EndUserInboxScreen> {
  bool _ackSent = false;

  void _maybeAck(List<notif.Notification> ns) {
    if (_ackSent || ns.isEmpty) return;
    _ackSent = true;
    // Fire-and-forget ack; if it fails, it does not block rendering.
    ref.read(notificationReceiveProvider(ns).future).ignore();
  }

  @override
  Widget build(BuildContext context) {
    final params = NotificationSearchParams(recipient: widget.profileId);
    final asyncNotifs = ref.watch(notificationSearchProvider(params));

    return asyncNotifs.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (e, _) => Center(child: Text('$e')),
      data: (ns) {
        WidgetsBinding.instance.addPostFrameCallback((_) => _maybeAck(ns));
        return EntityListPage<notif.Notification>(
          title: 'Inbox',
          icon: Icons.inbox,
          items: ns,
          itemBuilder: (context, n) => Padding(
            padding: const EdgeInsets.symmetric(vertical: 4),
            child: NotificationTile(
              notification: n,
              onTap: () =>
                  context.go('/notifications/detail/${n.id}', extra: n),
            ),
          ),
        );
      },
    );
  }
}
