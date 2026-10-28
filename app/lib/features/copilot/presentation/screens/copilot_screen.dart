import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/router/back_nav.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/copilot/presentation/controllers/copilot_controller.dart';
import 'package:app/features/sales/presentation/controllers/audiences_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// The Copilot destination (US-M026).
///
/// One chat that does the SDR's job through the same use cases the screens
/// call. It prepares work and hands it back with a link — it never sends:
/// approving outbound stays in the human queue, which is the product's
/// whole posture on cold outreach.
class CopilotScreen extends ConsumerStatefulWidget {
  const CopilotScreen({super.key, this.initialPrompt});

  /// What the caller wanted asked, carried in `?prompt=` from the home
  /// screen's one suggestion chip.
  ///
  /// It lands in the composer and stops there. Auto-sending would let a URL
  /// fire a tool that WRITES — `prospect_businesses` imports leads,
  /// `draft_approaches` fills the approval queue — and a link nobody read is
  /// not a human in the loop.
  final String? initialPrompt;

  @override
  ConsumerState<CopilotScreen> createState() => _CopilotScreenState();
}

class _CopilotScreenState extends ConsumerState<CopilotScreen> {
  final _input = TextEditingController();
  final _scroll = ScrollController();
  final _inputFocus = FocusNode();

  /// The last `?prompt=` this screen honoured. GoRouter keys a shell branch's
  /// page by PATH, so a warm tap on the home screen's suggestion chip never
  /// re-runs initState — [_syncPromptFromRoute] reads the live route instead,
  /// and this field keeps one prompt from re-applying on every rebuild.
  String? _seenPrompt;

  @override
  void initState() {
    super.initState();
    final prompt = widget.initialPrompt?.trim() ?? '';
    if (prompt.isEmpty) return;
    _seenPrompt = prompt;
    _applyPrompt(prompt);
  }

  void _applyPrompt(String prompt) {
    _input.text = prompt;
    // Cursor at the end, ready to edit or to press send: the point of
    // carrying the prompt is that nobody retypes it.
    _input.selection = TextSelection.collapsed(offset: _input.text.length);
    WidgetsBinding.instance
        .addPostFrameCallback((_) => _inputFocus.requestFocus());
  }

  void _syncPromptFromRoute(BuildContext context) {
    if (GoRouter.maybeOf(context) == null) return; // bare test harness
    final prompt =
        GoRouterState.of(context).uri.queryParameters['prompt']?.trim() ?? '';
    if (prompt.isEmpty || prompt == _seenPrompt) return;
    // Never clobber something the person typed themselves: a half-written
    // question outranks a suggestion.
    final typed = _input.text.trim();
    if (typed.isNotEmpty && typed != _seenPrompt) return;
    _seenPrompt = prompt;
    _applyPrompt(prompt);
  }

  @override
  void dispose() {
    _input.dispose();
    _scroll.dispose();
    _inputFocus.dispose();
    super.dispose();
  }

