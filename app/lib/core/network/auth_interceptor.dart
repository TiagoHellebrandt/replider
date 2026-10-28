import 'dart:convert';
import 'package:dio/dio.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/owner/data/impersonation_session.dart';

/// Automatically attaches Bearer token to all outgoing requests.
///
/// When an impersonation session is active (owner panel "act as
/// account"), its short-lived token wins over the regular cached
/// user. Expiry is checked inside ImpersonationStorage.read() so the
/// next request after the 15-min window naturally falls back to the
/// operator's own session.
class AuthInterceptor extends Interceptor {
  final FlutterSecureStorage _storage;

  AuthInterceptor(this._storage);

  static const _userKey = 'cached_user_model';

  @override
  void onRequest(
      RequestOptions options, RequestInterceptorHandler handler) async {
    // Skip if Authorization is already set (e.g., for /me endpoint)
    if (options.headers.containsKey('Authorization')) {
      return handler.next(options);
    }

    // Impersonation slot takes precedence. The owner panel writes here
    // on /impersonate success; the AuthInterceptor uses the short-
    // lived token until it expires (auto-cleared inside read()).
    try {
      final imp = await ImpersonationStorage(_storage).read();
      if (imp != null && imp.token.isNotEmpty) {
        options.headers['Authorization'] = 'Bearer ${imp.token}';
        return handler.next(options);
      }
    } catch (_) {
      // Non-fatal — fall through to the regular token.
    }

    try {
      final userJsonStr = await _storage.read(key: _userKey);
      if (userJsonStr != null) {
        final jsonMap = json.decode(userJsonStr) as Map<String, dynamic>;
        final user = UserModel.fromJson(jsonMap);
        if (user.token.isNotEmpty) {
          options.headers['Authorization'] = 'Bearer ${user.token}';
        }
      }
    } catch (_) {
      // If we can't read the token, just proceed without it
    }

    handler.next(options);
  }
}
