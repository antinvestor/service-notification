import 'package:antinvestor_api_notification/antinvestor_api_notification.dart';
import 'package:antinvestor_ui_core/api/api_base.dart';
import 'package:connectrpc/connect.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

const _notificationUrl = String.fromEnvironment(
  'NOTIFICATION_URL',
  defaultValue: 'https://api.antinvestor.com/notification',
);

final notificationTransportProvider = Provider<Transport>((ref) {
  final tokenProvider = ref.watch(authTokenProviderProvider);
  return createTransport(tokenProvider, baseUrl: _notificationUrl);
});

final notificationServiceClientProvider =
    Provider<NotificationServiceClient>((ref) {
  final transport = ref.watch(notificationTransportProvider);
  return NotificationServiceClient(transport);
});
