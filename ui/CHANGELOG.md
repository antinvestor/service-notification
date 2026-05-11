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
