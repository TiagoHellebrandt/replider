// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'onboarding_checklist_dismissal_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Locally persisted twin of the server-side 24h checklist dismissal
/// (`POST /me/onboarding-checklist/dismiss`). The card consults BOTH this
/// local flag AND [OnboardingProgressModel.isDismissActive] so a dismissal
/// survives a page reload immediately, without waiting for the next
/// `/me/onboarding-progress` round-trip to echo `checklist_dismissed_at`
/// back (US-112).
///
/// Persisted via [FlutterSecureStorage] — the app's KV-pref mechanism (the
/// Drift db carries no tables; see app/CLAUDE.md). The stored value is an
/// ISO-8601 timestamp; [build] returns true while it is younger than 24h,
/// mirroring the server window so local and server expire together.

@ProviderFor(OnboardingChecklistDismissal)
final onboardingChecklistDismissalProvider =
    OnboardingChecklistDismissalProvider._();

/// Locally persisted twin of the server-side 24h checklist dismissal
/// (`POST /me/onboarding-checklist/dismiss`). The card consults BOTH this
/// local flag AND [OnboardingProgressModel.isDismissActive] so a dismissal
/// survives a page reload immediately, without waiting for the next
/// `/me/onboarding-progress` round-trip to echo `checklist_dismissed_at`
/// back (US-112).
///
/// Persisted via [FlutterSecureStorage] — the app's KV-pref mechanism (the
/// Drift db carries no tables; see app/CLAUDE.md). The stored value is an
/// ISO-8601 timestamp; [build] returns true while it is younger than 24h,
/// mirroring the server window so local and server expire together.
final class OnboardingChecklistDismissalProvider
    extends $AsyncNotifierProvider<OnboardingChecklistDismissal, bool> {
  /// Locally persisted twin of the server-side 24h checklist dismissal
  /// (`POST /me/onboarding-checklist/dismiss`). The card consults BOTH this
  /// local flag AND [OnboardingProgressModel.isDismissActive] so a dismissal
  /// survives a page reload immediately, without waiting for the next
  /// `/me/onboarding-progress` round-trip to echo `checklist_dismissed_at`
  /// back (US-112).
  ///
  /// Persisted via [FlutterSecureStorage] — the app's KV-pref mechanism (the
  /// Drift db carries no tables; see app/CLAUDE.md). The stored value is an
  /// ISO-8601 timestamp; [build] returns true while it is younger than 24h,
  /// mirroring the server window so local and server expire together.
  OnboardingChecklistDismissalProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'onboardingChecklistDismissalProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$onboardingChecklistDismissalHash();

  @$internal
  @override
  OnboardingChecklistDismissal create() => OnboardingChecklistDismissal();
}

String _$onboardingChecklistDismissalHash() =>
    r'630527826f5228cd4b5d814235bfd2f8824dbfc1';

/// Locally persisted twin of the server-side 24h checklist dismissal
/// (`POST /me/onboarding-checklist/dismiss`). The card consults BOTH this
/// local flag AND [OnboardingProgressModel.isDismissActive] so a dismissal
/// survives a page reload immediately, without waiting for the next
/// `/me/onboarding-progress` round-trip to echo `checklist_dismissed_at`
/// back (US-112).
///
/// Persisted via [FlutterSecureStorage] — the app's KV-pref mechanism (the
/// Drift db carries no tables; see app/CLAUDE.md). The stored value is an
/// ISO-8601 timestamp; [build] returns true while it is younger than 24h,
/// mirroring the server window so local and server expire together.

abstract class _$OnboardingChecklistDismissal extends $AsyncNotifier<bool> {
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
