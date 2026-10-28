import 'dart:convert';
import 'package:dio/dio.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// Forces a client-side logout when the API rejects an *authenticated*
/// request with 401.
///
/// Auth is a single 24h HS256 access token with no refresh flow (see
/// api/CLAUDE.md — "no refresh token, no /refresh"). Once it expires every
/// request 401s and, without this, the user is stranded on generic error
/// snackbars with no path out but a manual page reload. On the first 401
/// that carried a stored session we run the normal logout teardown (clears
/// storage, invalidates user-scoped providers, resets auth state); the
/// router's refreshListenable then redirects to the login screen.
///
/// A 401 from a login/register attempt (no token stored yet) is a
/// credential error the auth screen surfaces itself — we must NEVER clear
/// on those, hence the stored-session gate. `_handling` guards re-entrancy:
/// [onSessionExpired] itself issues a `/auth/logout` request that races a
/// 401 on the just-revoked token.
class SessionExpiryInterceptor extends Interceptor {
  SessionExpiryInterceptor({
    required Future<void> Function() onSessionExpired,
    required Future<bool> Function(String token) isSessionDead,
    FlutterSecureStorage storage = const FlutterSecureStorage(),
  })  : _onSessionExpired = onSessionExpired,
        _isSessionDead = isSessionDead,
        _storage = storage;

  final Future<void> Function() _onSessionExpired;

  /// Asks the server whether the session REALLY died, with the same token.
  ///
  /// A 401 says "this request was not authorised" — concluding from that that the
  /// session ended is a guess. And it guessed wrong: an endpoint answering 401
  /// where it should answer 403 (insufficient role) tore down the whole session
  /// every ten seconds, and returning to the same screen tore it down again. A
  /// broken endpoint must not be able to throw anyone out of the product.
  final Future<bool> Function(String token) _isSessionDead;

  final FlutterSecureStorage _storage;

  static const _userKey = 'cached_user_model';

  bool _handling = false;

  @override
  void onError(DioException err, ErrorInterceptorHandler handler) async {
    if (err.response?.statusCode != 401 || _handling) {
      return handler.next(err);
    }

    // Only a request that carried a stored session is a real expiry, and only
    // if it carried the session we still have.
    //
    // Two tabs share one storage. A tab left open holds the OLD token in
    // memory; after the session is renewed anywhere, that tab's next request
    // 401s legitimately — and the old code answered by wiping the storage and
    // revoking the token, killing the session the other tab had just obtained.
    // The person is thrown out of a session that was working, by a tab they
    // were not even looking at.
    //
    // So the 401 only counts when the token that failed is still the current
    // one. A 401 on a superseded token is somebody else's stale request.
    if (!await _carriedCurrentSession(err)) {
      return handler.next(err);
    }

    // Confirmation: only end the session with proof. If the check itself fails,
    // nobody is thrown out — erring towards keeping the person in is right here,
    // because the other direction already cost a login loop.
    final token = await _storedToken();
    if (token == null) return handler.next(err);
    bool dead;
    try {
      dead = await _isSessionDead(token);
    } catch (_) {
      // The check did not answer: that is proof of nothing.
      dead = false;
    }
    if (!dead) return handler.next(err);

    _handling = true;
    try {
      await _onSessionExpired();
    } catch (_) {
      // Best-effort — the router still redirects on the auth state change.
    } finally {
      _handling = false;
    }
    handler.next(err);
  }

  /// Whether the request that 401'd was signed with the token we hold now.
  ///
  /// Falls back to "yes" when the request carried no readable Authorization
  /// header: an impersonation send or a future call path that signs elsewhere
  /// should still be able to end a dead session. Erring toward logging out is
  /// the safe direction for a genuine expiry — the unsafe one is only the
  /// stale-token case, which this check catches by comparison.
  Future<bool> _carriedCurrentSession(DioException err) async {
    final stored = await _storedToken();
    if (stored == null || stored.isEmpty) return false;

    final sent = err.requestOptions.headers['Authorization'];
    if (sent is! String || !sent.startsWith('Bearer ')) return true;

    return sent.substring(7) == stored;
  }

  Future<String?> _storedToken() async {
    try {
      final raw = await _storage.read(key: _userKey);
      if (raw == null) return null;
      final token = (json.decode(raw) as Map<String, dynamic>)['token'];
      return token is String ? token : null;
    } catch (_) {
      return null;
    }
  }
}
