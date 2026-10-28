import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:app/core/auth/account_role.dart';
import 'package:app/core/auth/session_countdown.dart';
import 'package:app/core/auth/session_ending_banner.dart';
import 'package:app/core/router/account_menu.dart';
import 'package:app/features/dashboard/presentation/controllers/dashboard_controller.dart'
    show unreadCountProvider;
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/auth/presentation/widgets/email_verification_banner.dart';
import 'package:app/i18n/strings.g.dart';

/// Persistent navigation shell (US-M016).
///
/// The pre-pivot app had no shell at all: navigation lived as icons in the
/// inbox header, and the inbox WAS the landing screen. That worked when the
/// product was one thing (answer messages). An SDR works across four —
/// their day, their pipeline, their conversations, and finding new
/// companies — and a product with no persistent navigation makes three of
/// those invisible.
///
/// One widget serves both form factors on purpose: a salesperson uses the
/// desktop at their desk and the phone between visits, and two separate
/// navigation implementations drift.
class AppShell extends ConsumerWidget {
  const AppShell({super.key, required this.navigationShell});

  /// Provided by StatefulShellRoute: keeps one Navigator per branch, so
  /// switching tabs preserves each branch's scroll position and stack
  /// instead of rebuilding it.
  final StatefulNavigationShell navigationShell;

  /// Breakpoint between bottom bar and side rail. 600 matches the value the
  /// rest of the app already uses (dashboard, onboarding), so a window
  /// resize never puts two components on different sides of the fence.
  static const double railBreakpoint = 600;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    // Role from /me. While it loads, and for an unrecognised value, `from`
    // degrades to the LEAST privileged role — the same rule the Go side uses, so
    // a slow or corrupt claim can never widen the bar.
    final role =
        AccountRole.from(ref.watch(authControllerProvider).asData?.value?.role);
    // The Conversas badge: threads holding a customer message nobody
    // opened (messages.read_at, cleared when the thread is opened). It
    // used to count "waiting on a human", which stayed lit after the
    // attendant had read everything. 0 (or a failed read) renders no
    // badge at all.
    final waiting = ref.watch(unreadCountProvider).asData?.value ?? 0;
    final destinations = destinationsFor(role);
    // The warning lives here because the session belongs to no single screen: it
    // ends wherever the person happens to be.
    final remaining = ref.watch(sessionCountdownProvider);
    // The verification nudge mounts HERE because the session belongs to no
    // single screen — and its old home (/home only) was a screen onboarding
    // never lands on, which made the resend button unreachable for exactly
    // the accounts that needed it.
    final needsVerification = ref.watch(authControllerProvider
        .select((s) => s.asData?.value?.isEmailVerified == false));
    Widget shell = navigationShell;
    if (needsVerification || shouldWarnAboutSession(remaining)) {
      shell = Column(children: [
        if (needsVerification) const EmailVerificationBanner(),
        if (shouldWarnAboutSession(remaining))
          SessionEndingBanner(remaining: remaining!),
        Expanded(child: navigationShell),
      ]);
    }

