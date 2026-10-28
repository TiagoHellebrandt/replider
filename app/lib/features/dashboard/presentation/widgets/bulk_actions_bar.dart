import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/dashboard/presentation/controllers/dashboard_controller.dart';
import 'package:app/core/error/api_error_parser.dart';
import 'package:app/i18n/strings.g.dart';

/// Top-of-list toolbar that materialises only when the operator is in
/// multiselect mode (`conversationSelectionProvider` non-empty). Shows
/// the row count + bulk actions (close, select-all, clear).
///
/// Assign-many is intentionally NOT wired here yet: it needs a team
/// picker, which is a meaningful UI of its own and not in scope for
/// the inline bar. Close is the most-requested batch action by far,
/// so it's the only one we expose at the bar level.
class BulkActionsBar extends ConsumerWidget {
  /// All conversation IDs currently visible in the list. Used by the
  /// "Select all" button so the operator can grab everything
  /// matching the active filter (open, pending_human, etc.) in one
  /// tap.
  final List<String> conversationIds;

  const BulkActionsBar({super.key, required this.conversationIds});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final selection = ref.watch(conversationSelectionProvider);
    if (selection.isEmpty) {
      return const SizedBox.shrink();
    }
    final allSelected = conversationIds.isNotEmpty &&
        selection.length == conversationIds.length;
    final actions = ref.watch(conversationActionsProvider);
    final busy = actions.isLoading;
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: colors.bluePrimary.withValues(alpha: 0.12),
        border: Border(
          bottom: BorderSide(color: colors.borderLight, width: 0.5),
        ),
      ),
      child: Row(
        children: [
          IconButton(
            tooltip: t.bulk_actions.clear_tooltip,
            icon: Icon(Icons.close, size: 18, color: colors.textPrimary),
            onPressed: busy
                ? null
                : () =>
                    ref.read(conversationSelectionProvider.notifier).clear(),
            splashRadius: 18,
            padding: EdgeInsets.zero,
            constraints: const BoxConstraints(minWidth: 36, minHeight: 36),
          ),
          Text(
            t.bulk_actions.selected_count(count: selection.length.toString()),
            style: TextStyle(
              color: colors.textPrimary,
              fontSize: 13,
              fontWeight: FontWeight.w600,
            ),
          ),
          const Spacer(),
          if (!allSelected)
            TextButton(
              onPressed: busy
                  ? null
                  : () => ref
                      .read(conversationSelectionProvider.notifier)
                      .selectAll(conversationIds),
              child: Text(
                t.bulk_actions.select_all,
                style: TextStyle(color: colors.bluePrimary, fontSize: 12),
              ),
            ),
          OutlinedButton.icon(
            onPressed: busy ? null : () => _confirmAndClose(context, ref),
            icon: busy
                ? SizedBox(
                    width: 12,
                    height: 12,
                    child: CircularProgressIndicator(
                      strokeWidth: 2,
                      color: colors.redAlert,
                    ),
                  )
                : Icon(Icons.check_circle_outline,
                    size: 14, color: colors.redAlert),
            label: Text(
              t.bulk_actions.close_btn,
              style: TextStyle(color: colors.redAlert, fontSize: 12),
            ),
            style: OutlinedButton.styleFrom(
              side: BorderSide(color: colors.redAlert.withValues(alpha: 0.5)),
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
              shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(8)),
            ),
          ),
        ],
      ),
    );
  }

  Future<void> _confirmAndClose(BuildContext context, WidgetRef ref) async {
    final selection = ref.read(conversationSelectionProvider);
    if (selection.isEmpty) return;
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: colors.backgroundSecondary,
        title: Text(
            t.bulk_actions
                .close_confirm_title(count: selection.length.toString()),
            style: TextStyle(color: colors.textPrimary)),
        content: Text(
          t.bulk_actions.close_confirm_body,
          style: TextStyle(color: colors.textSecondary),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: Text(t.common.cancel),
          ),
          ElevatedButton(
            onPressed: () => Navigator.pop(ctx, true),
            style: ElevatedButton.styleFrom(
                backgroundColor: colors.redAlert,
                foregroundColor: Colors.white),
            child: Text(t.bulk_actions.close_btn),
          ),
        ],
      ),
    );
    if (confirm != true || !context.mounted) return;
    try {
      final ok =
          await ref.read(conversationActionsProvider.notifier).bulkAction(
                action: 'resolve',
                conversationIds: selection.toList(),
              );
      if (!context.mounted) return;
      ref.read(conversationSelectionProvider.notifier).clear();
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(
        content: Text(t.bulk_actions.close_done(
                ok: ok.toString(), total: selection.length.toString()) +
            (ok < selection.length
                ? t.bulk_actions.close_partial_warning
                : '')),
        backgroundColor: ok == selection.length
            ? Colors.green.shade700
            : Colors.orange.shade700,
        behavior: SnackBarBehavior.floating,
      ));
    } catch (e) {
      if (!context.mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(
        content: Text(t.bulk_actions.close_failed(error: parseApiError(e))),
        backgroundColor: Colors.red.shade700,
      ));
    }
  }
}
