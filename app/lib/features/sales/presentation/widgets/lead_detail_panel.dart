import 'dart:async';

import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/components/app_dialogs.dart';
import 'package:app/design_system/components/gradient_pill.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/domain/models/approach_review.dart';
import 'package:app/features/sales/domain/models/board_model.dart';
import 'package:app/features/sales/presentation/controllers/board_controller.dart';
import 'package:app/features/sales/presentation/controllers/approval_queue_controller.dart';
import 'package:app/features/sales/presentation/controllers/lead_detail_controller.dart';
import 'package:app/features/sales/presentation/controllers/radar_controller.dart';
import 'package:app/features/sales/presentation/controllers/sales_record_controller.dart';
import 'package:app/features/sales/presentation/widgets/lead_panel_blocks.dart';
import 'package:app/features/sales/presentation/widgets/lead_timeline_block.dart';
import 'package:app/features/sales/presentation/widgets/register_sale_sheet.dart';
import 'package:app/features/settings/presentation/controllers/email_template_controller.dart';
import 'package:app/features/sales/domain/models/funnel_model.dart';
import 'package:app/features/sales/presentation/controllers/funnels_controller.dart';
import 'package:app/features/sales/presentation/widgets/metadata_fields_form.dart';
import 'package:app/features/sales/presentation/widgets/wa_template_picker.dart';
import 'package:app/features/settings/domain/models/channel_model.dart';
import 'package:app/features/settings/domain/models/whatsapp_template_model.dart';
import 'package:app/features/settings/presentation/controllers/channels_controller.dart';
import 'package:app/features/settings/presentation/controllers/team_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Lead detail (US-M019).
///
/// Presented as a bottom sheet on a phone and a side drawer on desktop —
/// same widget, because the CONTENT is identical and the salesperson uses
/// both devices for the same job. What changes is only how it enters the
/// screen.
///
/// The screen exists so a lead can be ACTED on, not admired: the approach
/// text is editable in place, and approving is one confirm away.
class LeadDetailPanel extends ConsumerStatefulWidget {
  const LeadDetailPanel({
    super.key,
    required this.card,
    required this.stageName,
    this.funnelId,
    this.canAct = true,
  });

  final BoardCard card;
  final String stageName;

  /// Which funnel this lead belongs to — so the panel knows which fields it
  /// asks for. Null when the panel is opened from somewhere that does not know
  /// the funnel; the fields section then simply does not appear.
  final String? funnelId;

  /// False when the viewer may read the lead but not change it (US-F009: a
  /// vendedor looking at a teammate's team-visible lead). The server refuses
  /// anyway; hiding the buttons means the user never discovers that by
  /// getting an error.
  final bool canAct;

  @override
  ConsumerState<LeadDetailPanel> createState() => _LeadDetailPanelState();
}

/// How the panel waits for a research it can only ask for. Three seconds is
/// short enough to feel like the screen is watching, and thirty attempts
/// (~90s) is longer than a grounded search takes without waiting forever on
/// one that died.
const _researchPollInterval = Duration(seconds: 3);
const _researchPollAttempts = 30;

class _LeadDetailPanelState extends ConsumerState<LeadDetailPanel> {
  /// What this lead's funnel asks for. Empty when the panel does not know the
  /// funnel, or when the funnel asks for nothing beyond the basics.
  List<MetadataFieldModel> get _schema {
    final id = widget.funnelId;
    if (id == null) return const [];
    return ref
            .watch(funnelsListProvider)
            .asData
            ?.value
            .where((f) => f.id == id)
            .firstOrNull
            ?.metadataSchema ??
        const [];
  }

  late final TextEditingController _approach;
  late final TextEditingController _subject;
  bool _busy = false;
  bool _enriching = false;

  /// What the server has stored, so an unchanged draft is never written.
  String _savedDraft = '';

  /// After a send the draft is history: the server refuses to rewrite the text
  /// of an already-contacted lead, and insisting from here would surface a
  /// network error while closing a screen that succeeded.
  bool _sent = false;

  Timer? _autosave;

  /// Captured early so `dispose` can still write the last stretch: `ref` is no
  /// longer usable there, but what the person wrote still is. Same pattern the
  /// approval queue uses for a pending discard.
  late final LeadActions _actions;

  /// The stored template the text came from, when it came from one.
  ///
  /// Carried to the send so the reply this earns lands on the numbers for
  /// the exact words that earned it. Cleared the moment the operator picks
  /// a different template — but NOT when they edit the text, because an
  /// edited template is still that template's idea, and dropping the
  /// attribution on the first keystroke would leave every send unattributed.
  String? _templateId;
  int _templateVersion = 0;

  /// The approved WhatsApp template a non-email approach is delivered
  /// through. Cold first-contact on WhatsApp is ONLY deliverable as a
  /// template — this panel used to send without one and every WhatsApp
  /// approval bounced off the server with a generic error.
  WhatsAppTemplateModel? _waTemplate;

  /// How long idle before storing what has been written.
  ///
  /// Short enough to survive a closed tab, long enough not to write on every
  /// keystroke.
  static const _autosaveDelay = Duration(milliseconds: 1500);

  @override
  void initState() {
    super.initState();
    _actions = ref.read(leadActionsProvider);
    _approach = TextEditingController()..addListener(_scheduleAutosave);
    _subject = TextEditingController();
  }

  @override
  void dispose() {
    _autosave?.cancel();
    // The last stretch, without waiting for the debounce. Closing the panel
    // used to lose whatever had been typed: the text lived in the controller
    // and nowhere else, because the only writers of the draft were the AI and
    // the send.
    _flushDraft();
    _approach.dispose();
    _subject.dispose();
    super.dispose();
  }

  void _scheduleAutosave() {
    _autosave?.cancel();
    _autosave = Timer(_autosaveDelay, _flushDraft);
  }

  /// Writes the draft when it differs from what the server holds.
  ///
  /// Fire-and-forget on purpose: in `dispose` there is no screen left to report
  /// an error to, and a failure here leaves the draft as it was — the old
  /// behaviour, not a new loss.
  void _flushDraft() {
    if (_sent || !widget.canAct) return;
    final text = _approach.text;
    if (text.trim() == _savedDraft.trim()) return;
    _savedDraft = text;
    unawaited(
      _actions
          .saveDraft(leadId: widget.card.leadId, text: text)
          .catchError((Object _) {}),
    );
  }

