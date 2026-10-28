import 'dart:async';
import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';

/// How long is left before the session ends.
///
/// Auth is a single 24-hour token with no refresh (`api/CLAUDE.md`: "no refresh
/// token, no /refresh"). Which means: once a day, in the middle of something, the
/// person is thrown out. That is the server's decision and does not change from
/// here — but being thrown out WITH NO WARNING is another matter. The 401 lands
/// mid-action, and whatever they were doing is lost with it.
///
/// The deadline already travels inside the token itself, in the `exp` claim. Here
/// it is only READ — the server always validates the signature — so the screen
/// can warn in advance and the person picks when to sign in again.

/// How early to warn.
const kSessionWarningWindow = Duration(minutes: 15);

/// Reads a JWT's `exp`, in UTC. Null for anything that is not a readable token —
/// an unexpected format is no reason to warn about anything.
DateTime? jwtExpiry(String token) {
  final parts = token.split('.');
  if (parts.length != 3) return null;
  try {
    var payload = parts[1].replaceAll('-', '+').replaceAll('_', '/');
    // Unpadded base64url is the norm in a JWT.
    payload = payload.padRight((payload.length + 3) ~/ 4 * 4, '=');
    final map = jsonDecode(utf8.decode(base64.decode(payload)));
    if (map is! Map) return null;
    final exp = map['exp'];
    if (exp is! int) return null;
    return DateTime.fromMillisecondsSinceEpoch(exp * 1000, isUtc: true);
  } catch (_) {
    return null;
  }
}

/// How long is left, or null when it cannot be known.
class SessionCountdown extends Notifier<Duration?> {
  Timer? _timer;

  @override
  Duration? build() {
    ref.onDispose(() => _timer?.cancel());
    // One minute: the figure is in minutes, and a per-second clock would rebuild
    // the whole screen 60 times a minute for nothing.
    _timer ??= Timer.periodic(const Duration(minutes: 1), (_) => _tick());
    // The first read happens OUTSIDE build. Reading auth in here means reading it
    // while it may itself be building — that is how the whole app shell broke,
    // over a warning.
    Future<void>.microtask(_tick);
    return null;
  }

  void _tick() {
    final next = _remaining();
    if (next != state) state = next;
  }

  Duration? _remaining() {
    // A session that cannot be read is no reason to warn about anything — and
    // this warning lives in the app shell, which has to come up even when auth
    // has not resolved (or failed).
    final UserModel? user;
    try {
      user = ref.read(authControllerProvider).asData?.value;
    } catch (_) {
      return null;
    }
    if (user == null) return null;
    final exp = jwtExpiry(user.token);
    if (exp == null) return null;
    final left = exp.difference(DateTime.now().toUtc());
    return left.isNegative ? Duration.zero : left;
  }
}

final sessionCountdownProvider =
    NotifierProvider<SessionCountdown, Duration?>(SessionCountdown.new);

/// Whether a warning is due now.
bool shouldWarnAboutSession(Duration? remaining) =>
    remaining != null && remaining <= kSessionWarningWindow;
