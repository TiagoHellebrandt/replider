import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:app/core/auth/account_role.dart';
import 'package:app/core/utils/account_currency.dart';
import 'package:app/design_system/components/empty_state_card.dart';
import 'package:app/design_system/components/glass_card.dart';
import 'package:app/design_system/components/retryable_error.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/sales/presentation/controllers/portfolio_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// The Carteira (§6.5, F2) — since 2026-09-01 THE customer list.
///
/// `/contacts` used to be a fourth list of the same people (Kanban, Carteira,
/// "Na minha caixa", Contatos). It is gone; what it offered that this view did
/// not — a search box and the door to the CSV import — lives here now, and
/// `GET /portfolio` lists every contact, bought or merely imported.
///
/// The list is ordered by URGENCY, not by name or revenue: whoever opens it
/// has an hour, and the only question is who to call first. A customer with
/// no purchase yet sorts last — worth listing, never the most urgent thing.
///
/// A body with no chrome of its own: it renders as a view of the Clientes
/// destination. `/sales/portfolio` used to wrap it in a Scaffold outside the
/// shell; that address is now a redirect to the view.
class PortfolioBody extends ConsumerStatefulWidget {
  const PortfolioBody({super.key});

  @override
  ConsumerState<PortfolioBody> createState() => _PortfolioBodyState();
}

class _PortfolioBodyState extends ConsumerState<PortfolioBody> {
  final _searchController = TextEditingController();
  String _query = '';

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final state = ref.watch(portfolioProvider);
    // The import is admin-only on the server (`/contacts/import`); showing
    // the door to an attendant teaches her a permission by 403.
    final canImport =
        roleIsAdmin(ref.watch(authControllerProvider).asData?.value?.role);

    return state.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (err, _) => RetryableError(
        error: err,
        onRetry: () => ref.invalidate(portfolioProvider),
      ),
      data: (customers) {
        // Client-side over the page the server sent: the list is small
        // enough per account that a round trip per keystroke buys nothing.
        final q = _query.trim().toLowerCase();
        final visible = q.isEmpty
            ? customers
            : customers
                .where((c) => c.contactName.toLowerCase().contains(q))
                .toList();
        return RefreshIndicator(
          onRefresh: () async => ref.invalidate(portfolioProvider),
          child: ListView(
            padding: const EdgeInsets.fromLTRB(16, 16, 16, 32),
            children: [
              Text(t.portfolio.subtitle,
                  style: typography.body.copyWith(
                      color: colors.textSecondary, fontSize: 13, height: 1.4)),
              const SizedBox(height: 4),
              // A Wrap, not a Row: three doors no longer fit beside the
              // subtitle on a phone, and an overflowed Row hides the last one.
              Wrap(
                spacing: 4,
                children: [
                  if (canImport) ...[
                    // The way a base gets filled. Without it the only answer
                    // to an empty list is "wait for a sale", which for a
                    // distributor with two hundred customers in a
                    // spreadsheet is not an answer.
                    TextButton.icon(
                      key: const Key('portfolio_import'),
                      onPressed: () => context.go('/contacts/import'),
                      icon: const Icon(Icons.upload_file, size: 16),
                      label: Text(t.customer_import.open),
                    ),
                    // The other half of the base: not who they are but what
                    // they bought. The reorder dates and the cross-sell read
                    // only `sales`, so without this door they run on air.
                    TextButton.icon(
                      key: const Key('portfolio_import_sales'),
                      onPressed: () => context.go('/sales/import'),
                      icon: const Icon(Icons.receipt_long_outlined, size: 16),
                      label: Text(t.sales_import.open),
                    ),
                  ],
                  // The other half of keeping a customer: the dates worth
                  // remembering. It belongs next to the carteira because both
                  // answer "who should I call, and why".
                  TextButton.icon(
                    key: const Key('portfolio_relationship_link'),
                    onPressed: () => context.go('/sales/relationship'),
                    icon: const Icon(Icons.cake_outlined, size: 16),
                    label: Text(t.relationship.title),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              if (customers.isNotEmpty) ...[
                Container(
                  height: 40,
                  decoration: BoxDecoration(
                    color: colors.glassLight,
                    borderRadius: BorderRadius.circular(10),
                    border: Border.all(color: colors.borderMedium),
                  ),
                  child: TextField(
                    key: const Key('portfolio_search'),
                    controller: _searchController,
                    onChanged: (v) => setState(() => _query = v),
                    style: TextStyle(fontSize: 14, color: colors.textPrimary),
                    decoration: InputDecoration(
                      hintText: t.common.search,
                      hintStyle:
                          TextStyle(color: colors.textMuted, fontSize: 14),
                      border: InputBorder.none,
                      contentPadding: const EdgeInsets.symmetric(
                          horizontal: 12, vertical: 8),
                      isDense: true,
                      prefixIcon:
                          Icon(Icons.search, size: 18, color: colors.textMuted),
                    ),
                  ),
                ),
                const SizedBox(height: 12),
              ],
              if (customers.isEmpty)
                EmptyStateCard(
                  key: const Key('portfolio_empty'),
                  icon: Icons.people_outline,
                  title: t.portfolio.empty,
                  subtitle: t.portfolio.empty_hint,
                )
              else if (visible.isEmpty)
                Padding(
                  padding: const EdgeInsets.only(top: 24),
                  child: Text(
                    t.common.no_results,
                    key: const Key('portfolio_search_empty'),
                    textAlign: TextAlign.center,
                    style: TextStyle(color: colors.textMuted, fontSize: 13),
                  ),
                )
              else
                for (final customer in visible)
                  _PortfolioCard(
                    key: Key('portfolio_${customer.contactId}'),
                    customer: customer,
                  ),
            ],
          ),
        );
      },
    );
  }
}

class _PortfolioCard extends ConsumerWidget {
  const _PortfolioCard({super.key, required this.customer});

