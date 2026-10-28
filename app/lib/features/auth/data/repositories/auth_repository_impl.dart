import 'package:dio/dio.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:app/features/owner/data/impersonation_session.dart';
import 'package:app/features/auth/domain/repositories/auth_repository.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/auth/domain/models/auth_dto.dart';
import 'package:app/features/auth/domain/exceptions/mfa_required_exception.dart';
import 'package:app/features/auth/data/datasources/auth_local_datasource.dart';
import 'package:app/features/auth/data/datasources/auth_remote_datasource.dart';
import 'package:app/core/analytics/utm_bootstrap.dart';
import 'package:app/core/auth/account_role.dart';
import 'package:app/core/utils/signup_locale.dart';

class AuthRepositoryImpl implements AuthRepository {
  final AuthRemoteDataSource remoteDataSource;
  final AuthLocalDataSource localDataSource;

  AuthRepositoryImpl({
    required this.remoteDataSource,
    required this.localDataSource,
  });

  Future<UserModel> _fetchAndSaveUserFromToken(String token,
      {bool isNew = false}) async {
    final me = await remoteDataSource.getCurrentUser('Bearer $token');

    final pictureRaw = me.user['picture'] as String?;
    final user = UserModel(
      id: me.user['id'] as String? ?? '',
      name: me.user['name'] as String? ?? '',
      email: me.user['email'] as String? ?? '',
      token: token,
      isNew: isNew,
      isTrialing: me.account['is_trialing'] as bool? ?? false,
      accountName: me.account['name'] as String? ?? '',
      planName: me.account['plan_name'] as String? ?? 'Trial',
      role: me.user['role'] as String? ?? 'attendant',
      picture:
          (pictureRaw != null && pictureRaw.isNotEmpty) ? pictureRaw : null,
      isContractPlan: me.account['is_contract_plan'] as bool? ?? false,
      openAIKeyMasked: me.account['openai_key_masked'] as String? ?? '',
      geminiKeyMasked: me.account['gemini_key_masked'] as String? ?? '',
      totpEnabled: me.user['totp_enabled'] as bool? ?? false,
      trialBypass: me.account['trial_bypass'] as bool? ?? false,
      isEmailVerified: me.user['is_email_verified'] as bool? ?? true,
      accountCurrency: me.account['currency'] as String? ?? 'brl',
      isPlatformAccount: me.isPlatformAccount,
      isSeller: me.isSeller,
      signature: me.user['signature'] as String?,
    );
    // ALWAYS persist. The AuthInterceptor reads the bearer token only
    // from secure storage (never from Riverpod state), so a session that
    // isn't written here cannot authenticate a single request — every
    // call 401s and a page reload logs the user out. This was the root
    // cause of the "logged in but everything 401s / refresh kicks back to
    // login" outage when "Lembrar-me" was left unticked. Remember-me must
    // never gate the write; if a true session-only mode is wanted it has
    // to clear on cold start, not skip persistence entirely.
    await localDataSource.saveUser(user);
    return user;
  }

  @override
  Future<UserModel> login(String email, String password,
      {bool rememberMe = true}) async {
    final res = await remoteDataSource
        .login(AuthenticateUserDTO(email: email, password: password));
    if (res.mfaRequired == true && (res.mfaSession ?? '').isNotEmpty) {
      throw MfaRequiredException(
        mfaSession: res.mfaSession!,
        rememberMe: rememberMe,
      );
    }
    final token = res.token ?? '';
    if (token.isEmpty) {
      throw StateError('login response missing token');
    }
    return _fetchAndSaveUserFromToken(token, isNew: res.isNew ?? false);
  }

  @override
  Future<UserModel> verifyMfaLogin(String mfaSession, String code,
      {bool rememberMe = true}) async {
    final res = await remoteDataSource.verifyMfaLogin(
      VerifyMfaLoginDTO(mfaSession: mfaSession, code: code),
    );
    return _fetchAndSaveUserFromToken(res.token, isNew: res.isNew ?? false);
  }

  @override
  Future<UserModel> register(
      String accountName, String adminName, String adminEmail, String password,
      {bool termsAccepted = false, String? inviteToken}) async {
    await remoteDataSource.register(CreateAccountDTO(
      accountName: accountName,
      adminName: adminName,
      adminEmail: adminEmail,
      password: password,
      termsAccepted: termsAccepted,
      inviteToken: inviteToken,
      utmSource: UtmBootstrap.source,
      utmMedium: UtmBootstrap.medium,
      utmCampaign: UtmBootstrap.campaign,
      referralCode: UtmBootstrap.referralCode,
      locale: signupLocale(),
    ));
    // Usually we would login automatically, or return token.
    // Assuming backend returns just message... we'll do an automatic login right after!
    return login(adminEmail, password);
  }

  @override
  Future<void> forgotPassword(String email) async {
    await remoteDataSource.forgotPassword(ForgotPasswordDTO(email: email));
  }

  @override
  Future<void> resetPassword(String token, String newPassword) async {
    await remoteDataSource.resetPassword(
      ResetPasswordDTO(token: token, newPassword: newPassword),
    );
  }

