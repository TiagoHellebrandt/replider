// Web implementation of [PushNotificationService]. Bridges to the
// window.repliderPush helper from web/push-client.js via dart:js_interop.
import 'dart:js_interop';

import 'push_notification_service.dart';

@JS('repliderPush')
external _RepliderPushJS? get _repliderPush;

extension type _RepliderPushJS._(JSObject _) implements JSObject {
  external bool supported();
  external String permission();
  external JSPromise<JSString> requestPermission();
  external JSPromise<JSAny?> subscribe(String vapidPublicKey);
  external JSPromise<JSAny?> getSubscription();
  external JSPromise<JSAny?> unsubscribe();
  external JSFunction onClick(JSFunction handler);
  external JSFunction onSubscriptionChange(JSFunction handler);
}

extension type _ChangeEventJS._(JSObject _) implements JSObject {
  external bool get resubscribed;
  external _SubscriptionJS? get subscription;
}

extension type _SubscriptionJS._(JSObject _) implements JSObject {
  external String? get endpoint;
  external String? get p256dh;
  external String? get auth;
}

PushPermission _parsePermission(String? raw) {
  switch (raw) {
    case 'granted':
      return PushPermission.granted;
    case 'denied':
      return PushPermission.denied;
    default:
      return PushPermission.unknown;
  }
}

PushSubscriptionPayload? _payloadFromJs(JSAny? js) {
  if (js == null) return null;
  final sub = js as _SubscriptionJS;
  final endpoint = sub.endpoint;
  if (endpoint == null || endpoint.isEmpty) return null;
  return PushSubscriptionPayload(
    platform: 'web',
    endpoint: endpoint,
    p256dh: sub.p256dh,
    auth: sub.auth,
  );
}

class _WebPushNotificationService implements PushNotificationService {
  const _WebPushNotificationService();

  _RepliderPushJS? get _api => _repliderPush;

  @override
  bool get isSupported {
    final api = _api;
    return api != null && api.supported();
  }

  @override
  PushPermission get currentPermission {
    final api = _api;
    if (api == null) return PushPermission.unknown;
    return _parsePermission(api.permission());
  }

  @override
  Future<PushPermission> requestPermission() async {
    final api = _api;
    if (api == null) return PushPermission.unknown;
    final result = await api.requestPermission().toDart;
    return _parsePermission(result.toDart);
  }

  @override
  Future<PushSubscriptionPayload?> subscribe(String vapidPublicKey) async {
    final api = _api;
    if (api == null) return null;
    final js = await api.subscribe(vapidPublicKey).toDart;
    return _payloadFromJs(js);
  }

  @override
  Future<PushSubscriptionPayload?> currentSubscription() async {
    final api = _api;
    if (api == null) return null;
    final js = await api.getSubscription().toDart;
    return _payloadFromJs(js);
  }

  @override
  Future<String?> unsubscribe() async {
    final api = _api;
    if (api == null) return null;
    final js = await api.unsubscribe().toDart;
    if (js == null) return null;
    // `unsubscribe()` in JS resolves with the removed endpoint string.
    return (js as JSString).toDart;
  }

  @override
  void Function() onNotificationClick(void Function(String target) handler) {
    final api = _api;
    if (api == null) return () {};
    final cb = (JSString target) {
      handler(target.toDart);
    }.toJS;
    final disposerRaw = api.onClick(cb);
    return () {
      disposerRaw.callAsFunction();
    };
  }

  @override
  void Function() onSubscriptionChange(
      void Function(SubscriptionChange event) handler) {
    final api = _api;
    if (api == null) return () {};
    final cb = (JSObject raw) {
      final ev = raw as _ChangeEventJS;
      final sub = ev.subscription;
      handler(SubscriptionChange(
        resubscribed: ev.resubscribed,
        subscription: sub == null ? null : _payloadFromJs(sub),
      ));
    }.toJS;
    final disposerRaw = api.onSubscriptionChange(cb);
    return () {
      disposerRaw.callAsFunction();
    };
  }
}

PushNotificationService createPushNotificationService() =>
    const _WebPushNotificationService();