    return LayoutBuilder(
      builder: (context, constraints) {
        final wide = constraints.maxWidth >= railBreakpoint;

        if (!wide) {
          final selected = _selectedIndex(destinations);
          return Scaffold(
            body: shell,
            bottomNavigationBar: NavigationBar(
              key: const Key('app_shell_bottom_nav'),
              // The bar cannot express "nothing selected" — its selectedIndex is
              // non-nullable — so it falls back to the account item, which is the
              // one entry in that list that is not a place. Tapping any tab still
              // navigates: `_go` compares against the SHELL's branch index, not
              // the bar's position. The indicator is suppressed in that case so
              // a branch this role cannot see does not paint "Conta" as if it
              // were the current screen.
              indicatorColor: selected == null ? Colors.transparent : null,
              selectedIndex: selected ?? destinations.length,
              onDestinationSelected: (index) =>
                  _select(context, destinations, index),
              // Labels always visible: four destinations with icons alone
              // is a guessing game, and this app's users are not power
              // users of it.
              labelBehavior: NavigationDestinationLabelBehavior.alwaysShow,
              backgroundColor: colors.backgroundSecondary,
              destinations: [
                for (final d in destinations)
                  NavigationDestination(
                    icon: _badged(d, Icon(d.icon), waiting),
                    selectedIcon: _badged(d, Icon(d.selectedIcon), waiting),
                    label: d.label,
                    tooltip: d.label,
                  ),
                // Account, not a destination: it opens a menu instead of
                // switching branch. It lives here because Settings,
                // Billing and Team belong to the ACCOUNT — hanging them
                // off the inbox meant reaching Billing required going to
                // Conversas first.
                NavigationDestination(
                  key: const Key('app_shell_account_tab'),
                  icon: const Icon(Icons.account_circle_outlined),
                  selectedIcon: const Icon(Icons.account_circle),
                  label: t.account_menu.title,
                  tooltip: t.account_menu.title,
                ),
              ],
            ),
          );
        }

        // The desktop rail is hand-rolled instead of NavigationRail: the
        // approved canvas ("Início — Briefing") draws a 76px strip with a
        // compact 40×28 pill behind the active ICON only and 11px labels —
        // Material's rail cannot be themed into that shape (its indicator
        // and label metrics are fixed families of sizes).
        final selectedIndex = _selectedIndex(destinations);
        return Scaffold(
          body: Row(
            children: [
              Container(
                key: const Key('app_shell_rail'),
                width: 76,
                color: colors.backgroundSecondary,
                child: Column(
                  children: [
                    const SizedBox(height: 14),
                    for (var i = 0; i < destinations.length; i++) ...[
                      _RailItem(
                        destination: destinations[i],
                        selected: selectedIndex == i,
                        waiting: waiting,
                        onTap: () => _select(context, destinations, i),
                      ),
                      const SizedBox(height: 18),
                    ],
                    // Account sits at the BOTTOM of the rail, away from the
                    // destinations: it is not a place in the app, it is who
                    // you are and what the account is.
                    const Spacer(),
                    Padding(
                      padding: const EdgeInsets.only(bottom: 12),
                      child: IconButton(
                        key: const Key('app_shell_account_button'),
                        tooltip: t.account_menu.title,
                        icon: Icon(Icons.account_circle_outlined,
                            color: colors.textSecondary),
                        onPressed: () => showAccountMenu(context),
                      ),
                    ),
                  ],
                ),
              ),
              const VerticalDivider(width: 1, thickness: 1),
              Expanded(child: shell),
            ],
          ),
        );
      },
    );
  }

  /// Bottom-bar taps: the trailing item is the account menu, not a
  /// branch. Keeping it in the bar (rather than a floating avatar) is what
  /// makes it reachable on a phone without every screen growing an app bar.
  void _select(
      BuildContext context, List<_Destination> destinations, int index) {
    // The trailing item is the account menu, not a branch.
    if (index >= destinations.length) {
      showAccountMenu(context);
      return;
    }
    _go(destinations[index].branch);
  }

  /// Which bar item to light up.
  ///
  /// The bar's positions and the shell's branch indices stop agreeing the moment
  /// a destination is hidden, so this maps one to the other instead of assuming
  /// they are the same number.
  ///
  /// A branch this role cannot see (an attendant who followed a link into the
  /// pipeline) selects NOTHING rather than lighting up an unrelated tab: the
  /// screen they are on is not one of these, and pretending otherwise is how
  /// somebody taps "the tab I am already on" and lands somewhere else.
  /// Returns NULL when the current branch is not one this role can see, and both
  /// components take null as "nothing selected".
  ///
  /// It cannot return `destinations.length` as a stand-in for that: the bottom bar
  /// has an extra account item at that index, but the rail keeps the account
  /// button in its `trailing` and asserts `selectedIndex < destinations.length`.
  /// The first version did exactly that and the shell's own test caught it.
  int? _selectedIndex(List<_Destination> destinations) {
    final current = navigationShell.currentIndex;
    for (var i = 0; i < destinations.length; i++) {
      if (destinations[i].branch == current) return i;
    }
    return null;
  }

  /// Tapping the ACTIVE destination pops that branch back to its root — the
  /// standard "tap the tab again to go home" gesture, and the only way out
  /// of a deep stack on mobile without hunting for back arrows.
  void _go(int index) {
    navigationShell.goBranch(
      index,
      initialLocation: index == navigationShell.currentIndex,
    );
  }

  /// Every destination, with the branch it switches to and the role the SERVER
  /// requires of it.
  ///
  /// The required roles are not a design choice; each mirrors an enforcement
  /// point that already exists:
  ///
  ///  * Início — `command_center.go` allows a manager or a vendedor and answers
  ///    `ErrForbidden` to an attendant.
  ///  * Clientes — the lead reads AND writes are actor-scoped and admit a
  ///    vendedor; an attendant is refused by the pipeline on both
  ///    (`TestRBAC_AttendantHasNoPipelineAccess`,
  ///    `TestLeadWrites_AnAttendantCannotWriteOnTheSalesPipeline`).
  ///  * Prospecção — `/funnels/*` is `RequireRole("admin")`, so neither an
  ///    attendant nor a vendedor may prospect.
  ///  * Conversas — `RequireRole("admin", "attendant")`, which a vendedor
  ///    deliberately does not satisfy: "opening every conversation in the
  ///    account is not part of the job".
  ///
  /// `test/guards/shell_matches_server_roles_test.dart` checks each `requires`
  /// against the Go router's `RequireRole` for the first request the
  /// destination makes, so this list cannot drift from the server again.
  ///
  /// Before this, all four showed to everybody. An attendant's bar offered two
  /// destinations that answer 403 and one that does — and a vendedor's offered a
  /// full inbox the API refuses. The pattern the rest of the app already
  /// follows (`if (isAdmin)` in Settings, `showSeller` in the account menu) is to
  /// hide what the server will refuse, so nobody discovers a permission by
  /// receiving an error.
  static List<_Destination> destinationsFor(AccountRole role) => [
        _Destination(
          branch: 0,
          label: t.shell.start,
          icon: Icons.home_outlined,
          selectedIcon: Icons.home,
          requires: AccountRole.vendedor,
        ),
        _Destination(
          branch: 1,
          label: t.shell.funnel,
          icon: Icons.view_kanban_outlined,
          selectedIcon: Icons.view_kanban,
          requires: AccountRole.vendedor,
        ),
        _Destination(
          branch: 2,
          label: t.shell.conversations,
          icon: Icons.forum_outlined,
          selectedIcon: Icons.forum,
          requires: AccountRole.attendant,
        ),
        // Named for what the person DOES there — approve offers — not for how
        // the offers get found. Hunting and signals are tabs inside it.
        //
        // VENDEDOR-level since the UX audit: the approval queue's server
        // routes (`/leads/*`) admit a vendedor with her own leads, and the
        // approval gate is the one screen the product is named after — the
        // admin-only gate here made WhatsApp commands the vendedora's ONLY
        // approval surface. What stays admin is the HUNT tab (`/funnels/*`
        // mints and spends), gated inside ProspectBranchScreen per tab.
        _Destination(
          branch: 3,
          label: t.shell.offers,
          icon: Icons.local_offer_outlined,
          selectedIcon: Icons.local_offer,
          requires: AccountRole.vendedor,
        ),
      ].where((d) => role.satisfies(d.requires)).toList();
}

