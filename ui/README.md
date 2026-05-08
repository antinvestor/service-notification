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
  under a `variants` key. See
  `docs/superpowers/specs/2026-05-08-notification-ui-deepening-design.md`
  §5.2 for the contract.
