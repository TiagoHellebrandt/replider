import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/network/dio_client.dart';

/// Inbound debounce, dashboard side (§6.15, K1).
///
/// The AI waits for the customer to stop typing before it answers. From the
/// attendant's chair that wait is indistinguishable from a broken bot — so the
/// conversation says what is happening, and offers a way to stop waiting.

/// What the server said about one conversation's pending reply.
class AiDebounceState {
  const AiDebounceState({required this.respondsAt, required this.messageCount});

  /// When the AI will answer.
  final DateTime respondsAt;

  /// How many inbound messages collapsed into the pending run. Shown because
  /// "aguardando o cliente terminar (3 mensagens)" explains the delay that
  /// would otherwise read as a stuck bot.
  final int messageCount;

  /// Seconds left, floored at zero. Zero means the worker is about to pick it
  /// up — the banner then says "respondendo" rather than counting negatives.
  int secondsLeft(DateTime now) {
    final left = respondsAt.difference(now).inSeconds;
    return left < 0 ? 0 : left;
  }
}

/// Pending AI replies, keyed by conversation.
///
/// Driven entirely by the `ai_debounce` WebSocket event: an entry with no
/// `responds_at` means the wait is over, so the entry is REMOVED rather than
/// kept at zero. A banner that lingers after the reply landed is worse than no
/// banner — it teaches the attendant to ignore it.
class AiDebounceStates extends Notifier<Map<String, AiDebounceState>> {
  @override
  Map<String, AiDebounceState> build() => const {};

  void set(String conversationId, AiDebounceState state) {
    this.state = {...this.state, conversationId: state};
  }

  void clear(String conversationId) {
    if (!state.containsKey(conversationId)) return;
    final next = {...state}..remove(conversationId);
    state = next;
  }

  AiDebounceState? forConversation(String conversationId) =>
      state[conversationId];
}

final aiDebounceStatesProvider =
    NotifierProvider<AiDebounceStates, Map<String, AiDebounceState>>(
        AiDebounceStates.new);

final aiDebounceActionsProvider = Provider<AiDebounceActions>(
  (ref) => AiDebounceActions(ref.watch(dioProvider)),
);

class AiDebounceActions {
  const AiDebounceActions(this._dio);

  final Dio _dio;

  /// Stops waiting and lets the AI answer now.
  ///
  /// 202 means scheduled, not sent: the worker picks it up on its next tick, so
  /// the UI clears the banner and lets the normal `new_message` event deliver
  /// the reply. Faking an immediate message here would render a bubble that does
  /// not exist yet.
  Future<void> answerNow(String conversationId) =>
      _dio.post<dynamic>('/conversations/$conversationId/ai-now');
}
