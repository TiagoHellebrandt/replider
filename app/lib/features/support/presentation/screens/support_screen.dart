import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/router/back_nav.dart';
import 'package:app/design_system/components/whatsapp_text.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/support/domain/support_message.dart';
import 'package:app/features/support/presentation/controllers/support_controller.dart';
import 'package:app/features/support/presentation/widgets/support_push_prompt.dart';
import 'package:app/i18n/strings.g.dart';

/// The customer-side support widget (canal-interno US-005): the logged-in
/// user's private thread with the Replider platform. Deliberately NOT part
/// of the normal inbox — it rides its own endpoints + `support_message`
/// WS event, so customer attendants never see it among their
/// conversations.
class SupportScreen extends ConsumerStatefulWidget {
  const SupportScreen({super.key, this.draft});

  /// Text to seed the composer with, from a screen that can describe the
  /// problem better than the user can. Never auto-sent: the person edits and
  /// presses send, exactly as if they had typed it.
  final String? draft;

  @override
  ConsumerState<SupportScreen> createState() => _SupportScreenState();
}

class _SupportScreenState extends ConsumerState<SupportScreen> {
  final _inputController = TextEditingController();
  final _scrollController = ScrollController();
  bool _isSending = false;

  @override
  void initState() {
    super.initState();
    // Seeded, never sent. The person reads what we wrote about their
    // problem, edits it if it is wrong, and presses send themselves.
    if (widget.draft?.trim().isNotEmpty ?? false) {
      _inputController.text = widget.draft!.trim();
    }
    // Opening the widget consumes the badge (the API stamps the thread
    // read server-side on the GET the controller fires).
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) {
        ref.read(supportUnreadCounterProvider.notifier).clear();
      }
    });
  }

  @override
  void dispose() {
    _inputController.dispose();
    _scrollController.dispose();
    super.dispose();
  }

  void _scrollToBottom() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (_scrollController.hasClients) {
        _scrollController.animateTo(
          _scrollController.position.maxScrollExtent,
          duration: const Duration(milliseconds: 250),
          curve: Curves.easeOut,
        );
      }
    });
  }

  Future<void> _sendMessage() async {
    final text = _inputController.text.trim();
    if (text.isEmpty || _isSending) return;

    setState(() => _isSending = true);
    try {
      await ref
          .read(supportThreadControllerProvider.notifier)
          .sendMessage(text);
      _inputController.clear();
      _scrollToBottom();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(parseApiError(e))),
        );
      }
    } finally {
      if (mounted) setState(() => _isSending = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final threadAsync = ref.watch(supportThreadControllerProvider);

    // A message arriving while the screen is open is read on sight —
    // keep the nav badge at zero and follow the conversation down.
    ref.listen(supportThreadControllerProvider, (previous, next) {
      if (next.hasValue) {
        ref.read(supportUnreadCounterProvider.notifier).clear();
        _scrollToBottom();
      }
    });

    return Scaffold(
      backgroundColor: colors.backgroundPrimary,
      appBar: AppBar(
        backgroundColor: colors.backgroundSecondary,
        elevation: 0,
        leading: IconButton(
          icon: Icon(Icons.arrow_back, color: colors.textSecondary),
          onPressed: () => appBack(context),
        ),
        title: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              t.support.title,
              style: TextStyle(
                fontSize: 15,
                fontWeight: FontWeight.w700,
                color: colors.textPrimary,
              ),
            ),
            Text(
              t.support.subtitle,
              style: TextStyle(fontSize: 11, color: colors.textMuted),
            ),
          ],
        ),
      ),
      body: Column(
        children: [
          // Contextual push opt-in on first use (US-006); renders nothing
          // once handled, denied, subscribed or unsupported.
          const SupportPushPrompt(),
          Expanded(
            child: threadAsync.when(
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (error, _) => Center(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(
                      t.support.load_error,
                      style: TextStyle(color: colors.textMuted, fontSize: 14),
                    ),
                    const SizedBox(height: 8),
                    TextButton(
                      onPressed: () =>
                          ref.invalidate(supportThreadControllerProvider),
                      child: Text(t.support.retry),
                    ),
                  ],
                ),
              ),
              data: (thread) {
                final messages =
                    thread.messages.where((m) => m.text.isNotEmpty).toList();
                if (messages.isEmpty) return _buildEmptyState(colors);
                return ListView.builder(
                  controller: _scrollController,
                  padding: const EdgeInsets.all(16),
                  itemCount: messages.length,
                  itemBuilder: (ctx, i) => _buildBubble(colors, messages[i]),
                );
              },
            ),
          ),
          _buildComposer(colors),
        ],
      ),
    );
  }

  Widget _buildEmptyState(AppColorsExtension colors) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.support_agent, size: 56, color: colors.textDisabled),
            const SizedBox(height: 12),
            Text(
              t.support.empty_title,
              style: TextStyle(
                color: colors.textPrimary,
                fontSize: 16,
                fontWeight: FontWeight.w600,
              ),
            ),
            const SizedBox(height: 6),
            Text(
              t.support.empty_subtitle,
              textAlign: TextAlign.center,
              style: TextStyle(color: colors.textMuted, fontSize: 13),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildBubble(AppColorsExtension colors, SupportMessage msg) {
    final fromMe = msg.fromMe;
    return Align(
      alignment: fromMe ? Alignment.centerRight : Alignment.centerLeft,
      child: Container(
        constraints: BoxConstraints(
          maxWidth: MediaQuery.of(context).size.width * 0.78,
        ),
        margin: const EdgeInsets.only(bottom: 10),
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
        decoration: BoxDecoration(
          color: fromMe ? colors.bluePrimary : colors.backgroundSecondary,
          borderRadius: BorderRadius.only(
            topLeft: const Radius.circular(16),
            topRight: const Radius.circular(16),
            bottomLeft: Radius.circular(fromMe ? 16 : 4),
            bottomRight: Radius.circular(fromMe ? 4 : 16),
          ),
          border: fromMe ? null : Border.all(color: colors.borderLight),
        ),
        child: WhatsAppText(
          msg.text,
          baseStyle: TextStyle(
            color: fromMe ? Colors.white : colors.textPrimary,
            fontSize: 14,
            height: 1.4,
          ),
        ),
      ),
    );
  }

  Widget _buildComposer(AppColorsExtension colors) {
    return Container(
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        border: Border(top: BorderSide(color: colors.borderLight)),
      ),
      padding: EdgeInsets.only(
        left: 12,
        right: 12,
        top: 10,
        bottom: MediaQuery.of(context).padding.bottom + 10,
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          Expanded(
            child: TextField(
              controller: _inputController,
              maxLines: 4,
              minLines: 1,
              textCapitalization: TextCapitalization.sentences,
              style: TextStyle(color: colors.textPrimary, fontSize: 14),
              onSubmitted: (_) => _sendMessage(),
              decoration: InputDecoration(
                hintText: t.support.input_hint,
                hintStyle: TextStyle(color: colors.textDisabled, fontSize: 14),
                filled: true,
                fillColor: colors.glassMedium,
                contentPadding: const EdgeInsets.symmetric(
                  horizontal: 14,
                  vertical: 10,
                ),
              ),
            ),
          ),
          const SizedBox(width: 8),
          GestureDetector(
            onTap: _sendMessage,
            child: AnimatedContainer(
              duration: const Duration(milliseconds: 150),
              width: 44,
              height: 44,
              decoration: BoxDecoration(
                color: _isSending ? colors.textDisabled : colors.bluePrimary,
                shape: BoxShape.circle,
              ),
              child: _isSending
                  ? const Center(
                      child: SizedBox(
                        width: 20,
                        height: 20,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          color: Colors.white,
                        ),
                      ),
                    )
                  : const Icon(Icons.send, color: Colors.white, size: 20),
            ),
          ),
        ],
      ),
    );
  }
}