  /// Whether this lead is written to by email.
  ///
  /// Read off the recipient rather than a channel field because the lead
  /// payload carries no channel: the address IS the routing decision the
  /// server makes, so an address with an @ is an email send.
  bool _isEmail(LeadDetail lead) => lead.recipientAddress.contains('@');

  /// Resolved channel type, funnel → channel — the same resolution the server
  /// does at send time. The '@' heuristic above cannot tell WhatsApp from
  /// SMS, and only WhatsApp demands an approved template.
  ChannelType? _channelType(LeadDetail lead) {
    final id = widget.funnelId;
    if (id != null && id.isNotEmpty) {
      final channelId = ref
          .watch(funnelsListProvider)
          .asData
          ?.value
          .where((f) => f.id == id)
          .firstOrNull
          ?.channelId;
      if (channelId != null && channelId.isNotEmpty) {
        final match = ref
            .watch(channelsListProvider)
            .asData
            ?.value
            .where((c) => c.id == channelId)
            .firstOrNull;
        if (match != null) return match.type;
      }
    }
    return _isEmail(lead) ? ChannelType.email : null;
  }

  void _applyTemplate(EmailTemplate tpl) {
    setState(() {
      _templateId = tpl.id;
      _templateVersion = tpl.version;
      _subject.text = tpl.subject;
      _approach.text = tpl.body;
    });
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final detail = ref.watch(leadDetailProvider(widget.card.leadId));

    // Keep the editable draft in sync with the server copy, but never
    // clobber what the user is typing.
    detail.whenData((lead) {
      if (_approach.text.isEmpty && lead.approachDraft.isNotEmpty) {
        _savedDraft = lead.approachDraft;
        _approach.text = lead.approachDraft;
      }
    });

    return Padding(
      padding: EdgeInsets.only(
        left: 20,
        right: 20,
        top: 20,
        // Sheet + keyboard: without this the compose field hides under the
        // keyboard exactly when the user wants to edit it.
        bottom: MediaQuery.of(context).viewInsets.bottom + 20,
      ),
      child: SingleChildScrollView(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    widget.card.title,
                    key: const Key('lead_detail_title'),
                    style: typography.display.copyWith(
                        fontSize: 20,
                        color: Colors.white,
                        fontWeight: FontWeight.w600),
                  ),
                ),
                IconButton(
                  key: const Key('lead_detail_close'),
                  onPressed: () => Navigator.of(context).maybePop(),
                  icon: Icon(Icons.close, color: colors.textSecondary),
                ),
              ],
            ),
            const SizedBox(height: 4),
            // The stage, and nothing else. A red "No owner" used to sit here,
            // read off the CARD — the snapshot the board held when the panel
            // opened, not the reloaded lead. Assigning somebody left the chip
            // contradicting the picker just below it, on the same screen, at
            // the same time.
            //
            // The owner now has one home: the picker, which shows whose it is.
            // Repeating its absence in red at the top was an alarm about a
            // field the screen itself resolves in one click. (The score chip
            // went with it: nothing writes a score.)
            // The blue tag is a funnel STAGE. Opened from the approval queue
            // the card carries no stage (stageId is empty), and painting the
            // queue's own name in stage colours claimed a stage called "Fila
            // de aprovação" that no funnel has.
            if (widget.card.stageId.isNotEmpty) ...[
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: [
                  _Tag(label: widget.stageName, color: colors.bluePrimary),
                ],
              ),
              const SizedBox(height: 12),
            ],
            // THE next action, first: a draft waiting in Hoje is the one
            // thing this panel can hand off in a click. Everything below is
            // context for it.
            if (widget.card.hasDraft) ...[
              GradientPill(
                key: const Key('lead_panel_open_queue'),
                label: t.board.panel_open_in_queue,
                icon: Icons.send,
                fontSize: 13.5,
                height: 42,
                padding: const EdgeInsets.symmetric(horizontal: 22),
                onTap: () {
                  Navigator.of(context).maybePop();
                  context.go('/prospect?tab=queue');
                },
              ),
              const SizedBox(height: 16),
            ],
            // What the public registry says THIS establishment holds — the
            // pivot's centerpiece, absent from this panel until now. Counts,
            // never equipment age; the match method qualifies the claim.
            Consumer(builder: (context, ref, _) {
              final park =
                  ref.watch(leadParkProvider(widget.card.leadId)).asData?.value;
              if (park == null) return const SizedBox.shrink();
              return Padding(
                padding: const EdgeInsets.only(bottom: 16),
                child: Container(
                  key: const Key('lead_panel_cnes_park'),
                  width: double.infinity,
                  padding: const EdgeInsets.all(14),
                  decoration: BoxDecoration(
                    color: colors.cyanGlow.withValues(alpha: 0.06),
                    borderRadius: BorderRadius.circular(14),
                    border: Border.all(
                        color: colors.cyanGlow.withValues(alpha: 0.35)),
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Icon(Icons.precision_manufacturing_outlined,
                              size: 15, color: colors.cyanGlow),
                          const SizedBox(width: 7),
                          Text(t.board.panel_cnes_title,
                              style: const TextStyle(
                                  fontSize: 13, fontWeight: FontWeight.w700)),
                          const SizedBox(width: 8),
                          Expanded(
                            child: Text(
                              park.matchedBy == 'phone'
                                  ? t.board.panel_cnes_by_phone
                                  : t.board.panel_cnes_by_name,
                              style: TextStyle(
                                  color: colors.textMuted, fontSize: 11),
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 6),
                      Text(
                        park.items.join(' · '),
                        style: TextStyle(
                            color: colors.textSecondary,
                            fontSize: 12.5,
                            height: 1.5),
                      ),
                      // What of the catalogue fits what they own.
                      //
                      // This is the question the panel could not answer:
                      // "what do I offer this hospital" only got a reply
                      // inside the paid draft, so seeing the opportunity cost
                      // a model call. The crossing is a word match the server
                      // does for free.
                      if (park.matches.isNotEmpty) ...[
                        const SizedBox(height: 12),
                        Row(
                          children: [
                            Icon(Icons.sell_outlined,
                                size: 14, color: colors.limeGlow),
                            const SizedBox(width: 6),
                            Text(t.board.panel_offer_title,
                                style: TextStyle(
                                    color: colors.limeGlow,
                                    fontSize: 12,
                                    fontWeight: FontWeight.w700)),
                          ],
                        ),
                        const SizedBox(height: 4),
                        // The product NAME opens its own page.
                        //
                        // Nobody knows a hundred-item catalogue by heart —
                        // that is the whole reason this crossing exists. Naming
                        // something the salesperson has never sold, with
                        // nowhere to read the specification, leaves them
                        // guessing in front of a buyer. One tap is the manual,
                        // the price, and the place to fix it if it is wrong.
                        for (final m in park.matches)
                          Padding(
                            key: Key('lead_panel_offer_${m.productName}'),
                            padding: const EdgeInsets.only(top: 2),
                            child: _OfferLine(match: m),
                          ),
                        const SizedBox(height: 4),
                        Text(
                          t.board.panel_offer_free,
                          style: TextStyle(
                              color: colors.textMuted, fontSize: 10.5),
                        ),
                      ],
                      // DATASUS lags 45–75 days: name the month, never
                      // imply "today".
                      if (park.competencia != null) ...[
                        const SizedBox(height: 4),
                        Text(
                          t.board.panel_cnes_month(
                              month: competenciaLabel(park.competencia!)),
                          key: const Key('lead_panel_cnes_month'),
                          style:
                              TextStyle(color: colors.textMuted, fontSize: 11),
                        ),
                      ],
                    ],
                  ),
                ),
              );
            }),
            detail.when(
              loading: () => const Padding(
                padding: EdgeInsets.symmetric(vertical: 32),
                child: Center(child: CircularProgressIndicator()),
              ),
              error: (err, _) => Text(parseApiError(err),
                  style: TextStyle(color: colors.redAlert)),
              data: (lead) => Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // WHERE the message goes, and what to do with it. This was
                  // missing entirely: the panel showed a draft and a timeline
                  // and never the number, so deciding whether to call meant
                  // starting a send just to read the confirmation.
                  LeadContactBlock(lead: lead),
                  const SizedBox(height: 20),
                  // Who answers for this lead, and who can see it.
                  //
                  // The board marked every card "no owner" in red and nothing
                  // could change that: the action existed in the use case and
                  // had no route. And `visibility` was a database column whose
                  // read filter worked over a value that never changed, because
                  // nothing in the product could write it.
                  if (widget.canAct) ...[
                    _OwnerAndVisibility(
                      leadId: widget.card.leadId,
                      ownerUserId: lead.ownerUserId,
                      visibility: lead.visibility,
                      // Reloads the panel AND the board behind it. The panel
                      // alone left the card still reading "no owner" beside a
                      // picker already showing the name — the board only
                      // refreshed on close, and until then one screen asserted
                      // both.
                      onChanged: () {
                        ref.invalidate(leadDetailProvider(widget.card.leadId));
                        final funnel = widget.funnelId;
                        if (funnel != null) {
                          ref.invalidate(boardProvider(funnel));
                        }
                      },
                    ),
                    const SizedBox(height: 20),
                  ],

                  // What is known about the company — or, when nothing is,
                  // what to do about that. The old panel simply dropped the
                  // whole section for an unenriched lead, with no explanation
                  // and no way to fill it.
                  //
                  // Open. I collapsed this section by default reasoning that
                  // "whoever opens a lead to act does not need the facts in
                  // front of the button" — and the real use is the opposite:
                  // you open a lead to find out WHO IT IS. Collapsed, the panel
                  // answered "3 facts about the company" to someone asking
                  // which.
                  _CollapsibleSection(
                    sectionKey: 'profile',
                    title: t.lead_detail.profile,
                    summary: lead.isEnriched
                        ? t.lead_detail
                            .profile_facts_count(count: lead.facts.length)
                        : t.lead_detail.profile_nothing_yet,
                    initiallyOpen: true,
                    child: LeadProfileBlock(
                      lead: lead,
                      onEnrich: _enrich,
                      enriching: _enriching,
                      showHeading: false,
                    ),
                  ),
                  const SizedBox(height: 20),
                  // The fields THIS funnel asks for, fillable by hand.
                  //
                  // They decide when a lead becomes qualified, and until now
                  // only the AI could answer them, during a conversation: a
                  // lead that arrived by referral could be qualified by nobody,
                  // because no screen called PATCH /leads/:id/metadata.
                  if (widget.canAct && _schema.isNotEmpty) ...[
                    _FunnelFields(
                      leadId: widget.card.leadId,
                      schema: _schema,
                      values: lead.collectedValues,
                      onSaved: () => ref
                          .invalidate(leadDetailProvider(widget.card.leadId)),
                    ),
                    const SizedBox(height: 20),
                  ],

                  // The thread in three sentences, for whoever picks this up
                  // after a week away or inherits it from a colleague.
                  if (lead.conversationId != null) ...[
                    LeadConversationSummary(
                        conversationId: lead.conversationId!),
                    const SizedBox(height: 20),
                  ],
                  if (_isEmail(lead)) ...[
                    _TemplatePicker(
                      selectedId: _templateId,
                      onPicked: _applyTemplate,
                      enabled: widget.canAct && !_busy,
                    ),
                    const SizedBox(height: 12),
                    Text(t.lead_detail.subject,
                        style: typography.body.copyWith(
                            color: colors.textSecondary, fontSize: 12)),
                    const SizedBox(height: 6),
                    TextField(
                      key: const Key('lead_detail_subject_field'),
                      controller: _subject,
                      enabled: widget.canAct && !_busy,
                      style: const TextStyle(color: Colors.white, fontSize: 14),
                      decoration: InputDecoration(
                        hintText: t.lead_detail.subject_hint,
                      ),
                    ),
                    const SizedBox(height: 12),
                  ],
                  // The WhatsApp counterpart of the subject field: the
                  // approved template the text is delivered through.
                  if (widget.canAct &&
                      widget.funnelId != null &&
                      _channelType(lead) == ChannelType.whatsapp) ...[
                    WaApprovedTemplatePicker(
                      funnelId: widget.funnelId!,
                      selected: _waTemplate,
                      enabled: !_busy,
                      onPicked: (tpl) => setState(() => _waTemplate = tpl),
                    ),
                    const SizedBox(height: 12),
                  ],
                  // What the customer already said, above the box where the
                  // next thing gets written. A reply that arrived is the single
                  // most relevant fact for what to write next — and until now a
                  // vendedor could not see it at all, because the inbox is
                  // attendant-gated and the reply landed there.
                  LeadTimelineBlock(leadId: widget.card.leadId),
                  const SizedBox(height: 16),
                  Text(t.lead_detail.approach,
                      style: typography.body
                          .copyWith(color: colors.textSecondary, fontSize: 12)),
                  const SizedBox(height: 6),
                  // Read-only once the approach was SENT: the server refuses
                  // the save (LeadAlreadyContacted) and the old fire-and-
                  // forget flush swallowed that — someone rewriting
                  // yesterday's sent message lost every word on close.
                  if (const {'contacted', 'won', 'lost'}
                      .contains(lead.status)) ...[
                    Text(
                      t.lead_detail.approach_locked_after_send,
                      key: const Key('lead_detail_approach_locked'),
                      style: typography.body
                          .copyWith(color: colors.textMuted, fontSize: 11),
                    ),
                    const SizedBox(height: 4),
                  ],
                  TextField(
                    key: const Key('lead_detail_approach_field'),
                    controller: _approach,
                    readOnly: const {'contacted', 'won', 'lost'}
                        .contains(lead.status),
                    enabled: widget.canAct && !_busy,
                    maxLines: 6,
                    minLines: 3,
                    style: const TextStyle(color: Colors.white, fontSize: 14),
                    decoration: InputDecoration(
                      hintText: t.lead_detail.approach_hint,
                    ),
                  ),
                  const SizedBox(height: 12),
                  // The same check the approval queue shows. It lived only
                  // there, and this panel — which ALSO sends, with the same
                  // button — sent without it. Two paths to one action, one with
                  // a net and one without, and the netless one is what somebody
                  // reaches for first: open the lead, generate, send.
                  //
                  // `findings` is the reading of the SAVED draft. The condition
                  // was the local text: writing a fresh promise into the field
                  // left the block still saying "no pattern found" — a green
                  // verdict over text nobody checked, which is precisely what
                  // this block exists to prevent. While the text diverges, it
                  // says it diverges.
                  ValueListenableBuilder<TextEditingValue>(
                    valueListenable: _approach,
                    builder: (context, value, _) {
                      final text = value.text.trim();
                      if (text.isEmpty) return const SizedBox.shrink();
                      if (text != lead.approachDraft.trim()) {
                        return Padding(
                          padding: const EdgeInsets.only(bottom: 12),
                          child: Text(
                            t.lead_detail.check_edited,
                            key: const Key('lead_detail_verdict_edited'),
                            style: typography.body.copyWith(
                                color: colors.textMuted,
                                fontSize: 11.5,
                                height: 1.35),
                          ),
                        );
                      }
                      return Padding(
                        padding: const EdgeInsets.only(bottom: 12),
                        child: _Verdict(findings: lead.findings),
                      );
                    },
                  ),
                  if (widget.canAct)
                    // Both the send button and the generate label depend on
                    // whether there is a draft, so they have to rebuild as the
                    // person types — a plain `_approach.text` read here is
                    // evaluated once and then lies for the rest of the session.
                    ValueListenableBuilder<TextEditingValue>(
                      valueListenable: _approach,
                      builder: (context, value, _) {
                        final empty = value.text.trim().isEmpty;
                        return Wrap(
                          spacing: 12,
                          runSpacing: 12,
                          children: [
                            OutlinedButton.icon(
                              key: const Key('lead_detail_regenerate'),
                              onPressed: _busy ? null : _regenerate,
                              icon: const Icon(Icons.autorenew, size: 18),
                              // "Generate another" in front of an empty box
                              // describes a draft that was never written.
                              label: Text(empty
                                  ? t.lead_detail.generate
                                  : t.lead_detail.regenerate),
                            ),
                            // A proposal belongs on the lead for the same reason
                            // the sale does: the moment somebody decides what to
                            // offer is the moment they are looking at who they
                            // are offering it to. It writes into this same
                            // approach draft, so the approval below carries it.
                            OutlinedButton.icon(
                              key: const Key('lead_detail_proposal'),
                              onPressed: _busy
                                  ? null
                                  : () {
                                      // Pop FIRST: on desktop this panel is a
                                      // root-navigator dialog, and go() alone
                                      // swapped the page BEHIND it — the user
                                      // saw nothing happen, then later
                                      // dismissed onto a screen they had
                                      // forgotten asking for.
                                      Navigator.of(context).maybePop();
                                      context.go(
                                          '/sales/proposals/new?lead=${lead.id}');
                                    },
                              icon: const Icon(Icons.request_quote_outlined,
                                  size: 18),
                              label: Text(t.proposal.open),
                            ),
                            // Recording the sale belongs HERE, on the lead that
                            // produced it: a salesperson who has to go find a
                            // separate screen after hanging up records it later,
                            // which in practice means never.
                            OutlinedButton.icon(
                              key: const Key('lead_detail_register_sale'),
                              onPressed:
                                  _busy ? null : () => _registerSale(lead),
                              icon: const Icon(Icons.attach_money, size: 18),
                              label: Text(t.sales_record.title),
                            ),
                            // Enabled over an empty box, this used to swallow
                            // the click — and enabled on a lead the server
                            // refuses (anything but pending_approval) it
                            // walked the operator through the full confirm
                            // dialog into an error. The whole point of that
                            // dialog is that "yes" SENDS.
                            if (lead.status == 'pending_approval')
                              FilledButton.icon(
                                key: const Key('lead_detail_approve'),
                                onPressed: _busy || empty
                                    ? null
                                    : () => _approveSend(lead),
                                icon: const Icon(Icons.send, size: 18),
                                label: Text(t.lead_detail.approve_send),
                              )
                            else
                              Tooltip(
                                message:
                                    t.lead_detail.approve_needs_queue_status,
                                child: FilledButton.icon(
                                  key: const Key('lead_detail_approve'),
                                  onPressed: null,
                                  icon: const Icon(Icons.send, size: 18),
                                  label: Text(t.lead_detail.approve_send),
                                ),
                              ),
                          ],
                        );
                      },
                    )
                  else
                    Text(
                      t.lead_detail.read_only,
                      key: const Key('lead_detail_read_only'),
                      style: typography.body
                          .copyWith(color: colors.textSecondary, fontSize: 12),
                    ),
                  const SizedBox(height: 20),
                  // Archaeology: whoever opens the lead to act does not need
                  // it, and whoever inherited the lead from a colleague needs
                  // it badly. Collapsed, with the count in the header, it
                  // serves both.
                  _CollapsibleSection(
                    sectionKey: 'timeline',
                    title: t.lead_detail.timeline,
                    summary: t.lead_detail
                        .timeline_events(count: lead.timeline.length),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        for (final entry in lead.timeline)
                          Padding(
                            padding: const EdgeInsets.only(bottom: 6),
                            child: Row(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Icon(Icons.circle,
                                    size: 8, color: colors.textSecondary),
                                const SizedBox(width: 8),
                                Expanded(
                                  child: Text(entry,
                                      style: typography.body.copyWith(
                                          color: colors.textSecondary,
                                          fontSize: 12)),
                                ),
                              ],
                            ),
                          ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  /// Asks the platform to look the company up.
  ///
  /// Fire and forget by design — the server answers 202 and enriches in the
  /// background, because a grounded search takes seconds and holding the
  /// panel hostage to it would be worse than coming back to a filled profile.
  Future<void> _enrich() async {
    if (_enriching) return;
    setState(() => _enriching = true);
    final messenger = ScaffoldMessenger.of(context);
    try {
      await ref
          .read(dioProvider)
          .post<dynamic>('/leads/${widget.card.leadId}/enrich');
      messenger
          .showSnackBar(SnackBar(content: Text(t.lead_detail.enrich_started)));
      await _awaitResearch();
    } catch (e) {
      messenger.showSnackBar(SnackBar(
        key: const Key('lead_detail_enrich_error'),
        content: Text(parseApiError(e)),
      ));
    } finally {
      if (mounted) setState(() => _enriching = false);
    }
  }

  Future<void> _regenerate() async {
    setState(() => _busy = true);
    final messenger = ScaffoldMessenger.of(context);
    try {
      final text =
          await ref.read(leadActionsProvider).draft(leadId: widget.card.leadId);
      // The generator already persisted this text: recording the baseline
      // stops the autosave writing it back as if the person had edited it.
      if (text != null && text.isNotEmpty) {
        _savedDraft = text;
        _approach.text = text;
      }
      ref.invalidate(leadDetailProvider(widget.card.leadId));
    } catch (e) {
      messenger.showSnackBar(SnackBar(content: Text(parseApiError(e))));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  /// Approving SENDS a message to a real person, so it always confirms —
  /// showing the exact text and the recipient. This is the platform's
  /// standing rule for outbound, not a per-screen choice.
  /// "Clínica Aurora — +5511999900101", collapsing to whichever half we
  /// actually have. Never repeats itself when the label IS the address.
  String _recipientLine(LeadDetail lead) {
    final label = lead.recipient.trim();
    final address = lead.recipientAddress.trim();
    if (address.isEmpty || address == label) return label;
    if (label.isEmpty) return address;
    return '$label — $address';
  }

  /// Opens the sale form, pre-read from the conversation when there is one.
  ///
  /// The extraction PROPOSES and the person confirms — a model that files
  /// sales on its own turns a misread message into a commission somebody is
  /// owed. A failed or absent reading is not an error: the form simply opens
  /// empty, which is the manual path and works fine.
  Future<void> _registerSale(LeadDetail lead) async {
    SaleProposal? proposal;
    if (lead.conversationId != null) {
      setState(() => _busy = true);
      try {
        proposal =
            await ref.read(saleActionsProvider).extract(lead.conversationId!);
      } catch (_) {
        // Deliberately silent: the manual form is right there.
      } finally {
        if (mounted) setState(() => _busy = false);
      }
    }
    if (!mounted) return;

    await showModalBottomSheet<bool>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Theme.of(context)
          .extension<AppColorsExtension>()!
          .backgroundSecondary,
      // The lead names the customer; the server resolves it and marks the
      // deal won. Sending the lead's own id as the customer used to come
      // back as CONTACT_NOT_FOUND and never linked the sale to the deal.
      builder: (_) => RegisterSaleSheet(
        contactId: lead.recipientAddress,
        leadId: lead.id,
        proposal: proposal,
      ),
    );
  }

  /// Wait for the research the 202 only PROMISED.
  ///
  /// The endpoint answers "started" and does the work in the background. The
  /// panel used to say "volte em instantes" and then never come back: the
  /// result landed in the database and the screen that asked for it was the
  /// last to know, still claiming nothing was known about the company.
  Future<void> _awaitResearch() async {
    final id = widget.card.leadId;
    for (var attempt = 0; attempt < _researchPollAttempts; attempt++) {
      await Future<void>.delayed(_researchPollInterval);
      if (!mounted) return;
      ref.invalidate(leadDetailProvider(id));
      LeadDetail lead;
      try {
        lead = await ref.read(leadDetailProvider(id).future);
      } catch (_) {
        return; // The panel already renders provider errors.
      }
      if (!mounted) return;
      if (!lead.isResearching) {
        // The research can move the card to another column, and a board
        // showing the old one is a second thing to disbelieve.
        ref.invalidate(boardProvider);
        return;
      }
    }
    // Ninety seconds of "Pesquisando…" ending in silence read as a hang.
    // The research keeps running server-side; say so.
    if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(
        key: const Key('lead_detail_enrich_slow'),
        content: Text(t.lead_detail.research_still_running),
      ));
    }
  }

  Future<void> _approveSend(LeadDetail lead) async {
    final text = _approach.text.trim();
    if (text.isEmpty) return;

    final messenger = ScaffoldMessenger.of(context);
    final navigator = Navigator.of(context);

    // A WhatsApp approach cannot leave without its approved template —
    // failing HERE, with the reason, beats a server refusal after the
    // confirmation dialog already said yes.
    final isWhatsApp = _channelType(lead) == ChannelType.whatsapp;
    if (isWhatsApp && _waTemplate == null) {
      messenger.showSnackBar(SnackBar(
        key: const Key('lead_detail_template_missing'),
        content: Text(t.sales.leads.template_required),
      ));
      return;
    }

    // The confirmation quotes the EXACT text and the recipient. That is the
    // platform's standing rule for anything that reaches a real person, not
    // a per-screen choice.
    final confirmed = await showConfirmDialog(
      context,
      title: t.lead_detail.confirm_title,
      // Name AND address. The name is how the operator recognises the
      // lead; the address is the only part that can prove the message is
      // going where they think it is.
      message: '${t.lead_detail.confirm_to}: ${_recipientLine(lead)}\n\n$text',
      confirmText: t.lead_detail.approve_send,
    );
    if (!confirmed) return;

    setState(() => _busy = true);
    try {
      final tpl = isWhatsApp ? _waTemplate : null;
      final scheduledFor = await ref.read(leadActionsProvider).approveSend(
            leadId: widget.card.leadId,
            text: text,
            subject: _isEmail(lead) ? _subject.text.trim() : null,
            templateName: tpl?.name,
            templateLanguage: tpl?.language,
            templateCategory: tpl?.category?.metaValue,
            templateButtons: tpl == null ? null : waTemplateButtons(tpl),
            emailTemplateId: _isEmail(lead) ? _templateId : null,
            emailTemplateVersion: _isEmail(lead) ? _templateVersion : 0,
          );
      _sent = true;
      messenger.showSnackBar(SnackBar(
          content: Text(scheduledFor != null
              ? t.lead_detail
                  .approve_scheduled(when: formatScheduledFor(scheduledFor))
              : t.lead_detail.sent)));
      navigator.maybePop(true);
    } catch (e) {
      messenger.showSnackBar(SnackBar(
        key: const Key('lead_detail_send_error'),
        content: Text(parseApiError(e)),
      ));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }
}

/// A section that folds, with its summary in the header.
///
/// The panel was a column of eight open blocks: at 480px wide, some two
/// thousand pixels of scrolling to reach the actions. Whoever opened a lead to
/// approve its draft passed through profile, fields and history before seeing
/// the button.
///
/// Folding only works with the summary alongside: a closed "Profile ›" hides
/// whether there is anything there; "3 facts about the company ›" answers
/// before the click, and the click becomes a choice rather than a probe.
class _CollapsibleSection extends StatefulWidget {
  const _CollapsibleSection({
    required this.sectionKey,
    required this.title,
    required this.summary,
    required this.child,
    this.initiallyOpen = false,
  });

  final String sectionKey;
  final String title;
  final String summary;
  final Widget child;
  final bool initiallyOpen;

  @override
  State<_CollapsibleSection> createState() => _CollapsibleSectionState();
}

class _CollapsibleSectionState extends State<_CollapsibleSection> {
  late bool _open = widget.initiallyOpen;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        InkWell(
          key: Key('lead_section_${widget.sectionKey}'),
          onTap: () => setState(() => _open = !_open),
          borderRadius: BorderRadius.circular(8),
          child: Padding(
            // 44px de alvo: a seção inteira depende deste toque.
            padding: const EdgeInsets.symmetric(vertical: 10),
            child: Row(
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(widget.title,
                          style: typography.body.copyWith(
                              color: colors.textSecondary, fontSize: 12)),
                      if (!_open && widget.summary.isNotEmpty)
                        Text(
                          widget.summary,
                          key: Key('lead_section_summary_${widget.sectionKey}'),
                          style: typography.body
                              .copyWith(color: Colors.white, fontSize: 13),
                        ),
                    ],
                  ),
                ),
                Icon(_open ? Icons.expand_less : Icons.expand_more,
                    size: 20, color: colors.textSecondary),
              ],
            ),
          ),
        ),
        if (_open) widget.child,
      ],
    );
  }
}

