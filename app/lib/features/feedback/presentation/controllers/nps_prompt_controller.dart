import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/network/dio_client.dart';

part 'nps_prompt_controller.g.dart';

/// Tracks whether the in-app NPS prompt should stay hidden for the current
/// account (US-178). The "once per quarter" cadence AND the "not on a
/// brand-new account / first login" gate are now decided **server-side**
/// (`GET /me/nps-prompt`) and persisted in the DB, replacing the old
/// client-only `FlutterSecureStorage` quarter flag — so the choice is
/// durable, per-account, and survives a storage clear or device switch.
///
/// `build()` returns `true` when the prompt should stay hidden (already
/// handled this quarter, account too new, or the check failed → fail
/// closed so a flaky request never nags). Both [submit] and [dismiss]
/// stamp the quarter server-side so a follow-up within the same quarter
/// never re-prompts.
@Riverpod(keepAlive: true)
class NpsPromptController extends _$NpsPromptController {
  @override
  Future<bool> build() async {
    try {
      final res = await ref
          .watch(dioProvider)
          .get<Map<String, dynamic>>('/me/nps-prompt');
      final eligible = res.data?['eligible'] as bool? ?? false;
      // eligible == show → not handled. Hidden when not eligible.
      return !eligible;
    } catch (_) {
      // Network/parse failure → don't prompt (fail closed).
      return true;
    }
  }

  /// Submits the NPS score (0-10) plus an optional comment to the API. The
  /// backend stamps the quarter on success. Returns `null` on success (and
  /// hides the prompt), or a localised error string the modal can surface
  /// while staying open.
  Future<String?> submit(int score, String comment) async {
    try {
      final dio = ref.read(dioProvider);
      await dio.post<dynamic>('/me/nps', data: {
        'score': score,
        'comment': comment,
      });
    } catch (e) {
      return parseApiError(e);
    }
    state = const AsyncValue.data(true);
    return null;
  }

  /// "Maybe later" — snoozes the prompt for the rest of the current quarter
  /// without submitting a score. Stamps the quarter server-side.
  Future<void> dismiss() async {
    // Hide immediately; the server stamp is best-effort (worst case the
    // prompt re-offers next quarter, never this session).
    state = const AsyncValue.data(true);
    try {
      await ref.read(dioProvider).post<dynamic>('/me/nps/dismiss');
    } catch (_) {
      // Non-fatal.
    }
  }
}
