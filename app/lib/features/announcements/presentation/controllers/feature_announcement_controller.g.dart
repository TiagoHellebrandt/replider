// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'feature_announcement_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Tracks whether the "What's new" feature-announcement modal has already been
/// shown for the CURRENT release (US-184). Persisted locally via
/// [FlutterSecureStorage] — the app's KV-pref mechanism (the Drift db carries
/// no tables), same shape as the product-tour / NPS one-shots.
///
/// The release is identified by [currentRelease]: bumping that constant (and
/// the `feature_announcement.*` i18n copy) re-surfaces the modal exactly once
/// per upgrade. `build()` returns `true` when the modal should stay HIDDEN
/// (already seen this release, or this is a brand-new install — see below).
///
/// Brand-new installs (no stored release) have nothing to announce: they're
/// seeing every feature for the first time. We stamp the current release
/// silently and suppress the modal, so it only ever fires for RETURNING users
/// after a real upgrade — which also guarantees it never stacks on top of the
/// new-account product tour.

@ProviderFor(FeatureAnnouncementController)
final featureAnnouncementControllerProvider =
    FeatureAnnouncementControllerProvider._();

/// Tracks whether the "What's new" feature-announcement modal has already been
/// shown for the CURRENT release (US-184). Persisted locally via
/// [FlutterSecureStorage] — the app's KV-pref mechanism (the Drift db carries
/// no tables), same shape as the product-tour / NPS one-shots.
///
/// The release is identified by [currentRelease]: bumping that constant (and
/// the `feature_announcement.*` i18n copy) re-surfaces the modal exactly once
/// per upgrade. `build()` returns `true` when the modal should stay HIDDEN
/// (already seen this release, or this is a brand-new install — see below).
///
/// Brand-new installs (no stored release) have nothing to announce: they're
/// seeing every feature for the first time. We stamp the current release
/// silently and suppress the modal, so it only ever fires for RETURNING users
/// after a real upgrade — which also guarantees it never stacks on top of the
/// new-account product tour.
final class FeatureAnnouncementControllerProvider
    extends $AsyncNotifierProvider<FeatureAnnouncementController, bool> {
  /// Tracks whether the "What's new" feature-announcement modal has already been
  /// shown for the CURRENT release (US-184). Persisted locally via
  /// [FlutterSecureStorage] — the app's KV-pref mechanism (the Drift db carries
  /// no tables), same shape as the product-tour / NPS one-shots.
  ///
  /// The release is identified by [currentRelease]: bumping that constant (and
  /// the `feature_announcement.*` i18n copy) re-surfaces the modal exactly once
  /// per upgrade. `build()` returns `true` when the modal should stay HIDDEN
  /// (already seen this release, or this is a brand-new install — see below).
  ///
  /// Brand-new installs (no stored release) have nothing to announce: they're
  /// seeing every feature for the first time. We stamp the current release
  /// silently and suppress the modal, so it only ever fires for RETURNING users
  /// after a real upgrade — which also guarantees it never stacks on top of the
  /// new-account product tour.
  FeatureAnnouncementControllerProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'featureAnnouncementControllerProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$featureAnnouncementControllerHash();

  @$internal
  @override
  FeatureAnnouncementController create() => FeatureAnnouncementController();
}

String _$featureAnnouncementControllerHash() =>
    r'76a02b0e8d27584f9c880fad6b8aac290207c9bc';

/// Tracks whether the "What's new" feature-announcement modal has already been
/// shown for the CURRENT release (US-184). Persisted locally via
/// [FlutterSecureStorage] — the app's KV-pref mechanism (the Drift db carries
/// no tables), same shape as the product-tour / NPS one-shots.
///
/// The release is identified by [currentRelease]: bumping that constant (and
/// the `feature_announcement.*` i18n copy) re-surfaces the modal exactly once
/// per upgrade. `build()` returns `true` when the modal should stay HIDDEN
/// (already seen this release, or this is a brand-new install — see below).
///
/// Brand-new installs (no stored release) have nothing to announce: they're
/// seeing every feature for the first time. We stamp the current release
/// silently and suppress the modal, so it only ever fires for RETURNING users
/// after a real upgrade — which also guarantees it never stacks on top of the
/// new-account product tour.

abstract class _$FeatureAnnouncementController extends $AsyncNotifier<bool> {
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