class _Tag extends StatelessWidget {
  const _Tag({required this.label, required this.color});

  final String label;
  final Color color;

  @override
  Widget build(BuildContext context) => Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
        decoration: BoxDecoration(
          color: color.withValues(alpha: 0.15),
          borderRadius: BorderRadius.circular(999),
          border: Border.all(color: color.withValues(alpha: 0.4)),
        ),
        child: Text(label, style: TextStyle(color: color, fontSize: 12)),
      );
}

/// Opens the detail the way this form factor expects: a sheet you can thumb
/// up from the bottom, or a drawer that leaves the board visible beside it.
Future<bool?> showLeadDetail(
  BuildContext context, {
  required BoardCard card,
  required String stageName,
  String? funnelId,
  bool canAct = true,
}) {
  final wide = MediaQuery.of(context).size.width >= 900;
  final panel = LeadDetailPanel(
      card: card, stageName: stageName, funnelId: funnelId, canAct: canAct);

  if (wide) {
    return showGeneralDialog<bool>(
      context: context,
      barrierDismissible: true,
      barrierLabel: card.title,
      pageBuilder: (context, _, __) => Align(
        alignment: Alignment.centerRight,
        child: Material(
          color: Theme.of(context)
              .extension<AppColorsExtension>()!
              .backgroundSecondary,
          child: SizedBox(
            width: 480,
            height: double.infinity,
            child: SafeArea(child: panel),
          ),
        ),
      ),
      transitionBuilder: (context, anim, _, child) => SlideTransition(
        position:
            Tween(begin: const Offset(1, 0), end: Offset.zero).animate(anim),
        child: child,
      ),
    );
  }

  return showModalBottomSheet<bool>(
    context: context,
    isScrollControlled: true,
    backgroundColor:
        Theme.of(context).extension<AppColorsExtension>()!.backgroundSecondary,
    builder: (context) => SafeArea(child: panel),
  );
}

