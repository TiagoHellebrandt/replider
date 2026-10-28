// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'nps_prompt_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Tracks whether the in-app NPS prompt should stay hidden for the current
/// account (US-178). The "once per quarter" cadence AND the "not on a
/// brand-new account / first login" gate are now decided **server-side**
/// (`GET /me/nps-prompt`) and persisted in the DB, replacing the old
/// client-only `FlutterSecureStorage` quarter flag — so the choice is
/// durable, per-account, and survives a storage clear or device switch.
///
/// `build()` returns `true` when the prompt should stay hidden (already
/// handled this quarter, account too new, or the check failed → fail
/// closed so a flaky request never nags). Both [submit] and [dismiss]
/// stamp the quarter server-side so a follow-up within the same quarter
/// never re-prompts.

@ProviderFor(NpsPromptController)
final npsPromptControllerProvider = NpsPromptControllerProvider._();

/// Tracks whether the in-app NPS prompt should stay hidden for the current
/// account (US-178). The "once per quarter" cadence AND the "not on a
/// brand-new account / first login" gate are now decided **server-side**
/// (`GET /me/nps-prompt`) and persisted in the DB, replacing the old
/// client-only `FlutterSecureStorage` quarter flag — so the choice is
/// durable, per-account, and survives a storage clear or device switch.
///
/// `build()` returns `true` when the prompt should stay hidden (already
/// handled this quarter, account too new, or the check failed → fail
/// closed so a flaky request never nags). Both [submit] and [dismiss]
/// stamp the quarter server-side so a follow-up within the same quarter
/// never re-prompts.
final class NpsPromptControllerProvider
    extends $AsyncNotifierProvider<NpsPromptController, bool> {
  /// Tracks whether the in-app NPS prompt should stay hidden for the current
  /// account (US-178). The "once per quarter" cadence AND the "not on a
  /// brand-new account / first login" gate are now decided **server-side**
  /// (`GET /me/nps-prompt`) and persisted in the DB, replacing the old
  /// client-only `FlutterSecureStorage` quarter flag — so the choice is
  /// durable, per-account, and survives a storage clear or device switch.
  ///
  /// `build()` returns `true` when the prompt should stay hidden (already
  /// handled this quarter, account too new, or the check failed → fail
  /// closed so a flaky request never nags). Both [submit] and [dismiss]
  /// stamp the quarter server-side so a follow-up within the same quarter
  /// never re-prompts.
  NpsPromptControllerProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'npsPromptControllerProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$npsPromptControllerHash();

  @$internal
  @override
  NpsPromptController create() => NpsPromptController();
}

String _$npsPromptControllerHash() =>
    r'0a0fe2df91aa208089f6f80177ff33e85e4a0ed5';

/// Tracks whether the in-app NPS prompt should stay hidden for the current
/// account (US-178). The "once per quarter" cadence AND the "not on a
/// brand-new account / first login" gate are now decided **server-side**
/// (`GET /me/nps-prompt`) and persisted in the DB, replacing the old
/// client-only `FlutterSecureStorage` quarter flag — so the choice is
/// durable, per-account, and survives a storage clear or device switch.
///
/// `build()` returns `true` when the prompt should stay hidden (already
/// handled this quarter, account too new, or the check failed → fail
/// closed so a flaky request never nags). Both [submit] and [dismiss]
/// stamp the quarter server-side so a follow-up within the same quarter
/// never re-prompts.

abstract class _$NpsPromptController extends $AsyncNotifier<bool> {
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
