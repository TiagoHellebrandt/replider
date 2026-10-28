import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/utils/account_currency.dart';
import 'package:app/core/utils/money_input.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/presentation/controllers/sales_record_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Records a sale (§5, F1).
///
/// The form asks for cost and tax rather than only a total, and says why:
/// in distribution the cost of goods is most of the invoice, and a sale that
/// looks good on gross can be a loss on margin.
///
/// It also asks for the repurchase cycle, because that one number is what
/// makes a customer come back on their own instead of being remembered.
class RegisterSaleSheet extends ConsumerStatefulWidget {
  const RegisterSaleSheet({
    super.key,
    this.contactId = '',
    this.leadId = '',
    this.proposal,
  });

  /// The customer, as the channel keys them. May be empty when the sheet
  /// opens from a lead: the server resolves the customer from [leadId].
  final String contactId;

  /// The deal this sale closes. Sent so the server links the sale to the
  /// lead and moves it to "won" — the card on the board should not still
  /// say "qualified" after the money came in.
  final String leadId;

  /// A model's reading of a conversation, awaiting confirmation. When set,
  /// the fields arrive filled and the question is shown at the top — the AI
  /// proposes, the human validates, which is the house pattern.
  final SaleProposal? proposal;

  @override
  ConsumerState<RegisterSaleSheet> createState() => _RegisterSaleSheetState();
}

class _RegisterSaleSheetState extends ConsumerState<RegisterSaleSheet> {
  final _gross = TextEditingController();
  final _cost = TextEditingController();
  final _tax = TextEditingController();
  final _cycle = TextEditingController();
  final _notes = TextEditingController();
  bool _busy = false;

  /// What this sale would pay the person recording it (§6.6, G3). Null means
  /// "no rule covers it" — never rendered as R$ 0,00, which would be a claim
  /// about their pay rather than about the rules.

  @override
  void initState() {
    super.initState();
    if (widget.proposal case final p? when p.found) {
      if (p.grossCents > 0) _gross.text = _reais(p.grossCents);
      if (p.cycleDays > 0) _cycle.text = p.cycleDays.toString();
    }
  }

  @override
  void dispose() {
    for (final c in [_gross, _cost, _tax, _cycle, _notes]) {
      c.dispose();
    }
    super.dispose();
  }

  /// Cents into the text the field shows. The form talks in whole units
  /// because that is what a person says out loud; the wire talks in cents
  /// because money in fractions accumulates error.
  ///
  /// Written in the account's notation so the person types over it in the same
  /// one they read — and so re-parsing an untouched field returns the amount it
  /// started with.
  String _reais(int cents) =>
      formatCentsForInput(cents, ref.read(accountCurrencyProvider));

  int _cents(TextEditingController c) => parseMoneyToCents(c.text);

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    final gross = _cents(_gross);
    final net = gross - _cents(_cost) - _cents(_tax);

    return Padding(
      padding: EdgeInsets.only(
        left: 20,
        right: 20,
        top: 20,
        bottom: MediaQuery.of(context).viewInsets.bottom + 20,
      ),
      child: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(t.sales_record.title,
                style: typography.display.copyWith(
                    fontSize: 18,
                    color: Colors.white,
                    fontWeight: FontWeight.w600)),

            // The model's question, verbatim. The whole interaction is this
            // one sentence, so it leads.
            if (widget.proposal case final p?
                when p.found && p.question.isNotEmpty) ...[
              const SizedBox(height: 12),
              Container(
                key: const Key('sale_ai_question'),
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(
                  color: colors.bluePrimary.withValues(alpha: 0.12),
                  borderRadius: BorderRadius.circular(12),
                ),
                child: Row(
                  children: [
                    Icon(Icons.auto_awesome,
                        size: 16, color: colors.bluePrimary),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(p.question,
                          style: TextStyle(
                              color: colors.textPrimary, fontSize: 13)),
                    ),
                  ],
                ),
              ),
            ],

