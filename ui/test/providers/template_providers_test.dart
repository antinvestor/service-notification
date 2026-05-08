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
    final fake = FakeNotificationClient()
      ..nextSavedTemplate =
          (notif.Template()
            ..id = 't1'
            ..name = 'welcome');
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
