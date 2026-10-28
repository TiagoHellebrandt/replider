import 'package:flutter_riverpod/flutter_riverpod.dart';

/// Whether the last trip to the login screen was an expiry rather than a
/// deliberate sign-out.
///
/// Auth is a single 24h HS256 token with no refresh flow (`api/CLAUDE.md`:
/// "no refresh token, no /refresh"), so every daily user gets thrown out once a
/// day, in the middle of something. Being thrown out is unavoidable today;
/// being thrown out with no explanation is not — a login screen that appears
/// unannounced reads as "the app broke" or "somebody logged me out", and the
/// person's next move is to call support.
///
/// A deliberate logout must NOT set this: telling someone their session expired
/// right after they pressed "Sair" is a lie, and lies here cost the trust that
/// the honest message is supposed to buy.
class SessionExpiry extends Notifier<bool> {
  @override
  bool build() => false;

  /// Called from the 401 path only.
  void markExpired() => state = true;

  /// Called once the notice has been shown and acted on, so the next visit to
  /// the login screen — a deliberate sign-out, say — starts clean.
  void clear() => state = false;
}

final sessionExpiryProvider =
    NotifierProvider<SessionExpiry, bool>(SessionExpiry.new);

/// Sanitizes the `?from=` the router carries through the login screen.
///
/// Only an in-app absolute path is allowed back. Anything else — a scheme, a
/// protocol-relative `//evil.com`, a backslash the browser may normalize into
/// one — is discarded rather than followed, because this value survives a round
/// trip through the address bar where anybody can edit it.
String? sanitizeReturnPath(String? raw) {
  if (raw == null || raw.isEmpty) return null;
  if (!raw.startsWith('/')) return null;
  if (raw.startsWith('//') || raw.startsWith(r'/\')) return null;
  if (raw.contains('://')) return null;
  // The login screen itself is not a destination — returning there would
  // bounce the person straight back to the form they just filled in.
  if (raw == '/') return null;
  return raw;
}
