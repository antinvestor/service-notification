import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('partitionIdProvider', () {
    test('reflects current TenancyContext.partitionId', () {
      final tenancy = TenancyContext()
        ..initializeFromLogin(LoginLevel.root, partitionId: 'p1');

      final container = ProviderContainer(overrides: [
        tenancyContextProvider.overrideWithValue(tenancy),
      ]);
      addTearDown(container.dispose);

      expect(container.read(partitionIdProvider), 'p1');
    });

    test('re-emits when TenancyContext notifies', () async {
      final tenancy = TenancyContext()
        ..initializeFromLogin(LoginLevel.root, partitionId: 'p1');

      final container = ProviderContainer(overrides: [
        tenancyContextProvider.overrideWithValue(tenancy),
      ]);
      addTearDown(container.dispose);

      // Establish a subscription so invalidation rebuilds.
      final sub = container.listen<String>(
        partitionIdProvider,
        (_, __) {},
      );
      addTearDown(sub.close);
      expect(container.read(partitionIdProvider), 'p1');

      tenancy.selectPartition('p2', 'Two');
      // Allow the listener to fire and provider to rebuild.
      await Future<void>.delayed(Duration.zero);

      expect(container.read(partitionIdProvider), 'p2');
    });
  });

  group('tenancyScopeProvider', () {
    test('returns partition + organization + branch', () {
      final tenancy = TenancyContext()
        ..initializeFromLogin(
          LoginLevel.root,
          partitionId: 'p1',
          orgId: 'o1',
          branchId: 'b1',
        );

      final container = ProviderContainer(overrides: [
        tenancyContextProvider.overrideWithValue(tenancy),
      ]);
      addTearDown(container.dispose);

      final scope = container.read(tenancyScopeProvider);
      expect(scope.partitionId, 'p1');
      expect(scope.organizationId, 'o1');
      expect(scope.branchId, 'b1');
    });

    test('TenancyScope equality is by all three fields', () {
      const a = TenancyScope(
          partitionId: 'p', organizationId: 'o', branchId: 'b');
      const b = TenancyScope(
          partitionId: 'p', organizationId: 'o', branchId: 'b');
      const c = TenancyScope(
          partitionId: 'p', organizationId: 'o', branchId: 'x');
      expect(a, equals(b));
      expect(a, isNot(equals(c)));
    });
  });
}
