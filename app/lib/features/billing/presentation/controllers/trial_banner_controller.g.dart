// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'trial_banner_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Tracks whether the trial countdown banner was dismissed for the current
/// calendar day. Persisted locally via [FlutterSecureStorage] — the app's
/// KV-pref mechanism (see locale_bootstrap.dart; the Drift db carries no
/// tables) — so a dismissal survives reloads but the banner re-surfaces the
/// next day, keeping the upgrade nudge alive without nagging within a day.

@ProviderFor(TrialBannerDismissal)
final trialBannerDismissalProvider = TrialBannerDismissalProvider._();

/// Tracks whether the trial countdown banner was dismissed for the current
/// calendar day. Persisted locally via [FlutterSecureStorage] — the app's
/// KV-pref mechanism (see locale_bootstrap.dart; the Drift db carries no
/// tables) — so a dismissal survives reloads but the banner re-surfaces the
/// next day, keeping the upgrade nudge alive without nagging within a day.
final class TrialBannerDismissalProvider
    extends $AsyncNotifierProvider<TrialBannerDismissal, bool> {
  /// Tracks whether the trial countdown banner was dismissed for the current
  /// calendar day. Persisted locally via [FlutterSecureStorage] — the app's
  /// KV-pref mechanism (see locale_bootstrap.dart; the Drift db carries no
  /// tables) — so a dismissal survives reloads but the banner re-surfaces the
  /// next day, keeping the upgrade nudge alive without nagging within a day.
  TrialBannerDismissalProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'trialBannerDismissalProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$trialBannerDismissalHash();

  @$internal
  @override
  TrialBannerDismissal create() => TrialBannerDismissal();
}

String _$trialBannerDismissalHash() =>
    r'f9680b1decbf9a9bc88972e894ae133872b05671';

/// Tracks whether the trial countdown banner was dismissed for the current
/// calendar day. Persisted locally via [FlutterSecureStorage] — the app's
/// KV-pref mechanism (see locale_bootstrap.dart; the Drift db carries no
/// tables) — so a dismissal survives reloads but the banner re-surfaces the
/// next day, keeping the upgrade nudge alive without nagging within a day.

abstract class _$TrialBannerDismissal extends $AsyncNotifier<bool> {
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
