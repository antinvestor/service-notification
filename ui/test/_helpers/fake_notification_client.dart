import 'dart:async';

import 'package:antinvestor_api_notification/antinvestor_api_notification.dart'
    as notif;
import 'package:connectrpc/connect.dart';

// SearchRequest comes from the common package, re-exported by
// antinvestor_api_notification.
typedef SearchRequest = notif.SearchRequest;

/// Hand-rolled fake of [notif.NotificationServiceClient] for tests.
///
/// Because [notif.NotificationServiceClient] is a Dart *extension type*
/// (not a class), it cannot be subclassed or `implements`-ed directly.
/// This helper wraps a [_FakeTransport] and exposes a real
/// [notif.NotificationServiceClient] that uses it.
///
/// Usage:
/// ```dart
/// final fake = FakeNotificationClient();
///
/// // In ProviderScope overrides:
/// notificationServiceClientProvider.overrideWithValue(fake.client)
///
/// // Configure canned responses before the call:
/// fake.nextSearchResults = [makeNotification(id: '1')];
///
/// // Assert on captured requests after the call:
/// expect(fake.searchRequests.last.query, 'hello');
/// ```
class FakeNotificationClient {
  FakeNotificationClient() {
    _transport = _FakeTransport(this);
    client = notif.NotificationServiceClient(_transport);
  }

  late final _FakeTransport _transport;

  /// The real [notif.NotificationServiceClient] backed by this fake.
  /// Pass to [notificationServiceClientProvider.overrideWithValue].
  late final notif.NotificationServiceClient client;

  // ── captured requests ─────────────────────────────────────────────────────

  final List<notif.SendRequest> sendRequests = [];
  final List<notif.ReleaseRequest> releaseRequests = [];
  final List<notif.ReceiveRequest> receiveRequests = [];
  final List<SearchRequest> searchRequests = [];
  final List<notif.TemplateSearchRequest> templateSearchRequests = [];
  final List<notif.TemplateSaveRequest> templateSaveRequests = [];

  // ── canned responses ──────────────────────────────────────────────────────

  /// Notifications returned from the next [notif.NotificationServiceClient.search] call.
  List<notif.Notification> nextSearchResults = const [];

  /// Templates returned from the next [notif.NotificationServiceClient.templateSearch] call.
  List<notif.Template> nextTemplateResults = const [];

  /// Template returned from the next [notif.NotificationServiceClient.templateSave] call.
  notif.Template nextSavedTemplate = notif.Template();
}

// ── Internal fake transport ───────────────────────────────────────────────────

const _svcPrefix = '/notification.v1.NotificationService/';

class _FakeTransport implements Transport {
  _FakeTransport(this._fake);

  final FakeNotificationClient _fake;

  static Headers get _emptyHeaders => Headers();

  @override
  Future<UnaryResponse<I, O>> unary<I extends Object, O extends Object>(
    Spec<I, O> spec,
    I input, [
    CallOptions? options,
  ]) async {
    final procedure = spec.procedure;
    final O response;

    if (procedure == '${_svcPrefix}TemplateSave') {
      _fake.templateSaveRequests.add(input as notif.TemplateSaveRequest);
      final saved = notif.TemplateSaveResponse()
        ..data = _fake.nextSavedTemplate;
      response = saved as O;
    } else {
      // Status / StatusUpdate — return empty output; tests that need these
      // can override the provider more specifically.
      response = spec.outputFactory();
    }

    return UnaryResponse<I, O>(
      spec,
      _emptyHeaders,
      response,
      _emptyHeaders,
    );
  }

  @override
  Future<StreamResponse<I, O>> stream<I extends Object, O extends Object>(
    Spec<I, O> spec,
    Stream<I> input, [
    CallOptions? options,
  ]) async {
    final procedure = spec.procedure;
    Stream<O> responseStream;

    if (procedure == '${_svcPrefix}Send') {
      final req = await input.first as notif.SendRequest;
      _fake.sendRequests.add(req);
      responseStream = Stream.value(notif.SendResponse() as O);
    } else if (procedure == '${_svcPrefix}Release') {
      final req = await input.first as notif.ReleaseRequest;
      _fake.releaseRequests.add(req);
      responseStream = Stream.value(notif.ReleaseResponse() as O);
    } else if (procedure == '${_svcPrefix}Receive') {
      final req = await input.first as notif.ReceiveRequest;
      _fake.receiveRequests.add(req);
      responseStream = Stream.value(notif.ReceiveResponse() as O);
    } else if (procedure == '${_svcPrefix}Search') {
      final req = await input.first as notif.SearchRequest;
      _fake.searchRequests.add(req);
      final resp = notif.SearchResponse()
        ..data.addAll(_fake.nextSearchResults);
      responseStream = Stream.value(resp as O);
    } else if (procedure == '${_svcPrefix}TemplateSearch') {
      final req = await input.first as notif.TemplateSearchRequest;
      _fake.templateSearchRequests.add(req);
      final resp = notif.TemplateSearchResponse()
        ..data.addAll(_fake.nextTemplateResults);
      responseStream = Stream.value(resp as O);
    } else {
      responseStream = Stream.value(spec.outputFactory());
    }

    return StreamResponse<I, O>(
      spec,
      _emptyHeaders,
      responseStream,
      _emptyHeaders,
    );
  }
}
