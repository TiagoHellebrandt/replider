import 'package:app/core/router/app_shell.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/dashboard/presentation/controllers/dashboard_controller.dart'
    show UnreadCount, unreadCountProvider;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

class _FakeWaitingCount extends UnreadCount {
  _FakeWaitingCount(this.value);
  final int value;
  @override
  Future<int> build() async => value;
}

class _FakeAuth extends AuthController {
  @override
  Future<UserModel?> build() async => const UserModel(
        id: 'u1',
        name: 'Tiago',
        email: 'tiago@replider.com',
        token: 'tok',
        role: 'admin',
      );
}

// US-M016. The shell is the only navigation an SDR has, so the properties
// worth pinning are the ones a user would notice breaking: the right control
// for the form factor, four reachable destinations, and branch state that
// survives a tab switch.

/// Builds a router whose branches are cheap stand-ins for the real screens —
/// this exercises the SHELL, not the screens inside it.
GoRouter _shellRouter({String initial = '/start'}) {
  Widget leaf(String label) => Scaffold(
        body: Center(child: Text(label, key: Key('leaf_$label'))),
      );

  return GoRouter(
    initialLocation: initial,
    routes: [
      StatefulShellRoute.indexedStack(
        builder: (context, state, shell) => AppShell(navigationShell: shell),
        branches: [
          StatefulShellBranch(routes: [
            GoRoute(path: '/start', builder: (_, __) => leaf('start')),
          ]),
          StatefulShellBranch(routes: [
            GoRoute(path: '/funnel', builder: (_, __) => leaf('funnel')),
          ]),
          StatefulShellBranch(routes: [
            GoRoute(
              path: '/home',
              builder: (_, __) => leaf('home'),
              routes: [
                GoRoute(path: 'detail', builder: (_, __) => leaf('detail')),
              ],
            ),
          ]),
          StatefulShellBranch(routes: [
            GoRoute(path: '/prospect', builder: (_, __) => leaf('prospect')),
          ]),
        ],
      ),
    ],
  );
}

Future<GoRouter> _pumpShell(
  WidgetTester tester, {
  required Size size,
  String initial = '/start',
  AuthController Function()? auth,
  int waiting = 0,
}) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  final router = _shellRouter(initial: initial);
  await tester.pumpWidget(ProviderScope(
    overrides: [
      // The account menu reads the signed-in user; a fake keeps this a
      // test of the SHELL.
      authControllerProvider.overrideWith(auth ?? _FakeAuth.new),
      // The badge count would otherwise open a real Dio + WebSocket.
      unreadCountProvider.overrideWith(() => _FakeWaitingCount(waiting)),
    ],
    child: MaterialApp.router(
      theme: AppTheme.darkTheme,
      routerConfig: router,
    ),
  ));
  await tester.pumpAndSettle();
  return router;
}

