// Non-web stub. Today mobile falls back to the in-app sound + title
// notifier in notification_service.dart; when we wire FCM/APNs later,
// swap this file's body with the real adapter.
import 'push_notification_service.dart';

class _StubPushNotificationService implements PushNotificationService {
  const _StubPushNotificationService();

  @override
  bool get isSupported => false;

  @override
  PushPermission get currentPermission => PushPermission.unknown;

  @override
  Future<PushPermission> requestPermission() async => PushPermission.unknown;

  @override
  Future<PushSubscriptionPayload?> subscribe(String vapidPublicKey) async =>
      null;

  @override
  Future<PushSubscriptionPayload?> currentSubscription() async => null;

  @override
  Future<String?> unsubscribe() async => null;

  @override
  void Function() onNotificationClick(void Function(String target) handler) {
    return () {};
  }

  @override
  void Function() onSubscriptionChange(
      void Function(SubscriptionChange event) handler) {
    return () {};
  }
}

PushNotificationService createPushNotificationService() =>
    const _StubPushNotificationService();