/// Picks one of the account's stored email templates.
///
/// Renders nothing at all when the account has no templates: an empty
/// dropdown is a dead control that makes the panel look broken, and a
/// salesperson who has never written a template is not helped by being told
/// the list is empty here.
class _TemplatePicker extends ConsumerWidget {
  const _TemplatePicker({
    required this.selectedId,
    required this.onPicked,
    required this.enabled,
  });

  final String? selectedId;
  final void Function(EmailTemplate) onPicked;
  final bool enabled;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // A failed or pending load hides the control rather than blocking the
    // send — a template is an accelerator, never a requirement.
    final templates = ref.watch(emailTemplatesProvider).asData?.value;
    if (templates == null || templates.isEmpty) return const SizedBox.shrink();

    return DropdownButtonFormField<String>(
      key: const Key('lead_detail_template_picker'),
      initialValue: selectedId,
      isExpanded: true,
      decoration: InputDecoration(
        labelText: t.lead_detail.use_template,
      ),
      items: [
        for (final tpl in templates)
          DropdownMenuItem<String>(
            value: tpl.id,
            child: Text(
              // The reply rate rides on the option itself: choosing between
              // templates without it is choosing by name, which is the one
              // thing that has nothing to do with whether it works.
              _labelFor(tpl),
              overflow: TextOverflow.ellipsis,
            ),
          ),
      ],
      onChanged: !enabled
          ? null
          : (id) {
              if (id == null) return;
              onPicked(templates.firstWhere((t) => t.id == id));
            },
    );
  }
}

