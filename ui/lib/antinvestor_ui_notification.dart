/// Notification management UI library for Antinvestor.
///
/// Provides embeddable screens, widgets, and Riverpod providers for sending,
/// receiving, searching notifications, and managing templates.
library;

// Providers
export 'src/providers/notification_transport_provider.dart';
export 'src/providers/tenancy_aware_providers.dart';
export 'src/providers/notification_providers.dart';
export 'src/providers/template_providers.dart';
export 'src/providers/language_providers.dart';

// Widgets
export 'src/widgets/notification_status_badge.dart';
export 'src/widgets/notification_tile.dart';
export 'src/widgets/priority_badge.dart';
export 'src/widgets/template_preview.dart';
export 'src/widgets/channel_selector.dart';
export 'src/widgets/language_selector.dart';
export 'src/widgets/notification_badge.dart';

// Screens
export 'src/screens/notification_inbox_screen.dart';
export 'src/screens/notification_detail_screen.dart';
export 'src/screens/notification_send_screen.dart';
export 'src/screens/template_list_screen.dart';
export 'src/screens/template_edit_screen.dart';

// Routing
export 'src/routing/notification_route_module.dart';
