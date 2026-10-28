import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:app/core/auth/account_role.dart';
import 'package:app/design_system/components/retryable_error.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/sales/presentation/controllers/funnels_controller.dart';
import 'package:app/features/sales/presentation/screens/kanban_screen.dart';
import 'package:app/features/sales/presentation/widgets/discovered_contacts_body.dart';
import 'package:app/features/sales/presentation/widgets/portfolio_body.dart';
import 'package:app/i18n/strings.g.dart';

/// The shell's "Clientes" destination (US-M017/M018, movement 6).
///
/// ONE list of people, with three views of it: the ones still being sold to, the
/// ones who already bought, and the ones found in her own Sent folder. They used
/// to be three separate destinations — plus two wrappers rendering the same
/// bodies outside the shell — so "which screen is this customer on?" had a
/// different answer depending on where you looked.
///
/// A board needs a funnel, and most accounts have exactly one — so the screen
/// picks it automatically instead of making the user choose every time. With
/// several funnels it shows a selector; with none it points at funnel creation
/// rather than rendering an empty board that looks broken.
/// The three views of the customer list.
///
/// Parsed from `?view=` so the addresses that used to be separate screens can
/// redirect into the view instead of 404ing. An unknown value falls back to the
/// pipeline rather than erroring: a bad link should land somewhere useful.
enum CustomersView {
  pipeline,
  portfolio,
  inbox;

  static CustomersView fromQuery(String? raw) => switch (raw) {
        'portfolio' => CustomersView.portfolio,
        'inbox' => CustomersView.inbox,
        _ => CustomersView.pipeline,
      };
}

class FunnelBranchScreen extends ConsumerStatefulWidget {
  const FunnelBranchScreen({super.key, this.openLeadId});

  /// Lead to open on arrival (`/funnel?lead=<id>`), used by the inbox's
  /// pipeline chip so an attendant reaches the deal in one tap instead of
  /// hunting for the card.
  ///
  /// Injectable for tests; at runtime the live route is the source of
  /// truth — see [_leadToOpen]. GoRouter keys a shell branch's page by
  /// PATH, so arriving at `/funnel?lead=X` from another branch can reuse
  /// the existing widget and never re-run the builder that read the
  /// query. Reading the router state during build sidesteps that
  /// entirely.
  final String? openLeadId;

  @override
  ConsumerState<FunnelBranchScreen> createState() => _FunnelBranchScreenState();
}

class _FunnelBranchScreenState extends ConsumerState<FunnelBranchScreen> {
  String? _selectedFunnelId;

  /// Which view of the customer list is showing — always the URL's answer.
  ///
  /// The toggle NAVIGATES (`/funnel?view=…`) instead of holding its own state:
  /// the first version kept a `_view` field that, once touched, shadowed the
  /// query forever — so `/sales/portfolio` and the checklist's deep links
  /// landed on whatever view was toggled last. One source of truth, no shadow.
  ///
  /// A view the role cannot open resolves to the pipeline: `/portfolio` is
  /// admin/attendant and `/discovered-contacts` admin-only server-side, so
  /// rendering them for a vendedor parked her on a 403 with a Retry that could
  /// never succeed. Same rule as the pills below — hide what the server refuses.
  CustomersView _currentView(BuildContext context, AccountRole role) {
    final view = CustomersView.fromQuery(
        GoRouterState.of(context).uri.queryParameters['view']);
    return switch (view) {
      CustomersView.portfolio when !_canSeePortfolio(role) =>
        CustomersView.pipeline,
      CustomersView.inbox when !_canSeeInbox(role) => CustomersView.pipeline,
      _ => view,
    };
  }

  /// GET /portfolio is RequireRole("admin", "attendant"); gestor and above
  /// satisfy attendant, a vendedor deliberately does not.
  static bool _canSeePortfolio(AccountRole role) =>
      role.satisfies(AccountRole.attendant);

  /// GET /discovered-contacts (the Sent-folder scan) is admin-only.
  static bool _canSeeInbox(AccountRole role) =>
      role.satisfies(AccountRole.admin);

