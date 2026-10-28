// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'account_settings_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Workspace-level settings: company name + danger-zone deletion.
/// Separate from ProfileController because Profile edits the authenticated
/// user (name/email/password) while this edits the Account row shared by
/// everyone in the workspace.
///
/// keepAlive: bouncing between settings tabs (account → channels → account)
/// would otherwise dispose this provider and refire GET /me each time.
/// Mutators (updateAccountName, updateBranding) patch
/// state in place, so keepAlive does not stale. Logout must invalidate
/// in auth_controller.dart.

@ProviderFor(AccountSettingsController)
final accountSettingsControllerProvider = AccountSettingsControllerProvider._();

/// Workspace-level settings: company name + danger-zone deletion.
/// Separate from ProfileController because Profile edits the authenticated
/// user (name/email/password) while this edits the Account row shared by
/// everyone in the workspace.
///
/// keepAlive: bouncing between settings tabs (account → channels → account)
/// would otherwise dispose this provider and refire GET /me each time.
/// Mutators (updateAccountName, updateBranding) patch
/// state in place, so keepAlive does not stale. Logout must invalidate
/// in auth_controller.dart.
final class AccountSettingsControllerProvider extends $AsyncNotifierProvider<
    AccountSettingsController, AccountSettingsState> {
  /// Workspace-level settings: company name + danger-zone deletion.
  /// Separate from ProfileController because Profile edits the authenticated
  /// user (name/email/password) while this edits the Account row shared by
  /// everyone in the workspace.
  ///
  /// keepAlive: bouncing between settings tabs (account → channels → account)
  /// would otherwise dispose this provider and refire GET /me each time.
  /// Mutators (updateAccountName, updateBranding) patch
  /// state in place, so keepAlive does not stale. Logout must invalidate
  /// in auth_controller.dart.
  AccountSettingsControllerProvider._()
      : super(
          from: null,
          argument: null,
          retry: null,
          name: r'accountSettingsControllerProvider',
          isAutoDispose: false,
          dependencies: null,
          $allTransitiveDependencies: null,
        );

  @override
  String debugGetCreateSourceHash() => _$accountSettingsControllerHash();

  @$internal
  @override
  AccountSettingsController create() => AccountSettingsController();
}

String _$accountSettingsControllerHash() =>
    r'8242fc41ca232ebf8d14b9d9d833ed448ed38a39';

/// Workspace-level settings: company name + danger-zone deletion.
/// Separate from ProfileController because Profile edits the authenticated
/// user (name/email/password) while this edits the Account row shared by
/// everyone in the workspace.
///
/// keepAlive: bouncing between settings tabs (account → channels → account)
/// would otherwise dispose this provider and refire GET /me each time.
/// Mutators (updateAccountName, updateBranding) patch
/// state in place, so keepAlive does not stale. Logout must invalidate
/// in auth_controller.dart.

abstract class _$AccountSettingsController
    extends $AsyncNotifier<AccountSettingsState> {
  FutureOr<AccountSettingsState> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref
        as $Ref<AsyncValue<AccountSettingsState>, AccountSettingsState>;
    final element = ref.element as $ClassProviderElement<
        AnyNotifier<AsyncValue<AccountSettingsState>, AccountSettingsState>,
        AsyncValue<AccountSettingsState>,
        Object?,
        Object?>;
    element.handleCreate(ref, build);
  }
}
