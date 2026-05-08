import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'fake_notification_client.dart';
import 'test_harness.dart';

void main() {
  testWidgets('TestHarness exposes overridden tenancyContextProvider',
      (tester) async {
    String? observedPartition;
    await tester.pumpWidget(
      TestHarness(
        child: Consumer(
          builder: (context, ref, _) {
            observedPartition = ref.watch(tenancyContextProvider).partitionId;
            return const SizedBox();
          },
        ),
      ),
    );
    expect(observedPartition, 'part-test');
  });

  testWidgets('TestHarness wires the FakeNotificationClient into the provider',
      (tester) async {
    final fake = FakeNotificationClient()
      ..nextSearchResults = [makeNotification(id: 'n1')];

    await tester.pumpWidget(TestHarness(
      client: fake,
      child: Consumer(
        builder: (context, ref, _) {
          ref.watch(
              notificationSearchProvider(const NotificationSearchParams()));
          return const SizedBox();
        },
      ),
    ));
    await tester.pumpAndSettle();

    expect(fake.searchRequests, hasLength(1));
  });
}