  /// The lead the current route asks for, falling back to the injected
  /// value. Reading it here (rather than only at construction) is what
  /// makes the inbox chip work when the funnel branch was already built.
  String? _leadToOpen(BuildContext context) {
    final fromRoute = GoRouterState.of(context).uri.queryParameters['lead'];
    if (fromRoute != null && fromRoute.isNotEmpty) return fromRoute;
    return widget.openLeadId;
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final funnels = ref.watch(funnelsListProvider);
    // Role from /me; while it loads (or on garbage) `from` degrades to the
    // least-privileged role, same rule as the shell's bar.
    final role =
        AccountRole.from(ref.watch(authControllerProvider).asData?.value?.role);

    return funnels.when(
      loading: () => const Scaffold(
        body: Center(child: CircularProgressIndicator()),
      ),
      error: (err, _) => Scaffold(
        body: RetryableError(
          error: err,
          onRetry: () => ref.invalidate(funnelsListProvider),
        ),
      ),
      data: (list) {
        if (list.isEmpty) {
          return Scaffold(
            backgroundColor: colors.backgroundPrimary,
            body: Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(
                    // Not "choose a funnel" — there is none to choose.
                    t.board.no_funnel_yet,
                    key: const Key('funnel_branch_empty'),
                    style: TextStyle(color: colors.textSecondary),
                  ),
                  const SizedBox(height: 12),
                  FilledButton(
                    onPressed: () => context.go('/sales/funnels/new'),
                    child: Text(t.sales.funnel_new_title),
                  ),
                ],
              ),
            ),
          );
        }

        final selected = _selectedFunnelId ?? list.first.id;
        final view = _currentView(context, role);
        Widget pill(CustomersView v, String label, Key key) => GestureDetector(
              key: key,
              onTap: () => context.go('/funnel?view=${v.name}'),
              child: Container(
                padding:
                    const EdgeInsets.symmetric(horizontal: 18, vertical: 7),
                decoration: view == v
                    ? BoxDecoration(
                        color: colors.backgroundTertiary,
                        borderRadius: BorderRadius.circular(999),
                        border: Border.all(color: colors.borderMedium),
                      )
                    : null,
                child: Text(
                  label,
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: view == v ? FontWeight.w600 : FontWeight.w400,
                    color: view == v ? Colors.white : colors.textSecondary,
                  ),
                ),
              ),
            );
        return Scaffold(
          backgroundColor: colors.backgroundPrimary,
          // The mockup's header: the title on the LEFT, the compact pill
          // switcher beside it — same anatomy as Ofertas, so the two
          // destinations read as one product.
          body: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              SafeArea(
                bottom: false,
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(24, 14, 24, 8),
                  child: Row(
                    children: [
                      Text(
                        t.shell.funnel,
                        style: TextStyle(
                          fontSize: 20,
                          fontWeight: FontWeight.w700,
                          color: colors.textPrimary,
                        ),
                      ),
                      const SizedBox(width: 20),
                      Container(
                        key: const Key('funnel_view_toggle'),
                        padding: const EdgeInsets.all(4),
                        decoration: BoxDecoration(
                          color: colors.glassMedium,
                          borderRadius: BorderRadius.circular(999),
                        ),
                        child: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            pill(CustomersView.pipeline, t.board.view_pipeline,
                                const Key('funnel_view_pipeline')),
                            if (_canSeePortfolio(role))
                              pill(CustomersView.portfolio, t.portfolio.title,
                                  const Key('funnel_view_portfolio')),
                            if (_canSeeInbox(role))
                              pill(
                                  CustomersView.inbox,
                                  t.my_customers.view_label,
                                  const Key('funnel_view_inbox')),
                          ],
                        ),
                      ),
                      // One funnel needs no chooser; a single-item dropdown
                      // is a decision the user does not have.
                      if (list.length > 1) ...[
                        const SizedBox(width: 16),
                        DropdownButton<String>(
                          key: const Key('funnel_branch_selector'),
                          value: selected,
                          underline: const SizedBox.shrink(),
                          dropdownColor: colors.backgroundSecondary,
                          items: [
                            for (final f in list)
                              DropdownMenuItem(
                                value: f.id,
                                child: Text(f.name,
                                    style:
                                        const TextStyle(color: Colors.white)),
                              ),
                          ],
                          onChanged: (id) =>
                              setState(() => _selectedFunnelId = id),
                        ),
                      ],
                    ],
                  ),
                ),
              ),
              Expanded(
                child: switch (view) {
                  CustomersView.portfolio => const PortfolioBody(),
                  // No channel passed: the body resolves the account's email
                  // channel itself, because being asked which channel to read
                  // is not a question somebody reviewing their own customers
                  // can answer.
                  CustomersView.inbox => const DiscoveredContactsBody(),
                  CustomersView.pipeline => KanbanScreen(
                      funnelId: selected,
                      openLeadId: _leadToOpen(context),
                    ),
                },
              ),
            ],
          ),
        );
      },
    );
  }
}
