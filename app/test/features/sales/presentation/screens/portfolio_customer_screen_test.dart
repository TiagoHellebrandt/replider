import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/sales/presentation/controllers/cross_sell_controller.dart';
import 'package:app/features/sales/presentation/controllers/portfolio_controller.dart';
import 'package:app/features/sales/presentation/controllers/relationship_controller.dart';
import 'package:app/features/sales/presentation/screens/portfolio_customer_screen.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

// The screen somebody opens before calling a customer: what they buy, when
// they need it again, what else to offer, and the date worth mentioning.

Future<void> _pump(
  WidgetTester tester, {
  List<RelationshipDate> dates = const [],
  CrossSellResult crossSell = const CrossSellResult(),
}) async {
  tester.view.physicalSize = const Size(900, 2000);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(ProviderScope(
    overrides: [
      dioProvider.overrideWithValue(Dio()),
      portfolioDetailProvider('c1').overrideWith((ref) async =>
          const PortfolioDetail(
              customer: PortfolioCustomer(
                  contactId: 'c1',
                  contactName: 'Clínica Bem Viver',
                  purchases: 3,
                  netCents: 1400000))),
      relationshipDatesProvider.overrideWith((ref) async => dates),
      crossSellProvider('c1').overrideWith((ref) async => crossSell),
    ],
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      home: const PortfolioCustomerScreen(contactId: 'c1'),
    ),
  ));
  await tester.pumpAndSettle();
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  testWidgets('shows what the customer is worth', (tester) async {
    await _pump(tester);

    expect(find.byKey(const Key('portfolio_detail_net')), findsOneWidget);
  });

  // Promised by the portfolio card since F2, answerable only now that F4 gave
  // those dates somewhere to live.
  testWidgets('shows the next date worth remembering', (tester) async {
    await _pump(tester, dates: const [
      RelationshipDate(
          id: 'd1', kind: 'birthday', contactId: 'c1', month: 5, day: 12),
    ]);

    expect(find.byKey(const Key('portfolio_detail_next_date')), findsOneWidget);
    expect(find.textContaining('12/05'), findsOneWidget);
  });

  // A sector day belongs to the whole portfolio. Showing it on every customer
  // would make it noise on all of them instead of a fact about one.
  testWidgets('an account-wide date is not shown as this customer\'s',
      (tester) async {
    await _pump(tester, dates: const [
      RelationshipDate(id: 'd1', kind: 'sector', month: 10, day: 18),
    ]);

    expect(find.byKey(const Key('portfolio_detail_next_date')), findsNothing);
  });

  // Another customer's birthday is not this customer's.
  testWidgets('a date belonging to somebody else is not shown', (tester) async {
    await _pump(tester, dates: const [
      RelationshipDate(
          id: 'd1', kind: 'birthday', contactId: 'c2', month: 5, day: 12),
    ]);

    expect(find.byKey(const Key('portfolio_detail_next_date')), findsNothing);
  });

  testWidgets('no recorded date means no line rather than a placeholder',
      (tester) async {
    await _pump(tester);

    expect(find.byKey(const Key('portfolio_detail_next_date')), findsNothing);
  });
}
