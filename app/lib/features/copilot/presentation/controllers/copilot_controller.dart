import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/network/dio_client.dart';

/// The in-app copilot (US-M026).
///
/// The transcript lives HERE, in the client: the copilot is a tool, not a
/// record. Persisting every exploratory question server-side would be a
/// data-retention liability nobody asked for, and the API takes the
/// history back on each turn.

enum CopilotRole { user, assistant }

class CopilotTurn {
  const CopilotTurn({
    required this.role,
    required this.text,
    this.actions = const [],
    this.toolsUsed = const [],
    this.receipts = const [],
    this.contextText,
    this.failed = false,
  });

  final CopilotRole role;
  final String text;
  final List<CopilotAction> actions;

  /// What actually ran. Shown so the user sees the WORK rather than being
  /// asked to trust a paragraph.
  final List<String> toolsUsed;

  /// What the tools that WRITE reported, word for word, straight from the
  /// server.
  ///
  /// A prospecting run that created 56 leads was announced here as "1 of them was
  /// added as a new lead" — the turn's text is the model's prose, and on an action
  /// that changes the account prose does not serve as an account of what
  /// happened.
  final List<String> receipts;

  /// O texto como o modelo escreveu, antes de a tela limpar identificadores.
  ///
  /// This is what goes back as history on the next question. Without it, the id
  /// the model just cited disappears from the conversation along with the
  /// stripping: it asks "confirm which funnel", the person answers "yes, use that
  /// one", and the next call goes out with a guessed id.
  ///
  /// Null when there was nothing to strip — the displayed text serves then.
  final String? contextText;

  /// What the next question sends as history.
  String get historyText => contextText ?? text;

  final bool failed;
}

class CopilotAction {
  const CopilotAction({required this.label, required this.route});

  final String label;
  final String route;

  factory CopilotAction.fromJson(Map<String, dynamic> json) => CopilotAction(
        label: json['label'] as String? ?? '',
        route: json['route'] as String? ?? '',
      );
}

class CopilotState {
  const CopilotState({this.turns = const [], this.thinking = false});

  final List<CopilotTurn> turns;
  final bool thinking;

  CopilotState copyWith({List<CopilotTurn>? turns, bool? thinking}) =>
      CopilotState(
        turns: turns ?? this.turns,
        thinking: thinking ?? this.thinking,
      );
}

class CopilotController extends Notifier<CopilotState> {
  @override
  CopilotState build() => const CopilotState();

  /// Sends one turn. The user's message is appended immediately — a chat
  /// that swallows what you typed until the server answers feels broken.
  Future<void> send(String message) async {
    final text = message.trim();
    if (text.isEmpty || state.thinking) return;

    final history = state.turns
        .where((t) => !t.failed)
        .map((t) => <String, dynamic>{
              'role': t.role == CopilotRole.user ? 'user' : 'model',
              'content': t.historyText,
            })
        .toList();

    state = state.copyWith(
      turns: [...state.turns, CopilotTurn(role: CopilotRole.user, text: text)],
      thinking: true,
    );

    try {
      final res = await ref.read(dioProvider).post<Map<String, dynamic>>(
            '/copilot/ask',
            data: <String, dynamic>{'message': text, 'history': history},
            // The global receive timeout is 10s, which is fine for a REST read and
            // far too short here: one turn is a model call plus however many tool
            // round-trips it decides to make. At 10s the client gives up on
            // answers the server is still producing, and the person is told their
            // internet is broken. nginx allows 120s on this API.
            options: Options(receiveTimeout: const Duration(seconds: 120)),
          );
      final data = res.data ?? const <String, dynamic>{};
      state = state.copyWith(
        turns: [
          ...state.turns,
          CopilotTurn(
            role: CopilotRole.assistant,
            text: data['message'] as String? ?? '',
            actions: (data['actions'] as List<dynamic>? ?? const [])
                .whereType<Map<String, dynamic>>()
                .map(CopilotAction.fromJson)
                .toList(),
            toolsUsed: (data['tools_used'] as List<dynamic>? ?? const [])
                .whereType<String>()
                .toList(),
            contextText: data['context_message'] as String?,
            receipts: (data['receipts'] as List<dynamic>? ?? const [])
                .whereType<Map<String, dynamic>>()
                .map((r) => (r['text'] as String? ?? '').trim())
                .where((t) => t.isNotEmpty)
                .toList(),
          ),
        ],
        thinking: false,
      );
    } on DioException catch (e) {
      // The failed turn stays in the transcript, marked: silently dropping
      // it would leave the user staring at their own question with no
      // explanation, unsure whether anything happened.
      state = state.copyWith(
        turns: [
          ...state.turns,
          CopilotTurn(role: CopilotRole.assistant, text: '', failed: true),
        ],
        thinking: false,
      );
      _lastError = e;
    }
  }

  DioException? _lastError;
  DioException? get lastError => _lastError;

  void clear() {
    state = const CopilotState();
  }
}

final copilotControllerProvider =
    NotifierProvider<CopilotController, CopilotState>(CopilotController.new);
