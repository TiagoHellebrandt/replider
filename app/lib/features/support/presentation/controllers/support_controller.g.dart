// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'support_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The logged-in user's support thread: hydrated from GET on open,
/// appended in real time from `support_message` WS events.

@ProviderFor(SupportThreadController)
final supportThreadControllerProvider = SupportThreadControllerProvider._();

/// The logged-in user's support thread: hydrated from GET on open,
/// appended in real time from `support_message` WS events.
final class SupportThreadControllerProvider
    extends $AsyncNotifierProvider<SupportThreadController, SupportThread> {
  /// The logged-in user's support thread: hydrated from GET on open,
  /// appended in real time from `support_message` WS events.
  SupportThreadControllerProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'supportThreadControllerProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$supportThreadControllerHash();

  @$internal
  @override
  SupportThreadController create() => SupportThreadController();
}

String _$supportThreadControllerHash() =>
    r'8b716b1c3a6789fb5e4f34f446d69914b6f64c20';

/// The logged-in user's support thread: hydrated from GET on open,
/// appended in real time from `support_message` WS events.

abstract class _$SupportThreadController extends $AsyncNotifier<SupportThread> {
  FutureOr<SupportThread> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<SupportThread>, SupportThread>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<AsyncValue<SupportThread>, SupportThread>,
        AsyncValue<SupportThread>,
        Object?,
        Object?>;
    element.handleCreate(ref, build);
  }
}

/// Unread count for the support nav badge. keepAlive so it accumulates
/// while the user is anywhere in the app; the support screen clears it
/// on open and while it stays visible.

@ProviderFor(SupportUnreadCounter)
final supportUnreadCounterProvider = SupportUnreadCounterProvider._();

/// Unread count for the support nav badge. keepAlive so it accumulates
/// while the user is anywhere in the app; the support screen clears it
/// on open and while it stays visible.
final class SupportUnreadCounterProvider
    extends $NotifierProvider<SupportUnreadCounter, int> {
  /// Unread count for the support nav badge. keepAlive so it accumulates
  /// while the user is anywhere in the app; the support screen clears it
  /// on open and while it stays visible.
  SupportUnreadCounterProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'supportUnreadCounterProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$supportUnreadCounterHash();

  @$internal
  @override
  SupportUnreadCounter create() => SupportUnreadCounter();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(int value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<int>(value),
    );
  }
}

String _$supportUnreadCounterHash() =>
    r'331d06c9155bd6d4e6f80fc8fe92275188e5bd86';

/// Unread count for the support nav badge. keepAlive so it accumulates
/// while the user is anywhere in the app; the support screen clears it
/// on open and while it stays visible.

abstract class _$SupportUnreadCounter extends $Notifier<int> {
  int build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<int, int>;
    final element = ref.element
        as $ClassProviderElement<AnyNotifier<int, int>, int, Object?, Object?>;
    element.handleCreate(ref, build);
  }
}
