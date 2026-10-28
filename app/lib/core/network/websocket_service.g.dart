// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'websocket_service.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(webSocketService)
final webSocketServiceProvider = WebSocketServiceProvider._();

final class WebSocketServiceProvider extends $FunctionalProvider<
    WebSocketService,
    WebSocketService,
    WebSocketService> with $Provider<WebSocketService> {
  WebSocketServiceProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'webSocketServiceProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$webSocketServiceHash();

  @$internal
  @override
  $ProviderElement<WebSocketService> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  WebSocketService create(Ref ref) {
    return webSocketService(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(WebSocketService value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<WebSocketService>(value),
    );
  }
}

String _$webSocketServiceHash() => r'8522b912791fa069d5523ee95b65d633accdf312';

@ProviderFor(wsEvents)
final wsEventsProvider = WsEventsProvider._();

final class WsEventsProvider
    extends $FunctionalProvider<AsyncValue<WSEvent>, WSEvent, Stream<WSEvent>>
    with $FutureModifier<WSEvent>, $StreamProvider<WSEvent> {
  WsEventsProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'wsEventsProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$wsEventsHash();

  @$internal
  @override
  $StreamProviderElement<WSEvent> $createElement($ProviderPointer pointer) =>
      $StreamProviderElement(pointer);

  @override
  Stream<WSEvent> create(Ref ref) {
    return wsEvents(ref);
  }
}

String _$wsEventsHash() => r'068ee378195f159d5fd52fa1d68c7b2f2968c013';

/// Reflects the underlying WebSocket connection state so UI can show a
/// "reconectando" banner. Defaults to `true` (connected) on first read
/// to avoid a flicker on cold start before the first handshake lands.

@ProviderFor(wsConnectionStatus)
final wsConnectionStatusProvider = WsConnectionStatusProvider._();

/// Reflects the underlying WebSocket connection state so UI can show a
/// "reconectando" banner. Defaults to `true` (connected) on first read
/// to avoid a flicker on cold start before the first handshake lands.

final class WsConnectionStatusProvider
    extends $FunctionalProvider<AsyncValue<bool>, bool, Stream<bool>>
    with $FutureModifier<bool>, $StreamProvider<bool> {
  /// Reflects the underlying WebSocket connection state so UI can show a
  /// "reconectando" banner. Defaults to `true` (connected) on first read
  /// to avoid a flicker on cold start before the first handshake lands.
  WsConnectionStatusProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'wsConnectionStatusProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$wsConnectionStatusHash();

  @$internal
  @override
  $StreamProviderElement<bool> $createElement($ProviderPointer pointer) =>
      $StreamProviderElement(pointer);

  @override
  Stream<bool> create(Ref ref) {
    return wsConnectionStatus(ref);
  }
}

String _$wsConnectionStatusHash() =>
    r'4e685cc89b24680d0f8115c00f8cc95e8a2c49fc';