/// Names a template with the number that decides between templates.
///
/// A template nobody has used shows its name alone — a rate computed from
/// zero sends is unknown, and rendering "0%" would condemn it before anyone
/// tried it.
String _labelFor(EmailTemplate tpl) {
  if (tpl.stats.isEmpty) return tpl.name;
  final rate = tpl.stats.first.replyRate;
  if (rate == null) return tpl.name;
  return '${tpl.name} — ${(rate * 100).toStringAsFixed(0)}% ${t.lead_detail.reply_rate_short}';
}

/// What the automatic check found in the draft.
///
/// It says what it did NOT find when it finds nothing — "no pattern", not
/// "reviewed and approved". The check is of known patterns; claiming the text
/// is fine would promise a reading nobody performed.
class _Verdict extends StatelessWidget {
  const _Verdict({required this.findings});

  final List<ApproachFinding> findings;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final clean = findings.isEmpty;
    final color = clean ? colors.textSecondary : colors.orangeAlert;

    return Container(
      key: const Key('lead_detail_verdict'),
      padding: const EdgeInsets.all(10),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: color.withValues(alpha: 0.3)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(clean ? Icons.check_circle_outline : Icons.error_outline,
                  size: 15, color: color),
              const SizedBox(width: 6),
              Expanded(
                child: Text(
                  clean
                      ? t.lead_detail.check_clean
                      : t.lead_detail
                          .check_found(count: findings.length.toString()),
                  style: TextStyle(color: color, fontSize: 11.5),
                ),
              ),
            ],
          ),
          for (final f in findings) ...[
            const SizedBox(height: 6),
            Text('“${f.quote}”',
                style: TextStyle(
                    color: colors.textPrimary,
                    fontSize: 12,
                    fontWeight: FontWeight.w600)),
            Text(f.why ?? _kindLabel(f.kind),
                style: TextStyle(color: colors.textSecondary, fontSize: 11)),
          ],
        ],
      ),
    );
  }

  String _kindLabel(String kind) => switch (kind) {
        'promise' => t.lead_detail.check_kind_promise,
        'invented_fact' => t.lead_detail.check_kind_invented_fact,
        'out_of_scope' => t.lead_detail.check_kind_out_of_scope,
        'placeholder' => t.lead_detail.check_kind_placeholder,
        'tone' => t.lead_detail.check_kind_tone,
        'missing_attachment' => t.lead_detail.check_kind_missing_attachment,
        _ => kind,
      };
}

