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
    notificationServiceClientProvider.overrideWithValue(fake.client),
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

    final french = await container.read(languageSearchProvider('french').future);
    expect(french.map((l) => l.code), ['fr']);
  });
}
