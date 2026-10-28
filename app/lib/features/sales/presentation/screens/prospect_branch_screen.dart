import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:app/core/auth/account_role.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/sales/presentation/controllers/approval_queue_controller.dart';
import 'package:app/features/sales/presentation/screens/approval_queue_screen.dart';
import 'package:app/features/sales/presentation/widgets/contract_calendar_body.dart';
import 'package:app/features/sales/presentation/widgets/radar_body.dart';
import 'package:app/i18n/strings.g.dart';

/// The shell's "Ofertas" destination — one list, no tabs.
///
/// It used to be four: the approval queue, the radar, the buying calendar and
/// the hunt. Every one of them answered the same question ("who should I talk
/// to today?") from a different source, and a seller who has to remember WHICH
/// list holds the good rows ends up working one list and letting the others
/// rot. The queue of half-sent cold outreach that this screen was built to
/// prevent was being produced by the screen's own shape.
///
/// So the sources merged instead. A tender, an expiring contract, an open ata,
/// an untouched installed park and a customer about to run out are five REASONS
/// to call someone, not five screens: they arrive as rows of one feed, ordered
/// by whatever expires first, and the card's own verb says the next step —
/// "Preparar oferta" when nothing is written, "Ler e enviar" when something is.
/// That verb is what let the approval queue stop being a tab.
///
/// The old `?tab=` links still resolve, because dashboard nudges, e-mails and
/// bookmarks point at them and a dead link is a worse answer than an old
/// screen. They render the old body with a way back to the panel.
class ProspectBranchScreen extends ConsumerWidget {
  const ProspectBranchScreen({super.key, this.initialTab});

  /// `queue` and `calendar` still open their own screen. Anything else —
  /// including `signals` and `hunt`, whose content is now IN the panel —
  /// lands on the panel, which is also the default.
  final String? initialTab;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final role =
        AccountRole.from(ref.watch(authControllerProvider).asData?.value?.role);
    final view = _resolveView(context, role);

    return Scaffold(
      backgroundColor: colors.backgroundPrimary,
      body: SafeArea(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(24, 16, 24, 10),
              child: Row(
                children: [
                  if (MediaQuery.of(context).size.width >= 640)
                    Text(
                      t.shell.offers,
                      style: TextStyle(
                        fontSize: 20,
                        fontWeight: FontWeight.w700,
                        color: colors.textPrimary,
                      ),
                    ),
                  // A deep link that opens an old screen with no way back is a
                  // trap, and these links live in e-mails nobody will re-send.
                  if (view != _ProspectView.panel) ...[
                    const SizedBox(width: 16),
                    TextButton.icon(
                      key: const Key('prospect_back_to_panel'),
                      // No invalidate on the way out: the queue provider is
                      // autoDispose and the panel does not watch it, so
                      // leaving disposes it and the next visit re-reads. The
                      // old screen needed the explicit invalidate only because
                      // a TabBarView kept both tabs mounted.
                      onPressed: () => context.go('/prospect'),
                      icon: const Icon(Icons.arrow_back, size: 16),
                      label: Text(t.radar.back_to_panel,
                          style: const TextStyle(fontSize: 13)),
                      style: TextButton.styleFrom(
                          foregroundColor: colors.textSecondary),
                    ),
                  ],
                  const Spacer(),
                  // The outcome line needs a desk-width header; on anything
                  // narrower the numbers live inside the list itself.
                  if (MediaQuery.of(context).size.width >= 1100)
                    const _WeekOutcome(),
                ],
              ),
            ),
            Expanded(
              child: switch (view) {
                _ProspectView.queue =>
                  const ApprovalQueueScreen(showHeader: false),
                _ProspectView.calendar => const ContractCalendarBody(),
                _ProspectView.panel => const RadarBody(),
              },
            ),
          ],
        ),
      ),
    );
  }

  /// The URL is the source of truth. GoRouter keys a shell branch's page by
  /// PATH, so `?tab=` on a warm navigation never re-runs the route builder —
  /// but `GoRouterState.of` subscribes, so reading it here follows the link
  /// where the old TabController had to be nudged into following it.
  _ProspectView _resolveView(BuildContext context, AccountRole role) {
    var tab = initialTab;
    if (GoRouter.maybeOf(context) != null) {
      tab = GoRouterState.of(context).uri.queryParameters['tab'] ?? tab;
    }
    return switch (tab) {
      // Both 403 server-side below vendedor. Rendering them anyway parked
      // whoever followed the link on a RetryableError whose Retry could
      // never succeed.
      'queue' when role.satisfies(AccountRole.vendedor) => _ProspectView.queue,
      'calendar' when role.satisfies(AccountRole.vendedor) =>
        _ProspectView.calendar,
      _ => _ProspectView.panel,
    };
  }
}

enum _ProspectView { panel, queue, calendar }

/// "Esta semana: 12 enviadas · 3 respostas · 1 proposta" — the header's
/// right side, colored by what each number is worth. Hidden until it loads;
/// the screen works without it.
class _WeekOutcome extends ConsumerWidget {
  const _WeekOutcome();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final outcome = ref.watch(offersOutcomeProvider).asData?.value;
    if (outcome == null) return const SizedBox.shrink();
    TextSpan n(String v, Color c) => TextSpan(
        text: v, style: TextStyle(color: c, fontWeight: FontWeight.w700));
    return Text.rich(
      TextSpan(
        style: TextStyle(color: colors.textMuted, fontSize: 12.5),
        children: [
          TextSpan(text: '${t.radar.week_prefix} '),
          n('${outcome.sent}', colors.textPrimary),
          TextSpan(text: ' ${t.radar.week_sent} · '),
          n('${outcome.replies}', colors.limeGlow),
          TextSpan(text: ' ${t.radar.week_replies} · '),
          n('${outcome.proposals}', colors.bluePrimary),
          TextSpan(text: ' ${t.radar.week_proposals}'),
        ],
      ),
      key: const Key('ofertas_outcome_line'),
    );
  }
}