/// The funnel's collection fields, editable.
///
/// Saves field by field, as each one blurs: a "save all" button would make
/// correcting one field rewrite the others with whatever was on screen, and the
/// server accepts exactly one per call.
class _FunnelFields extends ConsumerStatefulWidget {
  const _FunnelFields({
    required this.leadId,
    required this.schema,
    required this.values,
    required this.onSaved,
  });

  final String leadId;
  final List<MetadataFieldModel> schema;
  final Map<String, String> values;
  final VoidCallback onSaved;

  @override
  ConsumerState<_FunnelFields> createState() => _FunnelFieldsState();
}

class _FunnelFieldsState extends ConsumerState<_FunnelFields> {
  String? _error;
  String? _savedKey;

  Future<void> _save(String key, String value) async {
    if ((widget.values[key] ?? '') == value) return; // nada mudou
    setState(() {
      _error = null;
      _savedKey = null;
    });
    try {
      await ref.read(leadActionsProvider).saveMetadataField(
            leadId: widget.leadId,
            fieldKey: key,
            value: value,
          );
      if (!mounted) return;
      setState(() => _savedKey = key);
      widget.onSaved();
    } catch (e) {
      if (!mounted) return;
      setState(() => _error = parseApiError(e));
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    return Column(
      key: const Key('lead_funnel_fields'),
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          t.board.funnel_fields,
          style: TextStyle(
              color: colors.textSecondary,
              fontSize: 12,
              fontWeight: FontWeight.w600),
        ),
        const SizedBox(height: 10),
        MetadataFieldsForm(
          schema: widget.schema,
          values: widget.values,
          onChanged: _save,
        ),
        if (_savedKey != null)
          Text(t.board.funnel_fields_saved,
              key: const Key('lead_funnel_fields_saved'),
              style: TextStyle(color: colors.textMuted, fontSize: 11)),
        if (_error != null)
          Text(_error!,
              key: const Key('lead_funnel_fields_error'),
              style: TextStyle(color: colors.redAlert, fontSize: 11)),
      ],
    );
  }
}

