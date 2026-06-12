## 0.3.2

- Fix: the channel selector on `NotificationSendScreen` could never change
  selection — `channels.first` returned the previously selected channel
  (set insertion order), so taps on other chips were no-ops. The newly
  tapped channel is now selected.

## 0.3.0

- `NotificationDashboardScreen` now sources KPIs (sent, delivered, failed,
  queued, avg send time), the sent trend, and the channel mix from the thesa
  analytics gate via `antinvestor_ui_core`'s `analyticsDataSourceProvider`,
  using the `notifications_*` business metrics. The top failing templates
  panel stays derived from the notification search snapshot.
- Added `notificationAnalyticsSpec` (a `ServiceAnalyticsSpec`) for host apps
  to register on their `ThesaAnalyticsDataSource`, plus
  `analyticsGateMessage` for friendly gate error states (400 allowlist,
  403 unscoped, 5xx backend down).
- Requires `antinvestor_ui_core` >= 0.5.0 (unpublished; use a local path
  override during development).

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

(existing 0.1.x entries below)

## 0.1.1

- Migrate providers to Riverpod 3.x Notifier, fix lint warnings and deprecations

## 0.1.0

- Initial release
- Notification UI with inbox, compose, templates, delivery tracking, NotificationBadge widget
