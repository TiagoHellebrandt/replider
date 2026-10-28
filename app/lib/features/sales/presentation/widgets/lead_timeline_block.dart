import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/network/websocket_service.dart';
import 'package:app/design_system/components/gradient_pill.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/domain/models/lead_timeline.dart';
import 'package:app/features/sales/presentation/controllers/lead_timeline_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// What was said to this customer, inside the lead.
///
/// The hole this closes: a vendedor sends a proposal and the reply lands in an
/// inbox they are not allowed to open. They had no way to know they had been
/// answered — so the product asked them to follow up on customers who had
/// already said yes, and to wait on ones who had said no.
///
/// It reads, and — once there IS a thread — it answers. The composer under the
/// list posts to `/leads/{id}/reply`, which is NOT a second outbound-approach
/// path: it only works on a conversation that already exists (the approach was
/// approved and sent, or the customer wrote in), so the approval queue stays the
/// one gate for first contact. Before this, a vendedor could read "yes" here and
/// had nowhere to say "great" — every reply surface was attendant-gated.
class LeadTimelineBlock extends ConsumerWidget {
  const LeadTimelineBlock({super.key, required this.leadId});

  final String leadId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    // A reply that arrives while the panel is open must show up without a
    // reload. The timeline payload does not name its conversation, so any
    // inbound on the account re-fetches this one small GET — Riverpod
    // coalesces the invalidations of a burst into one rebuild.
    ref.listen(wsEventsProvider, (_, next) {
      final event = next.asData?.value;
      if (event == null) return;
      if (event.event == 'new_message' ||
          event.event == 'conversation_updated') {
        ref.invalidate(leadTimelineProvider(leadId));
      }
    });
    final async = ref.watch(leadTimelineProvider(leadId));

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          t.lead_timeline.title,
          style: TextStyle(
              color: colors.textSecondary,
              fontSize: 12,
              fontWeight: FontWeight.w700),
        ),
        const SizedBox(height: 8),
        async.when(
          loading: () => const Padding(
            padding: EdgeInsets.symmetric(vertical: 12),
            child: Center(
              child: SizedBox(
                width: 18,
                height: 18,
                child: CircularProgressIndicator(strokeWidth: 2),
              ),
            ),
          ),
          // A failed read says so. Rendering "no messages" would tell the
          // salesperson nobody ever wrote to this customer, which is a claim a
          // broken query has not earned.
          error: (e, _) => Text(
            parseApiError(e),
            key: const Key('lead_timeline_error'),
            style: TextStyle(color: colors.orangeAlert, fontSize: 12),
          ),
          data: (timeline) {
            if (timeline.messages.isEmpty) {
              // No thread yet, so no composer: the first contact goes through
              // the approval queue, and a reply box here would be a dead end
              // (the server answers LEAD_NO_CONVERSATION).
              return Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    t.lead_timeline.empty,
                    key: const Key('lead_timeline_empty'),
                    style: TextStyle(color: colors.textMuted, fontSize: 12),
                  ),
                  const SizedBox(height: 2),
                  Text(
                    t.lead_timeline.no_conversation,
                    key: const Key('lead_timeline_no_conversation'),
                    style: TextStyle(color: colors.textMuted, fontSize: 11),
                  ),
                ],
              );
            }
            return Column(
              key: const Key('lead_timeline_list'),
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (timeline.truncated) ...[
                  Text(
                    t.lead_timeline.truncated,
                    key: const Key('lead_timeline_truncated'),
                    style: TextStyle(color: colors.textMuted, fontSize: 11),
                  ),
                  const SizedBox(height: 6),
                ],
                for (final message in timeline.messages)
                  _Line(message: message),
                const SizedBox(height: 4),
                _ReplyComposer(leadId: leadId),
              ],
            );
          },
        ),
      ],
    );
  }
}

/// The answer box: a borderless field in a pill (the input shape is the
/// theme's — same anatomy as the board's search) and a "Responder" pill.
class _ReplyComposer extends ConsumerStatefulWidget {
  const _ReplyComposer({required this.leadId});

  final String leadId;

  @override
  ConsumerState<_ReplyComposer> createState() => _ReplyComposerState();
}

