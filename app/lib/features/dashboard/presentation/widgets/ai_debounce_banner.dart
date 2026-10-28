import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/dashboard/presentation/controllers/ai_debounce_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// "A IA responde em ~40s" (§6.15, K1).
///
/// The AI waits for the customer to finish typing before answering a burst. That
/// wait saves two model calls and two half-answers — but from the attendant's
/// chair, an inbound message with no reply is indistinguishable from a bot that
/// died. So the wait is stated, with a way out.
///
/// Renders nothing when there is no pending reply. A banner that is always
/// present is a banner nobody reads.
class AiDebounceBanner extends ConsumerStatefulWidget {
  const AiDebounceBanner({super.key, required this.conversationId});

  final String conversationId;

  @override
  ConsumerState<AiDebounceBanner> createState() => _AiDebounceBannerState();
}

class _AiDebounceBannerState extends ConsumerState<AiDebounceBanner> {
  Timer? _ticker;
  bool _busy = false;

  @override
  void dispose() {
    _ticker?.cancel();
    super.dispose();
  }

  /// One second, and only while a countdown is on screen. Started and stopped
  /// from build so a conversation with no pending reply costs no timer at all —
  /// this widget sits in the inbox, which is open all day.
  void _syncTicker({required bool needed}) {
    if (needed && _ticker == null) {
      _ticker = Timer.periodic(const Duration(seconds: 1), (_) {
        if (mounted) setState(() {});
      });
    } else if (!needed && _ticker != null) {
      _ticker?.cancel();
      _ticker = null;
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final state = ref.watch(aiDebounceStatesProvider)[widget.conversationId];

    // Scheduling the ticker change out of build: touching a Timer during build
    // is fine, but calling setState from inside one that fires mid-build is not.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) _syncTicker(needed: state != null);
    });

    if (state == null) return const SizedBox.shrink();

    final secondsLeft = state.secondsLeft(DateTime.now());

    return Container(
      key: const Key('ai_debounce_banner'),
      margin: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        color: colors.glassLight,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: colors.borderLight),
      ),
      child: Row(
        children: [
          Icon(Icons.hourglass_bottom, size: 16, color: colors.bluePrimary),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  // Zero left means the worker is about to pick it up. Saying
                  // "em 0s" would read as stuck; saying "respondendo" is what is
                  // actually happening.
                  secondsLeft > 0
                      ? t.ai_debounce.waiting(seconds: secondsLeft)
                      : t.ai_debounce.answering,
                  key: const Key('ai_debounce_text'),
                  style: TextStyle(color: colors.textPrimary, fontSize: 12),
                ),
                if (state.messageCount > 1)
                  Text(
                    // The count is the explanation: three messages is why the
                    // AI has not answered the first one.
                    t.ai_debounce.collapsed(count: state.messageCount),
                    key: const Key('ai_debounce_count'),
                    style: TextStyle(color: colors.textSecondary, fontSize: 10),
                  ),
              ],
            ),
          ),
          if (secondsLeft > 0)
            TextButton(
              key: const Key('ai_debounce_now'),
              onPressed: _busy ? null : _answerNow,
              child: Text(t.ai_debounce.answer_now),
            ),
        ],
      ),
    );
  }

  Future<void> _answerNow() async {
    setState(() => _busy = true);
    try {
      await ref
          .read(aiDebounceActionsProvider)
          .answerNow(widget.conversationId);
      // Clear locally rather than waiting for the server's event: the button
      // has to feel like it did something, and the reply itself arrives through
      // the normal new-message path.
      ref.read(aiDebounceStatesProvider.notifier).clear(widget.conversationId);
    } catch (e) {
      if (mounted) {
        setState(() => _busy = false);
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(parseApiError(e))));
      }
    }
  }
}
