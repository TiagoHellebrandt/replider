import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/domain/models/funnel_model.dart';
import 'package:app/features/sales/presentation/controllers/funnels_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Sales funnel list for the outbound-sales module (US-009). Mirrors the
/// products list: account-scoped cards, pull-to-refresh, FAB to create,
/// tap-to-configure (metadata schema + webhook), inline delete with confirm.
class FunnelsScreen extends ConsumerWidget {
  const FunnelsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    // The empty state already carries its own button, larger and with an
    // explanation. With the floating button on, the screen showed TWO buttons for
    // one action, one on top of the other. The floating one is for whoever
    // already has a list.
    final async = ref.watch(funnelsListProvider);

    return Scaffold(
      backgroundColor: colors.backgroundPrimary,
      appBar: AppBar(
        backgroundColor: colors.backgroundSecondary,
        elevation: 0,
        leading: IconButton(
          tooltip: t.sales.back_tooltip,
          icon: Icon(Icons.arrow_back, color: colors.textSecondary),
          onPressed: () =>
              context.canPop() ? context.pop() : context.go('/funnel'),
        ),
        title: Text(
          t.sales.funnels.title,
          style: TextStyle(
            fontSize: 20,
            fontWeight: FontWeight.w700,
            color: colors.textPrimary,
          ),
        ),
        actions: [
          IconButton(
            icon: Icon(Icons.refresh, color: colors.textSecondary),
            onPressed: () => ref.read(funnelsListProvider.notifier).refresh(),
          ),
        ],
      ),
      floatingActionButton: (async.asData?.value ?? const []).isEmpty
          ? null
          : FloatingActionButton.extended(
              onPressed: () => context.go('/sales/funnels/new'),
              backgroundColor: colors.bluePrimary,
              foregroundColor: Colors.white,
              icon: const Icon(Icons.add),
              label: Text(t.sales.funnels.new_funnel),
            ),
      body: async.when(
        data: (list) {
          if (list.isEmpty) return _empty(context, colors);
          return Center(
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 900),
              child: RefreshIndicator(
                onRefresh: () =>
                    ref.read(funnelsListProvider.notifier).refresh(),
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
                Text(t.sales.funnels.load_error,
                    style: TextStyle(color: colors.textPrimary, fontSize: 16)),
                const SizedBox(height: 4),
                Text(parseApiError(err),
                    style: TextStyle(color: colors.textMuted, fontSize: 12),
                    textAlign: TextAlign.center),
                const SizedBox(height: 16),
                TextButton.icon(
                  onPressed: () =>
                      ref.read(funnelsListProvider.notifier).refresh(),
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
            Icon(Icons.filter_alt_outlined,
                size: 64, color: colors.textDisabled),
            const SizedBox(height: 16),
            Text(t.sales.funnels.no_funnels,
                style: TextStyle(
                    color: colors.textPrimary,
                    fontSize: 18,
                    fontWeight: FontWeight.w700)),
            const SizedBox(height: 8),
            Text(
              t.sales.funnels.empty_subtitle,
              textAlign: TextAlign.center,
              style:
                  TextStyle(color: colors.textMuted, fontSize: 13, height: 1.5),
            ),
            const SizedBox(height: 20),
            ElevatedButton.icon(
              onPressed: () => context.go('/sales/funnels/new'),
              icon: const Icon(Icons.add),
              label: Text(t.sales.funnels.create_first),
              style: ElevatedButton.styleFrom(
                backgroundColor: colors.bluePrimary,
                foregroundColor: Colors.white,
                padding:
                    const EdgeInsets.symmetric(horizontal: 24, vertical: 14),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _card(BuildContext context, WidgetRef ref, AppColorsExtension colors,
      FunnelModel f) {
    final statusColor = f.active ? colors.whatsappGreen : colors.textMuted;
    final fieldCount = f.metadataSchema.length;
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
          onTap: () => context.go('/sales/funnels/${f.id}'),
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        f.name,
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
                        f.active
                            ? t.sales.funnels.active_label
                            : t.sales.funnels.inactive_label,
                        style: TextStyle(
                            color: statusColor,
                            fontSize: 11,
                            fontWeight: FontWeight.w700),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 6),
                Text(
                  t.sales.funnels.fields_count(n: fieldCount),
                  style: TextStyle(color: colors.textMuted, fontSize: 12),
                ),
                const SizedBox(height: 8),
                Row(
                  mainAxisAlignment: MainAxisAlignment.end,
                  children: [
                    // The default "Clientes" funnel cannot be deleted (the
                    // server refuses with 409). Offering the button and
                    // letting the refusal explain was a dead end in English.
                    if (f.isDefault)
                      Tooltip(
                        message: t.sales.funnels.default_undeletable,
                        child: Padding(
                          padding: const EdgeInsets.all(8),
                          child: Icon(Icons.lock_outline,
                              size: 16, color: colors.textMuted),
                        ),
                      )
                    else
                      TextButton.icon(
                        onPressed: () => _confirmDelete(context, ref, f),
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

  void _confirmDelete(BuildContext context, WidgetRef ref, FunnelModel f) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: colors.backgroundSecondary,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
        title: Text(t.sales.funnels.delete_title,
            style: TextStyle(color: colors.textPrimary)),
        content: Text(t.sales.funnels.delete_message(name: f.name),
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
                  await ref.read(funnelsListProvider.notifier).delete(f.id);
              if (context.mounted) {
                _toast(context, err ?? t.sales.funnels.deleted_success,
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
