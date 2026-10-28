import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/notifications/push_notification_controller.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/support/presentation/controllers/support_push_prompt.dart';
import 'package:app/i18n/strings.g.dart';

/// Contextual push opt-in banner shown at the top of the support widget on
/// first use (canal-interno US-006). Renders ONLY while all of these hold:
/// the prompt was never handled before ([SupportPushPromptDismissal]),
/// the platform can take a subscription, and the user isn't subscribed
/// yet — so a denied browser permission or an existing subscription both
/// hide it without any local bookkeeping.
class SupportPushPrompt extends ConsumerWidget {
  const SupportPushPrompt({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final dismissed =
        ref.watch(supportPushPromptDismissalProvider).value ?? true;
    if (dismissed) return const SizedBox.shrink();

    final push = ref.watch(pushNotificationControllerProvider).value;
    if (push == null || push.status != PushSubscriptionStatus.notSubscribed) {
      return const SizedBox.shrink();
    }

    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Container(
      width: double.infinity,
      margin: const EdgeInsets.fromLTRB(12, 10, 12, 0),
      padding: const EdgeInsets.fromLTRB(14, 12, 14, 8),
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: colors.borderLight),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.notifications_active_outlined,
                  size: 18, color: colors.bluePrimary),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  t.support.push_prompt_title,
                  style: TextStyle(
                    color: colors.textPrimary,
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 4),
          Text(
            t.support.push_prompt_body,
            style: TextStyle(color: colors.textMuted, fontSize: 12),
          ),
          const SizedBox(height: 4),
          Row(
            mainAxisAlignment: MainAxisAlignment.end,
            children: [
              TextButton(
                onPressed: () => ref
                    .read(supportPushPromptDismissalProvider.notifier)
                    .dismiss(),
                child: Text(
                  t.support.push_prompt_later,
                  style: TextStyle(color: colors.textMuted, fontSize: 12),
                ),
              ),
              const SizedBox(width: 4),
              FilledButton(
                onPressed: () async {
                  // Handled either way — enable() landing on "denied" must
                  // not re-arm the prompt on the next visit.
                  await ref
                      .read(pushNotificationControllerProvider.notifier)
                      .enable();
                  await ref
                      .read(supportPushPromptDismissalProvider.notifier)
                      .dismiss();
                },
                style: FilledButton.styleFrom(
                  backgroundColor: colors.bluePrimary,
                  padding: const EdgeInsets.symmetric(horizontal: 14),
                  minimumSize: const Size(0, 32),
                ),
                child: Text(
                  t.support.push_prompt_enable,
                  style: const TextStyle(fontSize: 12, color: Colors.white),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}
