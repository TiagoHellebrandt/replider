import 'package:flutter/material.dart';

import 'package:app/core/ui/one_shot_prompt.dart';

import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/billing/presentation/widgets/talk_to_us_card.dart';
import 'package:app/i18n/strings.g.dart';

/// Blocking modal shown when an account hits its AI-token cycle limit.
/// The bot stops replying automatically at 100%, so silence from the
/// agent is the *actual* state the user is experiencing.
///
/// Sales-led: there is ONE way to raise a limit — the conversation. This
/// wall used to sell a self-service upgrade and a Stripe add-on pack right
/// next to the "no price list, talk to us" card, two promises that cannot
/// both be true. Now it states the fact, offers the conversation, and
/// names the reset date for whoever prefers to wait.
class _QuotaWallModal extends StatelessWidget {
  const _QuotaWallModal();

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    // Guess the reset day from today's month length. Good enough for a
    // hint string — the exact billing-cycle reset lives server-side
    // and isn't exposed in /me yet.
    final now = DateTime.now();
    final resetDay = DateTime(now.year, now.month + 1, 1).day;

    return Dialog(
      backgroundColor: colors.backgroundPrimary,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(18)),
      insetPadding: const EdgeInsets.all(24),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 460),
        child: Padding(
          padding: const EdgeInsets.fromLTRB(24, 28, 24, 20),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Row(
                children: [
                  Container(
                    padding: const EdgeInsets.all(10),
                    decoration: BoxDecoration(
                      color: colors.redAlert.withValues(alpha: 0.12),
                      borderRadius: BorderRadius.circular(10),
                    ),
                    child: Icon(Icons.bolt_outlined,
                        color: colors.redAlert, size: 22),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Text(
                      t.billing.quota_wall_title,
                      style: TextStyle(
                        color: colors.textPrimary,
                        fontSize: 18,
                        fontWeight: FontWeight.w800,
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 14),
              Text(
                t.billing.quota_wall_body,
                style: TextStyle(
                    color: colors.textSecondary, fontSize: 14, height: 1.45),
              ),
              const SizedBox(height: 20),
              const TalkToUsCard(compact: true),
              const SizedBox(height: 10),
              TextButton(
                onPressed: () => Navigator.of(context).pop(),
                child: Text(
                  t.billing.quota_wall_cta_later,
                  style: TextStyle(
                      color: colors.textMuted,
                      fontSize: 13,
                      fontWeight: FontWeight.w600),
                ),
              ),
              const SizedBox(height: 4),
              Text(
                t.billing.quota_wall_reset_hint(day: resetDay.toString()),
                textAlign: TextAlign.center,
                style: TextStyle(
                    color: colors.textDisabled, fontSize: 11, height: 1.3),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Opens the quota-wall modal. Caller is responsible for deciding
/// *when* to show it (e.g. on first dashboard paint after usage>=100%).
/// The modal itself is unconditional — if you show it, the user is
/// expected to pick one of the three CTAs.
Future<void> showQuotaWallModal(BuildContext context) {
  return showDialog<void>(
    context: context,
    barrierDismissible: false,
    builder: (_) => const _QuotaWallModal(),
  );
}

/// Session-scoped dedupe for the wall. Kept as a top-level mutable bool
/// (not a provider) because Riverpod 3 codegen in this app discourages
/// ad-hoc StateProviders, and the semantics are dead-simple: "did we
/// show the wall this app launch yet?". Resets on restart.
bool _quotaWallShownThisSession = false;

/// Test-only reset hook so widget tests can force a clean session
/// between cases. Not exposed in the public API docs.
@visibleForTesting
void resetQuotaWallShownForTest() {
  _quotaWallShownThisSession = false;
}

/// Fire-and-forget helper: shows the wall iff (a) the user has hit
/// 100% of their AI-token allowance and (b) we haven't already shown
/// it in this session. Callers invoke this from a `ref.listen` so the
/// wall appears on both cold-start (when the initial /usage response
/// comes back saturated) and mid-session transitions (when the
/// WebSocket usage_alert bumps us over).
void maybeShowQuotaWall(
  BuildContext context, {
  required double tokensUsagePercent,
}) {
  if (tokensUsagePercent < 100) return;
  if (_quotaWallShownThisSession) return;
  _quotaWallShownThisSession = true;
  // Defer to the next frame so this is safe to call from inside a
  // ref.listen (which runs during build).
  WidgetsBinding.instance.addPostFrameCallback((_) {
    if (!context.mounted) return;
    showAsOneShotPrompt(
      () => showQuotaWallModal(context),
      onDeclined: () => _quotaWallShownThisSession = false,
    );
  });
}
