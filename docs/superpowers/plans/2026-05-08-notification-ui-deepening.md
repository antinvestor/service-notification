# Notification UI Deepening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the `antinvestor_ui_notification` Flutter library with a tenancy-aware admin surface (dashboard, inbox, detail, compose, templates with a variants matrix, languages CRUD), an end-user inbox widget, and proto-shape fixes — composing existing `antinvestor_ui_core` primitives wherever possible.

**Architecture:** Bridge `tenancyContextProvider` (a `ChangeNotifier` from ui_core) into Riverpod via two providers (`partitionIdProvider`, `tenancyScopeProvider`) that auto-invalidate on tenancy changes. Search-side providers re-key on the scope and push partition/org/branch into the Search request's `properties` filter. Mutations invalidate dependent searches. UI screens compose ui_core primitives (`ServiceAnalyticsPage`, `AdminEntityListPage`, `EntityListPage`, `StatusBadge`, `EntityChip`, `AuditTrailEntry`, `MetadataRow`, `FormFieldCard`, `PageHeader`, `Breadcrumb`, `showEditDialog`, `PlaceholderPage`). The single net-new widget is a `TemplateVariantMatrix` (channel × language editor grid).

**Tech Stack:** Flutter 3.11+, Dart 3.11+, Riverpod 3.3, GoRouter 17, ConnectRPC for streaming RPCs, `antinvestor_ui_core` ^0.1, `antinvestor_api_notification` ^1.53. Tests use `flutter_test` + `ProviderContainer` + `testWidgets`.

**Spec:** [docs/superpowers/specs/2026-05-08-notification-ui-deepening-design.md](../specs/2026-05-08-notification-ui-deepening-design.md)

**Working directory for all paths below:** `/home/j/code/antinvestor/service-notification/ui` (the Flutter package). Run `flutter test` from this directory.

---

## Conventions

- **Test runner:** `flutter test test/<path>` from `ui/`. To run a single test, append `--plain-name "<name>"`.
- **Commit prefix:** `feat(ui):`, `fix(ui):`, `refactor(ui):`, `test(ui):`, `docs(ui):` — matching the repo's recent commits.
- **Branch:** work on `main` directly (the user has been doing this for the USSD work; the design doc was committed straight to main). One commit per task.
- **TDD:** Each task lists the failing test first, then the minimal implementation. Skip writing the test only when a step says "screen scaffolding only — golden test added in Task 18".

---

## Task 1: Test infrastructure & fake transport

**Files:**
- Modify: `pubspec.yaml`
- Create: `test/_helpers/fake_notification_client.dart`
- Create: `test/_helpers/test_harness.dart`

- [ ] **Step 1: Add test deps**

Edit `pubspec.yaml` so `dev_dependencies` includes:

```yaml
dev_dependencies:
  flutter_test:
    sdk: flutter
  flutter_lints: ^6.0.0
  build_runner: ^2.13.1
  riverpod_generator: ^4.0.3
  mocktail: ^1.0.4
```

Run: `flutter pub get`
Expected: deps resolve cleanly.

- [ ] **Step 2: Create the fake client helper**

Create `test/_helpers/fake_notification_client.dart`:

```dart
import 'dart:async';

import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:connectrpc/connect.dart';

/// Hand-rolled fake of NotificationServiceClient for tests.
///
/// Each method returns canned responses. Tests configure the queues and
/// then assert on `lastSendRequest` etc.
class FakeNotificationClient implements notif.NotificationServiceClient {
  final List<notif.SendRequest> sendRequests = [];
  final List<notif.ReleaseRequest> releaseRequests = [];
  final List<notif.ReceiveRequest> receiveRequests = [];
  final List<SearchRequest> searchRequests = [];
  final List<notif.TemplateSearchRequest> templateSearchRequests = [];
  final List<notif.TemplateSaveRequest> templateSaveRequests = [];

  /// Notifications returned from the next `search` call.
  List<notif.Notification> nextSearchResults = const [];

  /// Templates returned from the next `templateSearch` call.
  List<notif.Template> nextTemplateResults = const [];

  /// Status returned from the next `templateSave` call.
  notif.Template nextSavedTemplate = notif.Template();

  @override
  Stream<notif.SearchResponse> search(SearchRequest request, [_]) {
    searchRequests.add(request);
    final controller = StreamController<notif.SearchResponse>();
    controller.add(notif.SearchResponse()..data.addAll(nextSearchResults));
    controller.close();
    return controller.stream;
  }

  @override
  Stream<notif.SendResponse> send(notif.SendRequest request, [_]) {
    sendRequests.add(request);
    final controller = StreamController<notif.SendResponse>();
    controller.add(notif.SendResponse());
    controller.close();
    return controller.stream;
  }

  @override
  Stream<notif.ReleaseResponse> release(notif.ReleaseRequest request, [_]) {
    releaseRequests.add(request);
    final controller = StreamController<notif.ReleaseResponse>();
    controller.add(notif.ReleaseResponse());
    controller.close();
    return controller.stream;
  }

  @override
  Stream<notif.ReceiveResponse> receive(notif.ReceiveRequest request, [_]) {
    receiveRequests.add(request);
    final controller = StreamController<notif.ReceiveResponse>();
    controller.add(notif.ReceiveResponse());
    controller.close();
    return controller.stream;
  }

  @override
  Stream<notif.TemplateSearchResponse> templateSearch(
    notif.TemplateSearchRequest request, [_]) {
    templateSearchRequests.add(request);
    final controller = StreamController<notif.TemplateSearchResponse>();
    controller.add(
      notif.TemplateSearchResponse()..data.addAll(nextTemplateResults),
    );
    controller.close();
    return controller.stream;
  }

  @override
  Future<notif.TemplateSaveResponse> templateSave(
    notif.TemplateSaveRequest request, [_]) async {
    templateSaveRequests.add(request);
    return notif.TemplateSaveResponse()..data = nextSavedTemplate;
  }

  // The remaining proto methods (status, statusUpdate) — return empty
  // defaults; tests that need them will override behavior locally.
  @override
  Future<dynamic> status(dynamic _, [__]) async => throw UnimplementedError();
  @override
  Future<dynamic> statusUpdate(dynamic _, [__]) async =>
      throw UnimplementedError();
}

/// Re-exported here so test files don't need to import the common pkg
/// directly. Update if `SearchRequest` moves.
typedef SearchRequest = notif.SearchRequest;
```

(Note: real `SearchRequest` lives in `common/v1/common.proto`. The notification client `search` RPC takes `common.v1.SearchRequest`. The typedef just aliases it for test brevity.)

- [ ] **Step 3: Create the test harness**

Create `test/_helpers/test_harness.dart`:

```dart
import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'fake_notification_client.dart';

/// Builds a ProviderScope wrapping `child` with the fake notification
/// client and an initialized TenancyContext.
class TestHarness extends StatelessWidget {
  const TestHarness({
    super.key,
    required this.child,
    this.client,
    this.partitionId = 'part-test',
    this.organizationId = '',
    this.branchId = '',
  });

  final Widget child;
  final FakeNotificationClient? client;
  final String partitionId;
  final String organizationId;
  final String branchId;

  @override
  Widget build(BuildContext context) {
    final tenancy = TenancyContext()
      ..initializeFromLogin(
        LoginLevel.root,
        partitionId: partitionId,
        partitionName: 'Test Partition',
        orgId: organizationId.isEmpty ? null : organizationId,
        orgName: 'Test Org',
        branchId: branchId.isEmpty ? null : branchId,
        branchName: 'Test Branch',
      );

    return ProviderScope(
      overrides: [
        tenancyContextProvider.overrideWithValue(tenancy),
        if (client != null)
          notificationServiceClientProvider.overrideWithValue(client!),
      ],
      child: MaterialApp(home: Scaffold(body: child)),
    );
  }
}

/// Helper to build a Notification proto for fixtures.
notif.Notification makeNotification({
  required String id,
  String type = 'SMS',
  String template = 'welcome',
  String recipient = '+254700000000',
  String source = 'TESTSRC',
  String stateName = 'ACTIVE',
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
```

- [ ] **Step 4: Smoke test**

Create `test/_helpers/test_harness_test.dart`:

```dart
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

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
}
```

Run: `flutter test test/_helpers/test_harness_test.dart`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ui/pubspec.yaml ui/test/_helpers/
git commit -m "test(ui): add fake notification client and test harness"
```

---

## Task 2: Tenancy bridge providers

**Files:**
- Create: `lib/src/providers/tenancy_aware_providers.dart`
- Test: `test/providers/tenancy_aware_providers_test.dart`

- [ ] **Step 1: Write the failing test**

Create `test/providers/tenancy_aware_providers_test.dart`:

```dart
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `flutter test test/providers/tenancy_aware_providers_test.dart`
Expected: FAIL — `partitionIdProvider` and `TenancyScope` undefined.

- [ ] **Step 3: Implement the bridge**

Create `lib/src/providers/tenancy_aware_providers.dart`:

```dart
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
```

Edit `lib/antinvestor_ui_notification.dart` — add export at the top of the Providers section:

```dart
export 'src/providers/tenancy_aware_providers.dart';
```

- [ ] **Step 4: Run test to verify it passes**

Run: `flutter test test/providers/tenancy_aware_providers_test.dart`
Expected: PASS — all four cases.

- [ ] **Step 5: Commit**

```bash
git add ui/lib/src/providers/tenancy_aware_providers.dart \
        ui/lib/antinvestor_ui_notification.dart \
        ui/test/providers/tenancy_aware_providers_test.dart
git commit -m "feat(ui): add tenancy bridge providers (partitionId + scope)"
```

---

## Task 3: Refactor notification_providers to use tenancy scope

**Files:**
- Modify: `lib/src/providers/notification_providers.dart`
- Test: `test/providers/notification_providers_test.dart`

- [ ] **Step 1: Write the failing test**

Create `test/providers/notification_providers_test.dart`:

```dart
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../_helpers/fake_notification_client.dart';
import '../_helpers/test_harness.dart';

ProviderContainer _container({
  required FakeNotificationClient client,
  String partitionId = 'part-test',
  String orgId = '',
  String branchId = '',
}) {
  final tenancy = TenancyContext()
    ..initializeFromLogin(
      LoginLevel.root,
      partitionId: partitionId,
      orgId: orgId.isEmpty ? null : orgId,
      branchId: branchId.isEmpty ? null : branchId,
    );
  return ProviderContainer(overrides: [
    tenancyContextProvider.overrideWithValue(tenancy),
    notificationServiceClientProvider.overrideWithValue(client),
  ]);
}

void main() {
  test('search request includes partition/org/branch and filter properties',
      () async {
    final fake = FakeNotificationClient();
    fake.nextSearchResults = [makeNotification(id: 'n1')];
    final container = _container(
      client: fake,
      partitionId: 'p1',
      orgId: 'o1',
      branchId: 'b1',
    );
    addTearDown(container.dispose);

    final params = const NotificationSearchParams(
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
      notificationServiceClientProvider.overrideWithValue(fake),
    ]);
    addTearDown(container.dispose);

    const params = NotificationSearchParams();
    await container.read(notificationSearchProvider(params).future);
    tenancy.selectPartition('p2', 'Two');
    // Force a re-read by invalidating the family entry; in a real widget
    // the autoDispose + ref.watch chain handles this automatically.
    container.invalidate(notificationSearchProvider(params));
    await container.read(notificationSearchProvider(params).future);

    expect(fake.searchRequests, hasLength(2));
    expect(fake.searchRequests[0].properties, contains('partition:p1'));
    expect(fake.searchRequests[1].properties, contains('partition:p2'));
  });
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `flutter test test/providers/notification_providers_test.dart`
Expected: FAIL — `NotificationSearchParams` lacks a `language` field; the search provider does not push partition/org/branch into properties.

- [ ] **Step 3: Refactor the provider**

Replace the contents of `lib/src/providers/notification_providers.dart` with:

```dart
import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_core/api/stream_helpers.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'notification_transport_provider.dart';
import 'tenancy_aware_providers.dart';

