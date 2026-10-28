import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/features/owner/data/datasources/owner_remote_datasource.dart';
import 'package:app/features/owner/domain/models/owner_account_models.dart';
import 'package:app/features/owner/presentation/controllers/owner_controller.dart';
import 'package:app/features/owner/presentation/screens/owner_accounts_screen.dart';
import 'package:app/i18n/strings.g.dart';

class _FakeOwnerAccounts extends OwnerAccounts {
  _FakeOwnerAccounts(this._seed);
  final OwnerAccountsListResponse _seed;

  @override
  Future<OwnerAccountsListResponse> build({
    String search = '',
    String status = '',
    int limit = 50,
    int offset = 0,
  }) async {
    return _seed;
  }
}

OwnerAccountListing _listing({
  String id = 'acc-1',
  String name = 'Acme',
  String status = 'active',
  bool suspended = false,
  int tokensUsed = 0,
  int planMaxTokens = 0,
  String planName = 'Pro',
  String whatsappQuality = '',
}) {
  return OwnerAccountListing(
    account: OwnerAccountSummary(
      id: id,
      name: name,
      subscriptionStatus: status,
      onboardingCompleted: true,
      csatEnabled: true,
      trialBypass: false,
      suspendedAt: suspended ? DateTime.utc(2026, 1, 1) : null,
      createdAt: DateTime.utc(2026, 1, 1),
      updatedAt: DateTime.utc(2026, 1, 1),
    ),
    planName: planName,
    ownerEmail: 'admin@acme.com',
    ownerName: 'Admin',
    tokensUsed: tokensUsed,
    planMaxTokens: planMaxTokens,
    whatsappQuality: whatsappQuality,
  );
}

GoRouter _router() {
  return GoRouter(routes: [
    GoRoute(path: '/', builder: (_, __) => const OwnerAccountsScreen()),
    GoRoute(path: '/owner/accounts/:id', builder: (_, __) => const Scaffold()),
    GoRoute(path: '/owner/accounts/new', builder: (_, __) => const Scaffold()),
    GoRoute(path: '/home', builder: (_, __) => const Scaffold()),
  ]);
}

