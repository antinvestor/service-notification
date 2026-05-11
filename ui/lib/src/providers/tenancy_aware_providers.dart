import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

/// Wires `tenancy.notifyListeners()` to `ref.invalidateSelf()`, with cleanup
/// on dispose. Use inside any Provider whose value is projected from a
/// ChangeNotifier so that downstream `ref.watch`ers rebuild on change.
void _bridgeChangeNotifier(Ref ref, TenancyContext tenancy) {
  void onChange() => ref.invalidateSelf();
  tenancy.addListener(onChange);
  ref.onDispose(() => tenancy.removeListener(onChange));
}

/// Riverpod-friendly view of TenancyContext.partitionId.
///
/// Reads `tenancyContextProvider`, listens for `notifyListeners()` calls,
/// and invalidates itself on change so downstream providers/widgets that
/// `ref.watch` this rebuild automatically.
final partitionIdProvider = Provider<String>((ref) {
  final tenancy = ref.watch(tenancyContextProvider);
  _bridgeChangeNotifier(ref, tenancy);
  return tenancy.partitionId;
});

/// The tenancy axes the notification UI cares about (partition, organization,
/// branch). `==`/`hashCode` are by all three so `.family` providers re-key
/// when any dimension changes.
class TenancyScope {
  const TenancyScope({
    required this.partitionId,
    required this.organizationId,
    required this.branchId,
  });

  final String partitionId;
  final String organizationId;
  final String branchId;

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is TenancyScope &&
          partitionId == other.partitionId &&
          organizationId == other.organizationId &&
          branchId == other.branchId;

  @override
  int get hashCode =>
      Object.hash(partitionId, organizationId, branchId);

  @override
  String toString() =>
      'TenancyScope(partition: $partitionId, org: $organizationId, branch: $branchId)';
}

/// Same bridge as `partitionIdProvider`, exposing the org/branch axes too.
final tenancyScopeProvider = Provider<TenancyScope>((ref) {
  final tenancy = ref.watch(tenancyContextProvider);
  _bridgeChangeNotifier(ref, tenancy);
  return TenancyScope(
    partitionId: tenancy.partitionId,
    organizationId: tenancy.organizationId,
    branchId: tenancy.branchId,
  );
});