  final PortfolioCustomer customer;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: GlassCard(
        child: InkWell(
          onTap: () => context.go('/sales/portfolio/${customer.contactId}'),
          borderRadius: BorderRadius.circular(16),
          child: Padding(
            padding: const EdgeInsets.all(14),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        customer.contactName,
                        style: const TextStyle(
                            color: Colors.white,
                            fontSize: 14,
                            fontWeight: FontWeight.w600),
                      ),
                    ),
                    if (_reorderChip(customer) case final chip?)
                      Container(
                        key: Key('portfolio_due_${customer.contactId}'),
                        padding: const EdgeInsets.symmetric(
                            horizontal: 8, vertical: 2),
                        decoration: BoxDecoration(
                          color: (customer.overdue
                                  ? colors.redAlert
                                  : colors.orangeAlert)
                              .withValues(alpha: 0.15),
                          borderRadius: BorderRadius.circular(6),
                        ),
                        child: Text(
                          chip,
                          style: TextStyle(
                            color: customer.overdue
                                ? colors.redAlert
                                : colors.orangeAlert,
                            fontSize: 11,
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                      ),
                  ],
                ),
                const SizedBox(height: 6),
                Row(
                  children: [
                    Text(
                      t.portfolio.purchases(count: customer.purchases),
                      style: TextStyle(color: colors.textMuted, fontSize: 11),
                    ),
                    const SizedBox(width: 10),
                    // Net, not gross: what a customer is worth is what the
                    // company kept, not what they were invoiced.
                    Text(
                      formatAccountMoney(ref, customer.netCents),
                      style: TextStyle(color: colors.textMuted, fontSize: 11),
                    ),
                  ],
                ),
                // Silence is its own risk, independent of any reorder date.
                if (customer.stale)
                  Padding(
                    padding: const EdgeInsets.only(top: 4),
                    child: Text(
                      t.portfolio
                          .silent_for(days: customer.daysSinceContact ?? 0),
                      key: Key('portfolio_silent_${customer.contactId}'),
                      style: TextStyle(color: colors.orangeAlert, fontSize: 11),
                    ),
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  /// The countdown, and only when there is one.
  ///
  /// A customer with no predicted date gets no chip at all rather than an
  /// empty or guessed one — the absence is the honest signal that nobody
  /// recorded a cycle for what they bought.
  String? _reorderChip(PortfolioCustomer c) {
    final days = c.daysUntilDue;
    if (days == null) return null;
    if (days < 0) return t.portfolio.ran_out(days: -days);
    if (days == 0) return t.portfolio.runs_out_today;
    if (days <= 7) return t.portfolio.runs_out_in(days: days);
    return null;
  }
}