            const SizedBox(height: 16),
            _field(colors, t.sales_record.gross, _gross,
                hint: t.sales_record.gross_hint),
            const SizedBox(height: 12),
            _field(colors, t.sales_record.cost, _cost,
                hint: t.sales_record.cost_hint),
            const SizedBox(height: 12),
            _field(colors, t.sales_record.tax, _tax,
                hint: t.sales_record.tax_hint),
            const SizedBox(height: 12),
            _field(colors, t.sales_record.cycle, _cycle,
                hint: t.sales_record.cycle_hint, integer: true),
            const SizedBox(height: 12),
            _field(colors, t.sales_record.notes, _notes, lines: 2),

            // The margin, shown while they type. Finding it out afterwards,
            // in a report, is how a good month turns out to have been a bad
            // one.
            const SizedBox(height: 16),
            if (gross > 0)
              Row(
                children: [
                  Expanded(
                    child: Text(t.sales_record.net,
                        style: TextStyle(
                            color: colors.textSecondary, fontSize: 13)),
                  ),
                  Text(
                    formatAccountMoney(ref, net),
                    key: const Key('sale_net_preview'),
                    style: TextStyle(
                      color: net < 0 ? colors.redAlert : colors.whatsappGreen,
                      fontSize: 15,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ],
              ),
            if (gross > 0 && net < 0)
              Padding(
                padding: const EdgeInsets.only(top: 4),
                child: Text(
                  t.sales_record.underwater_warning,
                  key: const Key('sale_underwater_warning'),
                  style: TextStyle(color: colors.redAlert, fontSize: 11),
                ),
              ),

            const SizedBox(height: 20),
            SizedBox(
              width: double.infinity,
              child: FilledButton(
                key: const Key('sale_submit'),
                // A sale with no value is refused by the server ("informe o
                // valor da venda"), and the refusal used to cost a round trip
                // and land in a snackbar under this sheet.
                onPressed: _busy || gross <= 0 ? null : _submit,
                child: Text(t.sales_record.submit),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _field(
    AppColorsExtension colors,
    String label,
    TextEditingController controller, {
    String? hint,
    int lines = 1,
    bool integer = false,
  }) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(label,
            style: TextStyle(
                color: colors.textSecondary,
                fontSize: 12,
                fontWeight: FontWeight.w600)),
        const SizedBox(height: 4),
        TextField(
          key: Key('sale_field_$label'),
          controller: controller,
          enabled: !_busy,
          maxLines: lines,
          keyboardType: lines > 1
              ? TextInputType.multiline
              : const TextInputType.numberWithOptions(decimal: true),
          inputFormatters: integer
              ? [FilteringTextInputFormatter.digitsOnly]
              : const <TextInputFormatter>[],
          onChanged: (_) => setState(() {}),
          style: const TextStyle(color: Colors.white, fontSize: 14),
          decoration: InputDecoration(
            hintText: hint,
            hintStyle: TextStyle(color: colors.textMuted, fontSize: 12),
          ),
        ),
      ],
    );
  }

  Future<void> _submit() async {
    final messenger = ScaffoldMessenger.of(context);
    final navigator = Navigator.of(context);

    setState(() => _busy = true);
    try {
      await ref.read(saleActionsProvider).register(
            contactId: widget.contactId,
            leadId: widget.leadId,
            grossCents: _cents(_gross),
            costCents: _cents(_cost),
            taxCents: _cents(_tax),
            cycleDays: int.tryParse(_cycle.text.trim()) ?? 0,
            items: widget.proposal?.items ?? const [],
            notes: _notes.text.trim(),
            fromAI: widget.proposal?.found ?? false,
          );
      ref.invalidate(salesProvider);
      messenger.showSnackBar(SnackBar(content: Text(t.sales_record.saved)));
      navigator.maybePop(true);
    } catch (e) {
      messenger.showSnackBar(SnackBar(
        key: const Key('sale_error'),
        content: Text(parseApiError(e)),
      ));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }
}