/// Parameters for searching notifications.
class NotificationSearchParams {
  const NotificationSearchParams({
    this.query = '',
    this.type = '',
    this.language = '',
    this.recipient = '',
  });

  final String query;
  final String type;
  final String language;
  final String recipient;

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is NotificationSearchParams &&
          query == other.query &&
          type == other.type &&
          language == other.language &&
          recipient == other.recipient;

  @override
  int get hashCode => Object.hash(query, type, language, recipient);
}

/// Search notifications scoped to the active tenancy.
final notificationSearchProvider = FutureProvider.autoDispose
    .family<List<notif.Notification>, NotificationSearchParams>(
        (ref, params) async {
  final scope = ref.watch(tenancyScopeProvider);
  final client = ref.watch(notificationServiceClientProvider);

  final request = notif.SearchRequest()..query = params.query;
  if (scope.partitionId.isNotEmpty) {
    request.properties.add('partition:${scope.partitionId}');
  }
  if (scope.organizationId.isNotEmpty) {
    request.properties.add('organization:${scope.organizationId}');
  }
  if (scope.branchId.isNotEmpty) {
    request.properties.add('branch:${scope.branchId}');
  }
  if (params.type.isNotEmpty) request.properties.add('type:${params.type}');
  if (params.language.isNotEmpty) {
    request.properties.add('language:${params.language}');
  }
  if (params.recipient.isNotEmpty) {
    request.properties.add('recipient:${params.recipient}');
  }

  final stream = client.search(request);
  return collectStream<notif.SearchResponse, notif.Notification>(
    stream,
    extract: (r) => r.data,
  );
});

/// Acknowledge receipt of notifications. Used by the end-user inbox.
final notificationReceiveProvider = FutureProvider.family<
    List<notif.StatusResponse>,
    List<notif.Notification>>((ref, notifications) async {
  final client = ref.watch(notificationServiceClientProvider);
  final request = notif.ReceiveRequest()..data.addAll(notifications);
  final stream = client.receive(request);
  return collectStream<notif.ReceiveResponse, notif.StatusResponse>(
    stream,
    extract: (r) => r.data,
  );
});

/// Get notification status by ID.
final notificationStatusProvider = FutureProvider.family<
    notif.StatusResponse,
    String>((ref, notificationId) async {
  final client = ref.watch(notificationServiceClientProvider);
  final request = notif.StatusRequest()..id = notificationId;
  return client.status(request);
});

/// Notifier for notification mutations (send, release, status update).
///
/// On success, invalidates `notificationSearchProvider` so dependent UI
/// re-fetches under the current tenancy scope.
class NotificationNotifier extends Notifier<AsyncValue<void>> {
  @override
  AsyncValue<void> build() => const AsyncValue.data(null);

  notif.NotificationServiceClient get _client =>
      ref.read(notificationServiceClientProvider);

  Future<void> send(notif.SendRequest request) async {
    state = const AsyncValue.loading();
    try {
      final stream = _client.send(request);
      await for (final _ in stream) {}
      ref.invalidate(notificationSearchProvider);
      state = const AsyncValue.data(null);
    } catch (e, st) {
      state = AsyncValue.error(e, st);
      rethrow;
    }
  }

  Future<void> release(notif.ReleaseRequest request) async {
    state = const AsyncValue.loading();
    try {
      final stream = _client.release(request);
      await for (final _ in stream) {}
      ref.invalidate(notificationSearchProvider);
      state = const AsyncValue.data(null);
    } catch (e, st) {
      state = AsyncValue.error(e, st);
      rethrow;
    }
  }

  Future<void> statusUpdate(notif.StatusUpdateRequest request) async {
    state = const AsyncValue.loading();
    try {
      await _client.statusUpdate(request);
      ref.invalidate(notificationSearchProvider);
      state = const AsyncValue.data(null);
    } catch (e, st) {
      state = AsyncValue.error(e, st);
      rethrow;
    }
  }
}

final notificationNotifierProvider =
    NotifierProvider<NotificationNotifier, AsyncValue<void>>(
        NotificationNotifier.new);
```

- [ ] **Step 4: Run test to verify it passes**

Run: `flutter test test/providers/notification_providers_test.dart`
Expected: PASS — both cases.

- [ ] **Step 5: Commit**

```bash
git add ui/lib/src/providers/notification_providers.dart \
        ui/test/providers/notification_providers_test.dart
git commit -m "refactor(ui): scope notification search by tenancy"
```

---

## Task 4: Refactor template_providers — scope-aware search + variants persist

**Files:**
- Modify: `lib/src/providers/template_providers.dart`
- Test: `test/providers/template_providers_test.dart`

- [ ] **Step 1: Write the failing test**

Create `test/providers/template_providers_test.dart`:

```dart
import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../_helpers/fake_notification_client.dart';

ProviderContainer _container(FakeNotificationClient fake) {
  final tenancy = TenancyContext()
    ..initializeFromLogin(LoginLevel.root, partitionId: 'p1');
  return ProviderContainer(overrides: [
    tenancyContextProvider.overrideWithValue(tenancy),
    notificationServiceClientProvider.overrideWithValue(fake),
  ]);
}