  Future<void> _send() async {
    final text = _input.text.trim();
    if (text.isEmpty) return;
    _input.clear();
    await ref.read(copilotControllerProvider.notifier).send(text);
    if (!mounted) return;
    // Follow the conversation down, the way every chat does.
    if (_scroll.hasClients) {
      await _scroll.animateTo(
        _scroll.position.maxScrollExtent + 200,
        duration: const Duration(milliseconds: 200),
        curve: Curves.easeOut,
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    _syncPromptFromRoute(context);
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final state = ref.watch(copilotControllerProvider);

    return Scaffold(
      backgroundColor: colors.backgroundPrimary,
      appBar: AppBar(
        backgroundColor: colors.backgroundSecondary,
        elevation: 0,
        // Outside the shell since 2026-09-01: no rail, no bar, so the
        // AppBar is the only way out.
        leading: IconButton(
          key: const Key('copilot_back'),
          tooltip: t.common.back,
          icon: Icon(Icons.arrow_back, color: colors.textSecondary),
          onPressed: () => appBack(context),
        ),
        title: Text(
          t.shell.copilot,
          style: TextStyle(
              fontSize: 20,
              fontWeight: FontWeight.w700,
              color: colors.textPrimary),
        ),
        actions: [
          if (state.turns.isNotEmpty)
            IconButton(
              key: const Key('copilot_clear'),
              tooltip: t.copilot.clear,
              icon: Icon(Icons.delete_outline, color: colors.textSecondary),
              onPressed: () =>
                  ref.read(copilotControllerProvider.notifier).clear(),
            ),
        ],
      ),
      body: SafeArea(
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 760),
            child: Column(
              children: [
                Expanded(
                  child: state.turns.isEmpty
                      // Lands in the field, caret at the end — it does not
                      // fire.
                      //
                      // The prospecting suggestion runs `prospect_businesses`,
                      // which IMPORTS (the server notes it in the tool itself:
                      // "the prospecting screen has a form that shows what will
                      // happen; here there is none"). One tap filled the funnel
                      // with twenty companies from a segment and a city nobody
                      // chose, and spent Places quota doing it.
                      //
                      // It is the same rule `initialPrompt` already follows on
                      // this screen, in writing: nothing that WRITES leaves on
                      // a click that did not pass through the text. A button
                      // inside the app is no more consent than a link — in both
                      // the person chose the subject, not the firing.
                      ? _Suggestions(onPick: (s) {
                          _input.text = s;
                          _input.selection =
                              TextSelection.collapsed(offset: s.length);
                          _inputFocus.requestFocus();
                        })
                      : ListView.builder(
                          key: const Key('copilot_transcript'),
                          controller: _scroll,
                          padding: const EdgeInsets.all(16),
                          itemCount:
                              state.turns.length + (state.thinking ? 1 : 0),
                          itemBuilder: (context, i) {
                            if (i >= state.turns.length) {
                              return const _Thinking();
                            }
                            return _TurnBubble(turn: state.turns[i]);
                          },
                        ),
                ),
                _Composer(
                  controller: _input,
                  focusNode: _inputFocus,
                  enabled: !state.thinking,
                  onSend: _send,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// The empty state does the hardest job on this screen: telling someone
/// what a chat box can actually do. A blinking cursor teaches nothing.
class _Suggestions extends ConsumerWidget {
  const _Suggestions({required this.onPick});

  final void Function(String) onPick;

  /// The prospecting example, in the reader's own segment.
  ///
  /// "Find 20 imaging clinics in São Paulo" taught the format and chose the
  /// vertical — in a product that is not about clinics. When the account has
  /// saved an audience, that audience IS the right answer and it is already
  /// stored; with none, the example becomes the template to fill in, which is
  /// honest about what is missing.
  String _prospectSample(WidgetRef ref) {
    final saved = ref.watch(audiencesProvider).asData?.value ?? const [];
    final audience = saved.firstOrNull;
    if (audience == null || audience.searchKeyword.trim().isEmpty) {
      return t.copilot.sample_prospect;
    }
    final where = audience.searchLocation.trim();
    if (where.isEmpty) return t.copilot.sample_prospect;
    return t.copilot.sample_prospect_audience(
        keyword: audience.searchKeyword, location: where);
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final samples = [
      _prospectSample(ref),
      t.copilot.sample_drafts,
      t.copilot.sample_report,
    ];

    return SingleChildScrollView(
      padding: const EdgeInsets.all(24),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const SizedBox(height: 24),
          Icon(Icons.auto_awesome_outlined,
              size: 32, color: colors.bluePrimary),
          const SizedBox(height: 12),
          Text(
            t.copilot.empty_title,
            style: typography.display.copyWith(
                fontSize: 20, color: Colors.white, fontWeight: FontWeight.w600),
          ),
          const SizedBox(height: 6),
          Text(
            t.copilot.empty_body,
            style: typography.body
                .copyWith(color: colors.textSecondary, fontSize: 13),
          ),
          const SizedBox(height: 20),
          for (final sample in samples)
            Padding(
              padding: const EdgeInsets.only(bottom: 8),
              child: OutlinedButton(
                key: Key('copilot_sample_${samples.indexOf(sample)}'),
                onPressed: () => onPick(sample),
                style: OutlinedButton.styleFrom(
                  alignment: Alignment.centerLeft,
                  padding:
                      const EdgeInsets.symmetric(horizontal: 14, vertical: 14),
                  side: BorderSide(color: colors.borderMedium),
                ),
                child: Text(sample,
                    style:
                        TextStyle(color: colors.textSecondary, fontSize: 13)),
              ),
            ),
        ],
      ),
    );
  }
}

class _TurnBubble extends ConsumerWidget {
  const _TurnBubble({required this.turn});

  final CopilotTurn turn;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final mine = turn.role == CopilotRole.user;

    if (turn.failed) {
      final err = ref.read(copilotControllerProvider.notifier).lastError;
      return Padding(
        key: const Key('copilot_error'),
        padding: const EdgeInsets.only(bottom: 12),
        child: Text(
          err == null ? t.copilot.failed : parseApiError(err),
          style: TextStyle(color: colors.redAlert, fontSize: 13),
        ),
      );
    }

    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Column(
        crossAxisAlignment:
            mine ? CrossAxisAlignment.end : CrossAxisAlignment.start,
        children: [
          Container(
            constraints: const BoxConstraints(maxWidth: 520),
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
            decoration: BoxDecoration(
              color: mine ? colors.bluePrimary : colors.backgroundSecondary,
              borderRadius: BorderRadius.circular(14),
              border: mine ? null : Border.all(color: colors.borderMedium),
            ),
            child: SelectableText(
              turn.text,
              style: typography.body
                  .copyWith(color: Colors.white, fontSize: 14, height: 1.4),
            ),
          ),
          // What the tool that WROTE reported, without passing through the
          // model.
          //
          // The prose above is the model's, and on an action that changes the
          // account it has already proved unreliable: a prospecting run that
          // created 56 leads was announced as "1 of them was added". The loop
          // even asked it to "use these numbers"; asking is not a mechanism.
          // This is the mechanism.
          for (final receipt in turn.receipts)
            Container(
              key: const Key('copilot_receipt'),
              margin: const EdgeInsets.only(top: 8),
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
              decoration: BoxDecoration(
                color: colors.glassLight,
                borderRadius: BorderRadius.circular(8),
                border: Border(
                  left: BorderSide(color: colors.bluePrimary, width: 2),
                ),
              ),
              child: Text(
                receipt,
                style: TextStyle(
                    color: colors.textPrimary, fontSize: 12.5, height: 1.35),
              ),
            ),
          // What actually ran. Showing the work is what makes a number in
          // the answer believable.
          if (turn.toolsUsed.isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(top: 4),
              child: Text(
                _toolsLine(turn.toolsUsed),
                key: const Key('copilot_tools_used'),
                style: TextStyle(color: colors.textMuted, fontSize: 11),
              ),
            ),
          if (turn.actions.isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(top: 8),
              child: Wrap(
                spacing: 8,
                runSpacing: 8,
                children: [
                  for (final action in turn.actions)
                    FilledButton.tonal(
                      key: Key('copilot_action_${action.route}'),
                      onPressed: () => context.go(action.route),
                      child: Text(action.label),
                    ),
                ],
              ),
            ),
        ],
      ),
    );
  }
}

/// The wait, acknowledging how long it takes.
///
/// One turn is a model call plus however many tool round-trips it decides to
/// make; the client waits up to 120s. A spinner reading "thinking…" frozen for
/// a minute and a half reads as stuck, and the person reloads the page in the
/// middle of a prospecting run that was working.
///
/// What it does NOT do is guess which tool is running. `/copilot/ask` is a
/// single POST with no streaming: per-tool progress only exists once the whole
/// answer arrives. Inventing "searching for companies…" would narrate work
/// nobody confirmed — the very error the receipts on this same screen exist to
/// correct. It says only what is true: it is still going, and how long that can
/// take.
class _Thinking extends StatefulWidget {
  const _Thinking();

  /// When the wait starts explaining itself. Short enough to arrive before the
  /// doubt, long enough that a fast answer never shows the notice.
  static const patienceAfter = Duration(seconds: 15);

  @override
  State<_Thinking> createState() => _ThinkingState();
}

class _ThinkingState extends State<_Thinking> {
  Timer? _timer;
  bool _long = false;

  @override
  void initState() {
    super.initState();
    _timer = Timer(_Thinking.patienceAfter, () {
      if (mounted) setState(() => _long = true);
    });
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Padding(
      key: const Key('copilot_thinking'),
      padding: const EdgeInsets.only(bottom: 12),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: const EdgeInsets.only(top: 2),
            child: SizedBox(
              width: 14,
              height: 14,
              child: CircularProgressIndicator(
                  strokeWidth: 2, color: colors.textSecondary),
            ),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(t.copilot.thinking,
                    style:
                        TextStyle(color: colors.textSecondary, fontSize: 13)),
                if (_long)
                  Text(
                    t.copilot.thinking_long,
                    key: const Key('copilot_thinking_long'),
                    style: TextStyle(
                        color: colors.textMuted, fontSize: 12, height: 1.35),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _Composer extends StatelessWidget {
  const _Composer({
    required this.controller,
    required this.enabled,
    required this.onSend,
    this.focusNode,
  });

  final TextEditingController controller;
  final bool enabled;
  final VoidCallback onSend;
  final FocusNode? focusNode;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
      child: Row(
        children: [
          Expanded(
            child: TextField(
              key: const Key('copilot_input'),
              controller: controller,
              focusNode: focusNode,
              enabled: enabled,
              minLines: 1,
              maxLines: 4,
              textInputAction: TextInputAction.send,
              onSubmitted: (_) => onSend(),
              style: const TextStyle(color: Colors.white, fontSize: 14),
              decoration: InputDecoration(
                hintText: t.copilot.hint,
                filled: true,
                fillColor: colors.backgroundSecondary,
              ),
            ),
          ),
          const SizedBox(width: 8),
          IconButton.filled(
            key: const Key('copilot_send'),
            onPressed: enabled ? onSend : null,
            icon: const Icon(Icons.arrow_upward, size: 20),
          ),
        ],
      ),
    );
  }
}

/// What a tool did, in words the person reading it uses.
///
/// The screen showed the raw identifiers — `case_report`, `list_funnels` —
/// which name the plumbing, not the work. Showing the work is what makes a
/// number believable; showing a function name just proves there is code.
///
/// An unknown id falls through unchanged rather than being hidden: a tool
/// nobody translated still ran, and pretending otherwise is the one thing
/// this line exists to prevent.
/// The footer, without repeating the same label.
///
/// Two calls to the same tool printed "searched companies · searched
/// companies" — a worse read than the truth, which is "twice".
String _toolsLine(List<String> tools) {
  final counts = <String, int>{};
  for (final tool in tools) {
    final label = _toolLabel(tool);
    counts[label] = (counts[label] ?? 0) + 1;
  }
  return counts.entries
      .map((e) => e.value > 1 ? '${e.key} ×${e.value}' : e.key)
      .join(' · ');
}

String _toolLabel(String tool) => switch (tool) {
      'list_funnels' => t.copilot.tool_list_funnels,
      'pipeline_overview' => t.copilot.tool_pipeline_overview,
      'list_approval_queue' => t.copilot.tool_list_approval_queue,
      'draft_approaches' => t.copilot.tool_draft_approaches,
      'case_report' => t.copilot.tool_case_report,
      _ => tool,
    };