/// One rail entry: a compact pill behind the active icon, an 11px label
/// underneath — the canvas's anatomy, at Material hit-target size (the
/// InkWell spans 64×52, comfortably past 44px).
class _RailItem extends StatelessWidget {
  const _RailItem({
    required this.destination,
    required this.selected,
    required this.waiting,
    required this.onTap,
  });

  final _Destination destination;
  final bool selected;
  final int waiting;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final d = destination;
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(12),
      child: SizedBox(
        width: 64,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              key: selected
                  ? Key('rail_item_selected_${d.branch}')
                  : Key('rail_item_${d.branch}'),
              width: 40,
              height: 28,
              decoration: BoxDecoration(
                color: selected ? colors.bluePrimary : Colors.transparent,
                borderRadius: BorderRadius.circular(999),
              ),
              alignment: Alignment.center,
              child: _badged(
                d,
                Icon(
                  selected ? d.selectedIcon : d.icon,
                  size: 20,
                  color: selected ? Colors.white : colors.textSecondary,
                ),
                waiting,
              ),
            ),
            const SizedBox(height: 4),
            Text(
              d.label,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                fontSize: 11,
                fontWeight: selected ? FontWeight.w600 : FontWeight.w400,
                color: selected ? colors.textPrimary : colors.textSecondary,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Wraps the Conversas icon in a count badge when somebody is waiting.
/// Only that destination: a badge on every tab is a dashboard, not a nudge.
Widget _badged(_Destination d, Widget icon, int waiting) {
  if (d.branch != 2 || waiting <= 0) return icon;
  return Badge.count(count: waiting, child: icon);
}

class _Destination {
  const _Destination({
    required this.branch,
    required this.requires,
    required this.label,
    required this.icon,
    required this.selectedIcon,
  });

  /// Index of the shell branch this destination switches to. Carried on the
  /// destination because hiding one makes the bar's position stop matching the
  /// branch index — the bug that "just filter the list" produces.
  final int branch;

  /// The role the SERVER requires. See [AppShell.destinationsFor].
  final AccountRole requires;

  final String label;
  final IconData icon;
  final IconData selectedIcon;
}
