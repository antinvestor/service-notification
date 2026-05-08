import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../_helpers/fake_notification_client.dart';
import '../_helpers/test_harness.dart';

ProviderContainer _container({
  required FakeNotificationClient fake,
  String partitionId = 'part-test',
  String? orgId,
  String? branchId,
}) {
  final tenancy = TenancyContext()
    ..initializeFromLogin(
      LoginLevel.root,
      partitionId: partitionId,
      orgId: orgId,
      branchId: branchId,
    );
  return ProviderContainer(overrides: [
    tenancyContextProvider.overrideWithValue(tenancy),
    notificationServiceClientProvider.overrideWithValue(fake.client),
  ]);
}

void main() {
  test('search request includes partition/org/branch and filter properties',
      () async {
    final fake = FakeNotificationClient();
    fake.nextSearchResults = [makeNotification(id: 'n1')];
    final container = _container(
      fake: fake,
      partitionId: 'p1',
      orgId: 'o1',
      branchId: 'b1',
    );
    addTearDown(container.dispose);

    const params = NotificationSearchParams(
      query: 'hi',
      type: 'SMS',
      language: 'sw',
      recipient: '+254',
    );
    await container.read(notificationSearchProvider(params).future);

    expect(fake.searchRequests, hasLength(1));
    final req = fake.searchRequests.single;
    expect(req.query, 'hi');
    expect(req.properties, containsAll(<String>[
      'partition:p1',
      'organization:o1',
      'branch:b1',
      'type:SMS',
      'language:sw',
      'recipient:+254',
    ]));
  });

  test('switching partition re-fires search with new partition filter',
      () async {
    final fake = FakeNotificationClient();
    final tenancy = TenancyContext()
      ..initializeFromLogin(LoginLevel.root, partitionId: 'p1');
    final container = ProviderContainer(overrides: [
      tenancyContextProvider.overrideWithValue(tenancy),
      notificationServiceClientProvider.overrideWithValue(fake.client),
    ]);
    addTearDown(container.dispose);

    const params = NotificationSearchParams();
    await container.read(notificationSearchProvider(params).future);
    tenancy.selectPartition('p2', 'Two');
    await container.pump();
    container.invalidate(notificationSearchProvider(params));
    await container.read(notificationSearchProvider(params).future);

    expect(fake.searchRequests, hasLength(2));
    expect(fake.searchRequests[0].properties, contains('partition:p1'));
    expect(fake.searchRequests[1].properties, contains('partition:p2'));
  });
}
