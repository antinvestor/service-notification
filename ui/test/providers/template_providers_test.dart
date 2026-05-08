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
        ..language = (notif.Language()
          ..code = 'en'
          ..name = 'English'),
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
    expect(list[0].structValue.fields['languageName']?.stringValue, 'English');
    expect(list[0].structValue.fields['detail']?.stringValue, 'Hi {{name}}');
    expect(list[1].structValue.fields['type']?.stringValue, 'EMAIL');
  });

  test('decodeTemplateVariants reads back from Struct fallback', () {
    // Build a Template as if returned by a backend that does NOT populate
    // the typed `data` field but does echo the variants in `extra`.
    final variantStruct = notif.Struct()
      ..fields['type'] = (notif.Value()..stringValue = 'SMS')
      ..fields['language'] = (notif.Value()..stringValue = 'en')
      ..fields['languageName'] = (notif.Value()..stringValue = 'English')
      ..fields['detail'] = (notif.Value()..stringValue = 'Hi {{name}}');
    final extra = notif.Struct()
      ..fields['variants'] = (notif.Value()
        ..listValue = (notif.ListValue()
          ..values.add(notif.Value()..structValue = variantStruct)));
    final template = notif.Template()
      ..name = 'welcome'
      ..extra = extra;

    final variants = decodeTemplateVariants(template);
    expect(variants, hasLength(1));
    expect(variants.single.type, 'SMS');
    expect(variants.single.language.code, 'en');
    expect(variants.single.language.name, 'English');
    expect(variants.single.detail, 'Hi {{name}}');
  });

  test('decodeTemplateVariants prefers typed data field when present', () {
    final template = notif.Template()
      ..name = 'welcome'
      ..data.add(notif.TemplateData()
        ..type = 'EMAIL'
        ..detail = 'Hello'
        ..language = (notif.Language()..code = 'fr'));
    final variants = decodeTemplateVariants(template);
    expect(variants, hasLength(1));
    expect(variants.single.type, 'EMAIL');
  });

  test('templateSearch forwards languageCode to the request', () async {
    final fake = FakeNotificationClient();
    final container = _container(fake);
    addTearDown(container.dispose);

    await container.read(
      templateSearchProvider(
              const TemplateSearchParams(query: 'q', languageCode: 'sw'))
          .future,
    );
    expect(fake.templateSearchRequests, hasLength(1));
    expect(fake.templateSearchRequests.single.query, 'q');
    expect(fake.templateSearchRequests.single.languageCode, 'sw');
  });

  test('save with empty variants still sends a request with empty list',
      () async {
    final fake = FakeNotificationClient()
      ..nextSavedTemplate = (notif.Template()
        ..id = 'tx'
        ..name = 'empty');
    final container = _container(fake);
    addTearDown(container.dispose);

    await container.read(templateNotifierProvider.notifier).save(
          name: 'empty',
          variants: const [],
        );
    expect(fake.templateSaveRequests, hasLength(1));
    final req = fake.templateSaveRequests.single;
    expect(req.name, 'empty');
    expect(req.languageCode, '');
    expect(req.data.fields['variants']?.listValue.values, isEmpty);
  });

  test('templateSearchProvider hides _lang_* placeholders', () async {
    final fake = FakeNotificationClient()
      ..nextTemplateResults = [
        notif.Template()
          ..id = 't1'
          ..name = 'welcome',
        notif.Template()
          ..id = 't2'
          ..name = '_lang_pt',
        notif.Template()
          ..id = 't3'
          ..name = 'reset',
      ];
    final container = _container(fake);
    addTearDown(container.dispose);

    final results = await container.read(
      templateSearchProvider(const TemplateSearchParams()).future,
    );
    expect(results.map((t) => t.name), unorderedEquals(['welcome', 'reset']));
  });

  test('switching partition invalidates templateSearchProvider', () async {
    final fake = FakeNotificationClient();
    final tenancy = TenancyContext()
      ..initializeFromLogin(LoginLevel.root, partitionId: 'p1');
    final container = ProviderContainer(overrides: [
      tenancyContextProvider.overrideWithValue(tenancy),
      notificationServiceClientProvider.overrideWithValue(fake.client),
    ]);
    addTearDown(container.dispose);

    const params = TemplateSearchParams();
    // Hold a subscription so autoDispose doesn't drop the cache.
    final sub = container.listen<AsyncValue<List<notif.Template>>>(
      templateSearchProvider(params),
      (_, _) {},
    );
    addTearDown(sub.close);

    await container.read(templateSearchProvider(params).future);
    expect(fake.templateSearchRequests, hasLength(1));

    tenancy.selectPartition('p2', 'Two');
    await container.pump();
    await container.read(templateSearchProvider(params).future);

    expect(fake.templateSearchRequests, hasLength(2));
  });
}