class _ReplyComposerState extends ConsumerState<_ReplyComposer> {
  final _controller = TextEditingController();
  bool _sending = false;

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  Future<void> _send() async {
    final text = _controller.text.trim();
    final messenger = ScaffoldMessenger.of(context);
    if (text.isEmpty) {
      // The click gets an answer: an empty reply is refused HERE, not by a
      // round-trip to hear what the screen already knew.
      messenger
          .showSnackBar(SnackBar(content: Text(t.lead_timeline.reply_hint)));
      return;
    }
    setState(() => _sending = true);
    try {
      await ref
          .read(leadTimelineActionsProvider)
          .replyToLead(widget.leadId, text);
      _controller.clear();
      // The sent line belongs in the list above, not only in a toast.
      ref.invalidate(leadTimelineProvider(widget.leadId));
      messenger
          .showSnackBar(SnackBar(content: Text(t.lead_timeline.reply_sent)));
    } catch (e) {
      messenger.showSnackBar(SnackBar(content: Text(parseApiError(e))));
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Row(
      crossAxisAlignment: CrossAxisAlignment.end,
      children: [
        Expanded(
          child: Container(
            padding: const EdgeInsets.symmetric(horizontal: 14),
            decoration: BoxDecoration(
              color: colors.backgroundSecondary,
              borderRadius: BorderRadius.circular(20),
              border: Border.all(color: colors.borderMedium),
            ),
            child: TextField(
              key: const Key('lead_reply_field'),
              controller: _controller,
              enabled: !_sending,
              minLines: 1,
              maxLines: 4,
              textInputAction: TextInputAction.newline,
              style: const TextStyle(color: Colors.white, fontSize: 13),
              decoration: InputDecoration(
                isDense: true,
                hintText: t.lead_timeline.reply_hint,
                border: InputBorder.none,
                enabledBorder: InputBorder.none,
                focusedBorder: InputBorder.none,
                disabledBorder: InputBorder.none,
                contentPadding: const EdgeInsets.symmetric(vertical: 10),
              ),
            ),
          ),
        ),
        const SizedBox(width: 8),
        GradientPill(
          key: const Key('lead_reply_send'),
          label: t.lead_timeline.reply_send,
          icon: Icons.send_outlined,
          onTap: _sending ? null : _send,
        ),
      ],
    );
  }
}

class _Line extends StatelessWidget {
  const _Line({required this.message});

  final TimelineMessage message;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final fromCustomer = message.isFromCustomer;

    return Padding(
      key: Key('lead_timeline_line_${message.id}'),
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // The customer's lines are the ones being looked for, so they carry
          // the accent and everything else recedes.
          Container(
            width: 3,
            height: 34,
            decoration: BoxDecoration(
              color: fromCustomer ? colors.whatsappGreen : colors.borderMedium,
              borderRadius: BorderRadius.circular(2),
            ),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  _who(message),
                  style: TextStyle(
                    color: fromCustomer ? colors.textPrimary : colors.textMuted,
                    fontSize: 11,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                Text(
                  // A media message renders as what it is instead of as a blank
                  // line: an empty row reads as a loading defect.
                  message.text.trim().isEmpty
                      ? t.lead_timeline.attachment(kind: message.kind)
                      : message.text,
                  style: TextStyle(color: colors.textSecondary, fontSize: 13),
                ),
                if (message.failed)
                  // Undelivered must not read as delivered: this line is the
                  // vendedor's only view of the exchange.
                  Tooltip(
                    message: message.failedReason,
                    child: Text(
                      t.lead_timeline.not_delivered,
                      key: Key('lead_timeline_failed_${message.id}'),
                      style: TextStyle(
                          color: colors.redAlert,
                          fontSize: 11,
                          fontWeight: FontWeight.w700),
                    ),
                  ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          Text(
            _when(message.sentAt),
            style: TextStyle(color: colors.textMuted, fontSize: 11),
          ),
        ],
      ),
    );
  }

  String _who(TimelineMessage m) => switch (m.author) {
        TimelineAuthor.customer => t.lead_timeline.from_customer,
        // A teammate's name when there is one, because "who answered this
        // customer" is a question the salesperson asks about their own team.
        TimelineAuthor.team => m.authorName.trim().isEmpty
            ? t.lead_timeline.from_team
            : m.authorName,
        TimelineAuthor.bot => t.lead_timeline.from_bot,
      };

  String _when(DateTime? at) {
    if (at == null) return '';
    return '${at.day.toString().padLeft(2, '0')}/'
        '${at.month.toString().padLeft(2, '0')} '
        '${at.hour.toString().padLeft(2, '0')}:'
        '${at.minute.toString().padLeft(2, '0')}';
  }
}
