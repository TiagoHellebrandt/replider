// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'two_factor_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(twoFactorDataSource)
final twoFactorDataSourceProvider = TwoFactorDataSourceProvider._();

final class TwoFactorDataSourceProvider extends $FunctionalProvider<
    TwoFactorRemoteDataSource,
    TwoFactorRemoteDataSource,
    TwoFactorRemoteDataSource> with $Provider<TwoFactorRemoteDataSource> {
  TwoFactorDataSourceProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'twoFactorDataSourceProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$twoFactorDataSourceHash();

  @$internal
  @override
  $ProviderElement<TwoFactorRemoteDataSource> $createElement(
          $ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  TwoFactorRemoteDataSource create(Ref ref) {
    return twoFactorDataSource(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(TwoFactorRemoteDataSource value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<TwoFactorRemoteDataSource>(value),
    );
  }
}

String _$twoFactorDataSourceHash() =>
    r'76107ab14910dc443dfd92ab94353d50995e1d56';

@ProviderFor(twoFactorStatus)
final twoFactorStatusProvider = TwoFactorStatusProvider._();

final class TwoFactorStatusProvider extends $FunctionalProvider<
        AsyncValue<TwoFactorStatus>, TwoFactorStatus, FutureOr<TwoFactorStatus>>
    with $FutureModifier<TwoFactorStatus>, $FutureProvider<TwoFactorStatus> {
  TwoFactorStatusProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'twoFactorStatusProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$twoFactorStatusHash();

  @$internal
  @override
  $FutureProviderElement<TwoFactorStatus> $createElement(
          $ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<TwoFactorStatus> create(Ref ref) {
    return twoFactorStatus(ref);
  }
}

String _$twoFactorStatusHash() => r'dbcbe292fa520b374a26a96b0bc177ce934aa2c6';

/// Imperative wrapper for the enrolment flow. The screen drives
/// `setup()` → `verify()` → `disable()`.
///
/// These methods MUST NOT touch `ref` after their `await`. This is an
/// autoDispose provider read via `.notifier` (no listener keeps it
/// alive), so it disposes during the network round-trip; a
/// `ref.invalidate(...)` after the await then throws StateError on the
/// dead ref — which surfaced as a bogus "unexpected error" right after
/// a successful 200 on Ativar 2FA. The caller (the card, whose
/// ConsumerState `ref` outlives the call) owns invalidating
/// [twoFactorStatusProvider] once the enabled state actually flips.

@ProviderFor(TwoFactorActions)
final twoFactorActionsProvider = TwoFactorActionsProvider._();

/// Imperative wrapper for the enrolment flow. The screen drives
/// `setup()` → `verify()` → `disable()`.
///
/// These methods MUST NOT touch `ref` after their `await`. This is an
/// autoDispose provider read via `.notifier` (no listener keeps it
/// alive), so it disposes during the network round-trip; a
/// `ref.invalidate(...)` after the await then throws StateError on the
/// dead ref — which surfaced as a bogus "unexpected error" right after
/// a successful 200 on Ativar 2FA. The caller (the card, whose
/// ConsumerState `ref` outlives the call) owns invalidating
/// [twoFactorStatusProvider] once the enabled state actually flips.
final class TwoFactorActionsProvider
    extends $NotifierProvider<TwoFactorActions, void> {
  /// Imperative wrapper for the enrolment flow. The screen drives
  /// `setup()` → `verify()` → `disable()`.
  ///
  /// These methods MUST NOT touch `ref` after their `await`. This is an
  /// autoDispose provider read via `.notifier` (no listener keeps it
  /// alive), so it disposes during the network round-trip; a
  /// `ref.invalidate(...)` after the await then throws StateError on the
  /// dead ref — which surfaced as a bogus "unexpected error" right after
  /// a successful 200 on Ativar 2FA. The caller (the card, whose
  /// ConsumerState `ref` outlives the call) owns invalidating
  /// [twoFactorStatusProvider] once the enabled state actually flips.
  TwoFactorActionsProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'twoFactorActionsProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$twoFactorActionsHash();

  @$internal
  @override
  TwoFactorActions create() => TwoFactorActions();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(void value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<void>(value),
    );
  }
}

String _$twoFactorActionsHash() => r'5c485705f2a602474adff48c3fc3f0339864f473';

/// Imperative wrapper for the enrolment flow. The screen drives
/// `setup()` → `verify()` → `disable()`.
///
/// These methods MUST NOT touch `ref` after their `await`. This is an
/// autoDispose provider read via `.notifier` (no listener keeps it
/// alive), so it disposes during the network round-trip; a
/// `ref.invalidate(...)` after the await then throws StateError on the
/// dead ref — which surfaced as a bogus "unexpected error" right after
/// a successful 200 on Ativar 2FA. The caller (the card, whose
/// ConsumerState `ref` outlives the call) owns invalidating
/// [twoFactorStatusProvider] once the enabled state actually flips.

abstract class _$TwoFactorActions extends $Notifier<void> {
  void build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<void, void>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<void, void>, void, Object?, Object?>;
    element.handleCreate(ref, build);
  }
}