  @override
  Future<UserModel> loginWithGoogle(String idToken) async {
    final res = await remoteDataSource.loginWithGoogle(SocialAuthDTO(
      idToken: idToken,
      locale: signupLocale(),
      referralCode: UtmBootstrap.referralCode,
    ));
    return _fetchAndSaveUserFromToken(res.token, isNew: res.isNew ?? false);
  }

  @override
  Future<UserModel> loginWithMeta(String accessToken) async {
    final res = await remoteDataSource.loginWithMeta(SocialAuthDTO(
      idToken: accessToken,
      locale: signupLocale(),
      referralCode: UtmBootstrap.referralCode,
    ));
    return _fetchAndSaveUserFromToken(res.token, isNew: res.isNew ?? false);
  }

  @override
  Future<void> logout() async {
    await localDataSource.clearUser();
  }

  /// Builds the refreshed UserModel from a /me response, falling back to
  /// the cached values for fields the response may omit. `token` keeps the
  /// operator's stored token — the AuthInterceptor only ever reads the
  /// token from secure storage, so this field is identity metadata, not an
  /// auth credential.
  UserModel _userFromMe(GetMeResponse me, UserModel cached) {
    final onboardingCompleted =
        me.account['onboarding_completed'] as bool? ?? false;
    // The wizard is the ADMIN's: its first step is PATCH /accounts. Below
    // admin the account's flag must not send a person to a screen that
    // answers 403 and has no Skip (the server's `is_new` applies the same
    // gate at login; this is the cold-start mirror).
    final role = AccountRole.from(me.user['role'] as String? ?? cached.role);
    final pictureRaw = me.user['picture'] as String?;
    return UserModel(
      id: me.user['id'] as String? ?? cached.id,
      name: me.user['name'] as String? ?? cached.name,
      email: me.user['email'] as String? ?? cached.email,
      token: cached.token,
      isNew: !onboardingCompleted && role.satisfies(AccountRole.admin),
      isTrialing: me.account['is_trialing'] as bool? ?? cached.isTrialing,
      planName: me.account['plan_name'] as String? ?? cached.planName,
      role: me.user['role'] as String? ?? cached.role,
      picture: (pictureRaw != null && pictureRaw.isNotEmpty)
          ? pictureRaw
          : cached.picture,
      trialDaysRemaining: me.account['trial_days_remaining'] as int?,
      isContractPlan:
          me.account['is_contract_plan'] as bool? ?? cached.isContractPlan,
      openAIKeyMasked:
          me.account['openai_key_masked'] as String? ?? cached.openAIKeyMasked,
      geminiKeyMasked:
          me.account['gemini_key_masked'] as String? ?? cached.geminiKeyMasked,
      totpEnabled: me.user['totp_enabled'] as bool? ?? cached.totpEnabled,
      trialBypass: me.account['trial_bypass'] as bool? ?? cached.trialBypass,
      isEmailVerified:
          me.user['is_email_verified'] as bool? ?? cached.isEmailVerified,
      accountCurrency:
          me.account['currency'] as String? ?? cached.accountCurrency,
      isPlatformAccount: me.isPlatformAccount,
      isSeller: me.isSeller,
      // /me omits signature entirely when unset — that IS the fresh
      // truth (cleared), so no cached fallback here.
      signature: me.user['signature'] as String?,
    );
  }

  @override
  Future<UserModel?> getCurrentUser() async {
    final cached = await localDataSource.getUser();
    if (cached == null) return null;

    // During an active impersonation session, /me must reflect the
    // IMPERSONATED account — settings, plan, billing all show the client,
    // which is the whole point of impersonation. Fetch with the
    // impersonation token but DON'T persist the result: the operator's own
    // cached user stays in storage so ending the session restores them.
    final imp = await ImpersonationStorage(const FlutterSecureStorage()).read();
    final meToken =
        (imp != null && imp.token.isNotEmpty) ? imp.token : cached.token;
    try {
      final me = await remoteDataSource.getCurrentUser('Bearer $meToken');
      final refreshed = _userFromMe(me, cached);
      if (imp == null) {
        await localDataSource.saveUser(refreshed); // persist only the operator
      }
      return refreshed;
    } catch (e) {
      if (e is DioException && e.response?.statusCode == 401) {
        if (imp != null) {
          // Impersonation token expired/revoked — drop the slot and fall
          // back to the operator's own /me. NEVER log the operator out
          // here: their stored session is still valid.
          await ImpersonationStorage(const FlutterSecureStorage()).clear();
          try {
            final me =
                await remoteDataSource.getCurrentUser('Bearer ${cached.token}');
            final refreshed = _userFromMe(me, cached);
            await localDataSource.saveUser(refreshed);
            return refreshed;
          } catch (_) {
            return cached;
          }
        }
        // 401 on the operator's own token — expired/revoked, force re-login.
        await localDataSource.clearUser();
        return null;
      }
      // Other network errors (timeout, DNS) are survivable: the cached
      // user might still work once connectivity resumes.
      return cached;
    }
  }
}
