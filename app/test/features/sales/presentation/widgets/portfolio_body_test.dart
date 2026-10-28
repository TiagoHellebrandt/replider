import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/sales/presentation/controllers/portfolio_controller.dart';
import 'package:app/features/sales/presentation/widgets/portfolio_body.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

// Consumables are recurring revenue. Whoever opens this has an hour, and the
// only question is who to call first — so everything here is about surfacing
// that. Since 2026-09-01 it is also THE customer list (`/contacts` merged into
// it), so the essentials that list had — search, the import door, opening a
// customer — are pinned here too.

class _FakeAuth extends AuthController {
  _FakeAuth(this.role);
  final String role;
  @override
  Future<UserModel?> build() async => UserModel(
        id: 'u1',
        name: 'Tiago',
        email: 'tiago@replider.com',
        token: 'tok',
        role: role,
      );
}

PortfolioCustomer _customer({
  String id = 'c1',
  String name = 'Clínica Bem Viver',
  int? daysUntilDue,
  int? daysSinceContact,
  int netCents = 1400000,
}) =>
    PortfolioCustomer(
      contactId: id,
      contactName: name,
      purchases: 4,
      netCents: netCents,
      daysUntilDue: daysUntilDue,
      daysSinceContact: daysSinceContact,
    );

Future<void> _pump(WidgetTester tester,
    {List<PortfolioCustomer> customers = const [],
    String role = 'admin'}) async {
  tester.view.physicalSize = const Size(900, 1600);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  // The body navigates (import, open a customer), so it needs a router;
  // the destinations are stand-ins that only say where we landed.
  final router = GoRouter(
    initialLocation: '/portfolio',
    routes: [
      GoRoute(
          path: '/portfolio',
          builder: (_, __) => const Scaffold(body: PortfolioBody())),
      GoRoute(
          path: '/contacts/import',
          builder: (_, __) => const Scaffold(body: Text('import-screen'))),
      GoRoute(
          path: '/sales/import',
          builder: (_, __) =>
              const Scaffold(body: Text('sales-import-screen'))),
      GoRoute(
          path: '/sales/relationship',
          builder: (_, __) => const Scaffold(body: Text('relationship'))),
      GoRoute(
          path: '/sales/portfolio/:id',
          builder: (_, state) =>
              Scaffold(body: Text('customer-${state.pathParameters['id']}'))),
    ],
  );

  await tester.pumpWidget(ProviderScope(
    overrides: [
      portfolioProvider.overrideWith((ref) async => customers),
      authControllerProvider.overrideWith(() => _FakeAuth(role)),
      dioProvider.overrideWithValue(Dio()),
    ],
    child: MaterialApp.router(theme: AppTheme.darkTheme, routerConfig: router),
  ));
  await tester.pumpAndSettle();
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  testWidgets('lists customers with what they are worth', (tester) async {
    await _pump(tester, customers: [_customer()]);

    expect(find.text('Clínica Bem Viver'), findsOneWidget);
    // Net, not gross: what a customer is worth is what the company kept.
    expect(find.text('R\$ 14.000,00'), findsOneWidget);
  });

  // The most expensive state in the model: they have run out, and somebody is
  // selling to them today.
  testWidgets('a customer who ran out says how long ago', (tester) async {
    await _pump(tester, customers: [_customer(daysUntilDue: -12)]);

    expect(find.text(t.portfolio.ran_out(days: 12)), findsOneWidget);
  });

  testWidgets('a customer about to run out gets a countdown', (tester) async {
    await _pump(tester, customers: [_customer(daysUntilDue: 3)]);

    expect(find.text(t.portfolio.runs_out_in(days: 3)), findsOneWidget);
  });

  // A comfortable customer is worth listing and is not worth a chip: a badge
  // on every row stops being read.
  testWidgets('a customer with weeks to go gets no chip', (tester) async {
    await _pump(tester, customers: [_customer(daysUntilDue: 45)]);

    expect(find.byKey(const Key('portfolio_due_c1')), findsNothing);
  });

  // The absence IS the signal: nobody recorded a cycle for what they bought.
  testWidgets('no prediction means no chip rather than a guess',
      (tester) async {
    await _pump(tester, customers: [_customer()]);

    expect(find.byKey(const Key('portfolio_due_c1')), findsNothing);
  });

  // Silence is its own risk, independent of any reorder date.
  testWidgets('a silent customer is flagged', (tester) async {
    await _pump(tester,
        customers: [_customer(daysSinceContact: 90, daysUntilDue: 45)]);

    expect(find.byKey(const Key('portfolio_silent_c1')), findsOneWidget);
  });

  // Sixty days, not thirty: shorter would flag every quarterly customer as
  // neglected, which trains people to ignore the flag.
  testWidgets('a recently contacted customer is not flagged', (tester) async {
    await _pump(tester, customers: [_customer(daysSinceContact: 20)]);

    expect(find.byKey(const Key('portfolio_silent_c1')), findsNothing);
  });

  testWidgets('an empty portfolio explains how one fills up', (tester) async {
    await _pump(tester);

    expect(find.byKey(const Key('portfolio_empty')), findsOneWidget);
    expect(find.text(t.portfolio.empty_hint), findsOneWidget);
    // The import is the primary way a base gets filled, so the door is
    // there even — especially — when the list is empty.
    expect(find.byKey(const Key('portfolio_import')), findsOneWidget);
  });

  // What the old /contacts list offered and this view did not.
  testWidgets('search narrows the list by name', (tester) async {
    await _pump(tester, customers: [
      _customer(id: 'c1', name: 'Clínica Bem Viver'),
      _customer(id: 'c2', name: 'Hospital Femina'),
    ]);

    await tester.enterText(find.byKey(const Key('portfolio_search')), 'femin');
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('portfolio_c2')), findsOneWidget);
    expect(find.byKey(const Key('portfolio_c1')), findsNothing);

    await tester.enterText(find.byKey(const Key('portfolio_search')), 'zzz');
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('portfolio_search_empty')), findsOneWidget);
    expect(find.text(t.common.no_results), findsOneWidget);
  });

  testWidgets('the import door opens the CSV import', (tester) async {
    await _pump(tester, customers: [_customer()]);

    await tester.tap(find.byKey(const Key('portfolio_import')));
    await tester.pumpAndSettle();
    expect(find.text('import-screen'), findsOneWidget);
  });

  // The other half of the base: what they bought. Same door pattern, same
  // server-side admin gate (`/sales/import`).
  testWidgets('the sales-history door opens the sales import', (tester) async {
    await _pump(tester, customers: [_customer()]);

    await tester.tap(find.byKey(const Key('portfolio_import_sales')));
    await tester.pumpAndSettle();
    expect(find.text('sales-import-screen'), findsOneWidget);
  });

  // `/contacts/import` is RequireRole("admin") on the server; offering it to
  // an attendant teaches her a permission by 403.
  testWidgets('an attendant is not offered the import', (tester) async {
    await _pump(tester, customers: [_customer()], role: 'attendant');

    expect(find.byKey(const Key('portfolio_import')), findsNothing);
    // Same gate for the sales-history door: `/sales/import` is admin-only.
    expect(find.byKey(const Key('portfolio_import_sales')), findsNothing);
    expect(find.byKey(const Key('portfolio_search')), findsOneWidget);
  });

  testWidgets('tapping a customer opens their page', (tester) async {
    await _pump(tester, customers: [_customer()]);

    await tester.tap(find.byKey(const Key('portfolio_c1')));
    await tester.pumpAndSettle();
    expect(find.text('customer-c1'), findsOneWidget);
  });
}
