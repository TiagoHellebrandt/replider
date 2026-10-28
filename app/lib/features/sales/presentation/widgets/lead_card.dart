import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/domain/models/board_model.dart';
import 'package:app/i18n/strings.g.dart';

/// One deal on the board (US-M017/M018, redesigned with the Clientes canvas).
///
/// Who · where · one line of state. The old card shouted three redundant
/// warnings ("Parado há 29 dia(s)" + "Esfriando" + "Sem dono") and hid the
/// two facts a seller acts on — a draft waiting in Hoje and declared buy
/// intent. Now the border color IS the priority (lime buy-intent > blue
/// draft > orange stalled), the state is one dotted line, and the draft chip
/// is a DOOR to the approval queue.
class LeadCardTile extends StatelessWidget {
  const LeadCardTile({
    super.key,
    required this.card,
    this.onTap,
    this.onMove,
    this.showOwnership = true,
  });

  final BoardCard card;
  final VoidCallback? onTap;

  /// Whether ownership is worth showing at all — see the chip below.
  /// Defaults true so a caller that has no board-wide view keeps the old
  /// behaviour.
  final bool showOwnership;

  /// Mobile's move affordance. Null on desktop, where dragging is the
  /// gesture — offering both would leave two code paths to keep honest.
  final VoidCallback? onMove;

  Color _border(AppColorsExtension colors) {
    if (card.buyIntent) return colors.limeGlow.withValues(alpha: 0.45);
    if (card.hasDraft) return colors.bluePrimary.withValues(alpha: 0.4);
    final days = card.daysIdle;
    if (days != null && days > BoardFilter.stalledAfterDays) {
      return colors.orangeAlert.withValues(alpha: 0.35);
    }
    return colors.borderMedium;
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final days = card.daysIdle;
    final stalled = days != null && days > BoardFilter.stalledAfterDays;

    return Card(
      key: Key('lead_card_${card.leadId}'),
      margin: const EdgeInsets.only(bottom: 8),
      color: colors.backgroundSecondary,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: BorderSide(color: _border(colors)),
      ),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(12),
        child: Padding(
          padding: const EdgeInsets.fromLTRB(14, 12, 14, 10),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Expanded(
                    child: Text(
                      card.title,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: typography.body.copyWith(
                        color: Colors.white,
                        fontSize: 13.5,
                        fontWeight: FontWeight.w600,
                        height: 1.3,
                      ),
                    ),
                  ),
                  if (onMove != null)
                    IconButton(
                      key: Key('lead_card_move_${card.leadId}'),
                      onPressed: onMove,
                      // 44px minimum touch target: this is the ONLY way to
                      // move a card on a phone.
                      constraints:
                          const BoxConstraints(minWidth: 44, minHeight: 44),
                      padding: EdgeInsets.zero,
                      tooltip: t.board.move_to,
                      icon: Icon(Icons.drive_file_move_outline,
                          size: 18, color: colors.textSecondary),
                    ),
                ],
              ),
              if (card.city != null) ...[
                const SizedBox(height: 2),
                Text(
                  card.city!,
                  key: Key('lead_card_city_${card.leadId}'),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: typography.body
                      .copyWith(color: colors.textMuted, fontSize: 11.5),
                ),
              ],
              // The facts a seller ACTS on, as chips — each one earns its
              // place or does not render.
              if (card.status == 'responded' ||
                  card.hasDraft ||
                  card.buyIntent ||
                  card.qualified) ...[
                const SizedBox(height: 7),
                Wrap(
                  spacing: 6,
                  runSpacing: 6,
                  children: [
                    if (card.status == 'responded')
                      _Chip(
                          label: t.board.tag_responded,
                          color: colors.bluePrimary),
                    if (card.hasDraft)
                      // A door, not a label: the draft is decided in Hoje.
                      GestureDetector(
                        key: Key('lead_card_draft_${card.leadId}'),
                        onTap: () => context.go('/prospect?tab=queue'),
                        child: _Chip(
                            label: t.board.tag_draft_ready,
                            color: colors.bluePrimary,
                            icon: Icons.send),
                      ),
                    if (card.buyIntent)
                      _Chip(
                          label: t.board.tag_buy_intent,
                          color: colors.limeGlow),
                    if (card.qualified)
                      _Chip(
                          label: t.board.tag_qualified, color: colors.cyanGlow),
                  ],
                ),
              ],
              const SizedBox(height: 7),
              // ONE line of state: how long still, and whose. "Esfriando"
              // died — it was the same fact as the day count, said twice.
              Row(
                children: [
                  Container(
                    width: 6,
                    height: 6,
                    decoration: BoxDecoration(
                      shape: BoxShape.circle,
                      color: stalled ? colors.orangeAlert : colors.borderHeavy,
                    ),
                  ),
                  const SizedBox(width: 6),
                  Flexible(
                    child: Text(
                      _idleLabel(card),
                      key: Key('lead_card_idle_${card.leadId}'),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: typography.body.copyWith(
                        color: stalled ? colors.orangeAlert : colors.textMuted,
                        fontSize: 11.5,
                      ),
                    ),
                  ),
                  const Spacer(),
                  // "Sem dono" on every card of a board where nobody owns
                  // anything is sixty-nine repetitions of a fact that names
                  // no action. A single-seat account never assigns, so the
                  // chip was pure noise there; the moment a team starts
                  // assigning, the ones still unowned are worth pointing at.
                  //
                  // Derived from the BOARD rather than from the team size: no
                  // extra request, and it turns itself on exactly when
                  // ownership starts to mean something.
                  if (showOwnership &&
                      (card.ownerUserId == null || card.ownerUserId!.isEmpty))
                    Text(
                      t.board.unassigned,
                      key: Key('lead_card_unowned_${card.leadId}'),
                      style: typography.body
                          .copyWith(color: colors.textMuted, fontSize: 11),
                    )
                  else
                    Container(
                      key: Key('lead_card_owner_${card.leadId}'),
                      width: 20,
                      height: 20,
                      decoration: BoxDecoration(
                        shape: BoxShape.circle,
                        gradient: Theme.of(context)
                            .extension<AppGradientsExtension>()!
                            .primaryButton,
                      ),
                      child: const Icon(Icons.person,
                          size: 12, color: Colors.white),
                    ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Age in words. An unknown timestamp says so — rendering it as "parado há 0
/// dias" would claim the lead was touched today, which is the opposite of
/// what a missing value means.
String _idleLabel(BoardCard card) {
  final days = card.daysIdle;
  if (days == null) return t.board.idle_unknown;
  if (days == 0) return t.board.idle_today;
  return t.board.idle_days(count: days);
}

class _Chip extends StatelessWidget {
  const _Chip({required this.label, required this.color, this.icon});

  final String label;
  final Color color;
  final IconData? icon;

  @override
  Widget build(BuildContext context) {
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 3),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.13),
        borderRadius: BorderRadius.circular(999),
        border: Border.all(color: color.withValues(alpha: 0.4)),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (icon != null) ...[
            Icon(icon, size: 10, color: color),
            const SizedBox(width: 4),
          ],
          Text(
            label,
            style: typography.body.copyWith(color: color, fontSize: 11),
          ),
        ],
      ),
    );
  }
}
