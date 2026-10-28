import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'mfa_login_controller.g.dart';

/// Snapshot of an in-progress MFA login. Populated when the password
/// step succeeds against a 2FA-enabled account; the login screen
/// switches to the code-entry view while it's non-null. Cleared on
/// successful verify, on cancel, or when the user navigates back to
/// the password step.
class MfaLoginPending {
  final String mfaSession;
  final bool rememberMe;

  const MfaLoginPending({required this.mfaSession, required this.rememberMe});
}

@riverpod
class MfaLoginController extends _$MfaLoginController {
  @override
  MfaLoginPending? build() => null;

  void start(String mfaSession, {required bool rememberMe}) {
    state = MfaLoginPending(mfaSession: mfaSession, rememberMe: rememberMe);
  }

  void clear() {
    state = null;
  }
}
