// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'product_tour_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Tracks whether the one-shot in-app product tour has already been shown
/// (US-183). Persisted locally via [FlutterSecureStorage] — the app's KV-pref
/// mechanism, same as the banner/NPS dismissals (the Drift db carries no
/// tables) — so the walkthrough surfaces exactly once per account/device and
/// the choice survives reloads.
///
/// `build()` returns `true` when the tour was already completed or skipped
/// (i.e. it should stay hidden). Unlike the NPS prompt this is a lifetime
/// one-shot, not period-based, so [complete] simply stamps a sticky flag.

@ProviderFor(ProductTourController)
final productTourControllerProvider = ProductTourControllerProvider._();

/// Tracks whether the one-shot in-app product tour has already been shown
/// (US-183). Persisted locally via [FlutterSecureStorage] — the app's KV-pref
/// mechanism, same as the banner/NPS dismissals (the Drift db carries no
/// tables) — so the walkthrough surfaces exactly once per account/device and
/// the choice survives reloads.
///
/// `build()` returns `true` when the tour was already completed or skipped
/// (i.e. it should stay hidden). Unlike the NPS prompt this is a lifetime
/// one-shot, not period-based, so [complete] simply stamps a sticky flag.
final class ProductTourControllerProvider
    extends $AsyncNotifierProvider<ProductTourController, bool> {
  /// Tracks whether the one-shot in-app product tour has already been shown
  /// (US-183). Persisted locally via [FlutterSecureStorage] — the app's KV-pref
  /// mechanism, same as the banner/NPS dismissals (the Drift db carries no
  /// tables) — so the walkthrough surfaces exactly once per account/device and
  /// the choice survives reloads.
  ///
  /// `build()` returns `true` when the tour was already completed or skipped
  /// (i.e. it should stay hidden). Unlike the NPS prompt this is a lifetime
  /// one-shot, not period-based, so [complete] simply stamps a sticky flag.
  ProductTourControllerProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'productTourControllerProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$productTourControllerHash();

  @$internal
  @override
  ProductTourController create() => ProductTourController();
}

String _$productTourControllerHash() =>
    r'87925187e68fe39995fdd484e3ae94745b712448';

/// Tracks whether the one-shot in-app product tour has already been shown
/// (US-183). Persisted locally via [FlutterSecureStorage] — the app's KV-pref
/// mechanism, same as the banner/NPS dismissals (the Drift db carries no
/// tables) — so the walkthrough surfaces exactly once per account/device and
/// the choice survives reloads.
///
/// `build()` returns `true` when the tour was already completed or skipped
/// (i.e. it should stay hidden). Unlike the NPS prompt this is a lifetime
/// one-shot, not period-based, so [complete] simply stamps a sticky flag.

abstract class _$ProductTourController extends $AsyncNotifier<bool> {
  FutureOr<bool> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<bool>, bool>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<AsyncValue<bool>, bool>,
        AsyncValue<bool>,
        Object?,
        Object?>;
    element.handleCreate(ref, build);
  }
}
