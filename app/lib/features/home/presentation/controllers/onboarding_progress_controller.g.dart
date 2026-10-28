// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'onboarding_progress_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Fetches and refreshes the onboarding checklist data for the logged-in
/// account. Kept alive so the card can survive widget re-mounts (tab
/// switches within the dashboard) without re-fetching.
///
/// Refreshes are triggered explicitly by:
///  * The "Dispensar por enquanto" action (invalidate after POST).
///  * The existing WebSocket `new_message` event path, which is already
///    the de-facto "something happened" signal — the dashboard invalidates
///    this provider alongside its conversation list so first_inbound and
///    channel_connected flip visibly without a full reload.

@ProviderFor(OnboardingProgressController)
final onboardingProgressControllerProvider =
    OnboardingProgressControllerProvider._();

/// Fetches and refreshes the onboarding checklist data for the logged-in
/// account. Kept alive so the card can survive widget re-mounts (tab
/// switches within the dashboard) without re-fetching.
///
/// Refreshes are triggered explicitly by:
///  * The "Dispensar por enquanto" action (invalidate after POST).
///  * The existing WebSocket `new_message` event path, which is already
///    the de-facto "something happened" signal — the dashboard invalidates
///    this provider alongside its conversation list so first_inbound and
///    channel_connected flip visibly without a full reload.
final class OnboardingProgressControllerProvider extends $AsyncNotifierProvider<
    OnboardingProgressController, OnboardingProgressModel?> {
  /// Fetches and refreshes the onboarding checklist data for the logged-in
  /// account. Kept alive so the card can survive widget re-mounts (tab
  /// switches within the dashboard) without re-fetching.
  ///
  /// Refreshes are triggered explicitly by:
  ///  * The "Dispensar por enquanto" action (invalidate after POST).
  ///  * The existing WebSocket `new_message` event path, which is already
  ///    the de-facto "something happened" signal — the dashboard invalidates
  ///    this provider alongside its conversation list so first_inbound and
  ///    channel_connected flip visibly without a full reload.
  OnboardingProgressControllerProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'onboardingProgressControllerProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$onboardingProgressControllerHash();

  @$internal
  @override
  OnboardingProgressController create() => OnboardingProgressController();
}

String _$onboardingProgressControllerHash() =>
    r'a49cb0cc334cd8603bd5516386791397438c3730';

/// Fetches and refreshes the onboarding checklist data for the logged-in
/// account. Kept alive so the card can survive widget re-mounts (tab
/// switches within the dashboard) without re-fetching.
///
/// Refreshes are triggered explicitly by:
///  * The "Dispensar por enquanto" action (invalidate after POST).
///  * The existing WebSocket `new_message` event path, which is already
///    the de-facto "something happened" signal — the dashboard invalidates
///    this provider alongside its conversation list so first_inbound and
///    channel_connected flip visibly without a full reload.

abstract class _$OnboardingProgressController
    extends $AsyncNotifier<OnboardingProgressModel?> {
  FutureOr<OnboardingProgressModel?> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref
        as $Ref<AsyncValue<OnboardingProgressModel?>, OnboardingProgressModel?>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<AsyncValue<OnboardingProgressModel?>,
            OnboardingProgressModel?>,
        AsyncValue<OnboardingProgressModel?>,
        Object?,
        Object?>;
    element.handleCreate(ref, build);
  }
}
