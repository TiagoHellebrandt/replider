import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:app/features/settings/presentation/screens/settings_screen.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/owner/presentation/controllers/owner_controller.dart';
import 'package:app/features/owner/data/impersonation_session.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// SettingsScreen renders ListTiles inside themed glass cards (DecoratedBox
/// with a background colour), tripping Flutter's debug-only "ListTile
/// background color or ink splashes may be invisible" assertion during paint,
/// which fails the whole widget test. The assertion is cosmetic and does not
/// affect release builds. Swallow only that specific assertion; everything
/// else still fails. Must be called from INSIDE each test body — the test
/// binding reinstalls its own handler after setUp. (See app/CLAUDE.md.)
void ignoreListTileAssertions() {
  final prev = FlutterError.onError;
  FlutterError.onError = (details) {
    if (details.exceptionAsString().contains('ListTile background color')) {
      return;
    }
    prev?.call(details);
  };
}

void main() {
  setUpAll(() {
    FlutterSecureStorage.setMockInitialValues({});
  });

  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
  });

  final testUser = UserModel(
    id: 'user-1',
    name: 'Test User',
    email: 'test@example.com',
    token: 'fake-token',
    role: 'admin',
  );

  final attendantUser = UserModel(
    id: 'user-2',
    name: 'Atendente',
    email: 'attendant@example.com',
    token: 'fake-token',
    role: 'attendant',
  );

  final vendedorUser = UserModel(
    id: 'user-3',
    name: 'Rafa',
    email: 'rafa@example.com',
    token: 'fake-token',
    role: 'vendedor',
  );

  Widget createWidgetUnderTest({UserModel? user}) {
    return ProviderScope(
      overrides: [
        authControllerProvider
            .overrideWith(() => _FakeAuthController(user ?? testUser)),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const SettingsScreen(),
      ),
    );
  }

  testWidgets('SettingsScreen renders app bar with title',
      (WidgetTester tester) async {
    ignoreListTileAssertions();
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pumpAndSettle();

    expect(find.text('Settings'), findsOneWidget);
  });

  testWidgets('SettingsScreen shows user name and email',
      (WidgetTester tester) async {
    // Tall viewport: the profile block sits below the account section, which
    // grows every time a settings tile is added. At the default 800px it is
    // offstage and never built.
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    ignoreListTileAssertions();
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pumpAndSettle();

    expect(find.text('Test User'), findsOneWidget);
    expect(find.text('test@example.com'), findsOneWidget);
  });

  testWidgets('SettingsScreen shows section headers',
      (WidgetTester tester) async {
    // Tall viewport: the account block grows with every settings tile added
    // (commissions and contracts, G1/G4), pushing the sections below it past
    // the lazy ListView's build window.
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    ignoreListTileAssertions();
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pumpAndSettle();

    // The top section headers are always in the viewport. Uppercased by the
    // label widget, like every other settings screen.
    expect(find.text('COMPANY AND BRAND'), findsOneWidget);
    expect(find.text('EMAIL AND SENDING', skipOffstage: false), findsOneWidget);

    // Sales-led (2026-08-19): no "Subscription" section — the hub offers
    // the conversation instead. Scroll until it builds (ListView lazily
    // instantiates off-screen rows).
    await tester.scrollUntilVisible(
      find.text(t.settings.section_contract.toUpperCase()),
      200,
      scrollable: find.byType(Scrollable).first,
    );
    expect(
        find.text(t.settings.section_contract.toUpperCase()), findsOneWidget);
    expect(find.byKey(const Key('talk_to_us_card')), findsOneWidget);
    expect(find.text('SUBSCRIPTION', skipOffstage: false), findsNothing);
  });

  testWidgets('SettingsScreen has no billing tile',
      (WidgetTester tester) async {
    ignoreListTileAssertions();
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pumpAndSettle();

    expect(find.text('Plan & Usage', skipOffstage: false), findsNothing);
  });

  testWidgets('SettingsScreen exposes logout exactly once',
      (WidgetTester tester) async {
    ignoreListTileAssertions();
    // The explicit Log out row came BACK on 2026-04-23 (mobile has no
    // app-bar UserAccountChip, so this is the only self-serve way out).
    // The old version of this test asserted its absence and only passed
    // because ListView built lazily and the row sat below the fold —
    // the page body builds everything now, so the test asserts the real
    // contract: one logout row, never two.
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pumpAndSettle();

    expect(find.text('Log out', skipOffstage: false), findsOneWidget);
    expect(find.byIcon(Icons.logout, skipOffstage: false), findsOneWidget);
  });

  // Funnels and relationship dates are kept and routable, but nobody on the
  // path from "my products" to "an offer went out" needs to learn what a
  // funnel is — so they sit behind one closed door at the bottom.
  testWidgets('funnels and relationship dates hide behind Avançado',
      (WidgetTester tester) async {
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    ignoreListTileAssertions();
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pumpAndSettle();

    final toggle = find.byKey(const Key('settings_advanced_toggle'));
    expect(toggle, findsOneWidget);
    // Collapsed by default: the tiles are not in the tree.
    expect(find.byKey(const Key('settings_funnels_tile'), skipOffstage: false),
        findsNothing);
    expect(
        find.byKey(const Key('settings_relationship_tile'),
            skipOffstage: false),
        findsNothing);

    await tester.tap(toggle);
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('settings_funnels_tile'), skipOffstage: false),
        findsOneWidget);
    expect(
        find.byKey(const Key('settings_relationship_tile'),
            skipOffstage: false),
        findsOneWidget);
  });

  // The automated-support product left Settings altogether on 2026-08-18
  // (it had been behind one door since the shell landed; the door is gone
  // too). Nothing of it may come back inline.
  testWidgets('the support-bot product is not in Settings at all',
      (WidgetTester tester) async {
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    ignoreListTileAssertions();
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('settings_support_bot_tile')), findsNothing);
    expect(find.byKey(const Key('settings_contracts_tile')), findsNothing);
    expect(find.byKey(const Key('settings_commissions_tile')), findsNothing);
    expect(find.byKey(const Key('settings_autopilot_tile')), findsNothing);
    expect(find.byKey(const Key('settings_templates_tile')), findsNothing);

    for (final gone in [
      t.settings.campaigns_title,
      t.settings.flows_title,
      t.settings.ai_agents,
    ]) {
      expect(find.text(gone, skipOffstage: false), findsNothing,
          reason: '$gone is not part of the product any more');
    }
  });

  // GET /brand-kit is RequireRole("admin", "attendant"); a vendedor satisfies
  // neither, so for her the tile was a 403 behind a working-looking row.
  testWidgets('vendedor role does not get the brand tile',
      (WidgetTester tester) async {
    ignoreListTileAssertions();
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(createWidgetUnderTest(user: vendedorUser));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('settings_brand_tile'), skipOffstage: false),
        findsNothing);
    // The section itself is still there; only the gated tile is gone.
    expect(
        find.text(t.settings.section_company.toUpperCase(),
            skipOffstage: false),
        findsOneWidget);
  });

  testWidgets('SettingsScreen shows back button', (WidgetTester tester) async {
    ignoreListTileAssertions();
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pumpAndSettle();

    expect(find.byIcon(Icons.arrow_back), findsOneWidget);
  });

  testWidgets(
      'logout tile stays enabled during an active impersonation session',
      (WidgetTester tester) async {
    // Regression: the dedicated "Log out" tile used to be disabled while
    // impersonating, so the only feedback was a dead grayed-out button —
    // tapping it did nothing. logout() is a local-only wipe that clears the
    // impersonation slot first, so a full sign-out from inside an
    // impersonated session is safe and the button must remain tappable.
    ignoreListTileAssertions();
    // Tall viewport so the lazy ListView builds the bottom logout tile.
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    final session = ImpersonationSession(
      token: 'imp-token',
      expiresAt: DateTime.now().add(const Duration(minutes: 15)),
      actor: 'operator@replider.com',
      accountId: 'account-9',
      accountName: 'Acme',
      impersonatingUser: 'owner@acme.com',
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authControllerProvider
              .overrideWith(() => _FakeAuthController(testUser)),
          impersonationControllerProvider
              .overrideWith(() => _FakeImpersonationController(session)),
        ],
        child: MaterialApp(
          theme: AppTheme.darkTheme,
          home: const SettingsScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    final tile = tester.widget<ListTile>(
      find.ancestor(
        of: find.text(t.settings.logout_title),
        matching: find.byType(ListTile),
      ),
    );
    expect(tile.enabled, isTrue);
    expect(tile.onTap, isNotNull);
  });

  testWidgets(
      'attendant role hides admin-only sections (agents/channels/billing/'
      'campaigns), keeps profile + logout', (WidgetTester tester) async {
    // Regression: attendants got a 403 FORBIDDEN tapping admin-only
    // settings (agent config etc.). Those tiles must not render for them.
    ignoreListTileAssertions();
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(
      createWidgetUnderTest(user: attendantUser),
    );
    await tester.pumpAndSettle();

    // Admin-only — hidden. Channels now lives inside the email group, and the
    // agents heading is a sub-label of the support-bot section, so neither has
    // a section label of its own to look for.
    // The company section now renders for every role — the brand screen is
    // read-only below admin and holds the forbidden-claims list the
    // vendedora is held to. Only the account-editing tile stays admin-only.
    expect(
        find.text(t.settings.section_company.toUpperCase(),
            skipOffstage: false),
        findsOneWidget);
    expect(find.byKey(const Key('settings_brand_tile'), skipOffstage: false),
        findsOneWidget);
    expect(find.text(t.settings.company_section_title, skipOffstage: false),
        findsNothing);
    expect(find.text(t.settings.ai_agents, skipOffstage: false), findsNothing);
    expect(find.text(t.settings.channels_title, skipOffstage: false),
        findsNothing);
    expect(
        find.text(t.settings.section_email.toUpperCase(), skipOffstage: false),
        findsNothing);
    expect(find.text(t.settings.team.toUpperCase(), skipOffstage: false),
        findsNothing);
    expect(
        find.text(t.settings.subscription.toUpperCase(), skipOffstage: false),
        findsNothing);
    expect(find.text(t.settings.campaigns_title, skipOffstage: false),
        findsNothing);

    // Attendant-allowed — visible.
    expect(find.text(t.settings.logout_title, skipOffstage: false),
        findsOneWidget);
    expect(find.text('Atendente', skipOffstage: false), findsOneWidget);
  });

  testWidgets('Catálogo offers the sales-history import to admins',
      (WidgetTester tester) async {
    // Tall viewport: the catalog section sits far down the admin hub.
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    ignoreListTileAssertions();
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pumpAndSettle();

    await tester.scrollUntilVisible(
      find.byKey(const Key('settings_sales_import_tile')),
      200,
      scrollable: find.byType(Scrollable).first,
    );
    // Next to the products it references: the history feeds the reorder
    // dates and the cross-sell those products appear in.
    expect(find.byKey(const Key('settings_products_tile')), findsOneWidget);
    expect(find.byKey(const Key('settings_sales_import_tile')), findsOneWidget);
    expect(find.text(t.sales_import.title), findsOneWidget);
  });

  testWidgets('an attendant is not offered the sales-history import',
      (WidgetTester tester) async {
    ignoreListTileAssertions();
    await tester.pumpWidget(createWidgetUnderTest(user: attendantUser));
    await tester.pumpAndSettle();

    // `/sales/import` is admin-only server-side; the whole Catálogo section
    // is hidden below admin.
    expect(
        find.byKey(const Key('settings_sales_import_tile'),
            skipOffstage: false),
        findsNothing);
  });
}

class _FakeAuthController extends AuthController {
  final UserModel? _user;
  _FakeAuthController(this._user);

  @override
  FutureOr<UserModel?> build() async => _user;
}

class _FakeImpersonationController extends ImpersonationController {
  final ImpersonationSession? _session;
  _FakeImpersonationController(this._session);

  @override
  Future<ImpersonationSession?> build() async => _session;
}