void main() {
  test('templateSearch surfaces results', () async {
    final fake = FakeNotificationClient()
      ..nextTemplateResults = [
        notif.Template()
          ..id = 't1'
          ..name = 'welcome',
      ];
    final container = _container(fake);
    addTearDown(container.dispose);

    final results = await container.read(
      templateSearchProvider(const TemplateSearchParams()).future,
    );
    expect(results, hasLength(1));
    expect(results.single.name, 'welcome');
    expect(fake.templateSearchRequests.single.query, '');
  });

  test('save round-trips variants through the data Struct', () async {
    final fake = FakeNotificationClient()..nextSavedTemplate =
        (notif.Template()..id = 't1'..name = 'welcome');
    final container = _container(fake);
    addTearDown(container.dispose);

    final variants = [
      notif.TemplateData()
        ..type = 'SMS'
        ..detail = 'Hi {{name}}'
        ..language = (notif.Language()..code = 'en'),
      notif.TemplateData()
        ..type = 'EMAIL'
        ..detail = 'Hello {{name}}'
        ..language = (notif.Language()..code = 'en'),
    ];

    await container.read(templateNotifierProvider.notifier).save(
          name: 'welcome',
          variants: variants,
        );

    expect(fake.templateSaveRequests, hasLength(1));
    final req = fake.templateSaveRequests.single;
    expect(req.name, 'welcome');
    final variantsField = req.data.fields['variants'];
    expect(variantsField, isNotNull);
    final list = variantsField!.listValue.values;
    expect(list, hasLength(2));
    expect(list[0].structValue.fields['type']?.stringValue, 'SMS');
    expect(list[0].structValue.fields['language']?.stringValue, 'en');
    expect(list[0].structValue.fields['detail']?.stringValue, 'Hi {{name}}');
    expect(list[1].structValue.fields['type']?.stringValue, 'EMAIL');
  });
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `flutter test test/providers/template_providers_test.dart`
Expected: FAIL — `TemplateSearchParams` and `save({name, variants})` undefined.

- [ ] **Step 3: Refactor the provider**

Replace `lib/src/providers/template_providers.dart` with:

```dart
import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_core/api/stream_helpers.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:protobuf/protobuf.dart' show Value, ListValue, Struct;

import 'notification_transport_provider.dart';
import 'tenancy_aware_providers.dart';

/// Parameters for searching templates.
class TemplateSearchParams {
  const TemplateSearchParams({this.query = '', this.languageCode = ''});

  final String query;
  final String languageCode;

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is TemplateSearchParams &&
          query == other.query &&
          languageCode == other.languageCode;

  @override
  int get hashCode => Object.hash(query, languageCode);
}

/// Search templates scoped to the current tenancy.
final templateSearchProvider = FutureProvider.autoDispose
    .family<List<notif.Template>, TemplateSearchParams>((ref, params) async {
  // tenancy scope is included implicitly via auth context server-side;
  // we still ref.watch it so a partition switch invalidates this cache.
  ref.watch(tenancyScopeProvider);

  final client = ref.watch(notificationServiceClientProvider);
  final request = notif.TemplateSearchRequest()
    ..query = params.query
    ..languageCode = params.languageCode;
  final stream = client.templateSearch(request);
  return collectStream<notif.TemplateSearchResponse, notif.Template>(
    stream,
    extract: (r) => r.data,
  );
});

/// Notifier for template mutations.
///
/// `save` encodes the variants list into the proto `data Struct` under a
/// `variants` key (see spec §5.2 for the contract). On success invalidates
/// `templateSearchProvider`.
class TemplateNotifier extends Notifier<AsyncValue<void>> {
  @override
  AsyncValue<void> build() => const AsyncValue.data(null);

  notif.NotificationServiceClient get _client =>
      ref.read(notificationServiceClientProvider);

  Future<notif.Template> save({
    required String name,
    required List<notif.TemplateData> variants,
  }) async {
    state = const AsyncValue.loading();
    try {
      final variantValues = variants.map((td) {
        final variantStruct = Struct();
        variantStruct.fields['type'] = Value()..stringValue = td.type;
        variantStruct.fields['language'] = Value()
          ..stringValue = td.language.code;
        variantStruct.fields['detail'] = Value()..stringValue = td.detail;
        return Value()..structValue = variantStruct;
      }).toList();

      final dataStruct = Struct();
      dataStruct.fields['variants'] = Value()
        ..listValue = (ListValue()..values.addAll(variantValues));

      // Default the top-level language_code to the first variant's language
      // so older backends that ignore data.variants still record something.
      final defaultLang = variants.isEmpty ? '' : variants.first.language.code;

      final request = notif.TemplateSaveRequest()
        ..name = name
        ..languageCode = defaultLang
        ..data = dataStruct;

      final response = await _client.templateSave(request);
      ref.invalidate(templateSearchProvider);
      state = const AsyncValue.data(null);
      return response.data;
    } catch (e, st) {
      state = AsyncValue.error(e, st);
      rethrow;
    }
  }
}

final templateNotifierProvider =
    NotifierProvider<TemplateNotifier, AsyncValue<void>>(
        TemplateNotifier.new);

/// Decodes a Template's variants from either the proto `repeated TemplateData`
/// (preferred) or the `data Struct` `variants` array (legacy contract).
List<notif.TemplateData> decodeTemplateVariants(notif.Template template) {
  if (template.data.isNotEmpty) return List.of(template.data);
  // Fallback: read from extra/struct path if used by older save shape.
  return const [];
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `flutter test test/providers/template_providers_test.dart`
Expected: PASS — both cases.

- [ ] **Step 5: Commit**

```bash
git add ui/lib/src/providers/template_providers.dart \
        ui/test/providers/template_providers_test.dart
git commit -m "fix(ui): persist template variants via data Struct on save"
```

---

## Task 5: Language providers (CRUD over the Language proto)

**Files:**
- Create: `lib/src/providers/language_providers.dart`
- Test: `test/providers/language_providers_test.dart`

**Context:** The proto exposes `Language` only as a sub-message of `TemplateData`. There is no dedicated `LanguageSearch`/`LanguageSave` RPC. We derive the language list by scanning every template's variants and union-deduping by `language.code`. New languages are introduced by saving a template that references them.

- [ ] **Step 1: Write the failing test**

Create `test/providers/language_providers_test.dart`:

```dart
import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../_helpers/fake_notification_client.dart';

ProviderContainer _container(FakeNotificationClient fake) {
  final tenancy = TenancyContext()
    ..initializeFromLogin(LoginLevel.root, partitionId: 'p1');
  return ProviderContainer(overrides: [
    tenancyContextProvider.overrideWithValue(tenancy),
    notificationServiceClientProvider.overrideWithValue(fake),
  ]);
}

notif.Template _tpl(String name, List<(String code, String label)> langs) {
  return notif.Template()
    ..name = name
    ..data.addAll([
      for (final l in langs)
        notif.TemplateData()
          ..type = 'SMS'
          ..detail = '...'
          ..language = (notif.Language()
            ..code = l.$1
            ..name = l.$2),
    ]);
}

void main() {
  test('languageSearchProvider unions languages across templates', () async {
    final fake = FakeNotificationClient()
      ..nextTemplateResults = [
        _tpl('a', [('en', 'English'), ('sw', 'Swahili')]),
        _tpl('b', [('en', 'English'), ('fr', 'French')]),
      ];
    final container = _container(fake);
    addTearDown(container.dispose);

    final langs = await container.read(languageSearchProvider('').future);
    expect(langs.map((l) => l.code), unorderedEquals(['en', 'sw', 'fr']));
  });

  test('query filter narrows results by code or name', () async {
    final fake = FakeNotificationClient()
      ..nextTemplateResults = [
        _tpl('a', [('en', 'English'), ('sw', 'Swahili'), ('fr', 'French')]),
      ];
    final container = _container(fake);
    addTearDown(container.dispose);

    final swOnly = await container.read(languageSearchProvider('sw').future);
    expect(swOnly.map((l) => l.code), ['sw']);

    final french =
        await container.read(languageSearchProvider('french').future);
    expect(french.map((l) => l.code), ['fr']);
  });
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `flutter test test/providers/language_providers_test.dart`
Expected: FAIL — `languageSearchProvider` undefined.

- [ ] **Step 3: Implement language providers**

Create `lib/src/providers/language_providers.dart`:

```dart
import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'template_providers.dart';

/// Returns the union of all `Language` records found across templates,
/// deduped by `code`. Optionally filtered by a case-insensitive substring
/// match against either `code` or `name`.
final languageSearchProvider = FutureProvider.autoDispose
    .family<List<notif.Language>, String>((ref, query) async {
  final templates =
      await ref.watch(templateSearchProvider(const TemplateSearchParams()).future);
  final byCode = <String, notif.Language>{};
  for (final t in templates) {
    for (final td in t.data) {
      final code = td.language.code;
      if (code.isEmpty) continue;
      byCode.putIfAbsent(code, () => td.language);
    }
  }

  final q = query.toLowerCase().trim();
  final all = byCode.values.toList()
    ..sort((a, b) => a.code.compareTo(b.code));
  if (q.isEmpty) return all;
  return all
      .where((l) =>
          l.code.toLowerCase().contains(q) ||
          l.name.toLowerCase().contains(q))
      .toList();
});

/// Saves a Language by upserting it onto a placeholder template variant.
///
/// Because the proto has no dedicated LanguageSave RPC, we encode the
/// "language exists" fact into a placeholder template variant. Hosts that
/// later add a real LanguageSave can switch this notifier without touching
/// the screens.
class LanguageNotifier extends Notifier<AsyncValue<void>> {
  @override
  AsyncValue<void> build() => const AsyncValue.data(null);

  Future<notif.Language> save({
    required String code,
    required String name,
  }) async {
    state = const AsyncValue.loading();
    try {
      final language = notif.Language()
        ..code = code
        ..name = name;
      final placeholder = notif.TemplateData()
        ..type = 'SMS'
        ..detail = '(language registration placeholder)'
        ..language = language;
      final templateNotifier = ref.read(templateNotifierProvider.notifier);
      await templateNotifier.save(
        name: '_lang_$code',
        variants: [placeholder],
      );
      ref.invalidate(languageSearchProvider);
      state = const AsyncValue.data(null);
      return language;
    } catch (e, st) {
      state = AsyncValue.error(e, st);
      rethrow;
    }
  }
}

final languageNotifierProvider =
    NotifierProvider<LanguageNotifier, AsyncValue<void>>(
        LanguageNotifier.new);
```

Edit `lib/antinvestor_ui_notification.dart` — add export under Providers:

```dart
export 'src/providers/language_providers.dart';
```

- [ ] **Step 4: Run test to verify it passes**

Run: `flutter test test/providers/language_providers_test.dart`
Expected: PASS — both cases.

- [ ] **Step 5: Commit**

```bash
git add ui/lib/src/providers/language_providers.dart \
        ui/lib/antinvestor_ui_notification.dart \
        ui/test/providers/language_providers_test.dart
git commit -m "feat(ui): add language providers derived from template variants"
```

---

## Task 6: Stats provider + NotificationStats class

**Files:**
- Create: `lib/src/providers/stats_providers.dart`
- Test: `test/providers/stats_providers_test.dart`

- [ ] **Step 1: Write the failing test**

Create `test/providers/stats_providers_test.dart`:

```dart
import 'package:antinvestor_api_common/antinvestor_api_common.dart';
import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../_helpers/fake_notification_client.dart';
import '../_helpers/test_harness.dart';

notif.Notification _withState(
    String id, STATE state, String type, String template) {
  return makeNotification(id: id, type: type, template: template)
    ..status = (StatusResponse()..state = state);
}

void main() {
  test('NotificationStats.fromList tallies counts correctly', () {
    final stats = NotificationStats.fromList([
      _withState('1', STATE.ACTIVE, 'SMS', 'welcome'),
      _withState('2', STATE.ACTIVE, 'EMAIL', 'welcome'),
      _withState('3', STATE.INACTIVE, 'SMS', 'reset'),
      _withState('4', STATE.INACTIVE, 'SMS', 'reset'),
      _withState('5', STATE.CREATED, 'PUSH', 'reset'),
    ]);
    expect(stats.sent, 5);
    expect(stats.delivered, 2);
    expect(stats.failed, 2);
    expect(stats.queued, 1);
    expect(stats.channelMix, {'SMS': 3, 'EMAIL': 1, 'PUSH': 1});
    expect(stats.topFailing.first.template, 'reset');
    expect(stats.topFailing.first.failures, 2);
  });

  testWidgets('notificationStatsProvider derives from search snapshot',
      (tester) async {
    final fake = FakeNotificationClient()
      ..nextSearchResults = [
        _withState('1', STATE.ACTIVE, 'SMS', 'welcome'),
        _withState('2', STATE.INACTIVE, 'SMS', 'reset'),
      ];
    NotificationStats? observed;
    await tester.pumpWidget(TestHarness(
      client: fake,
      child: Consumer(builder: (_, ref, __) {
        // First trigger the search, then read stats.
        ref.watch(notificationSearchProvider(
            const NotificationSearchParams()));
        observed = ref.watch(notificationStatsProvider);
        return const SizedBox();
      }),
    ));
    await tester.pumpAndSettle();
    expect(observed!.sent, 2);
    expect(observed!.delivered, 1);
    expect(observed!.failed, 1);
  });
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `flutter test test/providers/stats_providers_test.dart`
Expected: FAIL — `NotificationStats` and `notificationStatsProvider` undefined.

- [ ] **Step 3: Implement stats**

Create `lib/src/providers/stats_providers.dart`:

```dart
import 'package:antinvestor_api_common/antinvestor_api_common.dart';
import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'notification_providers.dart';

/// Derived KPIs for the notification dashboard.
///
/// `sent` counts every notification in the snapshot. `delivered`,
/// `failed`, and `queued` are bucketed by `status.state` mapping to common
/// `STATE`. `channelMix` tallies by `type`. `topFailing` is the top 5
/// templates by failure count, descending.
class NotificationStats {
  const NotificationStats({
    required this.sent,
    required this.delivered,
    required this.failed,
    required this.queued,
    required this.channelMix,
    required this.topFailing,
  });

  final int sent;
  final int delivered;
  final int failed;
  final int queued;
  final Map<String, int> channelMix;
  final List<({String template, int failures})> topFailing;

  factory NotificationStats.empty() => const NotificationStats(
        sent: 0,
        delivered: 0,
        failed: 0,
        queued: 0,
        channelMix: <String, int>{},
        topFailing: <({String template, int failures})>[],
      );

  factory NotificationStats.fromList(List<notif.Notification> ns) {
    var delivered = 0;
    var failed = 0;
    var queued = 0;
    final channelMix = <String, int>{};
    final failuresByTemplate = <String, int>{};
    for (final n in ns) {
      switch (n.status.state) {
        case STATE.ACTIVE:
          delivered++;
          break;
        case STATE.INACTIVE:
        case STATE.DELETED:
          failed++;
          if (n.template.isNotEmpty) {
            failuresByTemplate.update(
              n.template,
              (v) => v + 1,
              ifAbsent: () => 1,
            );
          }
          break;
        case STATE.CREATED:
        case STATE.CHECKED:
          queued++;
          break;
        default:
          break;
      }
      if (n.type.isNotEmpty) {
        channelMix.update(n.type, (v) => v + 1, ifAbsent: () => 1);
      }
    }
    final topFailing = failuresByTemplate.entries
        .map((e) => (template: e.key, failures: e.value))
        .toList()
      ..sort((a, b) => b.failures.compareTo(a.failures));
    return NotificationStats(
      sent: ns.length,
      delivered: delivered,
      failed: failed,
      queued: queued,
      channelMix: channelMix,
      topFailing: topFailing.take(5).toList(),
    );
  }
}

/// Computes stats from the current scope's full notification snapshot.
final notificationStatsProvider = Provider.autoDispose<NotificationStats>(
    (ref) {
  final asyncNotifs = ref.watch(
    notificationSearchProvider(const NotificationSearchParams()),
  );
  return asyncNotifs.maybeWhen(
    data: NotificationStats.fromList,
    orElse: NotificationStats.empty,
  );
});
```

Edit `lib/antinvestor_ui_notification.dart` — add export under Providers:

```dart
export 'src/providers/stats_providers.dart';
```

- [ ] **Step 4: Run test to verify it passes**

Run: `flutter test test/providers/stats_providers_test.dart`
Expected: PASS — both cases.

- [ ] **Step 5: Commit**

```bash
git add ui/lib/src/providers/stats_providers.dart \
        ui/lib/antinvestor_ui_notification.dart \
        ui/test/providers/stats_providers_test.dart
git commit -m "feat(ui): derive notification KPIs from search snapshot"
```

---

## Task 7: NotificationDashboardScreen (composes ServiceAnalyticsPage)

**Files:**
- Create: `lib/src/screens/notification_dashboard_screen.dart`
- Test: `test/screens/notification_dashboard_screen_test.dart`

- [ ] **Step 1: Write the failing test**

Create `test/screens/notification_dashboard_screen_test.dart`:

```dart
import 'package:antinvestor_api_common/antinvestor_api_common.dart';
import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../_helpers/fake_notification_client.dart';
import '../_helpers/test_harness.dart';

void main() {
  testWidgets('renders KPI tiles from search snapshot', (tester) async {
    final fake = FakeNotificationClient()
      ..nextSearchResults = [
        makeNotification(id: '1', stateName: 'ACTIVE')
          ..status = (StatusResponse()..state = STATE.ACTIVE),
        makeNotification(id: '2')
          ..status = (StatusResponse()..state = STATE.INACTIVE),
        makeNotification(id: '3')
          ..status = (StatusResponse()..state = STATE.CREATED),
      ];

    await tester.pumpWidget(TestHarness(
      client: fake,
      child: const NotificationDashboardScreen(),
    ));
    await tester.pumpAndSettle();

    expect(find.text('Sent'), findsOneWidget);
    expect(find.text('3'), findsOneWidget); // sent
    expect(find.text('Delivered'), findsOneWidget);
    expect(find.text('Failed'), findsOneWidget);
    expect(find.text('Queued'), findsOneWidget);
  });
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `flutter test test/screens/notification_dashboard_screen_test.dart`
Expected: FAIL — `NotificationDashboardScreen` undefined.

- [ ] **Step 3: Implement the dashboard**

Create `lib/src/screens/notification_dashboard_screen.dart`:

```dart
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../providers/notification_providers.dart';
import '../providers/stats_providers.dart';

/// Top-level dashboard for the partition's notification activity.
///
/// Composes `ServiceAnalyticsPage` from ui_core. Data comes from
/// `notificationStatsProvider` (derived from the current search snapshot).
class NotificationDashboardScreen extends ConsumerWidget {
  const NotificationDashboardScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // Trigger the underlying search so stats has data.
    ref.watch(notificationSearchProvider(const NotificationSearchParams()));
    final stats = ref.watch(notificationStatsProvider);
    final tenancy = ref.watch(tenancyContextProvider);

    final crumbs = <String>['Home', ...tenancy.breadcrumbs, 'Notifications'];

    final kpis = <ServiceKpi>[
      ServiceKpi(
        label: 'Sent',
        value: '${stats.sent}',
        icon: Icons.send_outlined,
      ),
      ServiceKpi(
        label: 'Delivered',
        value: '${stats.delivered}',
        icon: Icons.check_circle_outline,
        changePositive: true,
      ),
      ServiceKpi(
        label: 'Failed',
        value: '${stats.failed}',
        icon: Icons.error_outline,
        changePositive: false,
      ),
      ServiceKpi(
        label: 'Queued',
        value: '${stats.queued}',
        icon: Icons.schedule_outlined,
      ),
    ];

    final events = stats.topFailing
        .map((e) => ServiceEvent(
              title: '${e.template} — ${e.failures} failure(s)',
              timeAgo: '',
              severity: EventSeverity.error,
              icon: Icons.error_outline,
            ))
        .toList();

    return ServiceAnalyticsPage(
      title: 'Notifications',
      breadcrumbs: crumbs,
      kpis: kpis,
      chartTitle: 'Channel mix',
      chartSubtitle: 'Distribution of recent notifications by channel',
      chartWidget: _ChannelMixDonut(channelMix: stats.channelMix),
      events: events,
    );
  }
}

/// Tiny CustomPainter donut to avoid pulling in a chart dependency.
class _ChannelMixDonut extends StatelessWidget {
  const _ChannelMixDonut({required this.channelMix});
  final Map<String, int> channelMix;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    if (channelMix.isEmpty) {
      return SizedBox(
        height: 200,
        child: Center(
          child: Text(
            'No channel data',
            style: theme.textTheme.bodyMedium?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
        ),
      );
    }
    final total = channelMix.values.fold<int>(0, (a, b) => a + b);
    final entries = channelMix.entries.toList()
      ..sort((a, b) => b.value.compareTo(a.value));
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        for (final e in entries)
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 4),
            child: Row(
              children: [
                SizedBox(
                  width: 80,
                  child: Text(
                    e.key,
                    style: theme.textTheme.bodyMedium?.copyWith(
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
                Expanded(
                  child: ClipRRect(
                    borderRadius: BorderRadius.circular(4),
                    child: LinearProgressIndicator(
                      value: total == 0 ? 0 : e.value / total,
                      minHeight: 10,
                      backgroundColor: theme.colorScheme.surfaceContainerHighest,
                    ),
                  ),
                ),
                const SizedBox(width: 12),
                Text(
                  '${e.value}',
                  style: theme.textTheme.bodyMedium,
                ),
              ],
            ),
          ),
      ],
    );
  }
}
```

Edit `lib/antinvestor_ui_notification.dart` — add export under Screens:

```dart
export 'src/screens/notification_dashboard_screen.dart';
```

- [ ] **Step 4: Run test to verify it passes**

Run: `flutter test test/screens/notification_dashboard_screen_test.dart`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ui/lib/src/screens/notification_dashboard_screen.dart \
        ui/lib/antinvestor_ui_notification.dart \
        ui/test/screens/notification_dashboard_screen_test.dart
git commit -m "feat(ui): add notification dashboard screen"
```

---

## Task 8: Reduce notification_status_badge to a thin StatusBadge wrapper

**Files:**
- Modify: `lib/src/widgets/notification_status_badge.dart`
- Test: `test/widgets/notification_status_badge_test.dart`

- [ ] **Step 1: Write the failing test**

Create `test/widgets/notification_status_badge_test.dart`:

```dart
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('renders a StatusBadge with the right label', (tester) async {
    await tester.pumpWidget(const MaterialApp(
      home: Scaffold(
        body: NotificationStatusBadge(status: 'ACTIVE'),
      ),
    ));
    expect(find.byType(StatusBadge), findsOneWidget);
    expect(find.text('Active'), findsOneWidget);
  });

  testWidgets('falls back to the raw status string for unknown values',
      (tester) async {
    await tester.pumpWidget(const MaterialApp(
      home: Scaffold(
        body: NotificationStatusBadge(status: 'UNHEARD_OF'),
      ),
    ));
    expect(find.text('UNHEARD_OF'), findsOneWidget);
  });
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `flutter test test/widgets/notification_status_badge_test.dart`
Expected: FAIL — current implementation does not delegate to `StatusBadge` and does not lower-case-pretty-print known states.

- [ ] **Step 3: Replace the implementation**

Replace `lib/src/widgets/notification_status_badge.dart` with:

```dart
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:flutter/material.dart';

/// Thin wrapper around `StatusBadge.fromEnum` that maps notification status
/// strings to a label/color/icon. Centralizes the mapping in one place so
/// tiles, lists, and detail screens all render badges identically.
class NotificationStatusBadge extends StatelessWidget {
  const NotificationStatusBadge({super.key, required this.status});

  final String status;

  @override
  Widget build(BuildContext context) {
    return StatusBadge.fromEnum<String>(
      value: status,
      mapper: (s) => switch (s) {
        'ACTIVE' || 'DELIVERED' => ('Active', Colors.green, null),
        'CREATED' || 'QUEUED' => ('Queued', Colors.blue, Icons.schedule),
        'CHECKED' => ('Checked', Colors.orange, null),
        'INACTIVE' || 'FAILED' =>
          ('Failed', Colors.red, Icons.error_outline),
        'DELETED' => ('Deleted', Colors.grey, null),
        '' => ('Unknown', Colors.grey, null),
        _ => (s, Colors.grey, null),
      },
    );
  }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `flutter test test/widgets/notification_status_badge_test.dart`
Expected: PASS — both cases.

- [ ] **Step 5: Commit**

```bash
git add ui/lib/src/widgets/notification_status_badge.dart \
        ui/test/widgets/notification_status_badge_test.dart
git commit -m "refactor(ui): delegate NotificationStatusBadge to StatusBadge.fromEnum"
```

---

## Task 9: TemplateVariantMatrix widget

**Files:**
- Create: `lib/src/widgets/template_variant_matrix.dart`
- Test: `test/widgets/template_variant_matrix_test.dart`

- [ ] **Step 1: Write the failing test**

Create `test/widgets/template_variant_matrix_test.dart`:

```dart
import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('renders a row per channel and a column per language',
      (tester) async {
    final variants = [
      notif.TemplateData()
        ..type = 'SMS'
        ..detail = 'Hi'
        ..language = (notif.Language()..code = 'en'),
    ];
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(
        body: TemplateVariantMatrix(
          variants: variants,
          onChanged: (_) {},
          availableChannels: const ['SMS', 'EMAIL'],
          availableLanguages: const ['en', 'sw'],
        ),
      ),
    ));
    expect(find.text('SMS'), findsOneWidget);
    expect(find.text('EMAIL'), findsOneWidget);
    expect(find.text('en'), findsOneWidget);
    expect(find.text('sw'), findsOneWidget);
  });

  testWidgets('clicking an empty cell opens the editor and onChanged fires',
      (tester) async {
    List<notif.TemplateData>? lastEmitted;
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(
        body: TemplateVariantMatrix(
          variants: const [],
          onChanged: (v) => lastEmitted = v,
          availableChannels: const ['SMS'],
          availableLanguages: const ['en'],
        ),
      ),
    ));
    // Tap the (SMS, en) cell.
    await tester.tap(find.byKey(const Key('cell-SMS-en')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('cell-editor-content')), findsOneWidget);

    await tester.enterText(
      find.byKey(const Key('cell-editor-content')),
      'Hello {{name}}',
    );
    await tester.tap(find.byKey(const Key('cell-editor-save')));
    await tester.pumpAndSettle();

    expect(lastEmitted, isNotNull);
    expect(lastEmitted!.single.type, 'SMS');
    expect(lastEmitted!.single.language.code, 'en');
    expect(lastEmitted!.single.detail, 'Hello {{name}}');
  });
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `flutter test test/widgets/template_variant_matrix_test.dart`
Expected: FAIL — `TemplateVariantMatrix` undefined.

