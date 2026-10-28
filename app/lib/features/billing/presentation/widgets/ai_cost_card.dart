import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/design_system/components/retryable_error.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/core/utils/money_input.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/i18n/strings.g.dart';

part 'ai_cost_card.g.dart';

/// The BYOK cost surface: what this account actually spent on AI this
/// cycle, what a month looks like at its own pace, and the ceiling the
/// OWNER sets on themselves. The answer to "vou pagar uma conta
/// astronômica?" is this card: the spend is visible, the projection comes
/// from the account's own history, and the ceiling is a hard stop.
class AICostReport {
  const AICostReport({
    required this.cycleBrlCents,
    required this.projectedBrlCents,
    this.budgetCents,
    this.budgetUsedPct,
  });

  final int cycleBrlCents;
  final int projectedBrlCents;
  final int? budgetCents;
  final int? budgetUsedPct;

  factory AICostReport.fromJson(Map<String, dynamic> json) {
    final cycle = json['cycle'] as Map<String, dynamic>?;
    return AICostReport(
      cycleBrlCents: (cycle?['total_brl_cents'] as num?)?.toInt() ?? 0,
      projectedBrlCents:
          (json['projected_monthly_brl_cents'] as num?)?.toInt() ?? 0,
      budgetCents: (json['budget_cents'] as num?)?.toInt(),
      budgetUsedPct: (json['budget_used_pct'] as num?)?.toInt(),
    );
  }
}

@riverpod
Future<AICostReport> aiCostReport(Ref ref) async {
  final resp = await ref.read(dioProvider).get<dynamic>('/me/ai-costs');
  return AICostReport.fromJson(resp.data as Map<String, dynamic>);
}

String _brl(int cents) =>
    'R\$ ${(cents / 100).toStringAsFixed(2).replaceAll('.', ',')}';

class AICostCard extends ConsumerWidget {
  const AICostCard({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final report = ref.watch(aiCostReportProvider);

    // Never vanish: this is the screen someone opens asking "vou pagar uma
    // conta astronômica?", and it holds the only budget-editing control.
    // Disappearing on loading OR error was indistinguishable from "não tem".
    if (report.isLoading && !report.hasValue) {
      return const Padding(
        padding: EdgeInsets.symmetric(vertical: 24),
        child: Center(child: CircularProgressIndicator()),
      );
    }
    if (report.hasError && !report.hasValue) {
      return RetryableError(
        error: report.error!,
        onRetry: () => ref.invalidate(aiCostReportProvider),
      );
    }
    final data = report.asData?.value;
    if (data == null) return const SizedBox.shrink();

    final pct = data.budgetUsedPct;
    final nearCeiling = pct != null && pct >= 80;

    return Container(
      key: const Key('ai_cost_card'),
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(
            color: nearCeiling ? colors.orangeAlert : colors.borderLight),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.bolt_outlined, size: 18, color: colors.textPrimary),
              const SizedBox(width: 8),
              Text(
                t.billing.ai_cost_title,
                style: TextStyle(
                    color: colors.textPrimary,
                    fontWeight: FontWeight.w700,
                    fontSize: 15),
              ),
            ],
          ),
          const SizedBox(height: 14),
          Row(
            children: [
              Expanded(
                child: _stat(
                    colors, t.billing.ai_cost_cycle, _brl(data.cycleBrlCents)),
              ),
              Expanded(
                child: _stat(
                    colors,
                    t.billing.ai_cost_projection,
                    data.projectedBrlCents > 0
                        ? _brl(data.projectedBrlCents)
                        : '—'),
              ),
              Expanded(
                child: _stat(
                    colors,
                    t.billing.ai_cost_budget,
                    data.budgetCents != null
                        ? '${_brl(data.budgetCents!)}${pct != null ? ' ($pct%)' : ''}'
                        : t.billing.ai_cost_no_budget),
              ),
            ],
          ),
          const SizedBox(height: 12),
          Align(
            alignment: Alignment.centerRight,
            child: TextButton(
              key: const Key('ai_budget_edit'),
              onPressed: () => _editBudget(context, ref, data.budgetCents),
              child: Text(t.billing.ai_cost_set_budget,
                  style: TextStyle(color: colors.bluePrimary)),
            ),
          ),
        ],
      ),
    );
  }

  Widget _stat(AppColorsExtension colors, String label, String value) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(label, style: TextStyle(color: colors.textMuted, fontSize: 11)),
        const SizedBox(height: 4),
        Text(value,
            style: TextStyle(
                color: colors.textPrimary,
                fontSize: 15,
                fontWeight: FontWeight.w700)),
      ],
    );
  }

  Future<void> _editBudget(
      BuildContext context, WidgetRef ref, int? currentCents) async {
    final controller = TextEditingController(
        text: currentCents != null
            ? (currentCents / 100).toStringAsFixed(0)
            : '');
    final saved = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(t.billing.ai_cost_budget_dialog_title),
        content: TextField(
          key: const Key('ai_budget_field'),
          controller: controller,
          keyboardType: TextInputType.number,
          decoration: InputDecoration(
            prefixText: 'R\$ ',
            helperText: t.billing.ai_cost_budget_help,
            helperMaxLines: 3,
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(t.common.cancel),
          ),
          FilledButton(
            key: const Key('ai_budget_save'),
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(t.common.save),
          ),
        ],
      ),
    );
    if (saved != true) return;
    // The position-based parser, like every money field: the inline
    // `replaceAll(',', '.')` this replaced read "1.500" as ONE AND A HALF
    // reais — the person set a R$ 1.500 ceiling and the AI stopped on the
    // first request.
    final cents = parseMoneyToCents(controller.text);
    try {
      await ref
          .read(dioProvider)
          .put<dynamic>('/me/ai-budget', data: {'budget_cents': cents});
      ref.invalidate(aiCostReportProvider);
    } on DioException catch (e) {
      // This is the customer's only spend brake — a silent failure left the
      // dialog closed, the old ceiling in place and nobody told.
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(
          key: const Key('ai_budget_save_error'),
          content: Text(parseApiError(e)),
        ));
      }
    }
  }
}
