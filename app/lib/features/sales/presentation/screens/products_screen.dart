import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:app/core/utils/currency_format.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/domain/models/product_model.dart';
import 'package:app/features/sales/presentation/controllers/products_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Catalog product list for the outbound-sales module (US-008). Mirrors the
/// campaigns list screen: account-scoped cards, pull-to-refresh, FAB to
/// create, tap-to-edit, inline delete with confirm.
class ProductsScreen extends ConsumerWidget {
  const ProductsScreen({super.key});

  // The shared formatter, like every other money in the module: the list
  // used to print "BRL 1234.56" for the same product the proposal renders
  // as "R$ 1.234,56".
  String _formatPrice(ProductModel p) =>
      formatCurrencyCents(p.priceCents, p.currency);

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    // The empty state already carries its own button, larger and with an
    // explanation. With the floating button on, the screen showed TWO buttons for
    // one action, one on top of the other. The floating one is for whoever
    // already has a list.
    final async = ref.watch(productsListProvider);

    return Scaffold(
      backgroundColor: colors.backgroundPrimary,
      appBar: AppBar(
        backgroundColor: colors.backgroundSecondary,
        elevation: 0,
        leading: IconButton(
          tooltip: t.sales.back_tooltip,
          icon: Icon(Icons.arrow_back, color: colors.textSecondary),
          onPressed: () =>
              context.canPop() ? context.pop() : context.go('/start'),
        ),
        title: Text(
          t.sales.products.title,
          style: TextStyle(
            fontSize: 20,
            fontWeight: FontWeight.w700,
            color: colors.textPrimary,
          ),
        ),
        actions: [
          // Reachable from the catalogue itself, and from the empty state below:
          // somebody with four hundred SKUs in a spreadsheet meets this list
          // before they meet the one-product form, and the form is the reason
          // they would give up.
          IconButton(
            key: const Key('products_import'),
            tooltip: t.product_import.open,
            icon: Icon(Icons.upload_file, color: colors.textSecondary),
            onPressed: () => context.go('/sales/products/import'),
          ),
          IconButton(
            icon: Icon(Icons.refresh, color: colors.textSecondary),
            onPressed: () => ref.read(productsListProvider.notifier).refresh(),
          ),
        ],
      ),
      floatingActionButton: (async.asData?.value ?? const []).isEmpty
          ? null
          : FloatingActionButton.extended(
              onPressed: () => context.go('/sales/products/new'),
              backgroundColor: colors.bluePrimary,
              foregroundColor: Colors.white,
              icon: const Icon(Icons.add),
              label: Text(t.sales.products.new_product),
            ),
      body: async.when(
        data: (list) {
          if (list.isEmpty) return _empty(context, colors);
          return Center(
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 900),
              child: RefreshIndicator(
                onRefresh: () =>
                    ref.read(productsListProvider.notifier).refresh(),
                child: ListView.separated(
                  padding: const EdgeInsets.fromLTRB(16, 16, 16, 96),
                  itemCount: list.length,
                  separatorBuilder: (_, __) => const SizedBox(height: 10),
                  itemBuilder: (_, i) => _card(context, ref, colors, list[i]),
                ),
              ),
            ),
          );
        },
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, _) => Center(
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(Icons.error_outline, color: colors.redAlert, size: 48),
                const SizedBox(height: 12),
                Text(t.sales.products.load_error,
                    style: TextStyle(color: colors.textPrimary, fontSize: 16)),
                const SizedBox(height: 4),
                Text('$err',
                    style: TextStyle(color: colors.textMuted, fontSize: 12),
                    textAlign: TextAlign.center),
                const SizedBox(height: 16),
                TextButton.icon(
                  onPressed: () =>
                      ref.read(productsListProvider.notifier).refresh(),
                  icon: const Icon(Icons.refresh),
                  label: Text(t.common.retry),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _empty(BuildContext context, AppColorsExtension colors) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.inventory_2_outlined,
                size: 64, color: colors.textDisabled),
            const SizedBox(height: 16),
            Text(t.sales.products.no_products,
                style: TextStyle(
                    color: colors.textPrimary,
                    fontSize: 18,
                    fontWeight: FontWeight.w700)),
            const SizedBox(height: 8),
            Text(
              t.sales.products.empty_subtitle,
              textAlign: TextAlign.center,
              style:
                  TextStyle(color: colors.textMuted, fontSize: 13, height: 1.5),
            ),
            const SizedBox(height: 20),
            // The import FIRST: whoever arrives here with 400 SKUs in a
            // spreadsheet or a manufacturer's brochure should not meet a
            // one-product form as the only door — that is the moment they
            // decide the product is manual work.
            Wrap(
              spacing: 12,
              runSpacing: 12,
              alignment: WrapAlignment.center,
              children: [
                ElevatedButton.icon(
                  key: const Key('products_empty_import'),
                  onPressed: () => context.go('/sales/products/import'),
                  icon: const Icon(Icons.upload_file),
                  label: Text(t.sales.products.import_first),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: colors.bluePrimary,
                    foregroundColor: Colors.white,
                    padding: const EdgeInsets.symmetric(
                        horizontal: 24, vertical: 14),
                  ),
                ),
                OutlinedButton.icon(
                  onPressed: () => context.go('/sales/products/new'),
                  icon: const Icon(Icons.add),
                  label: Text(t.sales.products.create_first),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _card(BuildContext context, WidgetRef ref, AppColorsExtension colors,
      ProductModel p) {
    final statusColor = p.active ? colors.whatsappGreen : colors.textMuted;
    return Container(
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: colors.borderLight),
      ),
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          borderRadius: BorderRadius.circular(12),
          onTap: () => context.go('/sales/products/${p.id}'),
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        p.name,
                        style: TextStyle(
                          color: colors.textPrimary,
                          fontWeight: FontWeight.w700,
                          fontSize: 15,
                        ),
                      ),
                    ),
                    Container(
                      padding: const EdgeInsets.symmetric(
                          horizontal: 8, vertical: 3),
                      decoration: BoxDecoration(
                        color: statusColor.withValues(alpha: 0.15),
                        borderRadius: BorderRadius.circular(6),
                      ),
                      child: Text(
                        p.active
                            ? t.sales.products.active_label
                            : t.sales.products.inactive_label,
                        style: TextStyle(
                            color: statusColor,
                            fontSize: 11,
                            fontWeight: FontWeight.w700),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 4),
                Text(
                  _formatPrice(p),
                  style: TextStyle(
                    color: colors.bluePrimary,
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                if (p.description.isNotEmpty) ...[
                  const SizedBox(height: 6),
                  Text(
                    p.description,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(color: colors.textMuted, fontSize: 12),
                  ),
                ],
                const SizedBox(height: 8),
                Row(
                  mainAxisAlignment: MainAxisAlignment.end,
                  children: [
                    TextButton.icon(
                      onPressed: () => _confirmDelete(context, ref, p),
                      icon: Icon(Icons.delete_outline,
                          size: 16, color: colors.redAlert),
                      label: Text(t.common.delete,
                          style: TextStyle(color: colors.redAlert)),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  void _confirmDelete(BuildContext context, WidgetRef ref, ProductModel p) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: colors.backgroundSecondary,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
        title: Text(t.sales.products.delete_title,
            style: TextStyle(color: colors.textPrimary)),
        content: Text(t.sales.products.delete_message(name: p.name),
            style: TextStyle(color: colors.textSecondary, fontSize: 14)),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: Text(t.common.cancel,
                style: TextStyle(color: colors.textMuted)),
          ),
          ElevatedButton(
            onPressed: () async {
              Navigator.pop(ctx);
              final err =
                  await ref.read(productsListProvider.notifier).delete(p.id);
              if (context.mounted) {
                _toast(context, err ?? t.sales.products.deleted_success,
                    err == null);
              }
            },
            style: ElevatedButton.styleFrom(
              backgroundColor: colors.redAlert,
              foregroundColor: Colors.white,
            ),
            child: Text(t.common.delete),
          ),
        ],
      ),
    );
  }

  void _toast(BuildContext context, String msg, bool success) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      content: Text(msg),
      backgroundColor: success ? Colors.green.shade700 : Colors.red.shade700,
      behavior: SnackBarBehavior.floating,
    ));
  }
}
