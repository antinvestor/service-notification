# Notification UI deepening — design

**Status:** draft
**Date:** 2026-05-08
**Author:** Peter Bwire (with Claude Code)
**Scope:** the `ui/` Flutter package (`antinvestor_ui_notification`)

## 1. Goal

Bring the existing notification UI library to a polished, embeddable state that
operators, partition admins, and end users can consume with the same set of
widgets. The library must:

- Read the active partition (and optional org/branch) from the host's
  `tenancyContextProvider` exported by `antinvestor_ui_core`. No new
  library-owned partition provider.
- Provide a partition-scoped admin surface: dashboard, inbox, detail (with
  lifecycle), compose, templates (with variants matrix + preview), languages.
- Provide an end-user inbox widget that auto-acknowledges receipt.
- Expose embeddable widgets for badges, tiles, and the variant matrix.
- Reuse `antinvestor_ui_core` primitives wherever they already exist
  (ServiceAnalyticsPage, AdminEntityListPage, EntityListPage, StatusBadge,
  StateBadge, EntityChip, AuditTrailEntry, MetadataRow, FormFieldCard,
  PageHeader, Breadcrumb, showEditDialog, PlaceholderPage, breakpoints).

## 2. Non-goals

- Real-time push / live tail. The current Search RPC is one-shot streaming
  with no Watch/Subscribe; building a live tail would either require backend
  changes or polling-as-faked-stream. Out of scope for this iteration.
- A new server-side metrics endpoint. KPIs are derived client-side from the
  current Search snapshot.
- Markdown / rich-text template authoring.
- Cross-partition aggregation views. Single active partition at a time
  (the host can swap via `TenancyContext.selectPartition`).

## 3. Architecture

### 3.1 File layout

```
ui/lib/
  antinvestor_ui_notification.dart                # public exports
  src/
    providers/
      notification_transport_provider.dart        # existing
      tenancy_aware_providers.dart                # NEW
      notification_providers.dart                 # MODIFIED
      template_providers.dart                     # MODIFIED
      language_providers.dart                     # NEW
      stats_providers.dart                        # NEW
    screens/
      notification_dashboard_screen.dart          # NEW (landing)
      notification_inbox_screen.dart              # MODIFIED (moved path)
      notification_detail_screen.dart             # MODIFIED
      notification_send_screen.dart               # MODIFIED
      template_list_screen.dart                   # MODIFIED
      template_edit_screen.dart                   # MODIFIED (matrix)
      language_list_screen.dart                   # NEW
      language_edit_screen.dart                   # NEW
      end_user_inbox_screen.dart                  # NEW
    widgets/
      notification_status_badge.dart              # MODIFIED (thin wrapper)
      notification_tile.dart                      # existing
      notification_badge.dart                     # existing
      priority_badge.dart                         # existing
      channel_selector.dart                       # existing
      language_selector.dart                      # MODIFIED (reads providers)
      template_preview.dart                       # MODIFIED (channel mocks)
      template_variant_matrix.dart                # NEW
    routing/
      notification_route_module.dart              # MODIFIED
```

### 3.2 What we reuse from `antinvestor_ui_core`

| Need                                  | ui_core primitive                        |
| ------------------------------------- | ---------------------------------------- |
| Dashboard chrome (KPIs, events)       | `ServiceAnalyticsPage` + `ServiceKpi` + `ServiceEvent` |
| Status pill                           | `StatusBadge.fromEnum(...)`              |
| `STATE` enum from common              | `StateBadge`                             |
| Source/recipient/template links       | `EntityChip` + `EntityChipConfig`        |
| Lifecycle timeline                    | `AuditTrailEntry` composed in a `Column` |
| Detail key/value rows                 | `MetadataRow`                            |
| Page chrome                           | `PageHeader` + `Breadcrumb` (fed by `TenancyContext.breadcrumbs`) |
| Language quick-create modal           | `showEditDialog`                         |
| Inbox/template/language list          | `AdminEntityListPage`                    |
| End-user inbox                        | `EntityListPage<Notification>`           |
| Empty states                          | `PlaceholderPage`                        |
| Tenancy / partition_id source         | `tenancyContextProvider` (ChangeNotifier) |
| Audit hooks                           | `auditContextProvider`                   |
| Permissions / nav / route module      | `RouteModule` base                       |
| Responsive breakpoints                | `responsive/breakpoints.dart`            |
| Compose form chrome                   | `FormFieldCard`                          |

