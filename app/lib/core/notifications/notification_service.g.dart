// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'notification_service.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Singleton-ish provider so the WS listener always pokes the same
/// instance; counter state would otherwise reset on every read.

@ProviderFor(notificationService)
final notificationServiceProvider = NotificationServiceProvider._();

/// Singleton-ish provider so the WS listener always pokes the same
/// instance; counter state would otherwise reset on every read.

final class NotificationServiceProvider extends $FunctionalProvider<
    NotificationService,
    NotificationService,
    NotificationService> with $Provider<NotificationService> {
  /// Singleton-ish provider so the WS listener always pokes the same
  /// instance; counter state would otherwise reset on every read.
  NotificationServiceProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'notificationServiceProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$notificationServiceHash();

  @$internal
  @override
  $ProviderElement<NotificationService> $createElement(
          $ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  NotificationService create(Ref ref) {
    return notificationService(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(NotificationService value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<NotificationService>(value),
    );
  }
}

String _$notificationServiceHash() =>
    r'585c1e42ea844e71a2b76b80b165adfe2c5c8529';

/// Registers the WS → notifier pipe. Listens to inbound `new_message`
/// events and only pings when the user is NOT looking at the
/// originating conversation. Wire this provider once at app start
/// (e.g. via `ref.watch` in the dashboard root) and forget it.

@ProviderFor(inboundNotificationListener)
final inboundNotificationListenerProvider =
    InboundNotificationListenerProvider._();

/// Registers the WS → notifier pipe. Listens to inbound `new_message`
/// events and only pings when the user is NOT looking at the
/// originating conversation. Wire this provider once at app start
/// (e.g. via `ref.watch` in the dashboard root) and forget it.

final class InboundNotificationListenerProvider
    extends $FunctionalProvider<void, void, void> with $Provider<void> {
  /// Registers the WS → notifier pipe. Listens to inbound `new_message`
  /// events and only pings when the user is NOT looking at the
  /// originating conversation. Wire this provider once at app start
  /// (e.g. via `ref.watch` in the dashboard root) and forget it.
  InboundNotificationListenerProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'inboundNotificationListenerProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$inboundNotificationListenerHash();

  @$internal
  @override
  $ProviderElement<void> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  void create(Ref ref) {
    return inboundNotificationListener(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(void value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<void>(value),
    );
  }
}

String _$inboundNotificationListenerHash() =>
    r'a0c91e20299652f67567d6b458310f1085904ec2';
