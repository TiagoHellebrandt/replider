import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:google_sign_in/google_sign_in.dart';
import 'package:google_sign_in_platform_interface/google_sign_in_platform_interface.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/core/network/websocket_service.dart';
import 'package:app/features/auth/domain/exceptions/mfa_required_exception.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/auth/domain/repositories/auth_repository.dart';
import 'package:app/features/auth/presentation/controllers/mfa_login_controller.dart';
import 'package:app/features/auth/data/repositories/auth_repository_impl.dart';
import 'package:app/features/auth/data/datasources/auth_local_datasource.dart';
import 'package:app/features/auth/data/datasources/auth_local_datasource_impl.dart';
import 'package:app/features/auth/data/datasources/auth_remote_datasource.dart';
import 'package:app/features/owner/data/impersonation_session.dart';
import 'package:app/features/owner/presentation/controllers/owner_controller.dart';
// User-scoped keepAlive providers — must be invalidated on logout so a
// second user logging in on the same browser tab/web session does not
// see the previous user's cached agents, conversations, channels,
// knowledge, etc.
import 'package:app/features/dashboard/presentation/controllers/dashboard_controller.dart';
import 'package:app/features/settings/presentation/controllers/channels_controller.dart';
import 'package:app/features/settings/presentation/controllers/account_settings_controller.dart';
import 'package:app/features/billing/presentation/controllers/billing_controller.dart';

part 'auth_controller.g.dart';

@riverpod
FlutterSecureStorage secureStorage(Ref ref) {
  return const FlutterSecureStorage();
}

@riverpod
AuthLocalDataSource authLocalDataSource(Ref ref) {
  final storage = ref.watch(secureStorageProvider);
  return AuthLocalDataSourceImpl(storage);
}

@riverpod
AuthRemoteDataSource authRemoteDataSource(Ref ref) {
  final dio = ref.watch(dioProvider);
  return AuthRemoteDataSource(dio);
}

@riverpod
AuthRepository authRepository(Ref ref) {
  final localDataSource = ref.watch(authLocalDataSourceProvider);
  final remoteDataSource = ref.watch(authRemoteDataSourceProvider);
  return AuthRepositoryImpl(
    localDataSource: localDataSource,
    remoteDataSource: remoteDataSource,
  );
}

/// The Google OAuth2 client ID, injected at build time via
/// `--dart-define=GOOGLE_CLIENT_ID=...`. **MUST** be set or sign-in cannot
/// work — Google's `id.initialize()` accepts an empty string but every
/// subsequent prompt fails.
const _googleClientId = String.fromEnvironment('GOOGLE_CLIENT_ID');

/// Singleton GoogleSignIn instance.
///
/// CRITICAL: Must be a single instance per app lifetime. The
/// `google_sign_in_web` plugin internally calls
/// `google.accounts.id.initialize()` on construction, and Google's GSI library
/// logs a warning ("called multiple times") plus exhibits race conditions when
/// multiple instances exist concurrently. `keepAlive: true` ensures the
/// instance survives Riverpod's auto-dispose.
///
/// We also call `isSignedIn()` eagerly here. That method has no UI side
/// effects but it does internally invoke `_ensureInitialized()`, which is
/// what triggers the underlying platform plugin to load the GIS SDK and call
/// `id.initialize()` exactly once. Without this, the plugin only initializes
/// on the first user-triggered method call, which delays the first click and
/// can cause "Getting ready" placeholders to hang if any UI awaits the
/// `initialized` future.
@Riverpod(keepAlive: true)
GoogleSignIn googleSignIn(Ref ref) {
  if (kIsWeb && _googleClientId.isEmpty) {
    debugPrint(
      '[auth] WARNING: GOOGLE_CLIENT_ID is empty on web. Pass it at build time '
      'with --dart-define=GOOGLE_CLIENT_ID=<your-id>.apps.googleusercontent.com '
      'or Google Sign-In will fail silently.',
    );
  }

  final instance = GoogleSignIn(
    clientId: _googleClientId,
    scopes: const ['email', 'profile'],
  );

  // Web only: eagerly trigger plugin initialization in the background by
  // calling a no-side-effect method. This loads the GIS SDK, runs
  // `id.initialize()` exactly once, and resolves the plugin's `initialized`
  // future — which the rendered GIS button widget awaits via FutureBuilder.
  //
  // Without this pre-warm, the rendered button stays in its "Getting ready"
  // placeholder state until something else triggers init.
  //
  // We skip this on the Dart VM (tests) because the platform plugin requires
  // `WidgetsFlutterBinding.ensureInitialized()`, which test runners only set
  // up on demand.
  if (kIsWeb) {
    unawaited(instance.isSignedIn().catchError((Object e, StackTrace _) {
      debugPrint('[auth] eager isSignedIn() failed: $e');
      return false;
    }));
  }

  return instance;
}

