// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'mfa_login_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(MfaLoginController)
final mfaLoginControllerProvider = MfaLoginControllerProvider._();

final class MfaLoginControllerProvider
    extends $NotifierProvider<MfaLoginController, MfaLoginPending?> {
  MfaLoginControllerProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'mfaLoginControllerProvider',
          isAutoDispose: true,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$mfaLoginControllerHash();

  @$internal
  @override
  MfaLoginController create() => MfaLoginController();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(MfaLoginPending? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<MfaLoginPending?>(value),
    );
  }
}

String _$mfaLoginControllerHash() =>
    r'b3b055cb41216a6ff67d0bf40059ebd63bffd163';

abstract class _$MfaLoginController extends $Notifier<MfaLoginPending?> {
  MfaLoginPending? build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<MfaLoginPending?, MfaLoginPending?>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<MfaLoginPending?, MfaLoginPending?>,
        MfaLoginPending?,
        Object?,
        Object?>;
    element.handleCreate(ref, build);
  }
}