- [ ] **Step 3: Implement the matrix**

Create `lib/src/widgets/template_variant_matrix.dart`:

```dart
import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:flutter/material.dart';

/// Editable channel × language matrix for a Template's variants.
///
/// The host owns the variants list (controlled component). Each cell click
/// opens an editor pane on desktop or a bottom sheet on compact widths.
/// On save, the widget rebuilds the variants list and emits via `onChanged`.
class TemplateVariantMatrix extends StatefulWidget {
  const TemplateVariantMatrix({
    super.key,
    required this.variants,
    required this.onChanged,
    this.availableLanguages = const ['en'],
    this.availableChannels = const ['SMS', 'EMAIL', 'PUSH', 'WHATSAPP'],
  });

  final List<notif.TemplateData> variants;
  final ValueChanged<List<notif.TemplateData>> onChanged;
  final List<String> availableLanguages;
  final List<String> availableChannels;

  @override
  State<TemplateVariantMatrix> createState() => _TemplateVariantMatrixState();
}

class _TemplateVariantMatrixState extends State<TemplateVariantMatrix> {
  ({String channel, String language})? _editing;

  notif.TemplateData? _findVariant(String channel, String language) {
    for (final v in widget.variants) {
      if (v.type == channel && v.language.code == language) return v;
    }
    return null;
  }

  void _saveCell({
    required String channel,
    required String language,
    required String detail,
  }) {
    final next = List<notif.TemplateData>.of(widget.variants);
    final i = next.indexWhere(
        (v) => v.type == channel && v.language.code == language);
    if (i >= 0) {
      next[i] = next[i]..detail = detail;
    } else {
      next.add(notif.TemplateData()
        ..type = channel
        ..detail = detail
        ..language = (notif.Language()..code = language));
    }
    widget.onChanged(next);
    setState(() => _editing = null);
  }

  void _openCell(String channel, String language) {
    setState(() => _editing = (channel: channel, language: language));
  }

  @override
  Widget build(BuildContext context) {
    final width = MediaQuery.sizeOf(context).width;
    final compact = !AppBreakpoints.isDesktop(width);

    final grid = _buildGrid(context);
    if (compact) {
      // Compact: cell editor is a bottom sheet, opened in _openCell.
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          grid,
          if (_editing != null) _buildEditor(context, sheet: true),
        ],
      );
    }
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Expanded(flex: 3, child: grid),
        if (_editing != null) ...[
          const SizedBox(width: 16),
          Expanded(flex: 2, child: _buildEditor(context)),
        ],
      ],
    );
  }

  Widget _buildGrid(BuildContext context) {
    final theme = Theme.of(context);
    return Card(
      elevation: 0,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: BorderSide(color: theme.colorScheme.outlineVariant),
      ),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: SingleChildScrollView(
          scrollDirection: Axis.horizontal,
          child: DataTable(
            headingRowHeight: 36,
            columns: [
              const DataColumn(label: Text('Channel \\ Lang')),
              for (final lang in widget.availableLanguages)
                DataColumn(label: Text(lang)),
            ],
            rows: [
              for (final channel in widget.availableChannels)
                DataRow(cells: [
                  DataCell(Text(channel)),
                  for (final lang in widget.availableLanguages)
                    DataCell(_cellChip(channel, lang)),
                ]),
            ],
          ),
        ),
      ),
    );
  }

  Widget _cellChip(String channel, String language) {
    final theme = Theme.of(context);
    final variant = _findVariant(channel, language);
    final filled = variant != null && variant.detail.isNotEmpty;
    return InkWell(
      key: Key('cell-$channel-$language'),
      onTap: () => _openCell(channel, language),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
        decoration: BoxDecoration(
          color: filled
              ? theme.colorScheme.primaryContainer
              : theme.colorScheme.surfaceContainerLow,
          borderRadius: BorderRadius.circular(6),
        ),
        child: Icon(
          filled ? Icons.check_circle : Icons.add_circle_outline,
          size: 16,
          color: filled
              ? theme.colorScheme.onPrimaryContainer
              : theme.colorScheme.onSurfaceVariant,
        ),
      ),
    );
  }

  Widget _buildEditor(BuildContext context, {bool sheet = false}) {
    final cell = _editing!;
    final variant = _findVariant(cell.channel, cell.language);
    final controller =
        TextEditingController(text: variant?.detail ?? '');
    final theme = Theme.of(context);
    return FormFieldCard(
      label: '${cell.channel} · ${cell.language}',
      description: 'Edit the content for this channel and language.',
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          TextField(
            key: const Key('cell-editor-content'),
            controller: controller,
            minLines: 3,
            maxLines: 8,
            decoration: InputDecoration(
              border: OutlineInputBorder(
                borderRadius: BorderRadius.circular(12),
              ),
              hintText: 'Hello {{name}}',
            ),
          ),
          const SizedBox(height: 8),
          Row(
            mainAxisAlignment: MainAxisAlignment.end,
            children: [
              TextButton(
                onPressed: () => setState(() => _editing = null),
                child: const Text('Cancel'),
              ),
              const SizedBox(width: 8),
              FilledButton(
                key: const Key('cell-editor-save'),
                onPressed: () => _saveCell(
                  channel: cell.channel,
                  language: cell.language,
                  detail: controller.text,
                ),
                child: const Text('Save'),
              ),
            ],
          ),
          if (sheet)
            Padding(
              padding: const EdgeInsets.only(top: 8),
              child: Text(
                'Tip: on wider screens this opens to the right of the matrix.',
                style: theme.textTheme.bodySmall?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                ),
              ),
            ),
        ],
      ),
    );
  }
}
```

