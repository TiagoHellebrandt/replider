/// Platform-abstracted entry point for push notifications. The web
/// implementation drives the W3C Push API + a service worker; mobile is
/// a no-op stub today and will swap in an FCM/APNs adapter later without
/// touching callers.
///
/// Conditional imports resolve to the real implementation on web and the
/// stub on every other platform.
library;

import 'push_notification_service_stub.dart'
    if (dart.library.js_interop) 'push_notification_service_web.dart' as impl;

/// Result of a successful browser `PushManager.subscribe()` call. On
/// mobile this carries an FCM/APNs device token in [endpoint] and leaves
/// the key fields null — the server schema supports both.
class PushSubscriptionPayload {
  const PushSubscriptionPayload({
    required this.endpoint,
    this.p256dh,
    this.auth,
    this.deviceToken,
    this.userAgent,
    this.platform = 'web',
  });

  final String endpoint;
  final String? p256dh;
  final String? auth;
  final String? deviceToken;
  final String? userAgent;
  final String platform;

  Map<String, dynamic> toJson() => {
        'platform': platform,
        'endpoint': endpoint,
        if (p256dh != null && auth != null)
          'keys': {'p256dh': p256dh, 'auth': auth},
        if (deviceToken != null) 'device_token': deviceToken,
        if (userAgent != null) 'user_agent': userAgent,
      };
}

/// Permission states mirror the Notification API values. Mobile reports
/// `denied` until we wire a real adapter.
enum PushPermission { granted, denied, unknown }

abstract class PushNotificationService {
  /// Returns a platform-appropriate implementation. Singleton semantics
  /// are fine — state is stored on the SW registration, not the Dart
  /// object.
  factory PushNotificationService() => impl.createPushNotificationService();

  /// True when the browser exposes Service Worker + Push + Notification.
  /// Always false on mobile until FCM/APNs adapters land.
  bool get isSupported;

  /// Current browser permission. Does not prompt.
  PushPermission get currentPermission;

  /// Prompts the user. Safe to call when already granted (returns the
  /// existing state).
  Future<PushPermission> requestPermission();

  /// Registers the SW and subscribes to push with [vapidPublicKey].
  /// Returns null when the user denies permission or the platform is
  /// unsupported.
  Future<PushSubscriptionPayload?> subscribe(String vapidPublicKey);

  /// Returns the currently registered subscription on this device, or
  /// null when none exists.
  Future<PushSubscriptionPayload?> currentSubscription();

  /// Removes the browser subscription. Returns the endpoint that was
  /// removed so the caller can tell the server to forget it.
  Future<String?> unsubscribe();

  /// Wires [handler] to notification clicks coming from the service
  /// worker. Returns a disposer.
  void Function() onNotificationClick(void Function(String target) handler);

  /// Wires [handler] to subscription-rotation events from the SW. Fired
  /// when the browser rolls the push endpoint on its own (typical after
  /// a long-idle install or a push-service migration). Without this
  /// hook the API stays bound to a dead row and every send goes
  /// nowhere — the canonical "I stopped getting notifications" bug.
  ///
  /// [handler] receives a payload that's either
  ///   - `{resubscribed: true, subscription: NEW_SUB}` — POST it to
  ///     `/v1/push/subscriptions` to replace the dead row, or
  ///   - `{resubscribed: false}` — surface a "re-enable in Settings"
  ///     hint; the SW couldn't recover (no cached VAPID, permission
  ///     revoked, etc.).
  void Function() onSubscriptionChange(
      void Function(SubscriptionChange event) handler);
}

/// Payload delivered to the [PushNotificationService.onSubscriptionChange]
/// handler. [subscription] is non-null only when the SW successfully
/// re-subscribed and we have a fresh endpoint to POST to the API.
class SubscriptionChange {
  const SubscriptionChange({
    required this.resubscribed,
    this.subscription,
  });

  final bool resubscribed;
  final PushSubscriptionPayload? subscription;
}
