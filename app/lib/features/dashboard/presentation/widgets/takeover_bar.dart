import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/dashboard/presentation/controllers/dashboard_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Inline takeover controls shown above the composer in the
/// conversation detail. Exposes the explicit "Assumir conversa" /
/// "Devolver ao bot" actions and the "Em atendimento por X" badge.
///
/// Kept as its own widget (instead of a private method on the panel)
/// so it can be widget-tested in isolation without stubbing every
/// provider the full detail panel pulls in.
class TakeoverBar extends ConsumerWidget {
  final String conversationId;
  final bool isBotPaused;
  final bool assignedToMe;
  final bool assignedToOther;
  final String? assignedToName;
  final bool isSending;

  const TakeoverBar({
    super.key,
    required this.conversationId,
    required this.isBotPaused,
    required this.assignedToMe,
    required this.assignedToOther,
    required this.assignedToName,
    required this.isSending,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    final bgColor = isBotPaused
        ? colors.orangeAlert.withValues(alpha: 0.10)
        : colors.glassLight;
    final borderColor = isBotPaused
        ? colors.orangeAlert.withValues(alpha: 0.30)
        : colors.borderLight;

    String primaryLabel;
    IconData primaryIcon;
    VoidCallback? primaryAction;

    if (assignedToMe) {
      primaryLabel = t.conversation.release;
      primaryIcon = Icons.smart_toy_outlined;
      primaryAction = isSending
          ? null
          : () => ref
              .read(conversationActionsProvider.notifier)
              .releaseConversation(conversationId);
    } else {
      primaryLabel = t.conversation.assume;
      primaryIcon = Icons.support_agent;
      primaryAction = isSending
          ? null
          : () => ref
              .read(conversationActionsProvider.notifier)
              .assumeConversation(conversationId);
    }

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      decoration: BoxDecoration(
        color: bgColor,
        border: Border(top: BorderSide(color: borderColor, width: 0.5)),
      ),
      child: Row(
        children: [
          if (isBotPaused) ...[
            Icon(Icons.pause_circle_filled,
                size: 14, color: colors.orangeAlert),
            const SizedBox(width: 6),
            Expanded(
              child: Text(
                assignedToName != null
                    ? t.conversation.under_takeover_by(name: assignedToName!)
                    : t.conversation.under_takeover_anonymous,
                style: TextStyle(
                  color: colors.orangeAlert,
                  fontSize: 12,
                  fontWeight: FontWeight.w600,
                ),
                overflow: TextOverflow.ellipsis,
              ),
            ),
          ] else
            Expanded(
              child: Text(
                t.conversation.input_disabled_tooltip,
                style: TextStyle(color: colors.textMuted, fontSize: 12),
                overflow: TextOverflow.ellipsis,
              ),
            ),
          TextButton.icon(
            icon: Icon(primaryIcon, size: 16),
            label: Text(primaryLabel,
                style:
                    const TextStyle(fontSize: 12, fontWeight: FontWeight.w700)),
            onPressed: primaryAction,
            style: TextButton.styleFrom(
              foregroundColor:
                  assignedToMe ? colors.textSecondary : colors.bluePrimary,
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
              minimumSize: const Size(0, 28),
            ),
          ),
        ],
      ),
    );
  }
}
