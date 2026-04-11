# antinvestor_ui_notification

Embeddable notification management UI for Antinvestor applications. Provides screens and widgets for sending, receiving, searching notifications, and managing message templates.

## Installation

```yaml
dependencies:
  antinvestor_ui_notification: ^0.1.0
```

## Features

- **Notification Inbox**: Paginated inbox with search, filters, and CSV export
- **Notification Detail**: Full message view with delivery status
- **Send Notification**: Compose and send via multiple channels (SMS, email, push)
- **Template Management**: Create, edit, and preview message templates
- **Embeddable Widgets**: `NotificationStatusBadge`, `NotificationTile`, `PriorityBadge`, `TemplatePreview`, `ChannelSelector`, `LanguageSelector`, `NotificationBadge`
- **Routing**: `NotificationRouteModule` with GoRouter integration

## Usage

```dart
import 'package:antinvestor_ui_notification/antinvestor_ui_notification.dart';

// Unread notification count badge
NotificationBadge(profileId: 'user-123')

// Notification status indicator
NotificationStatusBadge(status: deliveryStatus)

// Register routes in your host app
final module = NotificationRouteModule();
ShellRoute(
  routes: [...ownRoutes, ...module.buildRoutes()],
);
```

## Routes

| Path | Screen |
|------|--------|
| `/notifications` | Notification inbox |
| `/notifications/detail/:id` | Notification detail |
| `/notifications/send` | Compose and send |
| `/notifications/templates` | Template list |
| `/notifications/templates/edit` | Create template |
| `/notifications/templates/edit/:id` | Edit template |

## Embedding Widgets

```dart
// Channel selector (SMS, email, push, etc.)
ChannelSelector(onChanged: (channels) => print(channels))

// Language picker for templates
LanguageSelector(onChanged: (lang) => print(lang))

// Template preview with variable substitution
TemplatePreview(template: templateObject, variables: {'name': 'John'})

// Priority indicator
PriorityBadge(priority: NotificationPriority.HIGH)
```