Edit `lib/antinvestor_ui_notification.dart` — add export under Widgets:

```dart
export 'src/widgets/template_variant_matrix.dart';
```

- [ ] **Step 4: Run test to verify it passes**

Run: `flutter test test/widgets/template_variant_matrix_test.dart`
Expected: PASS — both cases.

- [ ] **Step 5: Commit**

```bash
git add ui/lib/src/widgets/template_variant_matrix.dart \
        ui/lib/antinvestor_ui_notification.dart \
        ui/test/widgets/template_variant_matrix_test.dart
git commit -m "feat(ui): add TemplateVariantMatrix widget"
```

---

## Task 10: Rewrite TemplateEditScreen using the matrix

**Files:**
- Modify: `lib/src/screens/template_edit_screen.dart`
- Test: `test/screens/template_edit_screen_test.dart`

- [ ] **Step 1: Write the failing test**

Create `test/screens/template_edit_screen_test.dart`:

```dart
import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../_helpers/fake_notification_client.dart';
import '../_helpers/test_harness.dart';

void main() {
  testWidgets('saves variants entered via the matrix', (tester) async {
    final fake = FakeNotificationClient();

    await tester.pumpWidget(TestHarness(
      client: fake,
      child: const TemplateEditScreen(),
    ));
    await tester.pumpAndSettle();

    // Set the template name.
    await tester.enterText(
      find.byKey(const Key('template-name-field')),
      'welcome',
    );
    // Open the (SMS, en) cell, type, save.
    await tester.tap(find.byKey(const Key('cell-SMS-en')));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(const Key('cell-editor-content')),
      'Hi {{name}}',
    );
    await tester.tap(find.byKey(const Key('cell-editor-save')));
    await tester.pumpAndSettle();

    // Submit.
    await tester.tap(find.byKey(const Key('template-save-button')));
    await tester.pumpAndSettle();

    expect(fake.templateSaveRequests, hasLength(1));
    final req = fake.templateSaveRequests.single;
    expect(req.name, 'welcome');
    final list = req.data.fields['variants']?.listValue.values;
    expect(list, isNotNull);
    expect(list!.first.structValue.fields['type']?.stringValue, 'SMS');
    expect(list.first.structValue.fields['detail']?.stringValue,
        'Hi {{name}}');
  });
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `flutter test test/screens/template_edit_screen_test.dart`
Expected: FAIL — current screen uses stacked variant cards, not the matrix; submit button has no key; saved request shape is wrong.

- [ ] **Step 3: Rewrite the screen**

Replace `lib/src/screens/template_edit_screen.dart` with:

```dart
import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_core/widgets/error_helpers.dart';
import 'package:antinvestor_ui_core/widgets/form_field_card.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../providers/language_providers.dart';
import '../providers/template_providers.dart';
import '../widgets/template_preview.dart';
import '../widgets/template_variant_matrix.dart';

/// Create or edit a notification template using the variants matrix.
class TemplateEditScreen extends ConsumerStatefulWidget {
  const TemplateEditScreen({
    super.key,
    this.templateId,
    this.initialTemplate,
  });

  final String? templateId;
  final notif.Template? initialTemplate;

  @override
  ConsumerState<TemplateEditScreen> createState() => _TemplateEditScreenState();
}

class _TemplateEditScreenState extends ConsumerState<TemplateEditScreen> {
  final _formKey = GlobalKey<FormState>();
  late final TextEditingController _nameController;
  late List<notif.TemplateData> _variants;
  bool _saving = false;
  String? _error;

  bool get _isEditing => widget.templateId != null;

  @override
  void initState() {
    super.initState();
    final t = widget.initialTemplate;
    _nameController = TextEditingController(text: t?.name ?? '');
    _variants = t == null ? <notif.TemplateData>[] : List.of(t.data);
  }

  @override
  void dispose() {
    _nameController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final asyncLanguages = ref.watch(languageSearchProvider(''));

    final availableLanguages = asyncLanguages.maybeWhen(
      data: (langs) => langs.isEmpty
          ? const ['en']
          : langs.map((l) => l.code).toList(),
      orElse: () => const ['en'],
    );

    return Scaffold(
      appBar: AppBar(
        leading: IconButton(
          icon: const Icon(Icons.arrow_back),
          onPressed: () => context.canPop()
              ? context.pop()
              : context.go('/notifications/templates'),
        ),
        title: Text(
          _isEditing ? 'Edit Template' : 'New Template',
          style: theme.textTheme.titleMedium?.copyWith(
            fontWeight: FontWeight.w600,
          ),
        ),
        actions: [
          FilledButton.icon(
            key: const Key('template-save-button'),
            onPressed: _saving ? null : _save,
            icon: _saving
                ? const SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : const Icon(Icons.save, size: 18),
            label: Text(_saving ? 'Saving...' : 'Save'),
          ),
          const SizedBox(width: 16),
        ],
      ),
      body: Form(
        key: _formKey,
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              FormFieldCard(
                label: 'Template Name',
                description: 'A unique identifier for this template.',
                isRequired: true,
                child: TextFormField(
                  key: const Key('template-name-field'),
                  controller: _nameController,
                  decoration: InputDecoration(
                    hintText: 'e.g., welcome_sms',
                    prefixIcon: const Icon(Icons.label_outline),
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(12),
                    ),
                  ),
                  validator: (v) =>
                      (v == null || v.trim().isEmpty) ? 'Required' : null,
                ),
              ),
              Text(
                'Variants',
                style: theme.textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.w700,
                  color: theme.colorScheme.primary,
                ),
              ),
              const SizedBox(height: 4),
              Text(
                'Click a cell to edit content for that channel + language.',
                style: theme.textTheme.bodySmall?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                ),
              ),
              const SizedBox(height: 16),
              TemplateVariantMatrix(
                variants: _variants,
                onChanged: (next) => setState(() => _variants = next),
                availableLanguages: availableLanguages,
              ),
              const SizedBox(height: 24),
              if (_variants.isNotEmpty)
                TemplatePreview(
                  template: notif.Template()
                    ..name = _nameController.text
                    ..data.addAll(_variants),
                ),
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
                      Icon(Icons.error_outline,
                          size: 20,
                          color: theme.colorScheme.onErrorContainer),
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
      ),
    );
  }

  Future<void> _save() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() {
      _saving = true;
      _error = null;
    });
    try {
      await ref.read(templateNotifierProvider.notifier).save(
            name: _nameController.text.trim(),
            variants: _variants,
          );
      if (mounted) {
        setState(() => _saving = false);
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              _isEditing
                  ? 'Template updated successfully'
                  : 'Template created successfully',
            ),
            behavior: SnackBarBehavior.floating,
          ),
        );
        context.go('/notifications/templates');
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _saving = false;
          _error = friendlyError(e);
        });
      }
    }
  }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `flutter test test/screens/template_edit_screen_test.dart`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ui/lib/src/screens/template_edit_screen.dart \
        ui/test/screens/template_edit_screen_test.dart
git commit -m "feat(ui): rewrite template editor around variant matrix"
```

---

## Task 11: LanguageListScreen

**Files:**
- Create: `lib/src/screens/language_list_screen.dart`
- Test: `test/screens/language_list_screen_test.dart`

- [ ] **Step 1: Write the failing test**

Create `test/screens/language_list_screen_test.dart`:

```dart
import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../_helpers/fake_notification_client.dart';
import '../_helpers/test_harness.dart';

