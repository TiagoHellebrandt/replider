import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/design_system/components/save_feedback.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/billing/presentation/controllers/billing_controller.dart';
import 'package:app/features/billing/presentation/widgets/upgrade_plan_modal.dart';
import 'package:app/i18n/strings.g.dart';

/// Persistent billing status indicator for the app chrome. Clicking it
/// opens the upgrade modal for trial users, or the Stripe Customer
/// Portal for paid users. Renders nothing when the auth state isn't
/// ready yet so no ghost pill shows during cold-start.
///
/// Visual language mirrors the design-system palette so the pill fits
/// alongside [UserAccountChip] without extra theming.
class BillingPill extends ConsumerStatefulWidget {
  const BillingPill({super.key});

  @override
  ConsumerState<BillingPill> createState() => _BillingPillState();
}

class _BillingPillState extends ConsumerState<BillingPill> {
  bool _openingPortal = false;

  Future<void> _onTap({required bool trialing}) async {
    if (trialing) {
      await showUpgradePlanModal(context);
      return;
    }
    // Paid user — open Stripe Customer Portal so they can update
    // payment method, download invoices, or cancel. No intermediate
    // app screen because the portal is the authoritative surface.
    setState(() => _openingPortal = true);
    final err =
        await ref.read(billingActionsProvider.notifier).openBillingPortal();
    if (!mounted) return;
    setState(() => _openingPortal = false);
    if (err != null) {
      SaveFeedback.showError(context, err);
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final user = ref.watch(authControllerProvider).asData?.value;
    if (user == null) return const SizedBox.shrink();

    // A contract / trial-bypass account (the sales-led invite path) was
    // SOLD its plan in a conversation — a "Trial: X dias" pill at the top
    // of onboarding contradicts the invite banner that just said "sua
    // conta já nasce no plano contratado". No pill at all for them: the
    // plan is a fact of the contract, not a billing state to manage here.
    if (user.trialBypass || user.isContractPlan) {
      return const SizedBox.shrink();
    }

    final trialing = user.isTrialing;
    final appearance = _appearanceFor(
        trialing, user.trialDaysRemaining, user.planName, colors);

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 4),
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: _openingPortal ? null : () => _onTap(trialing: trialing),
          borderRadius: BorderRadius.circular(16),
          child: Container(
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
            decoration: BoxDecoration(
              color: appearance.background,
              borderRadius: BorderRadius.circular(16),
              border: Border.all(color: appearance.border),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                if (_openingPortal)
                  SizedBox(
                    width: 12,
                    height: 12,
                    child: CircularProgressIndicator(
                        strokeWidth: 2, color: appearance.foreground),
                  )
                else
                  Icon(appearance.icon, size: 14, color: appearance.foreground),
                const SizedBox(width: 6),
                Text(
                  appearance.label,
                  style: TextStyle(
                    color: appearance.foreground,
                    fontSize: 12,
                    fontWeight: FontWeight.w700,
                    letterSpacing: 0.2,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  _PillAppearance _appearanceFor(
    bool trialing,
    int? trialDaysRemaining,
    String planName,
    AppColorsExtension colors,
  ) {
    if (trialing) {
      final days = trialDaysRemaining ?? 0;
      if (days <= 0) {
        // Trial already expired but account not upgraded yet. Red —
        // urgent action required. Shouldn't stay in this state long:
        // the server blocks AI replies once trial expires.
        return _PillAppearance(
          label: t.billing.billing_pill_trial_expired,
          icon: Icons.error_outline,
          foreground: colors.redAlert,
          background: colors.redAlert.withValues(alpha: 0.12),
          border: colors.redAlert.withValues(alpha: 0.35),
        );
      }
      if (days <= 1) {
        return _PillAppearance(
          label: t.billing.billing_pill_trial_last_day,
          icon: Icons.warning_amber_rounded,
          foreground: colors.orangeAlert,
          background: colors.orangeAlert.withValues(alpha: 0.12),
          border: colors.orangeAlert.withValues(alpha: 0.35),
        );
      }
      if (days <= 3) {
        // Amber earlier than the last day so the user has time to act
        // without the scramble of a same-day expiry.
        return _PillAppearance(
          label: t.billing.billing_pill_trial_days(days: days.toString()),
          icon: Icons.schedule,
          foreground: colors.orangeAlert,
          background: colors.orangeAlert.withValues(alpha: 0.12),
          border: colors.orangeAlert.withValues(alpha: 0.35),
        );
      }
      // Healthy trial window — use the neutral "info" palette.
      return _PillAppearance(
        label: t.billing.billing_pill_trial_days(days: days.toString()),
        icon: Icons.rocket_launch_outlined,
        foreground: colors.bluePrimary,
        background: colors.bluePrimary.withValues(alpha: 0.12),
        border: colors.bluePrimary.withValues(alpha: 0.35),
      );
    }

    // Paid plan: compact "⚡ Pro" chip. No warning colors — this is
    // steady-state. Clicking opens the Stripe portal.
    return _PillAppearance(
      label: planName,
      icon: Icons.bolt_outlined,
      foreground: colors.textPrimary,
      background: colors.backgroundSecondary,
      border: colors.borderLight,
    );
  }
}

class _PillAppearance {
  final String label;
  final IconData icon;
  final Color foreground;
  final Color background;
  final Color border;
  const _PillAppearance({
    required this.label,
    required this.icon,
    required this.foreground,
    required this.background,
    required this.border,
  });
}
