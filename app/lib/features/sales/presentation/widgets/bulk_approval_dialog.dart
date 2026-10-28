import 'package:flutter/material.dart';

import 'package:app/design_system/components/custom_text_field.dart';
import 'package:app/design_system/components/primary_pill_button.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/presentation/controllers/approval_queue_controller.dart'
    show QueueLead, recipientLine;
import 'package:app/features/sales/presentation/screens/approval_queue_screen.dart'
    show kindLabel;
import 'package:app/i18n/strings.g.dart';

/// What the review produced: the ids that survived it, and the subject the
/// batch's EMAILS go out under. The single approval collects a subject in
/// its dialog precisely because "text only" used to ship the server default
/// — the volume path had silently regressed to exactly that.
class BulkApprovalResult {
  const BulkApprovalResult({required this.keptIds, required this.subject});

  final List<String> keptIds;
  final String subject;
}

/// Reviewing a batch of approaches, one by one, before any of them goes out.
///
/// The bulk send used to confirm through the generic dialog, with every
/// recipient and every message concatenated into one string separated by
/// `———`. That dialog scrolls and selects, so nothing was clipped — but twelve
/// drafts became a three-thousand-character wall with no way to tell where one
/// ended, no count, and no way to drop one after reading it. It was legible and
/// not reviewable: the only two moves were send all twelve or cancel all twelve.
///
/// The whole product posture on cold outbound is that a human sees the exact
/// words before a stranger does. Doing twelve at once must not be the loophole
/// in it, so each one is its own card, with its own recipient, its own findings
/// and its own tick — and the count on the button is what will actually be sent.
///
/// [blocked] rows render disabled with [blockedReason] and can never be
/// ticked: a WhatsApp lead needs a template no batch can choose, and finding
/// that out AFTER reading and ticking twelve cards ("Enviar 12" → 0) made the
/// review a lie.
Future<BulkApprovalResult?> showBulkApprovalDialog(
  BuildContext context, {
  required List<QueueLead> leads,
  Set<String> blocked = const {},
  String? blockedReason,
  bool askSubject = false,
}) {
  return showDialog<BulkApprovalResult>(
    context: context,
    builder: (_) => _BulkApprovalDialog(
      leads: leads,
      blocked: blocked,
      blockedReason: blockedReason,
      askSubject: askSubject,
    ),
  );
}

class _BulkApprovalDialog extends StatefulWidget {
  const _BulkApprovalDialog({
    required this.leads,
    required this.blocked,
    required this.blockedReason,
    required this.askSubject,
  });

  final List<QueueLead> leads;
  final Set<String> blocked;
  final String? blockedReason;
  final bool askSubject;

  @override
  State<_BulkApprovalDialog> createState() => _BulkApprovalDialogState();
}

class _BulkApprovalDialogState extends State<_BulkApprovalDialog> {
  /// Everything SENDABLE starts ticked: the operator already chose these in
  /// the queue, and making them re-tick twelve leads they just picked is
  /// punishing them for having read the list. Blocked rows never enter.
  late final Set<String> _keep = {
    for (final l in widget.leads)
      if (!widget.blocked.contains(l.id)) l.id,
  };

  final _subjectController = TextEditingController();

  @override
  void initState() {
    super.initState();
    _subjectController.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _subjectController.dispose();
    super.dispose();
  }

