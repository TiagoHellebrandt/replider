import 'package:app/features/auth/domain/models/user_model.dart';

abstract class AuthRepository {
  /// [rememberMe] controls whether the session token is persisted to
  /// `FlutterSecureStorage`. When false, the user stays authenticated
  /// only for the current app lifetime — closing the tab/app wipes
  /// them back to the login screen.
  ///
  /// Throws [MfaRequiredException] when the user has 2FA enabled —
  /// the controller catches it and pivots the UI to the MFA-code
  /// step. The exception carries the short-lived `mfa_session` that
  /// must be traded back via [verifyMfaLogin].
  Future<UserModel> login(String email, String password,
      {bool rememberMe = true});

  /// Second leg of the 2FA login flow. Trades the [mfaSession]
  /// returned by [login] together with a current 6-digit TOTP code
  /// (or a single-use recovery code) for a regular session JWT.
  Future<UserModel> verifyMfaLogin(String mfaSession, String code,
      {bool rememberMe = true});
  Future<UserModel> register(
      String accountName, String adminName, String adminEmail, String password,
      {bool termsAccepted = false, String? inviteToken});
  Future<void> forgotPassword(String email);
  Future<void> resetPassword(String token, String newPassword);
  Future<UserModel> loginWithGoogle(String idToken);
  Future<UserModel> loginWithMeta(String accessToken);
  Future<void> logout();
  Future<UserModel?> getCurrentUser();
}