/// A long-lived stream of [GoogleSignInUserData] emitted by the web plugin
/// whenever a credential is delivered (One Tap, FedCM, or the rendered GIS
/// button). The relevant field is `idToken`, which the backend exchanges for
/// a session JWT.
///
/// Why a stream and not a `Future`: the rendered GIS button is asynchronous —
/// the user clicks, picks an account in the Google popup, consents, and only
/// then does the credential arrive. There's no caller-side `await` — we have
/// to listen.
///
/// Empty on non-web platforms (mobile uses the [loginWithGoogle] method
/// directly).
@Riverpod(keepAlive: true)
Stream<GoogleSignInUserData?> googleUserDataEvents(Ref ref) {
  // Make sure the GoogleSignIn singleton has been built so the underlying
  // platform plugin is registered before we attach to its stream.
  ref.watch(googleSignInProvider);
  if (!kIsWeb) return const Stream.empty();
  return GoogleSignInPlatform.instance.userDataEvents ?? const Stream.empty();
}

@riverpod
class AuthController extends _$AuthController {
  @override
  FutureOr<UserModel?> build() async {
    // Attempt to load the user silently on startup
    final repository = ref.watch(authRepositoryProvider);
    return await repository.getCurrentUser();
  }

  Future<void> login(String email, String password,
      {bool rememberMe = true}) async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final repository = ref.read(authRepositoryProvider);
      try {
        return await repository.login(email, password, rememberMe: rememberMe);
      } on MfaRequiredException catch (e) {
        // Stash the short-lived MFA session for the login screen and
        // settle the auth state back to "logged out" — no JWT exists
        // yet, so any guard checking `data != null` correctly keeps
        // the user on the login flow.
        ref
            .read(mfaLoginControllerProvider.notifier)
            .start(e.mfaSession, rememberMe: e.rememberMe);
        return null;
      }
    });
  }

  /// Trades the in-progress MFA session for a real session JWT. The
  /// rememberMe choice is read from [mfaLoginControllerProvider] so
  /// the user can't accidentally lose it between the password and
  /// code screens.
  Future<void> verifyMfaLogin(String code) async {
    final pending = ref.read(mfaLoginControllerProvider);
    if (pending == null) {
      state = AsyncValue.error(
        StateError('no MFA session in progress'),
        StackTrace.current,
      );
      return;
    }
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final repository = ref.read(authRepositoryProvider);
      return await repository.verifyMfaLogin(
        pending.mfaSession,
        code,
        rememberMe: pending.rememberMe,
      );
    });
    if (state.hasValue && state.value != null) {
      ref.read(mfaLoginControllerProvider.notifier).clear();
    }
  }

  Future<void> register(
    String accountName,
    String adminName,
    String adminEmail,
    String password, {
    bool termsAccepted = false,
    String? inviteToken,
  }) async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final repository = ref.read(authRepositoryProvider);
      return await repository.register(
          accountName, adminName, adminEmail, password,
          termsAccepted: termsAccepted, inviteToken: inviteToken);
    });
  }

  /// Triggers the Google Sign-In flow appropriate for the current platform.
  ///
  /// **Web (FedCM / One Tap):** uses `signInSilently()` which delegates to
  /// `google.accounts.id.prompt()`. This is the **only** path that returns a
  /// real `idToken` on the web — `signIn()` is deprecated for web and yields
  /// a synthesized account with `idToken == null`.
  ///
  /// **Mobile / desktop:** uses the native `signIn()` flow which opens the
  /// account chooser and returns an account with both access and id tokens.
  ///
  /// Either way, the resulting `idToken` is exchanged with the backend for
  /// a session JWT.
  Future<void> loginWithGoogle() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      debugPrint('[auth] loginWithGoogle: start (kIsWeb=$kIsWeb)');

      if (kIsWeb && _googleClientId.isEmpty) {
        debugPrint('[auth] loginWithGoogle: GOOGLE_CLIENT_ID is empty — abort');
        throw Exception(
          'GOOGLE_CLIENT_ID not configured. Build/run with '
          '--dart-define=GOOGLE_CLIENT_ID=<your-id>.apps.googleusercontent.com',
        );
      }

      final googleSignIn = ref.read(googleSignInProvider);
      debugPrint(
        '[auth] loginWithGoogle: provider ready, clientId='
        '${_googleClientId.isEmpty ? "<empty>" : "${_googleClientId.substring(0, _googleClientId.length.clamp(0, 12))}..."}',
      );

      // NB: we do NOT call signOut() before signInSilently(). On web,
      // signOut() calls `id.disableAutoSelect()` which then makes
      // `id.prompt()` skip the UI ("dismissed: opt_out_or_no_session").
      // The result would be `signInSilently` returning null silently and
      // the user thinking the button is broken.

      final GoogleSignInAccount? account;
      try {
        if (kIsWeb) {
          // Web: signInSilently triggers FedCM / One Tap. This is the only
          // path on web that returns a real id_token. Requires the user to
          // already be signed into Google in this browser.
          debugPrint('[auth] loginWithGoogle: calling signInSilently (web)');
          account = await googleSignIn.signInSilently(suppressErrors: false);
        } else {
          debugPrint('[auth] loginWithGoogle: calling signIn (mobile)');
          account = await googleSignIn.signIn();
        }
      } catch (e, s) {
        debugPrint('[auth] loginWithGoogle: sign-in threw: $e\n$s');
        rethrow;
      }

      if (account == null) {
        debugPrint(
          '[auth] loginWithGoogle: account is NULL. On web this usually means '
          'the user is not currently signed into Google in this browser. '
          'Open https://accounts.google.com in another tab, sign in, and try again.',
        );
        throw Exception(
          kIsWeb
              ? 'No Google session found in this browser. Sign into Google first.'
              : 'Google sign-in cancelled',
        );
      }

      debugPrint('[auth] loginWithGoogle: got account email=${account.email}');

      debugPrint('[auth] loginWithGoogle: awaiting account.authentication');
      final GoogleSignInAuthentication auth;
      try {
        auth = await account.authentication;
      } catch (e, s) {
        debugPrint(
            '[auth] loginWithGoogle: account.authentication threw: $e\n$s');
        rethrow;
      }
      debugPrint('[auth] loginWithGoogle: authentication object obtained');
      final String? idToken;
      try {
        idToken = auth.idToken;
      } catch (e, s) {
        debugPrint('[auth] loginWithGoogle: reading idToken threw: $e\n$s');
        rethrow;
      }
      debugPrint(
        '[auth] loginWithGoogle: idToken=${idToken == null ? "<NULL>" : "${idToken.substring(0, idToken.length.clamp(0, 20))}... (${idToken.length} chars)"}',
      );

      if (idToken == null) {
        throw Exception(
          'Google did not return an id_token. This is a known issue with the '
          'web `signIn()` method (deprecated). signInSilently should provide '
          'one — verify FedCM is enabled in your browser.',
        );
      }

      debugPrint('[auth] loginWithGoogle: exchanging id_token with backend');
      final repository = ref.read(authRepositoryProvider);
      final user = await repository.loginWithGoogle(idToken);
      debugPrint('[auth] loginWithGoogle: success, user.email=${user.email}');
      return user;
    });
  }

  /// Exchanges a Google id_token (already obtained from the GIS rendered
  /// button on web, via the [googleUserDataEventsProvider] stream) for a
  /// backend session JWT. Used by [GoogleSignInButton] on web.
  Future<void> exchangeGoogleIdTokenForSession(String idToken) async {
    debugPrint('[auth] exchangeGoogleIdTokenForSession: start');
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final repository = ref.read(authRepositoryProvider);
      final user = await repository.loginWithGoogle(idToken);
      debugPrint(
          '[auth] exchangeGoogleIdTokenForSession: success, user.email=${user.email}');
      return user;
    });
  }

  Future<void> loginWithMeta(String accessToken) async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final repository = ref.read(authRepositoryProvider);
      return await repository.loginWithMeta(accessToken);
    });
  }

  Future<void> forgotPassword(String email) async {
    // We don't change the User state for a forgot password request, it just returns void
    final repository = ref.read(authRepositoryProvider);
    await repository.forgotPassword(email);
  }

  /// Consumes the emailed reset token + sets the new password. No auth-state
  /// change — the user logs in fresh afterwards.
  Future<void> resetPassword(String token, String newPassword) async {
    final repository = ref.read(authRepositoryProvider);
    await repository.resetPassword(token, newPassword);
  }

  /// Updates the in-memory user state AND saves to local storage reactively.
  /// Called after a successful PUT /me so screens update without re-login.
  /// [signature]: null preserves the current one, '' clears it.
  Future<void> updateUserInState(String name, String email,
      {bool? isNew, String? signature}) async {
    final current = state.asData?.value;
    if (current == null) return;
    final newSignature = signature == null
        ? current.signature
        : (signature.isEmpty ? null : signature);
    final updated = UserModel(
      id: current.id,
      name: name,
      email: email,
      token: current.token,
      isNew: isNew ?? false,
      isTrialing: current.isTrialing,
      planName: current.planName,
      role: current.role,
      picture: current.picture,
      trialDaysRemaining: current.trialDaysRemaining,
      subscriptionStatus: current.subscriptionStatus,
      isContractPlan: current.isContractPlan,
      openAIKeyMasked: current.openAIKeyMasked,
      geminiKeyMasked: current.geminiKeyMasked,
      impersonatingActor: current.impersonatingActor,
      totpEnabled: current.totpEnabled,
      trialBypass: current.trialBypass,
      isEmailVerified: current.isEmailVerified,
      accountCurrency: current.accountCurrency,
      isPlatformAccount: current.isPlatformAccount,
      isSeller: current.isSeller,
      signature: newSignature,
    );
    // Save to local storage so it survives restarts
    final localDataSource = ref.read(authLocalDataSourceProvider);
    await localDataSource.saveUser(updated);
    // Update Riverpod state so all watchers rebuild immediately
    state = AsyncValue.data(updated);
  }

  /// Re-fetches /me so screens that mutated account state (BYOK API
  /// keys, onboarding flags, etc.) see the fresh values without a full
  /// page reload. Falls back to the previous state on error so a
  /// transient network blip doesn't bounce the user to login.
  Future<void> refreshMe() async {
    final repository = ref.read(authRepositoryProvider);
    final fresh = await repository.getCurrentUser();
    if (fresh != null) {
      state = AsyncValue.data(fresh);
    }
  }

  /// Invalidate every keepAlive provider that holds per-user data so the
  /// next read refetches under whatever token is now current. autoDispose
  /// providers clean themselves up when the dashboard widgets unmount, but
  /// keepAlive providers (settings, channels, dashboard
  /// sort/filter/realtime) retain the previous
  /// session's rows in memory.
  ///
  /// Shared by [logout] (a different user signing in on the same tab) and
  /// [endImpersonation] (the operator returning from a client's session) —
  /// both leave stale rows behind otherwise. Without this, the dashboard
  /// keeps showing the previous account's agents/conversations until each
  /// provider naturally refetches. We close the leak deterministically.
  void _invalidateUserScopedProviders() {
    ref.invalidate(webSocketServiceProvider);
    ref.invalidate(realtimeListenerProvider);
    // The conversation list + per-conversation messages are autoDispose,
    // so a logout that unmounts the dashboard disposes them for free. But
    // endImpersonation navigates to /home with the dashboard still mounted
    // — without an explicit invalidation the messages screen keeps showing
    // the impersonated client's threads loaded under the now-cleared token.
    ref.invalidate(conversationsProvider);
    ref.invalidate(messagesProvider);
    ref.invalidate(selectedConversationProvider);
    ref.invalidate(conversationSortProvider);
    ref.invalidate(conversationStatusFilterProvider);
    ref.invalidate(typingStateProvider);
    ref.invalidate(usageAlertProvider);
    ref.invalidate(channelsListProvider);
    ref.invalidate(accountSettingsControllerProvider);
    ref.invalidate(accountUsageProvider);
    ref.invalidate(accountPlanLimitsProvider);
  }

  /// Exit an impersonation session and return the operator to their own
  /// account WITHOUT logging them out. The owner-panel "End" banner calls
  /// this.
  ///
  /// /me always runs on the operator's cached token (see
  /// AuthRepositoryImpl.getCurrentUser — it sets the Bearer header
  /// explicitly), so the operator's identity in [state] is already
  /// intact; only the dashboard DATA caches — loaded under the client's
  /// short-lived impersonation token via the AuthInterceptor default —
  /// need to be dropped so they refetch as the operator. Clearing the
  /// impersonation slot is what flips the interceptor back to the
  /// operator token; the invalidation is what makes the UI follow.
  Future<void> endImpersonation() async {
    try {
      await ImpersonationStorage(const FlutterSecureStorage()).clear();
    } catch (_) {
      // Non-fatal — the in-memory teardown still runs.
    }
    ref.invalidate(impersonationControllerProvider);
    _invalidateUserScopedProviders();
    // The owner-access probe (keepAlive) may have resolved to `forbidden`
    // while it ran under the impersonation token — the impersonated admin
    // is not a super-admin. Re-probe under the operator's own token so the
    // owner-panel menu comes back after returning from a session.
    ref.invalidate(ownerAccessStatusProvider);
    ref.invalidate(ownerIsSuperAdminProvider);
    // /me reflected the impersonated client during the session; now the
    // slot is cleared, re-fetch it on the operator's own token to restore
    // the operator's identity (plan, settings, billing).
    await refreshMe();
  }

  Future<void> logout() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      // Order matters:
      //
      // 0) An active impersonation session takes precedence — the
      //    AuthInterceptor uses the impersonation token first, so a
      //    naive logout would blacklist the impersonation token and
      //    leave the operator's own session orphaned (slot still
      //    persisted, next request 401s on the blacklisted token).
      //    Clear the slot BEFORE the logout call so the regular user
      //    token is the one that hits /auth/logout.
      //
      // 1) Close the WebSocket FIRST so it cannot reconnect with the
      //    soon-to-be-revoked token and trigger a 3s reconnect loop
      //    against the backend during logout.
      //
      // 2) Invalidate every keepAlive provider that holds per-user
      //    state (see _invalidateUserScopedProviders).
      //
      // 3) Hit the backend logout endpoint to revoke the token and
      //    clear secure storage.
      try {
        await ImpersonationStorage(const FlutterSecureStorage()).clear();
        ref.invalidate(impersonationControllerProvider);
      } catch (_) {
        // Non-fatal — logout proceeds either way.
      }
      _invalidateUserScopedProviders();
      // Owner-panel access is probed once and cached keepAlive. Without
      // invalidating it here, a stale `forbidden` (probed before the
      // operator was added to SUPER_ADMIN_EMAILS / before the env went
      // live) survives a logout→login on the same tab and the menu stays
      // hidden until a full browser reload. Invalidate so the next login
      // re-probes /owner with the fresh session.
      ref.invalidate(ownerAccessStatusProvider);
      ref.invalidate(ownerIsSuperAdminProvider);
      final repository = ref.read(authRepositoryProvider);
      await repository.logout();
      return null;
    });
  }
}
