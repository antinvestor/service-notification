import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
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

    // Hold a subscription open so autoDispose doesn't clear the cache between
    // the two reads. This forces the rebuild to come from the tenancy switch,
    // not from autoDispose recreating the provider.
    final sub = container.listen<AsyncValue<List<notif.Notification>>>(
      notificationSearchProvider(params),
      (_, _) {},
    );
    addTearDown(sub.close);

    await container.read(notificationSearchProvider(params).future);
    expect(fake.searchRequests, hasLength(1));

    tenancy.selectPartition('p2', 'Two');
    await container.pump();
    await container.read(notificationSearchProvider(params).future);

    expect(fake.searchRequests, hasLength(2));
    expect(fake.searchRequests[0].properties, contains('partition:p1'));
    expect(fake.searchRequests[1].properties, contains('partition:p2'));
  });

  test('NotificationNotifier.send invalidates the search cache', () async {
    final fake = FakeNotificationClient();
    final container = _container(fake: fake);
    addTearDown(container.dispose);

    const params = NotificationSearchParams();
    // Initial read populates the cache (1 request).
    await container.read(notificationSearchProvider(params).future);
    expect(fake.searchRequests, hasLength(1));

    // Send a notification.
    final n = makeNotification(id: 'n42');
    final sendReq = notif.SendRequest()..data.add(n);
    await container
        .read(notificationNotifierProvider.notifier)
        .send(sendReq);

    // Re-reading the search provider should now fire a second network call,
    // because the notifier invalidated the cache on success.
    await container.read(notificationSearchProvider(params).future);
    expect(fake.searchRequests, hasLength(2));
  });
}