  bool get _subjectMissing =>
      widget.askSubject && _subjectController.text.trim().isEmpty;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    return AlertDialog(
      key: const Key('bulk_approval_dialog'),
      backgroundColor: colors.backgroundSecondary,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      title: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            t.lead_detail.bulk_review_title(count: widget.leads.length),
            style: TextStyle(
                color: colors.textPrimary,
                fontSize: 16,
                fontWeight: FontWeight.w700),
          ),
          const SizedBox(height: 4),
          Text(
            t.lead_detail.bulk_review_hint,
            style: typography.body
                .copyWith(color: colors.textSecondary, fontSize: 12),
          ),
        ],
      ),
      content: SizedBox(
        // Bounded so the list scrolls inside the dialog rather than the dialog
        // growing past the screen: twelve drafts are taller than any phone.
        width: 460,
        height: MediaQuery.sizeOf(context).height * 0.55,
        child: Column(
          children: [
            // One subject for the batch's emails. Required: an empty one
            // silently fell back to the server's default subject — the exact
            // failure the single-approval dialog was built to prevent.
            if (widget.askSubject) ...[
              CustomTextField(
                key: const Key('bulk_approval_subject'),
                controller: _subjectController,
                hint: t.lead_detail.bulk_subject_hint,
              ),
              const SizedBox(height: 10),
            ],
            Expanded(
              child: ListView.separated(
                key: const Key('bulk_approval_list'),
                itemCount: widget.leads.length,
                separatorBuilder: (_, __) => const SizedBox(height: 10),
                itemBuilder: (context, i) {
                  final lead = widget.leads[i];
                  final isBlocked = widget.blocked.contains(lead.id);
                  return _ReviewCard(
                    lead: lead,
                    kept: _keep.contains(lead.id),
                    blockedReason: isBlocked ? widget.blockedReason : null,
                    onChanged: isBlocked
                        ? null
                        : (on) => setState(() =>
                            on ? _keep.add(lead.id) : _keep.remove(lead.id)),
                  );
                },
              ),
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          key: const Key('bulk_approval_cancel'),
          onPressed: () => Navigator.of(context).pop(),
          child: Text(t.common.cancel,
              style: TextStyle(color: colors.textSecondary)),
        ),
        PrimaryPillButton(
          key: const Key('bulk_approval_send'),
          // Nothing ticked is nothing to send. A live button there would open
          // the confirmation and then do nothing, which is the dead click this
          // product already has a test forbidding.
          onPressed: _keep.isEmpty || _subjectMissing
              ? null
              : () => Navigator.of(context).pop(BulkApprovalResult(
                    keptIds: _keep.toList(),
                    subject: _subjectController.text.trim(),
                  )),
          text: _keep.isEmpty
              ? t.lead_detail.bulk_review_none
              : _subjectMissing
                  ? t.lead_detail.bulk_subject_missing
                  : t.lead_detail.bulk_review_send(count: _keep.length),
        ),
      ],
    );
  }
}

/// One approach, as it will be sent.
class _ReviewCard extends StatelessWidget {
  const _ReviewCard({
    required this.lead,
    required this.kept,
    required this.onChanged,
    this.blockedReason,
  });

  final QueueLead lead;
  final bool kept;
  final ValueChanged<bool>? onChanged;

  /// Why this row cannot be part of the batch (null = it can).
  final String? blockedReason;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    return Container(
      key: Key('bulk_approval_card_${lead.id}'),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: colors.backgroundPrimary,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          // A dropped card stays visible and greyed rather than disappearing:
          // the list has to keep matching what the operator read, or the count
          // stops being checkable against the screen.
          color: kept ? colors.borderMedium : colors.borderLight,
        ),
      ),
      child: Opacity(
        opacity: kept ? 1 : 0.45,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                SizedBox(
                  width: 28,
                  height: 28,
                  child: Checkbox(
                    key: Key('bulk_approval_check_${lead.id}'),
                    value: kept,
                    onChanged: onChanged == null
                        ? null
                        : (v) => onChanged!(v ?? false),
                  ),
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    // Name AND address. The name is how the operator recognises
                    // the lead; the address is the only part that proves the
                    // message goes where they think it does.
                    recipientLine(lead.recipient, lead.recipientAddress),
                    style: typography.body.copyWith(
                        color: Colors.white,
                        fontSize: 13,
                        fontWeight: FontWeight.w600),
                  ),
                ),
              ],
            ),
            if (blockedReason != null) ...[
              const SizedBox(height: 6),
              Text(
                blockedReason!,
                key: Key('bulk_approval_blocked_${lead.id}'),
                style: typography.body
                    .copyWith(color: colors.orangeAlert, fontSize: 11),
              ),
            ],
            const SizedBox(height: 8),
            // Verbatim and unshortened: an approval based on a truncated
            // preview is not an approval.
            SelectableText(
              lead.draft,
              key: Key('bulk_approval_text_${lead.id}'),
              style: typography.body
                  .copyWith(color: Colors.white, fontSize: 12.5, height: 1.4),
            ),
            // The automatic check, per lead. It is the reason a batch review can
            // be faster than twelve single reviews without being worse: the one
            // draft that promises a deadline is marked, instead of hiding in a
            // wall of text that all looks alike.
            if (lead.findings.isNotEmpty) ...[
              const SizedBox(height: 8),
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Icon(Icons.error_outline,
                      size: 14, color: colors.orangeAlert),
                  const SizedBox(width: 6),
                  Expanded(
                    child: Text(
                      lead.findings
                          .map((f) => f.why ?? kindLabel(f.kind))
                          .join(' · '),
                      key: Key('bulk_approval_findings_${lead.id}'),
                      style: typography.body.copyWith(
                          color: colors.orangeAlert,
                          fontSize: 11,
                          height: 1.35),
                    ),
                  ),
                ],
              ),
            ],
          ],
        ),
      ),
    );
  }
}