The matrix below was the originally-planned new code that we are NOT writing
because ui_core already provides it:

- `kpi_tile.dart`, `channel_mix_chart.dart`, `recent_activity_strip.dart`
  — consumed via `ServiceAnalyticsPage`.
- `lifecycle_timeline.dart` — composed from `AuditTrailEntry`.
- `partition_scope_banner.dart` — `Breadcrumb(items: tenancy.breadcrumbs)`.
- `notification_status_badge.dart` is reduced to a one-line wrapper around
  `StatusBadge.fromEnum` so existing call sites and exports keep working.

## 4. Tenancy bridge

`tenancyContextProvider` exposes a `ChangeNotifier`. Riverpod won't react to
`notifyListeners()` unless we adapt it. The bridge is small:

```dart
// providers/tenancy_aware_providers.dart
final partitionIdProvider = Provider<String>((ref) {
  final tenancy = ref.watch(tenancyContextProvider);
  void onChange() => ref.invalidateSelf();
  tenancy.addListener(onChange);
  ref.onDispose(() => tenancy.removeListener(onChange));
  return tenancy.partitionId;
});

class TenancyScope {
  const TenancyScope({
    required this.partitionId,
    required this.organizationId,
    required this.branchId,
  });
  final String partitionId;
  final String organizationId;
  final String branchId;
  // == / hashCode by all three fields so .family providers re-key correctly.
}

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

**When to use which:**

- `partitionIdProvider` — read by simple, partition-only consumers (e.g.
  `NotificationBadge`, `EndUserInboxScreen`, header crumbs). Returns a plain
  `String`.
- `tenancyScopeProvider` — read by search-side providers that need to push
  partition + organization + branch into request `properties`. Returns a
  value class with `==`/`hashCode` so `.family` providers re-key on any
  axis change, not just partition.

Every search-side provider in the library `ref.watch(tenancyScopeProvider)`,
pushes the scope into the Search request's `properties` filter, and is marked
`autoDispose` so a partition switch releases the prior cache:

```dart
final notificationSearchProvider = FutureProvider.autoDispose
    .family<List<Notification>, NotificationSearchParams>((ref, params) async {
  final scope = ref.watch(tenancyScopeProvider);
  final client = ref.watch(notificationServiceClientProvider);
  final request = SearchRequest()..query = params.query;
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
  return collectStream<SearchResponse, Notification>(
    stream,
    extract: (r) => r.data,
  );
});
```

Mutations (`Send`, `Release`, `StatusUpdate`, `TemplateSave`, `LanguageSave`)
do not read tenancy themselves — auth context server-side enforces partition
boundaries. After a successful mutation, the notifier invalidates the
relevant search provider so dependent UI re-fetches under the current scope.

## 5. Proto-shape fixes

### 5.1 Send screen — `Struct payload` vs `string data`

The proto distinguishes `payload google.protobuf.Struct` (template variables)
from `string data` (pre-rendered content). The current screen concatenates
key/value entries into the `data` string. Fix:

```dart
final payload = Struct();
for (final entry in _dataEntries) {
  if (entry.key.isNotEmpty) {
    payload.fields[entry.key] = Value()..stringValue = entry.value;
  }
}
notification.payload = payload;
if (_payloadController.text.trim().isNotEmpty) {
  notification.data = _payloadController.text.trim();
}
```

### 5.2 Template save — variants round-trip

`TemplateSaveRequest` exposes `name`, `language_code`, `data Struct`, `extra
Struct`. The current notifier sends only `name` + `languageCode`, dropping
the variants entered in the editor.

Decision: encode the variants inside `data` as

```json
{
  "variants": [
    {"type": "SMS",   "language": "en", "detail": "Hello {{name}}..."},
    {"type": "EMAIL", "language": "en", "detail": "..."}
  ]
}
```

Document the contract at the call site and in the proto repo. If a future
proto revision adds typed `repeated TemplateData data` to
`TemplateSaveRequest`, switch to that without changing the editor.

## 6. Stats derivation

```dart
class NotificationStats {
  const NotificationStats({
    required this.sent,
    required this.delivered,
    required this.failed,
    required this.queued,
    required this.channelMix,
    required this.topFailing,
  });
  final int sent, delivered, failed, queued;
  final Map<String, int> channelMix;
  final List<({String template, int failures})> topFailing;

