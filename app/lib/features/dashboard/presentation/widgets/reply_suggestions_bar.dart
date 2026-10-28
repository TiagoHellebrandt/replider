import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/dashboard/presentation/controllers/dashboard_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Strip of LLM-generated reply chips above the composer. Hidden by
/// default; tapping the spark icon fires the suggest endpoint and
/// inflates the chips. Tapping a chip fills the composer (does NOT
/// auto-send) so the operator can edit before shipping. Same UX as
/// Gmail Smart Compose / iMessage suggestions.
class ReplySuggestionsBar extends ConsumerWidget {
  final String conversationId;
  final void Function(String text) onPick;

  const ReplySuggestionsBar({
    super.key,
    required this.conversationId,
    required this.onPick,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final state = ref.watch(replySuggestionsProvider(conversationId));
    final notifier =
        ref.read(replySuggestionsProvider(conversationId).notifier);

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        border: Border(
          bottom: BorderSide(color: colors.borderLight, width: 0.5),
        ),
      ),
      child: state.when(
        loading: () => Row(
          children: [
            SizedBox(
              width: 14,
              height: 14,
              child: CircularProgressIndicator(
                  strokeWidth: 2, color: colors.bluePrimary),
            ),
            const SizedBox(width: 8),
            Text(
              t.reply_suggestions.loading,
              style: TextStyle(color: colors.textMuted, fontSize: 12),
            ),
          ],
        ),
        error: (e, _) => Row(
          children: [
            Icon(Icons.error_outline, size: 14, color: colors.redAlert),
            const SizedBox(width: 6),
            Expanded(
              child: Text(
                t.reply_suggestions.load_failed,
                style: TextStyle(color: colors.redAlert, fontSize: 11),
              ),
            ),
            TextButton(
              onPressed: notifier.refresh,
              child: Text(t.reply_suggestions.retry,
                  style: TextStyle(color: colors.bluePrimary, fontSize: 12)),
            ),
          ],
        ),
        data: (suggestions) {
          if (suggestions.isEmpty) {
            return Align(
              alignment: Alignment.centerLeft,
              child: TextButton.icon(
                onPressed: notifier.refresh,
                icon: Icon(Icons.auto_awesome,
                    size: 14, color: colors.bluePrimary),
                label: Text(
                  t.reply_suggestions.suggest_replies,
                  style: TextStyle(color: colors.bluePrimary, fontSize: 12),
                ),
                style: TextButton.styleFrom(
                  padding:
                      const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                  visualDensity: VisualDensity.compact,
                ),
              ),
            );
          }
          return SingleChildScrollView(
            scrollDirection: Axis.horizontal,
            child: Row(
              children: [
                Icon(Icons.auto_awesome, size: 12, color: colors.textMuted),
                const SizedBox(width: 6),
                ...suggestions.map((s) => Padding(
                      padding: const EdgeInsets.only(right: 6),
                      child: ActionChip(
                        label: Text(s,
                            style: TextStyle(
                                color: colors.textPrimary, fontSize: 12)),
                        backgroundColor: colors.glassMedium,
                        side: BorderSide(color: colors.borderLight),
                        onPressed: () {
                          onPick(s);
                          notifier.clear();
                        },
                      ),
                    )),
                IconButton(
                  tooltip: t.reply_suggestions.refresh_tooltip,
                  icon: Icon(Icons.refresh, size: 14, color: colors.textMuted),
                  onPressed: notifier.refresh,
                  splashRadius: 14,
                  padding: EdgeInsets.zero,
                  constraints:
                      const BoxConstraints(minWidth: 28, minHeight: 28),
                ),
                IconButton(
                  tooltip: t.reply_suggestions.hide_tooltip,
                  icon: Icon(Icons.close, size: 14, color: colors.textMuted),
                  onPressed: notifier.clear,
                  splashRadius: 14,
                  padding: EdgeInsets.zero,
                  constraints:
                      const BoxConstraints(minWidth: 28, minHeight: 28),
                ),
              ],
            ),
          );
        },
      ),
    );
  }
}
