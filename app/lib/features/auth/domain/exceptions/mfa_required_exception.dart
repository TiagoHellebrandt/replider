/// Thrown by [AuthRepository.login] when the API returns
/// `mfa_required: true`. The [mfaSession] is the short-lived token
/// the client must trade for the real session via
/// [AuthRepository.verifyMfaLogin]. Carries [rememberMe] so the
/// downstream verify call can honour the same persistence choice
/// the user picked on the password screen.
class MfaRequiredException implements Exception {
  final String mfaSession;
  final bool rememberMe;

  MfaRequiredException({required this.mfaSession, required this.rememberMe});

  @override
  String toString() => 'MfaRequiredException(mfaSession=<redacted>)';
}