  factory NotificationStats.empty() => const NotificationStats(
    sent: 0, delivered: 0, failed: 0, queued: 0,
    channelMix: {}, topFailing: [],
  );

  factory NotificationStats.fromList(List<Notification> ns) {
    // Single pass over `ns`:
    //   - increment `sent` for every record (treat the snapshot as "sent").
    //   - increment `delivered`, `failed`, `queued` based on
    //     `n.status.state` mapping to common.STATE
    //     (ACTIVE = delivered, INACTIVE/DELETED = failed, CREATED/CHECKED = queued).
    //   - tally `channelMix[n.type]++`.
    //   - for failed records, group by `n.template` and emit the top 5
    //     entries sorted by failure count desc.
  }
}

final notificationStatsProvider = Provider.autoDispose<NotificationStats>((ref) {
  final asyncNotifs = ref.watch(
    notificationSearchProvider(const NotificationSearchParams()),
  );
  return asyncNotifs.maybeWhen(
    data: NotificationStats.fromList,
    orElse: NotificationStats.empty,
  );
});
```

The dashboard consumes this provider only; no new RPCs. When the backend later
exposes a metrics endpoint, the provider gets swapped without touching the
dashboard screen.

## 7. Template variant matrix

The single genuinely-new widget. Replaces stacked variant cards with a grid
that scales when a template has 4 channels × 6 languages = 24 cells.

### 7.1 Desktop layout (≥ md breakpoint)

```
+-------------------+----+----+----+----+----+
| channel \ lang    | en | sw | fr | es | +  |
+-------------------+----+----+----+----+----+
| SMS               | ●  | ●  | ○  | ●  | +  |
| EMAIL             | ●  | ○  | ○  | ●  | +  |
| PUSH              | ●  | ●  | ●  | ●  | +  |
| WHATSAPP          | ○  | ○  | ○  | ○  | +  |
+-------------------+----+----+----+----+----+
| + add channel                              |
+--------------------------------------------+
```

`●` = filled cell, `○` = missing variant, `+` = add row/column. Clicking a
cell opens an editor pane to the right of the matrix in the same scaffold
(not a modal): a two-column layout `[matrix | editor + preview]` at desktop
widths. The editor uses the existing `FormFieldCard` + `TextFormField`
chrome bound to that `(channel, language)` pair. Empty cells initialize a
new `TemplateData` on first save. On compact widths (< md) the editor
becomes a bottom sheet instead of a side pane.

### 7.2 Compact layout (< md)

Stacked accordion: one channel per `ExpansionTile`, each containing language
sub-rows. Same data model, different presentation.

### 7.3 Public API

```dart
class TemplateVariantMatrix extends StatefulWidget {
  const TemplateVariantMatrix({
    super.key,
    required this.variants,           // List<TemplateData>
    required this.onChanged,          // (List<TemplateData>) -> void
    this.availableLanguages = const ['en'],
    this.availableChannels = const ['SMS', 'EMAIL', 'PUSH', 'WHATSAPP'],
  });
}
```

Host owns the variants list; widget is a controlled component.

### 7.4 Side preview

When a cell editor is open, a side pane reuses the existing
`TemplatePreview` widget with sample variables auto-detected from
`{{var}}` placeholders in the cell text.

### 7.5 Placeholder validation

Regex `\{\{\s*([a-z_][a-z0-9_]*)\s*\}\}` over all variants. If a placeholder
appears in some variants but not others, surface a non-blocking yellow chip:
`{{otp_code}} missing in EMAIL/sw, EMAIL/fr`.

## 8. Routing & permissions

| Path                                         | Screen                              | Permission              |
| -------------------------------------------- | ----------------------------------- | ----------------------- |
| `/notifications`                             | `NotificationDashboardScreen` (NEW) | `notification_search`   |
| `/notifications/inbox`                       | inbox (moved here)                  | `notification_search`   |
| `/notifications/detail/:id`                  | detail                              | `notification_status_view` |
| `/notifications/send`                        | compose                             | `notification_send`     |
| `/notifications/templates`                   | template list                       | `template_view`         |
| `/notifications/templates/edit[/:id]`        | template editor (matrix)            | `template_manage`       |
| `/notifications/languages`                   | language list                       | `template_view`         |
| `/notifications/languages/edit[/:id]`        | language editor                     | `template_manage`       |
| `/me/notifications`                          | end-user inbox                      | `notification_search`   |

Languages reuse `template_manage` / `template_view` keys — Language is a
sub-concept of templating. No new manifest entries.

`buildNavItems()` adds Dashboard, Languages, plus existing Inbox/Compose/
Templates children. The end-user inbox at `/me/notifications` is intentionally
absent from the admin sidebar; it is meant for embedding into a profile drawer.

## 9. Embedding patterns (host-facing)

```dart
// 1. Tenant admin — partition resolved by host's tenancy context.
//    No code change beyond registering the module.
final module = NotificationRouteModule();
ShellRoute(routes: [...module.buildRoutes()]);

