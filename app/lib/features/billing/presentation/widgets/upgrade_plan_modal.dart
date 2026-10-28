import 'package:flutter/material.dart';

import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/billing/presentation/widgets/talk_to_us_card.dart';
import 'package:app/i18n/strings.g.dart';

/// The sheet every "upgrade" entry point opens — the trial banner, the
/// quota wall, the billing pill, the plan-limit error handler.
///
/// Until 2026-08-19 it listed the plans with a Subscribe button each. The
/// product is sales-led now: there is no price list to show, so the sheet
/// offers the conversation instead. The entry points and this function's
/// name are unchanged on purpose — nine call sites, one meaning ("I want
/// more"), one answer.
Future<void> showUpgradePlanModal(BuildContext context) {
  return showModalBottomSheet<void>(
    context: context,
    isScrollControlled: true,
    backgroundColor: Colors.transparent,
    builder: (_) => const _TalkToUsSheet(),
  );
}

class _TalkToUsSheet extends StatelessWidget {
  const _TalkToUsSheet();

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return SafeArea(
      child: Container(
        key: const Key('upgrade_plan_modal'),
        margin: const EdgeInsets.all(12),
        padding: const EdgeInsets.fromLTRB(20, 12, 20, 20),
        decoration: BoxDecoration(
          color: colors.backgroundSecondary,
          borderRadius: BorderRadius.circular(20),
          border: Border.all(color: colors.borderLight),
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Center(
              child: Container(
                width: 40,
                height: 4,
                margin: const EdgeInsets.only(bottom: 14),
                decoration: BoxDecoration(
                  color: colors.borderMedium,
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
            ),
            const TalkToUsCard(),
            const SizedBox(height: 8),
            TextButton(
              onPressed: () => Navigator.of(context).pop(),
              child: Text(t.common.close,
                  style: TextStyle(color: colors.textSecondary)),
            ),
          ],
        ),
      ),
    );
  }
}
