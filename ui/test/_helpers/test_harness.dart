import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'fake_notification_client.dart';

/// Builds a [ProviderScope] wrapping [child] with an overridden
/// [tenancyContextProvider] and, optionally, an overridden
/// [notificationServiceClientProvider].
///
/// Use [FakeNotificationClient] via the [client] parameter to inject canned
/// responses in tests without network access.
class TestHarness extends StatelessWidget {
  const TestHarness({
    super.key,
    required this.child,
    this.client,
    this.partitionId = 'part-test',
    this.organizationId,
    this.branchId,
  });

  final Widget child;
  final FakeNotificationClient? client;
  final String partitionId;
  final String? organizationId;
  final String? branchId;

  @override
  Widget build(BuildContext context) {
    final tenancy = TenancyContext()
      ..initializeFromLogin(
        LoginLevel.root,
        partitionId: partitionId,
        partitionName: 'Test Partition',
        orgId: organizationId,
        orgName: 'Test Org',
        branchId: branchId,
        branchName: 'Test Branch',
      );

    return ProviderScope(
      overrides: [
        tenancyContextProvider.overrideWithValue(tenancy),
        if (client != null)
          notificationServiceClientProvider.overrideWithValue(client!.client),
      ],
      child: MaterialApp(home: Scaffold(body: child)),
    );
  }
}

/// Builds a [notif.Notification] proto with sensible test defaults.
notif.Notification makeNotification({
  required String id,
  String type = 'SMS',
  String template = 'welcome',
  String recipient = '+254700000000',
  String source = 'TESTSRC',
  notif.PRIORITY priority = notif.PRIORITY.LOW,
  String language = 'en',
}) {
  return notif.Notification()
    ..id = id
    ..type = type
    ..template = template
    ..language = language
    ..priority = priority
    ..source = (notif.ContactLink()..detail = source)
    ..recipient = (notif.ContactLink()..detail = recipient);
}
