import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/utils/account_currency.dart';
import 'package:app/core/utils/currency_format.dart';
import 'package:app/design_system/components/glass_card.dart';
import 'package:app/design_system/components/retryable_error.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/presentation/controllers/portfolio_controller.dart';
import 'package:app/features/sales/presentation/controllers/relationship_controller.dart';
import 'package:app/features/sales/presentation/widgets/cross_sell_card.dart';
import 'package:app/i18n/strings.g.dart';

/// One customer's history and predictions (§6.5, F2).
///
/// The screen a salesperson opens before calling: what this customer buys,
/// how often, what it is worth, and when they are next going to need it.
class PortfolioCustomerScreen extends ConsumerWidget {
  const PortfolioCustomerScreen({super.key, required this.contactId});

  final String contactId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final state = ref.watch(portfolioDetailProvider(contactId));

    return Scaffold(
      backgroundColor: colors.backgroundPrimary,
      appBar: AppBar(
        // A deep link outside the shell: without this the screen has neither a
        // back button nor a menu — you get in and only get out by editing the
        // URL.
        leading: IconButton(
          tooltip: t.common.back,
          icon: const Icon(Icons.arrow_back),
          onPressed: () => context.canPop()
              ? context.pop()
              : context.go('/funnel?view=portfolio'),
        ),
        backgroundColor: colors.backgroundPrimary,
        title:
            Text(state.asData?.value.customer.contactName ?? t.portfolio.title),
      ),
      body: state.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, _) => RetryableError(
          error: err,
          onRetry: () => ref.invalidate(portfolioDetailProvider(contactId)),
        ),
        data: (detail) => ListView(
          padding: const EdgeInsets.fromLTRB(16, 16, 16, 32),
          children: [
            GlassCard(
              child: Padding(
                padding: const EdgeInsets.all(14),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      t.portfolio.purchases(count: detail.customer.purchases),
                      style:
                          TextStyle(color: colors.textSecondary, fontSize: 13),
                    ),
                    const SizedBox(height: 4),
                    // Net leads: what a customer is worth is what the company
                    // kept, not what they were invoiced.
                    Text(
                      formatAccountMoney(ref, detail.customer.netCents),
                      key: const Key('portfolio_detail_net'),
                      style: typography.display.copyWith(
                          fontSize: 22,
                          color: Colors.white,
                          fontWeight: FontWeight.w700),
                    ),
                    // The next date worth remembering (F4). Promised by the
                    // portfolio card since F2 and only answerable now that
                    // there is somewhere for those dates to live.
                    if (_nextDate(ref, contactId) case final next?) ...[
                      const SizedBox(height: 6),
                      Row(
                        children: [
                          Icon(Icons.cake_outlined,
                              size: 14, color: colors.textSecondary),
                          const SizedBox(width: 6),
                          Text(
                            next,
                            key: const Key('portfolio_detail_next_date'),
                            style: TextStyle(
                                color: colors.textSecondary, fontSize: 12),
                          ),
                        ],
                      ),
                    ],
                    if (detail.customer.daysUntilDue case final days?) ...[
                      const SizedBox(height: 8),
                      Text(
                        days < 0
                            ? t.portfolio.ran_out(days: -days)
                            : t.portfolio.runs_out_in(days: days),
                        key: const Key('portfolio_detail_due'),
                        style: TextStyle(
                          color:
                              days < 0 ? colors.redAlert : colors.orangeAlert,
                          fontSize: 13,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ],
                  ],
                ),
              ),
            ),
            // What else to offer, before the history: the history explains
            // the past, this is the reason to pick up the phone.
            CrossSellCard(contactId: contactId),
            const SizedBox(height: 20),
            Text(t.portfolio.history,
                style: TextStyle(
                    color: colors.textPrimary,
                    fontSize: 13,
                    fontWeight: FontWeight.w700)),
            const SizedBox(height: 8),
            for (final sale in detail.sales)
              Padding(
                padding: const EdgeInsets.only(bottom: 8),
                child: GlassCard(
                  child: Padding(
                    padding: const EdgeInsets.all(12),
                    child: Row(
                      children: [
                        Expanded(
                          child: Text(
                            sale.items.isEmpty
                                ? t.portfolio.sale_without_items
                                : sale.items.first.description,
                            style: TextStyle(
                                color: colors.textSecondary, fontSize: 12),
                          ),
                        ),
                        Text(
                          formatCurrencyCents(sale.grossCents, sale.currency),
                          style: const TextStyle(
                              color: Colors.white,
                              fontSize: 13,
                              fontWeight: FontWeight.w600),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

/// The soonest date recorded for THIS customer, as "Aniversário · 12/05".
///
/// Account-wide dates are excluded: a sector day belongs to the whole
/// portfolio, and showing it on every customer would make it noise on all of
/// them instead of a fact about one.
String? _nextDate(WidgetRef ref, String contactId) {
  final dates = ref.watch(relationshipDatesProvider).asData?.value;
  if (dates == null) return null;

  final mine = dates.where((d) => d.contactId == contactId).toList();
  if (mine.isEmpty) return null;

  // Already ordered soonest-first by the API; the client does not re-sort so
  // the two can never disagree about what "next" means.
  final next = mine.first;
  final label = next.label.isEmpty ? _dateKindLabel(next.kind) : next.label;
  final due = next.dueAt;
  final when = due != null
      ? '${due.day.toString().padLeft(2, '0')}/'
          '${due.month.toString().padLeft(2, '0')}'
      : '${next.day.toString().padLeft(2, '0')}/'
          '${next.month.toString().padLeft(2, '0')}';
  return '$label · $when';
}

String _dateKindLabel(String kind) => switch (kind) {
      'birthday' => t.relationship.kind_birthday,
      'founded' => t.relationship.kind_founded,
      'sector' => t.relationship.kind_sector,
      'maintenance' => t.relationship.kind_maintenance,
      _ => t.relationship.kind_custom,
    };