void main() {
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  group('role gating', _roleGatingTests);

  testWidgets('phone width shows the bottom bar, not the rail', (tester) async {
    await _pumpShell(tester, size: const Size(390, 844)); // iPhone-ish

    expect(find.byKey(const Key('app_shell_bottom_nav')), findsOneWidget);
    expect(find.byKey(const Key('app_shell_rail')), findsNothing);
    // Four destinations, all labelled: icons alone are a guessing game for
    // users who are not power users of this app.
    expect(find.text(t.shell.start), findsWidgets);
    expect(find.text(t.shell.funnel), findsWidgets);
    expect(find.text(t.shell.conversations), findsWidgets);
    expect(find.text(t.shell.offers), findsWidgets);
    // The copilot is not a place: it left the bar (2026-08-18) and then the
    // account menu (2026-09-01) — it answers by URL only until 2026-10-01.
    expect(find.text(t.shell.copilot), findsNothing);
  });

  testWidgets('the account menu offers the Carteira, not a copilot',
      (tester) async {
    final router = await _pumpShell(tester, size: const Size(1200, 900));

    await tester.tap(find.byKey(const Key('app_shell_account_button')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('account_menu_copilot')), findsNothing);
    await tester.tap(find.byKey(const Key('account_menu_contacts')));
    await tester.pumpAndSettle();

    expect(router.state.uri.toString(), '/funnel?view=portfolio');
    expect(find.byKey(const Key('leaf_funnel')), findsOneWidget);
  });

  testWidgets('tablet width switches to the rail', (tester) async {
    await _pumpShell(tester, size: const Size(834, 1112));

    expect(find.byKey(const Key('app_shell_rail')), findsOneWidget);
    expect(find.byKey(const Key('app_shell_bottom_nav')), findsNothing);
  });

  testWidgets('desktop width keeps the rail', (tester) async {
    await _pumpShell(tester, size: const Size(1440, 900));

    expect(find.byKey(const Key('app_shell_rail')), findsOneWidget);
    expect(find.byKey(const Key('app_shell_bottom_nav')), findsNothing);
  });

  testWidgets('tapping a destination switches branch', (tester) async {
    final router = await _pumpShell(tester, size: const Size(390, 844));
    expect(find.byKey(const Key('leaf_start')), findsOneWidget);

    await tester.tap(find.text(t.shell.funnel).last);
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('leaf_funnel')), findsOneWidget);
    expect(router.state.uri.toString(), '/funnel');
  });

  // A deep link must land INSIDE the shell with the right tab selected —
  // otherwise a push notification opens the app on the wrong screen.
  testWidgets('deep link selects the matching branch', (tester) async {
    await _pumpShell(tester, size: const Size(1440, 900), initial: '/prospect');

    expect(find.byKey(const Key('leaf_prospect')), findsOneWidget);
    // The Ofertas pill (branch 3) is the one painted as selected.
    expect(find.byKey(const Key('rail_item_selected_3')), findsOneWidget);
  });

  // Each branch keeps its own Navigator: leaving a nested screen and coming
  // back must not dump the user at the branch root.
  testWidgets('branch state survives a tab switch', (tester) async {
    final router = await _pumpShell(tester, size: const Size(1440, 900));

    router.go('/home/detail');
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('leaf_detail')), findsOneWidget);

    await tester.tap(find.text(t.shell.funnel).last);
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('leaf_funnel')), findsOneWidget);

    await tester.tap(find.text(t.shell.conversations).last);
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('leaf_detail')), findsOneWidget,
        reason: 'returning to a branch restores where the user was');
  });

  // Tapping the ACTIVE tab pops to its root — the standard gesture, and the
  // only escape from a deep stack on mobile without hunting for back arrows.
  testWidgets('tapping the active destination pops to the branch root',
      (tester) async {
    final router = await _pumpShell(tester, size: const Size(390, 844));

    router.go('/home/detail');
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('leaf_detail')), findsOneWidget);

    await tester.tap(find.text(t.shell.conversations).last);
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('leaf_home')), findsOneWidget);
    expect(find.byKey(const Key('leaf_detail')), findsNothing);
  });

  // The account menu. Settings, Billing and Team belong to the ACCOUNT, so
  // they must be reachable from EVERY destination — they used to hang off
  // the inbox's toolbar, which meant opening Conversas to reach Billing.
  testWidgets('desktop reaches the account menu from any destination',
      (tester) async {
    await _pumpShell(tester, size: const Size(1200, 900), initial: '/funnel');

    expect(find.byKey(const Key('app_shell_account_button')), findsOneWidget);
    await tester.tap(find.byKey(const Key('app_shell_account_button')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('account_menu_settings')), findsOneWidget);
    // Sales-led (2026-08-19): no "Cobrança" entry anywhere.
    expect(find.byKey(const Key('account_menu_billing')), findsNothing);

    // ...anchored beside the rail button that opened it. A bottom sheet
    // stretched across a desktop reads as a slab dropped in the middle of
    // the page, nowhere near the control the user clicked.
    final menuLeft =
        tester.getTopLeft(find.byKey(const Key('account_menu_settings'))).dx;
    expect(menuLeft, lessThan(300));
  });

  // The menu had a threshold of its own (900) while the shell swaps bar for
  // rail at 600. Every window between the two showed the rail on the left and
  // opened the menu as a sheet pinned to the bottom: the answer on the opposite
  // side of the screen from the button that asked for it.
  testWidgets('between the bar and 900px the menu still anchors to the rail',
      (tester) async {
    await _pumpShell(tester, size: const Size(800, 900), initial: '/funnel');

    expect(find.byKey(const Key('app_shell_rail')), findsOneWidget);
    await tester.tap(find.byKey(const Key('app_shell_account_button')));
    await tester.pumpAndSettle();

    final menuLeft =
        tester.getTopLeft(find.byKey(const Key('account_menu_settings'))).dx;
    expect(menuLeft, lessThan(300),
        reason:
            'with the rail on screen the menu must not open as a bottom sheet');
  });

  testWidgets('mobile reaches it from the bar without leaving the branch',
      (tester) async {
    await _pumpShell(tester, size: const Size(390, 900), initial: '/funnel');

    await tester.tap(find.byKey(const Key('app_shell_account_tab')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('account_menu_settings')), findsOneWidget);
    // It opened a menu, it did not switch destination.
    expect(find.byKey(const Key('leaf_funnel')), findsOneWidget);
  });

  // An attendant cannot change the plan or the team; offering them the
  // entry teaches them to expect a 403.
  testWidgets('account-level entries are admin-only', (tester) async {
    tester.view.physicalSize = const Size(1200, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(ProviderScope(
      overrides: [authControllerProvider.overrideWith(_FakeAttendant.new)],
      child: MaterialApp.router(
        theme: AppTheme.darkTheme,
        routerConfig: _shellRouter(),
      ),
    ));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('app_shell_account_button')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('account_menu_team')), findsNothing);
    // ...but their own profile and the way out are always there.
    expect(find.byKey(const Key('account_menu_profile')), findsOneWidget);
    expect(find.byKey(const Key('account_menu_logout')), findsOneWidget);
  });

  // The account CREATOR is an `owner`, not an `admin`. While `isAdmin` was
  // a string comparison, the highest role in the account saw fewer entries
  // than an admin — Team, Channels, Products, Billing and Compliance all
  // vanished for the one person who owns the plan.
  testWidgets('an owner sees the admin entries too', (tester) async {
    tester.view.physicalSize = const Size(1200, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(ProviderScope(
      overrides: [authControllerProvider.overrideWith(_FakeOwner.new)],
      child: MaterialApp.router(
        theme: AppTheme.darkTheme,
        routerConfig: _shellRouter(),
      ),
    ));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('app_shell_account_button')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('account_menu_team')), findsOneWidget);
    expect(find.byKey(const Key('account_menu_channels')), findsOneWidget);
    expect(find.byKey(const Key('account_menu_products')), findsOneWidget);
  });
}

