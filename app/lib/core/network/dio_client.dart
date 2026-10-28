import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:app/core/auth/session_expiry.dart';
import 'package:app/core/router/app_router.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'accept_language_interceptor.dart';
import 'api_origin_interceptor.dart';
import 'auth_interceptor.dart';
import 'billing_redirect_interceptor.dart';
import 'mock_interceptor.dart';
import 'session_expiry_interceptor.dart';

part 'dio_client.g.dart';

const _apiBaseUrl = String.fromEnvironment(
  'API_BASE_URL',
  defaultValue: 'http://localhost:8080/v1',
);

const _useMock = bool.fromEnvironment('USE_MOCK');

@riverpod
Dio dio(Ref ref) {
  // Prevent mock mode in release builds
  assert(!_useMock || kDebugMode,
      'USE_MOCK must not be enabled in release builds');

  final dio = Dio(
    BaseOptions(
      baseUrl: _apiBaseUrl,
      connectTimeout: const Duration(seconds: 10),
      receiveTimeout: const Duration(seconds: 10),
    ),
  );

  // FIRST in the chain: a response that is not the API answering must be
  // refused before anything downstream tries to read an object out of it.
  // See api_origin_interceptor.dart — a build pointed at the wrong host got
  // HTTP 200 and a page of HTML on every call, and the failure that reached
  // the screen was `DioException [unknown]: null`.
  dio.interceptors.add(ApiOriginInterceptor());
  dio.interceptors.add(AcceptLanguageInterceptor());
  dio.interceptors.add(AuthInterceptor(const FlutterSecureStorage()));
  // 402 TRIAL_EXPIRED / SUBSCRIPTION_INACTIVE → push /billing so the user
  // isn't left stuck looking at a snackbar with no next step.
  dio.interceptors.add(BillingRedirectInterceptor(rootNavigatorKey));
  // 401 on an authenticated request → the 24h token expired (no refresh
  // flow). Force a logout so the router redirects to login instead of
  // stranding the user on error snackbars. Lazy `ref.read` — reading the
  // auth controller at build time would be a dependency cycle.
  //
  // The flag is raised BEFORE the logout so the login screen can tell an
  // expiry apart from someone pressing "Sair". Without it the person is
  // dropped on a login form with no explanation, which reads as a crash.
  dio.interceptors.add(SessionExpiryInterceptor(
    onSessionExpired: () async {
      ref.read(sessionExpiryProvider.notifier).markExpired();
      await ref.read(authControllerProvider.notifier).logout();
    },
    // The proof comes from /me, on a Dio with NO interceptors: with them, this
    // very check's own 401 would come back through here.
    isSessionDead: (token) async {
      try {
        final probe = Dio(BaseOptions(
          baseUrl: _apiBaseUrl,
          connectTimeout: const Duration(seconds: 10),
          receiveTimeout: const Duration(seconds: 10),
        ));
        await probe.get<dynamic>(
          '/me',
          options: Options(headers: {'Authorization': 'Bearer $token'}),
        );
        return false;
      } on DioException catch (e) {
        // Only a 401 is proof. Anything else — network down, 500, timeout — is no
        // reason to throw anyone out.
        return e.response?.statusCode == 401;
      } catch (_) {
        return false;
      }
    },
  ));

  // Debug-only request logging. NEVER log request/response bodies or
  // headers here: the Authorization header carries a live Bearer JWT and
  // bodies carry PII (phone/email/CPF), which would leak into debug web
  // consoles and shared dev logs. Method + URL + status only.
  if (kDebugMode) {
    dio.interceptors.add(LogInterceptor(
      request: true,
      requestHeader: false,
      requestBody: false,
      responseHeader: false,
      responseBody: false,
    ));
  }

  if (_useMock && kDebugMode) {
    dio.interceptors.add(MockInterceptor());
  }

  return dio;
}