/// Owner and visibility, side by side.
///
/// They sit together because they answer one question in two steps: whose lead
/// is this, and who else can see it. Private with no owner hides it from
/// nobody, so the second control only exists once the first is set.
class _OwnerAndVisibility extends ConsumerStatefulWidget {
  const _OwnerAndVisibility({
    required this.leadId,
    required this.ownerUserId,
    required this.visibility,
    required this.onChanged,
  });

  final String leadId;
  final String ownerUserId;
  final String visibility;
  final VoidCallback onChanged;

  @override
  ConsumerState<_OwnerAndVisibility> createState() =>
      _OwnerAndVisibilityState();
}

class _OwnerAndVisibilityState extends ConsumerState<_OwnerAndVisibility> {
  String? _error;
  bool _busy = false;

  Future<void> _run(Future<void> Function() action) async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await action();
      if (!mounted) return;
      widget.onChanged();
    } catch (e) {
      if (!mounted) return;
      setState(() => _error = parseApiError(e));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    // Only someone still active can receive a lead: the endpoint returns
    // deactivated members too, and assigning work to someone who left is the
    // same as losing it.
    final team = (ref.watch(teamMembersProvider).asData?.value ?? const [])
        .where((m) => m.isActive)
        .toList();
    final isPrivate = widget.visibility == 'private';

    return Column(
      key: const Key('lead_owner_block'),
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(t.board.owner,
            style: TextStyle(color: colors.textSecondary, fontSize: 12)),
        const SizedBox(height: 6),
        DropdownButtonFormField<String>(
          key: const Key('lead_owner_picker'),
          initialValue: team.any((m) => m.id == widget.ownerUserId)
              ? widget.ownerUserId
              : null,
          isExpanded: true,
          hint: Text(t.board.owner_none,
              style: TextStyle(color: colors.textMuted, fontSize: 14)),
          dropdownColor: colors.backgroundSecondary,
          style: TextStyle(color: colors.textPrimary, fontSize: 14),
          decoration: InputDecoration(
            filled: true,
            fillColor: colors.backgroundTertiary,
            isDense: true,
          ),
          items: [
            // Taking it away from everyone has to be an option in the list:
            // without it, a lead assigned by mistake stays assigned forever.
            DropdownMenuItem<String>(
              value: '',
              child: Text(t.board.owner_none,
                  style: TextStyle(color: colors.textMuted)),
            ),
            for (final member in team)
              DropdownMenuItem<String>(
                value: member.id,
                child: Text(
                    member.name.trim().isEmpty ? member.email : member.name),
              ),
          ],
          onChanged: _busy
              ? null
              : (v) => _run(() => ref.read(leadActionsProvider).assignOwner(
                    leadId: widget.leadId,
                    ownerUserId: (v == null || v.isEmpty) ? null : v,
                  )),
        ),
        // Private with no owner hides it from nobody — the server's filter
        // lets every ownerless lead through.
        if (widget.ownerUserId.isNotEmpty) ...[
          const SizedBox(height: 12),
          Row(
            children: [
              Switch(
                key: const Key('lead_visibility_toggle'),
                value: isPrivate,
                onChanged: _busy
                    ? null
                    : (on) =>
                        _run(() => ref.read(leadActionsProvider).setVisibility(
                              leadId: widget.leadId,
                              visibility: on ? 'private' : 'team',
                            )),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  isPrivate
                      ? t.board.visibility_private
                      : t.board.visibility_team,
                  style: TextStyle(color: colors.textPrimary, fontSize: 13),
                ),
              ),
            ],
          ),
          if (isPrivate)
            Text(t.board.visibility_private_hint,
                style: TextStyle(color: colors.textMuted, fontSize: 11)),
        ],
        if (_error != null)
          Text(_error!,
              key: const Key('lead_owner_error'),
              style: TextStyle(color: colors.redAlert, fontSize: 11)),
      ],
    );
  }
}

