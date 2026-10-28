import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/design_system/components/glass_card.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/presentation/controllers/cross_sell_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// What else to offer this customer (§6.5, F5).
///
/// Every suggestion carries its reason. A salesperson who can say "os
/// customers who buy gloves from us tend to take masks" is making an
/// argument; one reading an unexplained product name is guessing out loud,
/// and the customer can hear the difference.
class CrossSellCard extends ConsumerWidget {
  const CrossSellCard({super.key, required this.contactId});

  final String contactId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final state = ref.watch(crossSellProvider(contactId));

    // Silent while loading and silent on error. This is a helpful extra next
    // to the purchase history, not the reason the screen exists — a spinner
    // or a red box here would make a working screen look broken.
    final result = state.asData?.value;
    if (result == null) return const SizedBox.shrink();

    return Padding(
      padding: const EdgeInsets.only(top: 20),
      child: GlassCard(
        child: Padding(
          padding: const EdgeInsets.all(14),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Icon(Icons.auto_awesome_outlined,
                      size: 16, color: colors.textSecondary),
                  const SizedBox(width: 6),
                  Text(
                    t.cross_sell.title,
                    style: TextStyle(
                        color: colors.textPrimary,
                        fontSize: 13,
                        fontWeight: FontWeight.w700),
                  ),
                ],
              ),
              const SizedBox(height: 10),
              if (result.suggestions.isEmpty)
                Text(
                  t.cross_sell.empty,
                  key: const Key('cross_sell_empty'),
                  style: TextStyle(color: colors.textSecondary, fontSize: 12),
                )
              else
                for (final s in result.suggestions)
                  Padding(
                    padding: const EdgeInsets.only(bottom: 10),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          s.suggest,
                          key: Key('cross_sell_${s.suggest}'),
                          style: const TextStyle(
                              color: Colors.white,
                              fontSize: 14,
                              fontWeight: FontWeight.w600),
                        ),
                        const SizedBox(height: 2),
                        // The evidence, in customers rather than a percentage:
                        // "8 customers who buy gloves also take" is a
                        // sentence somebody can repeat on a call. "80% de
                        // confiança" is not.
                        Text(
                          t.cross_sell
                              .evidence(count: s.customers, item: s.because),
                          style: TextStyle(
                              color: colors.textSecondary, fontSize: 12),
                        ),
                      ],
                    ),
                  ),
              if (result.suggestions.isNotEmpty) ...[
                const SizedBox(height: 2),
                // Says plainly where this came from. A distributor asked to
                // trust a suggestion deserves to know it was not learned from
                // a competitor's sales.
                Text(
                  t.cross_sell.learned_here,
                  style: TextStyle(color: colors.textSecondary, fontSize: 11),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}