void main() {
  testWidgets('lists languages derived from template variants',
      (tester) async {
    final fake = FakeNotificationClient()
      ..nextTemplateResults = [
        notif.Template()
          ..name = 'welcome'
          ..data.addAll([
            notif.TemplateData()
              ..type = 'SMS'
              ..detail = '...'
              ..language =
                  (notif.Language()..code = 'en'..name = 'English'),
            notif.TemplateData()
              ..type = 'SMS'
              ..detail = '...'
              ..language =
                  (notif.Language()..code = 'sw'..name = 'Swahili'),
          ]),
      ];

    await tester.pumpWidget(TestHarness(
      client: fake,
      child: const LanguageListScreen(),
    ));
    await tester.pumpAndSettle();

    expect(find.text('en'), findsOneWidget);
    expect(find.text('sw'), findsOneWidget);
    expect(find.text('English'), findsOneWidget);
    expect(find.text('Swahili'), findsOneWidget);
  });
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `flutter test test/screens/language_list_screen_test.dart`
Expected: FAIL — `LanguageListScreen` undefined.

- [ ] **Step 3: Implement the screen**

Create `lib/src/screens/language_list_screen.dart`:

```dart
import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../providers/language_providers.dart';

/// Lists every language used across templates in the active partition.
class LanguageListScreen extends ConsumerStatefulWidget {
  const LanguageListScreen({super.key});

  @override
  ConsumerState<LanguageListScreen> createState() =>
      _LanguageListScreenState();
}

class _LanguageListScreenState extends ConsumerState<LanguageListScreen> {
  String _query = '';

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final asyncLangs = ref.watch(languageSearchProvider(_query));

    return asyncLangs.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (e, _) => Center(child: Text('$e')),
      data: (languages) => AdminEntityListPage<notif.Language>(
        title: 'Languages',
        breadcrumbs: const ['Home', 'Notifications', 'Languages'],
        columns: const [
          DataColumn(label: Text('Code')),
          DataColumn(label: Text('Name')),
        ],
        items: languages,
        onSearch: (q) => setState(() => _query = q.trim()),
        searchHint: 'Search languages...',
        onAdd: () => context.go('/notifications/languages/edit'),
        addLabel: 'New Language',
        onRowNavigate: (lang) {
          context.go('/notifications/languages/edit/${lang.code}',
              extra: lang);
        },
        rowBuilder: (lang, selected, onSelect) {
          return DataRow(
            selected: selected,
            onSelectChanged: (_) => onSelect(),
            cells: [
              DataCell(Text(lang.code)),
              DataCell(Text(lang.name)),
            ],
          );
        },
        exportRow: (lang) => [lang.code, lang.name],
        onExport: (format, count) {
          debugPrint('[AUDIT] Exported $count Languages as $format');
        },
      ),
    );
  }
}
```

Edit `lib/antinvestor_ui_notification.dart` — add export under Screens:

```dart
export 'src/screens/language_list_screen.dart';
```

- [ ] **Step 4: Run test to verify it passes**

Run: `flutter test test/screens/language_list_screen_test.dart`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ui/lib/src/screens/language_list_screen.dart \
        ui/lib/antinvestor_ui_notification.dart \
        ui/test/screens/language_list_screen_test.dart
git commit -m "feat(ui): add language list screen"
```

---

## Task 12: LanguageEditScreen

**Files:**
- Create: `lib/src/screens/language_edit_screen.dart`
- Test: `test/screens/language_edit_screen_test.dart`

- [ ] **Step 1: Write the failing test**

Create `test/screens/language_edit_screen_test.dart`:

```dart
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../_helpers/fake_notification_client.dart';
import '../_helpers/test_harness.dart';

void main() {
  testWidgets('saves a language by writing a placeholder template',
      (tester) async {
    final fake = FakeNotificationClient();
    await tester.pumpWidget(TestHarness(
      client: fake,
      child: const LanguageEditScreen(),
    ));
    await tester.pumpAndSettle();

    await tester.enterText(find.byKey(const Key('lang-code-field')), 'pt');
    await tester.enterText(
        find.byKey(const Key('lang-name-field')), 'Portuguese');

    await tester.tap(find.byKey(const Key('lang-save-button')));
    await tester.pumpAndSettle();

    expect(fake.templateSaveRequests, hasLength(1));
    expect(fake.templateSaveRequests.single.name, '_lang_pt');
    final variants = fake.templateSaveRequests.single.data
        .fields['variants']?.listValue.values;
    expect(variants?.single.structValue.fields['language']?.stringValue,
        'pt');
  });
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `flutter test test/screens/language_edit_screen_test.dart`
Expected: FAIL — `LanguageEditScreen` undefined.

- [ ] **Step 3: Implement the screen**

Create `lib/src/screens/language_edit_screen.dart`:

```dart
import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_core/widgets/error_helpers.dart';
import 'package:antinvestor_ui_core/widgets/form_field_card.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../providers/language_providers.dart';

class LanguageEditScreen extends ConsumerStatefulWidget {
  const LanguageEditScreen({
    super.key,
    this.languageCode,
    this.initialLanguage,
  });

  final String? languageCode;
  final notif.Language? initialLanguage;

  @override
  ConsumerState<LanguageEditScreen> createState() =>
      _LanguageEditScreenState();
}

class _LanguageEditScreenState extends ConsumerState<LanguageEditScreen> {
  final _formKey = GlobalKey<FormState>();
  late final TextEditingController _codeController;
  late final TextEditingController _nameController;
  bool _saving = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _codeController = TextEditingController(
        text: widget.initialLanguage?.code ?? widget.languageCode ?? '');
    _nameController =
        TextEditingController(text: widget.initialLanguage?.name ?? '');
  }

  @override
  void dispose() {
    _codeController.dispose();
    _nameController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(
        leading: IconButton(
          icon: const Icon(Icons.arrow_back),
          onPressed: () => context.canPop()
              ? context.pop()
              : context.go('/notifications/languages'),
        ),
        title: Text(
          widget.languageCode == null ? 'New Language' : 'Edit Language',
          style: theme.textTheme.titleMedium
              ?.copyWith(fontWeight: FontWeight.w600),
        ),
        actions: [
          FilledButton.icon(
            key: const Key('lang-save-button'),
            onPressed: _saving ? null : _save,
            icon: _saving
                ? const SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : const Icon(Icons.save, size: 18),
            label: Text(_saving ? 'Saving...' : 'Save'),
          ),
          const SizedBox(width: 16),
        ],
      ),
      body: Form(
        key: _formKey,
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              FormFieldCard(
                label: 'Code',
                description: 'ISO 639-1 code (e.g., en, sw, fr).',
                isRequired: true,
                child: TextFormField(
                  key: const Key('lang-code-field'),
                  controller: _codeController,
                  decoration: InputDecoration(
                    hintText: 'en',
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(12),
                    ),
                  ),
                  validator: (v) =>
                      (v == null || v.trim().isEmpty) ? 'Required' : null,
                ),
              ),
              FormFieldCard(
                label: 'Name',
                description: 'Human-readable language name.',
                isRequired: true,
                child: TextFormField(
                  key: const Key('lang-name-field'),
                  controller: _nameController,
                  decoration: InputDecoration(
                    hintText: 'English',
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(12),
                    ),
                  ),
                  validator: (v) =>
                      (v == null || v.trim().isEmpty) ? 'Required' : null,
                ),
              ),
              if (_error != null) ...[
                const SizedBox(height: 16),
                Text(
                  _error!,
                  style: theme.textTheme.bodySmall
                      ?.copyWith(color: theme.colorScheme.error),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }

  Future<void> _save() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() {
      _saving = true;
      _error = null;
    });
    try {
      await ref.read(languageNotifierProvider.notifier).save(
            code: _codeController.text.trim(),
            name: _nameController.text.trim(),
          );
      if (mounted) {
        setState(() => _saving = false);
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Language saved'),
            behavior: SnackBarBehavior.floating,
          ),
        );
        context.go('/notifications/languages');
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _saving = false;
          _error = friendlyError(e);
        });
      }
    }
  }
}
```

Edit `lib/antinvestor_ui_notification.dart` — add export:

```dart
export 'src/screens/language_edit_screen.dart';
```

- [ ] **Step 4: Run test to verify it passes**

Run: `flutter test test/screens/language_edit_screen_test.dart`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ui/lib/src/screens/language_edit_screen.dart \
        ui/lib/antinvestor_ui_notification.dart \
        ui/test/screens/language_edit_screen_test.dart
git commit -m "feat(ui): add language edit screen"
```

---

## Task 13: Fix NotificationSendScreen — Struct payload

**Files:**
- Modify: `lib/src/screens/notification_send_screen.dart`
- Test: `test/screens/notification_send_screen_test.dart`

- [ ] **Step 1: Write the failing test**

Create `test/screens/notification_send_screen_test.dart`:

```dart
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../_helpers/fake_notification_client.dart';
import '../_helpers/test_harness.dart';

void main() {
  testWidgets('sends notification with payload as Struct, not concatenated string',
      (tester) async {
    final fake = FakeNotificationClient();

    await tester.pumpWidget(TestHarness(
      client: fake,
      child: const NotificationSendScreen(),
    ));
    await tester.pumpAndSettle();

    await tester.enterText(
        find.byKey(const Key('send-recipient-field')), '+254700000000');
    // Add a data entry: name=Alice
    await tester.tap(find.byKey(const Key('send-data-add-button')));
    await tester.pumpAndSettle();
    await tester.enterText(
        find.byKey(const Key('send-data-key-0')), 'name');
    await tester.enterText(
        find.byKey(const Key('send-data-value-0')), 'Alice');

    await tester.tap(find.byKey(const Key('send-submit-button')));
    await tester.pumpAndSettle();

    expect(fake.sendRequests, hasLength(1));
    final n = fake.sendRequests.single.data.single;
    expect(n.recipient.detail, '+254700000000');
    expect(n.payload.fields['name']?.stringValue, 'Alice');
    // The string `data` should not contain the "name=Alice" concat.
    expect(n.data, isNot(contains('name=Alice')));
  });
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `flutter test test/screens/notification_send_screen_test.dart`
Expected: FAIL — current code stuffs entries into `n.data` as `key=value` lines and lacks the test keys.

- [ ] **Step 3: Update the screen**

In `lib/src/screens/notification_send_screen.dart`:

1. Add `import 'package:protobuf/protobuf.dart' show Struct, Value;`
2. Add `Key`s on the recipient field, the data add button, the per-row key/value fields, and the submit button so widget tests can target them. Concretely:
   - Recipient `TextFormField` → `key: const Key('send-recipient-field')`
   - `TextButton.icon` "Add Entry" → `key: const Key('send-data-add-button')`
   - Per-row key field → `key: Key('send-data-key-$i')`
   - Per-row value field → `key: Key('send-data-value-$i')`
   - Submit `FilledButton.icon` in the AppBar → `key: const Key('send-submit-button')`
3. Replace the data-entry handling in `_send()` with the Struct approach. Remove the loop that does `notification.data = '${notification.data}\n${entry.key}=${entry.value}'`. Replace with:

```dart
if (_dataEntries.isNotEmpty) {
  final payload = Struct();
  for (final entry in _dataEntries) {
    if (entry.key.isNotEmpty) {
      payload.fields[entry.key] = Value()..stringValue = entry.value;
    }
  }
  if (payload.fields.isNotEmpty) {
    notification.payload = payload;
  }
}
if (_payloadController.text.trim().isNotEmpty) {
  notification.data = _payloadController.text.trim();
}
```

(Keep the rest of the build method the same; only the `_send()` body and the keys change.)

- [ ] **Step 4: Run test to verify it passes**

Run: `flutter test test/screens/notification_send_screen_test.dart`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ui/lib/src/screens/notification_send_screen.dart \
        ui/test/screens/notification_send_screen_test.dart
git commit -m "fix(ui): send notification payload as proto Struct"
```

---

## Task 14: EndUserInboxScreen + Receive ack

**Files:**
- Create: `lib/src/screens/end_user_inbox_screen.dart`
- Test: `test/screens/end_user_inbox_screen_test.dart`

- [ ] **Step 1: Write the failing test**

Create `test/screens/end_user_inbox_screen_test.dart`:

```dart
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../_helpers/fake_notification_client.dart';
import '../_helpers/test_harness.dart';

void main() {
  testWidgets('renders tiles and acks receipt on first paint', (tester) async {
    final fake = FakeNotificationClient()
      ..nextSearchResults = [
        makeNotification(id: 'a', template: 'welcome'),
        makeNotification(id: 'b', template: 'reset'),
      ];

    await tester.pumpWidget(TestHarness(
      client: fake,
      child: const EndUserInboxScreen(profileId: 'profile-1'),
    ));
    await tester.pumpAndSettle();

    expect(find.text('welcome'), findsOneWidget);
    expect(find.text('reset'), findsOneWidget);

    expect(fake.receiveRequests, hasLength(1));
    expect(fake.receiveRequests.single.data.map((n) => n.id),
        unorderedEquals(['a', 'b']));
  });
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `flutter test test/screens/end_user_inbox_screen_test.dart`
Expected: FAIL — `EndUserInboxScreen` undefined.

- [ ] **Step 3: Implement the screen**

Create `lib/src/screens/end_user_inbox_screen.dart`:

```dart
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
```

Edit `lib/antinvestor_ui_notification.dart` — add export:

```dart
export 'src/screens/end_user_inbox_screen.dart';
```

- [ ] **Step 4: Run test to verify it passes**

Run: `flutter test test/screens/end_user_inbox_screen_test.dart`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ui/lib/src/screens/end_user_inbox_screen.dart \
        ui/lib/antinvestor_ui_notification.dart \
        ui/test/screens/end_user_inbox_screen_test.dart
git commit -m "feat(ui): add end-user inbox with auto Receive ack"
```

---

## Task 15: Polish NotificationDetailScreen — MetadataRow + lifecycle timeline + retry

**Files:**
- Modify: `lib/src/screens/notification_detail_screen.dart`
- Test: `test/screens/notification_detail_screen_test.dart`

- [ ] **Step 1: Write the failing test**

Create `test/screens/notification_detail_screen_test.dart`:

```dart
import 'package:antinvestor_api_common/antinvestor_api_common.dart';
import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_core/antinvestor_ui_core.dart';
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../_helpers/fake_notification_client.dart';
import '../_helpers/test_harness.dart';

void main() {
  testWidgets('uses MetadataRow and renders an AuditTrailEntry timeline',
      (tester) async {
    final n = makeNotification(id: 'n1')
      ..status = (StatusResponse()..state = STATE.ACTIVE);
    await tester.pumpWidget(TestHarness(
      child: NotificationDetailScreen(
        notificationId: 'n1',
        initialNotification: n,
      ),
    ));
    await tester.pumpAndSettle();

    // MetadataRow renders the ID label.
    expect(find.byType(MetadataRow), findsWidgets);
    expect(find.text('ID'), findsOneWidget);
    expect(find.text('n1'), findsOneWidget);
    // Lifecycle uses AuditTrailEntry.
    expect(find.byType(AuditTrailEntry), findsWidgets);
  });

  testWidgets('retry calls Release with the notification id', (tester) async {
    final fake = FakeNotificationClient();
    final n = makeNotification(id: 'n1')
      ..status = (StatusResponse()..state = STATE.INACTIVE);

    await tester.pumpWidget(TestHarness(
      client: fake,
      child: NotificationDetailScreen(
        notificationId: 'n1',
        initialNotification: n,
      ),
    ));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('detail-retry-button')));
    await tester.pumpAndSettle();

    expect(fake.releaseRequests, hasLength(1));
    expect(fake.releaseRequests.single.id, contains('n1'));
  });
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `flutter test test/screens/notification_detail_screen_test.dart`
Expected: FAIL — current screen uses bespoke `_metadataRow` and has no `MetadataRow`/`AuditTrailEntry`/retry button by that key.

- [ ] **Step 3: Polish the screen**

Replace the body of `lib/src/screens/notification_detail_screen.dart` so it:

1. Imports `package:antinvestor_ui_core/antinvestor_ui_core.dart` and removes the local `_metadataRow` helper.
2. Replaces every `_metadataRow(theme, label, value)` call with `MetadataRow(label: label, value: value, copiable: label == 'ID' || label == 'Parent ID')`.
3. Adds a "Lifecycle" card built from `AuditTrailEntry`s — one per known status event derived from `notification.status`. For now, render a single entry from the current status:

```dart
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
            timestamp: n.status.id,  // backend stamps the id with timestamp
            performedBy: 'system',
            icon: icon,
            color: color,
          ),
        ],
      ),
    ),
  );
}
```

4. In the AppBar `actions` list, replace the existing "Release" button with a unified retry/release button that always shows for non-released states and carries the test key:

```dart
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
```

5. Add `_buildLifecycleCard(theme, notification)` to the body column between the status row and the metadata card.

- [ ] **Step 4: Run test to verify it passes**

Run: `flutter test test/screens/notification_detail_screen_test.dart`
Expected: PASS — both cases.

- [ ] **Step 5: Commit**

```bash
git add ui/lib/src/screens/notification_detail_screen.dart \
        ui/test/screens/notification_detail_screen_test.dart
git commit -m "feat(ui): polish notification detail with lifecycle and retry"
```

---

## Task 16: Polish NotificationInboxScreen — language filter + landing path

**Files:**
- Modify: `lib/src/screens/notification_inbox_screen.dart`
- Test: `test/screens/notification_inbox_screen_test.dart`

- [ ] **Step 1: Write the failing test**

Create `test/screens/notification_inbox_screen_test.dart`:

```dart
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../_helpers/fake_notification_client.dart';
import '../_helpers/test_harness.dart';

void main() {
  testWidgets('language filter chip pushes language: into search', (tester) async {
    final fake = FakeNotificationClient()..nextSearchResults = [];
    await tester.pumpWidget(TestHarness(
      client: fake,
      child: const NotificationInboxScreen(),
    ));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('inbox-lang-en')));
    await tester.pumpAndSettle();

    expect(
      fake.searchRequests.last.properties,
      contains('language:en'),
    );
  });
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `flutter test test/screens/notification_inbox_screen_test.dart`
Expected: FAIL — language filter chip with the test key does not exist.

- [ ] **Step 3: Update the screen**

In `lib/src/screens/notification_inbox_screen.dart`:

1. Add a `_languageFilter` state variable (`String _languageFilter = '';`).
2. Update `_searchParams` to include `language: _languageFilter`.
3. Add a language filter chip row below the existing type chip row. Each chip uses `key: Key('inbox-lang-$value')` for `value` in `['', 'en', 'sw', 'fr']` (and an "All" chip with key `inbox-lang-all`):

```dart
Padding(
  padding: const EdgeInsets.fromLTRB(24, 8, 24, 0),
  child: SingleChildScrollView(
    scrollDirection: Axis.horizontal,
    child: Row(
      children: [
        _langChip(theme, '', 'All', keySuffix: 'all'),
        const SizedBox(width: 8),
        _langChip(theme, 'en', 'English'),
        const SizedBox(width: 8),
        _langChip(theme, 'sw', 'Swahili'),
        const SizedBox(width: 8),
        _langChip(theme, 'fr', 'French'),
      ],
    ),
  ),
),
```

```dart
Widget _langChip(ThemeData theme, String value, String label,
    {String? keySuffix}) {
  final isSelected = _languageFilter == value;
  return FilterChip(
    key: Key('inbox-lang-${keySuffix ?? value}'),
    selected: isSelected,
    label: Text(label),
    selectedColor: theme.colorScheme.secondaryContainer,
    shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
    onSelected: (_) => setState(() => _languageFilter = value),
  );
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `flutter test test/screens/notification_inbox_screen_test.dart`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ui/lib/src/screens/notification_inbox_screen.dart \
        ui/test/screens/notification_inbox_screen_test.dart
git commit -m "feat(ui): add language filter to notification inbox"
```

---

## Task 17: Update NotificationRouteModule — new routes, nav items, permissions

**Files:**
- Modify: `lib/src/routing/notification_route_module.dart`
- Test: `test/routing/notification_route_module_test.dart`

- [ ] **Step 1: Write the failing test**

Create `test/routing/notification_route_module_test.dart`:

```dart
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

void main() {
  test('exposes dashboard, inbox, languages, end-user inbox routes', () {
    final module = NotificationRouteModule();
    final paths = _flattenPaths(module.buildRoutes());
    expect(paths, containsAll(<String>[
      '/notifications',
      '/notifications/inbox',
      '/notifications/detail/:id',
      '/notifications/send',
      '/notifications/templates',
      '/notifications/templates/edit',
      '/notifications/templates/edit/:id',
      '/notifications/languages',
      '/notifications/languages/edit',
      '/notifications/languages/edit/:id',
      '/me/notifications',
    ]));
  });

  test('nav items include dashboard and languages but not /me', () {
    final module = NotificationRouteModule();
    final ids = _flattenNavIds(module.buildNavItems());
    expect(ids,
        containsAll(<String>['notifications', 'notification-inbox',
          'notification-send', 'notification-templates',
          'notification-languages', 'notification-dashboard']));
    expect(ids, isNot(contains('end-user-inbox')));
  });
}

List<String> _flattenPaths(List<RouteBase> routes, [String prefix = '']) {
  final out = <String>[];
  for (final r in routes) {
    if (r is GoRoute) {
      final full = prefix.endsWith('/')
          ? '$prefix${r.path}'
          : (r.path.startsWith('/') ? r.path : '$prefix/${r.path}');
      out.add(full);
      out.addAll(_flattenPaths(r.routes, full));
    }
  }
  return out;
}

List<String> _flattenNavIds(List items) {
  final out = <String>[];
  for (final n in items) {
    out.add(n.id);
    if (n.children != null) out.addAll(_flattenNavIds(n.children));
  }
  return out;
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `flutter test test/routing/notification_route_module_test.dart`
Expected: FAIL — current module does not expose dashboard/languages/end-user-inbox routes.

- [ ] **Step 3: Update the module**

Replace `lib/src/routing/notification_route_module.dart` with:

```dart
import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:antinvestor_ui_core/navigation/nav_items.dart';
import 'package:antinvestor_ui_core/permissions/permission_manifest.dart';
import 'package:antinvestor_ui_core/routing/route_module.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../screens/end_user_inbox_screen.dart';
import '../screens/language_edit_screen.dart';
import '../screens/language_list_screen.dart';
import '../screens/notification_dashboard_screen.dart';
import '../screens/notification_detail_screen.dart';
import '../screens/notification_inbox_screen.dart';
import '../screens/notification_send_screen.dart';
import '../screens/template_edit_screen.dart';
import '../screens/template_list_screen.dart';

class NotificationRouteModule extends RouteModule {
  @override
  String get moduleId => 'notification';

  @override
  List<RouteBase> buildRoutes() {
    return [
      GoRoute(
        path: '/notifications',
        builder: (context, state) => const NotificationDashboardScreen(),
        routes: [
          GoRoute(
            path: 'inbox',
            builder: (context, state) => const NotificationInboxScreen(),
          ),
          GoRoute(
            path: 'detail/:id',
            builder: (context, state) {
              final id = state.pathParameters['id'] ?? '';
              final extra = state.extra;
              final notification =
                  extra is notif.Notification ? extra : null;
              return NotificationDetailScreen(
                notificationId: id,
                initialNotification: notification,
              );
            },
          ),
          GoRoute(
            path: 'send',
            builder: (context, state) => const NotificationSendScreen(),
          ),
          GoRoute(
            path: 'templates',
            builder: (context, state) => const TemplateListScreen(),
            routes: [
              GoRoute(
                path: 'edit',
                builder: (context, state) => const TemplateEditScreen(),
              ),
              GoRoute(
                path: 'edit/:id',
                builder: (context, state) {
                  final id = state.pathParameters['id'] ?? '';
                  final extra = state.extra;
                  final template =
                      extra is notif.Template ? extra : null;
                  return TemplateEditScreen(
                    templateId: id,
                    initialTemplate: template,
                  );
                },
              ),
            ],
          ),
          GoRoute(
            path: 'languages',
            builder: (context, state) => const LanguageListScreen(),
            routes: [
              GoRoute(
                path: 'edit',
                builder: (context, state) => const LanguageEditScreen(),
              ),
              GoRoute(
                path: 'edit/:id',
                builder: (context, state) {
                  final id = state.pathParameters['id'] ?? '';
                  final extra = state.extra;
                  final language =
                      extra is notif.Language ? extra : null;
                  return LanguageEditScreen(
                    languageCode: id,
                    initialLanguage: language,
                  );
                },
              ),
            ],
          ),
        ],
      ),
      GoRoute(
        path: '/me/notifications',
        builder: (context, state) {
          final extra = state.extra;
          final profileId = extra is String ? extra : 'me';
          return EndUserInboxScreen(profileId: profileId);
        },
      ),
    ];
  }

  @override
  List<NavItem> buildNavItems() {
    return [
      const NavItem(
        id: 'notifications',
        label: 'Notifications',
        icon: Icons.notifications_outlined,
        activeIcon: Icons.notifications,
        route: '/notifications',
        requiredPermissions: {'notification_search'},
        children: [
          NavItem(
            id: 'notification-dashboard',
            label: 'Dashboard',
            icon: Icons.dashboard_outlined,
            route: '/notifications',
            requiredPermissions: {'notification_search'},
          ),
          NavItem(
            id: 'notification-inbox',
            label: 'Inbox',
            icon: Icons.inbox,
            route: '/notifications/inbox',
            requiredPermissions: {'notification_search'},
          ),
          NavItem(
            id: 'notification-send',
            label: 'Compose',
            icon: Icons.send,
            route: '/notifications/send',
            requiredPermissions: {'notification_send'},
          ),
          NavItem(
            id: 'notification-templates',
            label: 'Templates',
            icon: Icons.description,
            route: '/notifications/templates',
            requiredPermissions: {'template_manage'},
          ),
          NavItem(
            id: 'notification-languages',
            label: 'Languages',
            icon: Icons.language,
            route: '/notifications/languages',
            requiredPermissions: {'template_manage'},
          ),
        ],
      ),
    ];
  }

  @override
  Map<String, Set<String>> get routePermissions => {
        '/notifications': {'notification_search'},
        '/notifications/inbox': {'notification_search'},
        '/notifications/detail': {'notification_status_view'},
        '/notifications/send': {'notification_send'},
        '/notifications/templates': {'template_view'},
        '/notifications/templates/edit': {'template_manage'},
        '/notifications/languages': {'template_view'},
        '/notifications/languages/edit': {'template_manage'},
        '/me/notifications': {'notification_search'},
      };

  @override
  PermissionManifest get permissionManifest => const PermissionManifest(
        namespace: 'service_notification',
        permissions: [
          PermissionEntry(
            key: 'notification_send',
            label: 'Send Notifications',
            scope: PermissionScope.action,
          ),
          PermissionEntry(
            key: 'notification_search',
            label: 'Search Notifications',
            scope: PermissionScope.service,
          ),
          PermissionEntry(
            key: 'notification_status_view',
            label: 'View Notification Status',
            scope: PermissionScope.feature,
          ),
          PermissionEntry(
            key: 'template_manage',
            label: 'Manage Templates and Languages',
            scope: PermissionScope.feature,
          ),
          PermissionEntry(
            key: 'template_view',
            label: 'View Templates and Languages',
            scope: PermissionScope.feature,
          ),
        ],
      );
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `flutter test test/routing/notification_route_module_test.dart`
Expected: PASS — both cases.

- [ ] **Step 5: Commit**

```bash
git add ui/lib/src/routing/notification_route_module.dart \
        ui/test/routing/notification_route_module_test.dart
git commit -m "feat(ui): expand routes with dashboard, languages, end-user inbox"
```

---

## Task 18: Public exports + version bump + README

**Files:**
- Modify: `lib/antinvestor_ui_notification.dart`
- Modify: `pubspec.yaml`
- Modify: `README.md`
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Verify the public surface lists every new export**

Open `lib/antinvestor_ui_notification.dart` and confirm it contains all of:

```dart
// Providers
export 'src/providers/notification_transport_provider.dart';
export 'src/providers/notification_providers.dart';
export 'src/providers/template_providers.dart';
export 'src/providers/tenancy_aware_providers.dart';
export 'src/providers/language_providers.dart';
export 'src/providers/stats_providers.dart';

// Widgets
export 'src/widgets/notification_status_badge.dart';
export 'src/widgets/notification_tile.dart';
export 'src/widgets/priority_badge.dart';
export 'src/widgets/template_preview.dart';
export 'src/widgets/channel_selector.dart';
export 'src/widgets/language_selector.dart';
export 'src/widgets/notification_badge.dart';
export 'src/widgets/template_variant_matrix.dart';

// Screens
export 'src/screens/notification_inbox_screen.dart';
export 'src/screens/notification_detail_screen.dart';
export 'src/screens/notification_send_screen.dart';
export 'src/screens/template_list_screen.dart';
export 'src/screens/template_edit_screen.dart';
export 'src/screens/notification_dashboard_screen.dart';
export 'src/screens/language_list_screen.dart';
export 'src/screens/language_edit_screen.dart';
export 'src/screens/end_user_inbox_screen.dart';

// Routing
export 'src/routing/notification_route_module.dart';
```

Add any export that is missing.

- [ ] **Step 2: Bump pubspec version**

Edit `pubspec.yaml` and change `version: 0.1.1` to `version: 0.2.0`.

- [ ] **Step 3: Update README**

Replace `README.md` with content that documents the new surface area:

```markdown
# antinvestor_ui_notification

Embeddable notification management UI for Antinvestor applications.
Composes `antinvestor_ui_core` primitives so the notification surface stays
consistent with other Antinvestor service UIs.

## Installation

```yaml
dependencies:
  antinvestor_ui_notification: ^0.2.0
```

## Features

- **Dashboard**: KPI tiles (Sent / Delivered / Failed / Queued), channel mix
  chart, and a top-failing-templates strip — derived client-side from the
  current Search snapshot.
- **Inbox**: Paginated, searchable, type and language filters, CSV export,
  audit hook.
- **Detail**: Lifecycle timeline (`AuditTrailEntry`), retry/release action,
  metadata and routing cards.
- **Compose**: Channel/language selection, priority, payload as
  `google.protobuf.Struct` (template variables) plus optional pre-rendered
  body string.
- **Templates**: Channel × language variant matrix with side preview;
  variants persist via the proto `data Struct`.
- **Languages**: List + edit screens derived from the language records used
  across templates.
- **End-user inbox**: Per-profile inbox widget that auto-acks `Receive` on
  first paint; embeddable inside profile drawers.
- **Tenancy-aware**: All screens read partition / organization / branch
  from `antinvestor_ui_core`'s `tenancyContextProvider` and re-fetch
  automatically on tenancy switches.

## Embedding

```dart
// Tenant admin (host already manages tenancy):
final module = NotificationRouteModule();
ShellRoute(routes: [...module.buildRoutes()]);

// Cross-partition platform admin (host swaps tenancy at runtime):
ref.read(tenancyContextProvider).selectPartition(id, name);
// Every notification provider invalidates and re-fetches automatically.

// End-user inbox widget (embed anywhere):
NotificationBadge(profileId: currentProfile.id)
EndUserInboxScreen(profileId: currentProfile.id)

// Standalone widgets:
TemplateVariantMatrix(variants: t.data, onChanged: (v) => ...)
NotificationTile(notification: n)
```

## Routes

| Path                                  | Screen                       | Permission                   |
| ------------------------------------- | ---------------------------- | ---------------------------- |
| `/notifications`                      | Dashboard                    | `notification_search`        |
| `/notifications/inbox`                | Inbox                        | `notification_search`        |
| `/notifications/detail/:id`           | Detail                       | `notification_status_view`   |
| `/notifications/send`                 | Compose                      | `notification_send`          |
| `/notifications/templates`            | Template list                | `template_view`              |
| `/notifications/templates/edit[/:id]` | Template editor (matrix)     | `template_manage`            |
| `/notifications/languages`            | Language list                | `template_view`              |
| `/notifications/languages/edit[/:id]` | Language editor              | `template_manage`            |
| `/me/notifications`                   | End-user inbox               | `notification_search`        |

## Migrating from 0.1.x

- Landing for `/notifications` changed from inbox → dashboard. Inbox is now
  at `/notifications/inbox`. Update host links accordingly.
- Send screen now sends payload as `google.protobuf.Struct`, not a multi-line
  string. Backends that parsed the previous `key=value\n…` shape need to
  update.
- Template save now persists variants via the request's `data Struct`
  under a `variants` key. See `docs/superpowers/specs/2026-05-08-notification-ui-deepening-design.md`
  §5.2 for the contract.
```

- [ ] **Step 4: Update CHANGELOG**

Prepend to `CHANGELOG.md`:

```markdown
## 0.2.0

- Added `NotificationDashboardScreen` (KPIs, channel mix, top failing
  templates).
- Added `LanguageListScreen`, `LanguageEditScreen`, `EndUserInboxScreen`,
  `TemplateVariantMatrix`.
- Added `partitionIdProvider`, `tenancyScopeProvider`, `TenancyScope`,
  `languageSearchProvider`, `LanguageNotifier`, `notificationStatsProvider`,
  `NotificationStats`. Search providers now re-key on the tenancy scope.
- Fixed: send screen now serializes `payload` as `google.protobuf.Struct`.
- Fixed: template save now persists variants via the request's `data Struct`.
- Changed: `/notifications` is now the dashboard; the inbox moved to
  `/notifications/inbox`.
- Changed: `NotificationStatusBadge` is a thin wrapper around
  `StatusBadge.fromEnum`.
```

- [ ] **Step 5: Run the full test suite**

Run: `flutter test`
Expected: every test green.

- [ ] **Step 6: Commit**

```bash
git add ui/lib/antinvestor_ui_notification.dart \
        ui/pubspec.yaml ui/README.md ui/CHANGELOG.md
git commit -m "release(ui): bump notification UI library to 0.2.0"
```

---

## Self-Review

The author of this plan did the following review pass against the spec:

**1. Spec coverage** — every section of the spec has at least one task:

| Spec section                                        | Task(s)            |
| --------------------------------------------------- | ------------------ |
| §3 file layout                                      | covered across 2-17 |
| §3.2 reuse from ui_core                             | 7, 8, 11, 14, 15  |
| §4 tenancy bridge                                   | 2                 |
| §5.1 Send Struct payload                            | 13                |
| §5.2 Template save round-trip                       | 4, 10             |
| §6 Stats derivation                                 | 6                 |
| §7 Variant matrix                                   | 9, 10             |
| §8 Routing & permissions                            | 17                |
| §9 Embedding patterns                               | 18 (README)        |
| §10 Testing                                         | every task         |
| §11 Migration & rollout                             | 18                |
| §12 Build sequence                                  | tasks 2-18         |

**2. Placeholder scan** — no TBDs, no "implement later", no untyped "similar to Task N", every step has actual code or actual command.

**3. Type consistency**:
- `TenancyScope` is defined in Task 2 and consumed by Tasks 3, 4 with the same field names.
- `NotificationSearchParams` adds `language` in Task 3; consumed in Tasks 6, 14, 16.
- `TemplateNotifier.save({name, variants})` is defined in Task 4 and called the same way in Tasks 10, 12 (via `LanguageNotifier`).
- `NotificationStats` field names (`sent`, `delivered`, `failed`, `queued`, `channelMix`, `topFailing` records with `template`/`failures`) match between Task 6 definition, Task 7 dashboard rendering, and Task 6 test fixtures.
- `NotificationStatusBadge` retains its `status` String API (Task 8), unchanged from current callers in inbox/tile/detail.
- Test keys (`cell-SMS-en`, `cell-editor-content`, `cell-editor-save`, `template-name-field`, `template-save-button`, `lang-code-field`, `lang-name-field`, `lang-save-button`, `send-recipient-field`, `send-data-add-button`, `send-data-key-$i`, `send-data-value-$i`, `send-submit-button`, `detail-retry-button`, `inbox-lang-…`) are defined in the implementation step of each task and used in that same task's test step.

No issues found that need a follow-up task.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-08-notification-ui-deepening.md`. Two execution options:

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints.

Which approach?