class _FakeOwner extends AuthController {
  @override
  Future<UserModel?> build() async => const UserModel(
        id: 'u3',
        name: 'Maria',
        email: 'maria@replider.com',
        token: 'tok',
        role: 'owner',
      );
}

class _FakeAttendant extends AuthController {
  @override
  Future<UserModel?> build() async => const UserModel(
        id: 'u2',
        name: 'Ana',
        email: 'ana@replider.com',
        token: 'tok',
        role: 'attendant',
      );
}

class _FakeVendedor extends AuthController {
  @override
  Future<UserModel?> build() async => const UserModel(
        id: 'u4',
        name: 'Rafa',
        email: 'rafa@replider.com',
        token: 'tok',
        role: 'vendedor',
      );
}

/// A role the bar must never widen for: an unrecognised claim degrades to the
/// least-privileged role, the same rule the Go side uses.
class _FakeGarbageRole extends AuthController {
  @override
  Future<UserModel?> build() async => const UserModel(
        id: 'u5',
        name: 'Zé',
        email: 'ze@replider.com',
        token: 'tok',
        role: 'diretor-supremo',
      );
}

/// The bar shows only what the SERVER will serve.
///
/// Before this, all five destinations showed to everybody: an attendant's bar
/// offered two that answer 403 (the pipeline refuses them) and a vendedor's
/// offered the full inbox the API refuses. The pattern everywhere else in this app
/// is to hide what the server will refuse, so nobody discovers a permission by
/// receiving an error.
void _roleGatingTests() {
  testWidgets('an attendant gets the inbox, nothing else', (tester) async {
    await _pumpShell(tester,
        size: const Size(390, 844), initial: '/home', auth: _FakeAttendant.new);

    expect(find.text(t.shell.conversations), findsOneWidget);
    // The pipeline answers ErrForbidden for this role.
    expect(find.text(t.shell.funnel), findsNothing);
    expect(find.text(t.shell.offers), findsNothing);
    expect(find.text(t.shell.start), findsNothing);
  });

  testWidgets('a vendedor gets the selling destinations, not the inbox',
      (tester) async {
    await _pumpShell(tester,
        size: const Size(390, 844), auth: _FakeVendedor.new);

    expect(find.text(t.shell.start), findsOneWidget);
    expect(find.text(t.shell.funnel), findsOneWidget);
    // "Opening every conversation in the account is not part of the job".
    expect(find.text(t.shell.conversations), findsNothing);
    // Ofertas IS her job: the approval queue's routes admit a vendedor with
    // her own leads. Only the hunt tab inside is admin (it mints and
    // spends), gated per tab by ProspectBranchScreen — hiding the whole
    // destination made WhatsApp commands her only approval surface.
    expect(find.text(t.shell.offers), findsOneWidget);
  });

  testWidgets('an owner sees all four', (tester) async {
    await _pumpShell(tester, size: const Size(390, 844), auth: _FakeOwner.new);

    for (final label in [
      t.shell.start,
      t.shell.funnel,
      t.shell.conversations,
      t.shell.offers,
    ]) {
      expect(find.text(label), findsOneWidget);
    }
  });

  // An unrecognised claim must never widen the bar.
  testWidgets('a garbage role gets the least-privileged bar', (tester) async {
    await _pumpShell(tester,
        size: const Size(390, 844),
        initial: '/home',
        auth: _FakeGarbageRole.new);

    expect(find.text(t.shell.conversations), findsOneWidget);
    expect(find.text(t.shell.funnel), findsNothing);
  });

  // Hiding a destination makes the bar's positions stop matching the shell's
  // branch indices.
  testWidgets('a tap reaches the branch, not the bar position', (tester) async {
    final router = await _pumpShell(tester,
        size: const Size(390, 844), auth: _FakeVendedor.new);

    // A vendedor's second item is Clientes (branch 1), not whatever sits at bar
    // position 1 — which is the mapping this asserts.
    await tester.tap(find.text(t.shell.funnel));
    await tester.pumpAndSettle();

    expect(router.state.uri.path, '/funnel');
  });

  // A branch this role cannot see selects nothing rather than lighting up an
  // unrelated tab — the rail would assert on an out-of-range index, and the bar
  // would claim the person is somewhere they are not.
  testWidgets('a branch outside the role highlights nothing on the rail',
      (tester) async {
    await _pumpShell(tester,
        size: const Size(1200, 800),
        initial: '/funnel',
        auth: _FakeAttendant.new);

    // No pill is painted as selected — the person is on a branch the rail
    // does not offer, and the rail must not claim otherwise.
    for (var branch = 0; branch < 4; branch++) {
      expect(find.byKey(Key('rail_item_selected_$branch')), findsNothing);
    }
  });

  testWidgets('Conversas wears the waiting badge; other tabs never do',
      (tester) async {
    await _pumpShell(tester, size: const Size(390, 844), waiting: 4);

    expect(find.byType(Badge), findsWidgets);
    expect(find.text('4'), findsOneWidget);
  });

  testWidgets('zero waiting renders no badge at all', (tester) async {
    await _pumpShell(tester, size: const Size(390, 844), waiting: 0);

    expect(find.byType(Badge), findsNothing);
  });
}
