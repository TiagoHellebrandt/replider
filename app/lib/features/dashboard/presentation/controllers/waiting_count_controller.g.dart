// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'waiting_count_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// How many customers are waiting on a human reply — the number on the
/// Conversas tab. Before this badge existed the only way to learn a reply
/// had arrived was to open the screen; silence looked identical to no news.
///
/// keepAlive because the SHELL wears it: an autoDispose provider would die
/// whenever the inbox is closed, which is exactly when the badge matters.
/// Refreshed by the same WebSocket events that refresh the inbox, debounced
/// so a burst of messages costs one request, not one each.

@ProviderFor(WaitingCount)
final waitingCountProvider = WaitingCountProvider._();

/// How many customers are waiting on a human reply — the number on the
/// Conversas tab. Before this badge existed the only way to learn a reply
/// had arrived was to open the screen; silence looked identical to no news.
///
/// keepAlive because the SHELL wears it: an autoDispose provider would die
/// whenever the inbox is closed, which is exactly when the badge matters.
/// Refreshed by the same WebSocket events that refresh the inbox, debounced
/// so a burst of messages costs one request, not one each.
final class WaitingCountProvider
    extends $AsyncNotifierProvider<WaitingCount, int> {
  /// How many customers are waiting on a human reply — the number on the
  /// Conversas tab. Before this badge existed the only way to learn a reply
  /// had arrived was to open the screen; silence looked identical to no news.
  ///
  /// keepAlive because the SHELL wears it: an autoDispose provider would die
  /// whenever the inbox is closed, which is exactly when the badge matters.
  /// Refreshed by the same WebSocket events that refresh the inbox, debounced
  /// so a burst of messages costs one request, not one each.
  WaitingCountProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'waitingCountProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$waitingCountHash();

  @$internal
  @override
  WaitingCount create() => WaitingCount();
}

String _$waitingCountHash() => r'6985cf07651a0f45459ac7f7f5486311ac89de7c';

/// How many customers are waiting on a human reply — the number on the
/// Conversas tab. Before this badge existed the only way to learn a reply
/// had arrived was to open the screen; silence looked identical to no news.
///
/// keepAlive because the SHELL wears it: an autoDispose provider would die
/// whenever the inbox is closed, which is exactly when the badge matters.
/// Refreshed by the same WebSocket events that refresh the inbox, debounced
/// so a burst of messages costs one request, not one each.

abstract class _$WaitingCount extends $AsyncNotifier<int> {
  FutureOr<int> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<int>, int>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<AsyncValue<int>, int>, AsyncValue<int>, Object?, Object?>;
    element.handleCreate(ref, build);
  }
}
