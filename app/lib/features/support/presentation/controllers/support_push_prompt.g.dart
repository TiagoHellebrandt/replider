// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'support_push_prompt.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Whether the contextual push opt-in prompt inside the support widget has
/// already been handled (enabled OR declined) — canal-interno US-006. Once
/// true the banner never comes back: the user can still flip push from
/// Settings, and a browser-level denial is independently reflected by
/// [PushSubscriptionStatus.denied], which also hides the prompt.
///
/// Persisted via [FlutterSecureStorage], the app's KV-pref mechanism (same
/// pattern as OnboardingChecklistDismissal — the Drift db carries no
/// tables). Unlike the checklist there is no expiry window: a nag that
/// returns is worse than a Settings toggle the user seeks out.

@ProviderFor(SupportPushPromptDismissal)
final supportPushPromptDismissalProvider =
    SupportPushPromptDismissalProvider._();

/// Whether the contextual push opt-in prompt inside the support widget has
/// already been handled (enabled OR declined) — canal-interno US-006. Once
/// true the banner never comes back: the user can still flip push from
/// Settings, and a browser-level denial is independently reflected by
/// [PushSubscriptionStatus.denied], which also hides the prompt.
///
/// Persisted via [FlutterSecureStorage], the app's KV-pref mechanism (same
/// pattern as OnboardingChecklistDismissal — the Drift db carries no
/// tables). Unlike the checklist there is no expiry window: a nag that
/// returns is worse than a Settings toggle the user seeks out.
final class SupportPushPromptDismissalProvider
    extends $AsyncNotifierProvider<SupportPushPromptDismissal, bool> {
  /// Whether the contextual push opt-in prompt inside the support widget has
  /// already been handled (enabled OR declined) — canal-interno US-006. Once
  /// true the banner never comes back: the user can still flip push from
  /// Settings, and a browser-level denial is independently reflected by
  /// [PushSubscriptionStatus.denied], which also hides the prompt.
  ///
  /// Persisted via [FlutterSecureStorage], the app's KV-pref mechanism (same
  /// pattern as OnboardingChecklistDismissal — the Drift db carries no
  /// tables). Unlike the checklist there is no expiry window: a nag that
  /// returns is worse than a Settings toggle the user seeks out.
  SupportPushPromptDismissalProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'supportPushPromptDismissalProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$supportPushPromptDismissalHash();

  @$internal
  @override
  SupportPushPromptDismissal create() => SupportPushPromptDismissal();
}

String _$supportPushPromptDismissalHash() =>
    r'df52958755c90dc49ddca387bf4ca630638c122c';

/// Whether the contextual push opt-in prompt inside the support widget has
/// already been handled (enabled OR declined) — canal-interno US-006. Once
/// true the banner never comes back: the user can still flip push from
/// Settings, and a browser-level denial is independently reflected by
/// [PushSubscriptionStatus.denied], which also hides the prompt.
///
/// Persisted via [FlutterSecureStorage], the app's KV-pref mechanism (same
/// pattern as OnboardingChecklistDismissal — the Drift db carries no
/// tables). Unlike the checklist there is no expiry window: a nag that
/// returns is worse than a Settings toggle the user seeks out.

abstract class _$SupportPushPromptDismissal extends $AsyncNotifier<bool> {
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