// 2. Cross-partition platform admin — host swaps tenancy at runtime.
ref.read(tenancyContextProvider).selectPartition(id, name);
// every notification provider invalidates and re-fetches automatically.

// 3. End-user inbox — embeddable anywhere.
NotificationBadge(profileId: currentProfile.id)            // header bell
EndUserInboxScreen()                                       // drawer body

// 4. Standalone widgets.
TemplateVariantMatrix(variants: t.data, onChanged: (v) => ...)
NotificationTile(notification: n, isUnread: !n.received)
```

## 10. Testing

Per `testing-flutter`:

- **Provider tests**
  - `partitionIdProvider` re-emits when `TenancyContext` notifies.
  - `notificationSearchProvider` includes correct `properties` entries for
    partition, organization, branch, type, language, recipient.
  - `notificationStatsProvider` derives correct counts from a fixture list.
- **Widget tests**
  - `TemplateVariantMatrix`: cell click → editor opens → `onChanged` fires
    with correct variant added/updated.
  - Matrix collapses to accordion below md breakpoint.
  - `NotificationDashboardScreen` renders 4 KPI tiles with stats.
  - `EndUserInboxScreen` calls `Receive` ack on first paint with the
    rendered notifications.
- **Golden tests**: at least one per net-new screen at desktop (1280) and
  mobile (375) widths.

The transport layer is exercised through a fake `NotificationServiceClient`
only for deterministic stream fixtures, per project testing conventions.
No wholesale mocking of the connect transport.

## 11. Migration & rollout

The library is at version `0.1.1` and not yet pinned by external hosts.
Changes ship in one minor version bump (`0.2.0`):

- Public API additions: `partitionIdProvider`, `tenancyScopeProvider`,
  `TenancyScope`, `languageSearchProvider`, `LanguageNotifier`,
  `notificationStatsProvider`, `NotificationStats`,
  `NotificationDashboardScreen`, `LanguageListScreen`, `LanguageEditScreen`,
  `EndUserInboxScreen`, `TemplateVariantMatrix`,
  `NotificationSearchParams.language` (added field).
- Breaking: route landing for `/notifications` changes from inbox to
  dashboard; inbox moves to `/notifications/inbox`. Hosts that linked
  directly to `/notifications` for the inbox need to update to
  `/notifications/inbox`.
- Behavioral: send screen now sends `payload` as `Struct`; template save now
  persists all variants. Both fixes are backward-incompatible from the
  server's perspective only if the server was relying on the previous (buggy)
  shape — which it should not be.

## 12. Build sequence

The implementation plan (next document) sequences these as:

1. Tenancy bridge + provider rewrites (no UI change yet).
2. Stats provider + dashboard screen.
3. Send screen Struct fix + template save fix.
4. Template variant matrix + editor rewrite.
5. Language CRUD providers + screens.
6. End-user inbox + Receive-on-view.
7. Detail screen lifecycle timeline + retry/resend.
8. Inbox polish (filters, empty states, language column).
9. Route module + nav item updates.
10. Tests + goldens, package version bump, README update.