/// One crossing: "<product> — they have N of <equipment>", where the product
/// name opens its own page.
///
/// A RichText rather than two widgets so the sentence still wraps as one
/// sentence — a product name on its own line above the count reads as a
/// heading, not as a claim about this hospital.
class _OfferLine extends StatefulWidget {
  const _OfferLine({required this.match});

  final ParkMatch match;

  @override
  State<_OfferLine> createState() => _OfferLineState();
}

class _OfferLineState extends State<_OfferLine> {
  // Held and disposed: a TapGestureRecognizer built inside build() is a leak
  // per rebuild, and this panel rebuilds on every provider tick.
  final _tap = TapGestureRecognizer();

  ParkMatch get match => widget.match;

  @override
  void dispose() {
    _tap.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final base =
        TextStyle(color: colors.textSecondary, fontSize: 12.5, height: 1.45);

    final tail = t.board.panel_offer_tail(
      count: '${match.qtExist}',
      unit: match.qtExist == 1 ? 'aparelho' : 'aparelhos',
      equipment: match.equipment,
    );

    // An older server sends no id. The line stays a sentence rather than
    // becoming a link to nowhere.
    if (match.productId.isEmpty) {
      return Text('${match.productName}$tail', style: base);
    }

    return Semantics(
      link: true,
      label: t.board.panel_offer_open(product: match.productName),
      // Text.rich, not RichText: a Text carries its plain form, so the sentence
      // stays findable as text — by a screen reader, by a search, and by the
      // tests that assert the panel says what it claims to say.
      child: Text.rich(
        TextSpan(
          style: base,
          children: [
            TextSpan(
              text: match.productName,
              style: TextStyle(
                color: colors.limeGlow,
                fontWeight: FontWeight.w600,
                decoration: TextDecoration.underline,
                decorationColor: colors.limeGlow.withValues(alpha: 0.45),
              ),
              recognizer: _tap
                ..onTap =
                    () => context.go('/sales/products/${match.productId}'),
            ),
            TextSpan(text: tail),
          ],
        ),
      ),
    );
  }
}
