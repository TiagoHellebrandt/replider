import 'dart:convert';

import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// Persisted "I'm currently acting as another account" flag. The owner
/// panel writes this on /impersonate success; the AuthInterceptor reads
/// it to override the regular Bearer token on every request; the home
/// banner reads it to render a red "Acting as X" strip with an end CTA.
///
/// We deliberately keep this in its own storage slot so a power-cycle
/// during an impersonation session is recoverable (the operator can hit
/// "End" without the owner panel having to repopulate state) AND the
/// regular logged-in session token in `cached_user_model` is left
/// untouched — ending the impersonation just deletes this slot.
class ImpersonationSession {
  final String token;
  final DateTime expiresAt;
  final String actor; // super-admin email
  final String accountId;
  final String accountName;
  final String impersonatingUser;

  const ImpersonationSession({
    required this.token,
    required this.expiresAt,
    required this.actor,
    required this.accountId,
    required this.accountName,
    required this.impersonatingUser,
  });

  bool get isExpired => DateTime.now().isAfter(expiresAt);

  Map<String, dynamic> toJson() => {
        'token': token,
        'expires_at': expiresAt.toIso8601String(),
        'actor': actor,
        'account_id': accountId,
        'account_name': accountName,
        'impersonating_user': impersonatingUser,
      };

  factory ImpersonationSession.fromJson(Map<String, dynamic> json) =>
      ImpersonationSession(
        token: json['token'] as String,
        expiresAt: DateTime.parse(json['expires_at'] as String),
        actor: json['actor'] as String,
        accountId: json['account_id'] as String,
        accountName: json['account_name'] as String,
        impersonatingUser: json['impersonating_user'] as String,
      );
}

const _kImpersonationKey = 'owner_impersonation_session';

class ImpersonationStorage {
  final FlutterSecureStorage _storage;
  ImpersonationStorage(this._storage);

  Future<void> write(ImpersonationSession s) async {
    await _storage.write(
        key: _kImpersonationKey, value: jsonEncode(s.toJson()));
  }

  Future<ImpersonationSession?> read() async {
    try {
      final raw = await _storage.read(key: _kImpersonationKey);
      if (raw == null || raw.isEmpty) return null;
      final s = ImpersonationSession.fromJson(
          jsonDecode(raw) as Map<String, dynamic>);
      if (s.isExpired) {
        await clear();
        return null;
      }
      return s;
    } catch (_) {
      return null;
    }
  }

  Future<void> clear() async {
    await _storage.delete(key: _kImpersonationKey);
  }
}