Widget _harness({
  required List<OwnerAccountListing> seed,
  OwnerSystemHealth? health,
  OwnerAccessStatus access = OwnerAccessStatus.allowed,
}) {
  return ProviderScope(
    overrides: [
      ownerAccountsProvider().overrideWith(
        () => _FakeOwnerAccounts(
          OwnerAccountsListResponse(items: seed, total: seed.length),
        ),
      ),
      ownerTopAccountsProvider()
          .overrideWith((ref) async => TopAccountsResponse(items: const [])),
      ownerAccessStatusProvider.overrideWith((ref) async => access),
      // The health bar stays hidden (SizedBox.shrink) unless a snapshot is
      // supplied, so the status-chip assertions aren't doubled by the
      // bar's own Active/Suspended tiles.
      if (health != null)
        ownerSystemHealthProvider.overrideWith((ref) async => health),
    ],
    child: MaterialApp.router(
      theme: AppTheme.darkTheme,
      routerConfig: _router(),
    ),
  );
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.en));

  group('OwnerAccountsScreen', () {
    testWidgets('renders empty state when no accounts', (tester) async {
      await tester.pumpWidget(_harness(seed: const []));
      await tester.pumpAndSettle();

      expect(find.text(t.owner.empty_title), findsOneWidget);
      expect(find.text(t.owner.create_contract_cta), findsOneWidget);
    });

    testWidgets('renders account row with plan + status pills', (tester) async {
      await tester.pumpWidget(_harness(seed: [_listing(name: 'Acme Co')]));
      await tester.pumpAndSettle();

      expect(find.text('Acme Co'), findsOneWidget);
      expect(find.text('Pro'), findsOneWidget);
      expect(find.text('active'), findsOneWidget);
    });

    testWidgets('renders a usage bar with cap% for a capped account',
        (tester) async {
      await tester.pumpWidget(_harness(seed: [
        _listing(tokensUsed: 90, planMaxTokens: 100),
      ]));
      await tester.pumpAndSettle();

      expect(find.byType(LinearProgressIndicator), findsOneWidget);
      expect(find.textContaining('90%'), findsOneWidget);
      final bar = tester.widget<LinearProgressIndicator>(
          find.byType(LinearProgressIndicator));
      expect(bar.value, closeTo(0.9, 0.001));
    });

    testWidgets('shows "no limit" instead of a bar when the plan is uncapped',
        (tester) async {
      await tester.pumpWidget(_harness(seed: [
        _listing(tokensUsed: 50, planMaxTokens: 0),
      ]));
      await tester.pumpAndSettle();

      expect(find.byType(LinearProgressIndicator), findsNothing);
      expect(find.textContaining(t.owner.usage_no_cap), findsOneWidget);
    });

    testWidgets('hides the plan badge when the plan name is empty',
        (tester) async {
      await tester.pumpWidget(_harness(seed: [_listing(planName: '')]));
      await tester.pumpAndSettle();

      expect(find.text('Pro'), findsNothing);
      // The dash placeholder must be gone from the redesign.
      expect(find.text('—'), findsNothing);
    });

    testWidgets('row overflow menu exposes impersonate + suspend',
        (tester) async {
      await tester.pumpWidget(_harness(seed: [_listing()]));
      await tester.pumpAndSettle();

      await tester.tap(find.byIcon(Icons.more_vert));
      await tester.pumpAndSettle();

      expect(find.text(t.owner.impersonate_cta), findsOneWidget);
      expect(find.text(t.owner.suspend_cta), findsOneWidget);
    });

    testWidgets('suspended row overflow menu offers reactivate',
        (tester) async {
      await tester.pumpWidget(_harness(seed: [
        _listing(status: 'active', suspended: true),
      ]));
      await tester.pumpAndSettle();

      await tester.tap(find.byIcon(Icons.more_vert));
      await tester.pumpAndSettle();

      expect(find.text(t.owner.reactivate_cta), findsOneWidget);
      expect(find.text(t.owner.suspend_cta), findsNothing);
    });

    testWidgets('flags a RED WhatsApp number with a ban-risk badge',
        (tester) async {
      await tester
          .pumpWidget(_harness(seed: [_listing(whatsappQuality: 'RED')]));
      await tester.pumpAndSettle();

      expect(find.text(t.owner.wa_risk_ban), findsOneWidget);
    });

    testWidgets('healthy GREEN WhatsApp number shows no badge', (tester) async {
      await tester
          .pumpWidget(_harness(seed: [_listing(whatsappQuality: 'GREEN')]));
      await tester.pumpAndSettle();

      expect(find.text(t.owner.wa_risk_ban), findsNothing);
      expect(find.text(t.owner.wa_quality_warning), findsNothing);
    });

    testWidgets('clickable health tile applies its status filter',
        (tester) async {
      await tester.pumpWidget(_harness(
        seed: [_listing()],
        health: OwnerSystemHealth(
          accountsTotal: 7,
          accountsActive: 1,
          accountsSuspended: 2,
        ),
      ));
      await tester.pumpAndSettle();

      // The suspended count tile renders; tapping it must not throw and
      // selects the suspended filter chip (now highlighted).
      expect(find.text('2'), findsWidgets);
      await tester.tap(find.text('2').first);
      await tester.pumpAndSettle();

      final chip = tester.widget<ChoiceChip>(
        find.widgetWithText(ChoiceChip, t.owner.filter_suspended),
      );
      expect(chip.selected, isTrue);
    });

    testWidgets('renders all 5 status filter chips', (tester) async {
      await tester.pumpWidget(_harness(seed: const []));
      await tester.pumpAndSettle();

      expect(
          find.widgetWithText(ChoiceChip, t.owner.filter_all), findsOneWidget);
      expect(find.widgetWithText(ChoiceChip, t.owner.filter_active),
          findsOneWidget);
      expect(find.widgetWithText(ChoiceChip, t.owner.filter_trialing),
          findsOneWidget);
      expect(find.widgetWithText(ChoiceChip, t.owner.filter_past_due),
          findsOneWidget);
      expect(find.widgetWithText(ChoiceChip, t.owner.filter_suspended),
          findsOneWidget);
    });
  });

  group('expired 2FA proof', () {
    // Regression: the gate used to say "sign in again" and send the operator
    // to the login screen, losing the panel. A lapsed proof is re-proved in
    // place — the CTA opens the step-up dialog, it does NOT navigate away.
    testWidgets('the gate offers the step-up dialog, not a re-login',
        (tester) async {
      await tester.pumpWidget(_harness(
        seed: const [],
        access: OwnerAccessStatus.requiresFreshProof,
      ));
      await tester.pumpAndSettle();

      expect(find.text(t.owner.gate_refresh_title), findsOneWidget);
      expect(find.text(t.owner.gate_refresh_cta), findsOneWidget);

      await tester.tap(find.byKey(const Key('owner_gate_cta')));
      await tester.pumpAndSettle();

      expect(find.text(t.owner.stepup_title), findsOneWidget,
          reason: 'the code is asked for here, not behind a full re-login');
      expect(find.byKey(const Key('owner_stepup_code')), findsOneWidget);
    });
  });
}
