import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

/// Riverpod-friendly view of TenancyContext.partitionId.
///
/// Reads `tenancyContextProvider`, listens for `notifyListeners()` calls,
/// and invalidates itself on change so downstream providers/widgets that
/// `ref.watch` this rebuild automatically.
final partitionIdProvider = Provider<String>((ref) {
  final tenancy = ref.watch(tenancyContextProvider);
  void onChange() => ref.invalidateSelf();
  tenancy.addListener(onChange);
  ref.onDispose(() => tenancy.removeListener(onChange));
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
}

/// Same bridge as `partitionIdProvider`, exposing the org/branch axes too.
final tenancyScopeProvider = Provider<TenancyScope>((ref) {
  final tenancy = ref.watch(tenancyContextProvider);
  void onChange() => ref.invalidateSelf();
  tenancy.addListener(onChange);
  ref.onDispose(() => tenancy.removeListener(onChange));
  return TenancyScope(
    partitionId: tenancy.partitionId,
    organizationId: tenancy.organizationId,
    branchId: tenancy.branchId,
  );
});
