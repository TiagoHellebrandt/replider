import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/components/app_dialogs.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/dashboard/domain/models/email_risk.dart';
import 'package:app/features/dashboard/presentation/controllers/dashboard_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// What the platform found wrong with an inbound message, and what to do.
///
/// Three levels, and the loudest one exists for a single scenario: a supplier
/// you have paid before is asking for money at a new account. That is the
/// golpe do boleto, and it is the last moment anyone can stop the transfer —
/// so it does not just warn, it says to pick up the phone and call a number
/// the email did not give you.
///
/// The dismissal is as important as the warning. A false positive that cannot
/// be answered teaches the reader to scroll past the banner, and then the
/// real one is invisible too.
class EmailRiskBanner extends ConsumerStatefulWidget {
  const EmailRiskBanner({
    super.key,
    required this.risk,
    required this.messageId,
    required this.conversationId,
  });

  final EmailRisk risk;
  final String messageId;
  final String conversationId;

  @override
  ConsumerState<EmailRiskBanner> createState() => _EmailRiskBannerState();
}

class _EmailRiskBannerState extends ConsumerState<EmailRiskBanner> {
  bool _busy = false;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final risk = widget.risk;

    // Money about to move somewhere new is red. Everything else is orange:
    // painting every finding the same colour is how the one that matters
    // stops standing out.
    final severe = risk.hasPaymentChange;
    final color = severe
        ? colors.redAlert
        : (risk.isQuarantined ? colors.orangeAlert : colors.textMuted);

    return Container(
      key: Key('email_risk_${widget.messageId}'),
      margin: const EdgeInsets.only(top: 8),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: colors.backgroundTertiary,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: color, width: severe ? 2 : 1),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Icon(
                  severe
                      ? Icons.report_gmailerrorred
                      : Icons.warning_amber_rounded,
                  size: 18,
                  color: color),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  _title(),
                  key: Key('email_risk_title_${widget.messageId}'),
                  style: typography.body.copyWith(
                      color: Colors.white,
                      fontSize: 14,
                      fontWeight: FontWeight.w600),
                ),
              ),
            ],
          ),
          // The instruction comes BEFORE the evidence when money is involved:
          // someone about to pay an invoice needs the action first.
          if (severe) ...[
            const SizedBox(height: 6),
            Text(
              t.email_risk.payment_action,
              key: Key('email_risk_action_${widget.messageId}'),
              style: typography.body
                  .copyWith(color: Colors.white, fontSize: 13, height: 1.4),
            ),
          ],
          const SizedBox(height: 8),
          for (final s in risk.signals)
            Padding(
              padding: const EdgeInsets.only(bottom: 4),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('• ',
                      style: typography.body
                          .copyWith(color: colors.textMuted, fontSize: 13)),
                  Expanded(
                    child: Text(
                      s.detail,
                      style: typography.body.copyWith(
                          color: colors.textSecondary,
                          fontSize: 13,
                          height: 1.4),
                    ),
                  ),
                ],
              ),
            ),
          Wrap(
            spacing: 4,
            children: [
              TextButton(
                key: Key('email_risk_legit_${widget.messageId}'),
                onPressed: _busy ? null : _vouch,
                child: Text(t.email_risk.legit),
              ),
              // The other half of the loop. One report is one company's
              // judgement; two independent ones block the sender for
              // everybody, which is what makes being on the network worth
              // something.
              TextButton(
                key: Key('email_risk_scam_${widget.messageId}'),
                onPressed: _busy ? null : _reportScam,
                child: Text(t.email_risk.scam,
                    style: TextStyle(color: colors.redAlert)),
              ),
            ],
          ),
        ],
      ),
    );
  }

  String _title() {
    if (widget.risk.hasPaymentChange) return t.email_risk.payment_title;
    if (widget.risk.isQuarantined) return t.email_risk.quarantine_title;
    return t.email_risk.attention_title;
  }

  /// Reporting asks WHY, optionally.
  ///
  /// The reason is the highest-value thing in a report — "esse fornecedor
  /// nunca manda boleto por email" is knowledge no technical check has — but
  /// it is not required: demanding it would cost reports from people in a
  /// hurry, and a report with no reason still protects the network.
  Future<void> _reportScam() async {
    final reason = await showInputDialog(
      context,
      title: t.email_risk.scam_title,
      hint: t.email_risk.scam_why_hint,
      confirmText: t.email_risk.scam,
    );
    // Null means cancelled; an empty string means "reported, no explanation".
    if (reason == null) return;

    setState(() => _busy = true);
    final messenger = ScaffoldMessenger.of(context);
    try {
      await ref.read(dioProvider).post<dynamic>(
        '/messages/${widget.messageId}/report-scam',
        data: <String, dynamic>{'reason': reason},
      );
      ref.invalidate(messagesProvider(widget.conversationId));
      messenger.showSnackBar(SnackBar(content: Text(t.email_risk.scam_done)));
    } catch (e) {
      messenger.showSnackBar(SnackBar(
        key: Key('email_risk_scam_error_${widget.messageId}'),
        content: Text(parseApiError(e)),
      ));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  /// Vouching is a security decision, so it is confirmed rather than done on
  /// a single tap — and the confirmation says what it actually means: we stop
  /// warning about this sender on this account, from now on.
  Future<void> _vouch() async {
    final ok = await showConfirmDialog(
      context,
      title: t.email_risk.legit,
      message: t.email_risk.legit_confirm,
      confirmText: t.email_risk.legit,
    );
    if (!ok) return;

    setState(() => _busy = true);
    final messenger = ScaffoldMessenger.of(context);
    try {
      await ref
          .read(dioProvider)
          .post<void>('/messages/${widget.messageId}/legitimate');
      // The banner disappears because the warning was ANSWERED. Re-reading
      // the thread is what makes that true rather than cosmetic.
      ref.invalidate(messagesProvider(widget.conversationId));
      messenger.showSnackBar(SnackBar(content: Text(t.email_risk.legit_done)));
    } catch (e) {
      messenger.showSnackBar(SnackBar(
        key: Key('email_risk_error_${widget.messageId}'),
        content: Text(parseApiError(e)),
      ));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }
}
