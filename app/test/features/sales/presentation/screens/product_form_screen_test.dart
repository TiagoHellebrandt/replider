import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/sales/domain/models/product_model.dart';
import 'package:app/features/sales/presentation/controllers/products_controller.dart';
import 'package:app/features/sales/presentation/screens/product_form_screen.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

// The price field used to run `double.tryParse(raw.replaceAll(',', '.'))
// ?? 0`: the Brazilian notation "1.234,56" parsed to ZERO and the product
// was saved at R$ 0,00 with a success toast. Currency was a free-text field
// where "reais" silently became a USD product.

class _CapturingProducts extends ProductsList {
  int? createdCents;
  String? createdCurrency;
  int? createdCycleDays;
  int? updatedCycleDays;

  /// What the edit form loads. Empty by default (the create tests).
  List<ProductModel> seeded = const [];

  @override
  Future<List<ProductModel>> build() async => seeded;

  @override
  Future<String?> create({
    required String name,
    String description = '',
    int priceCents = 0,
    String currency = 'BRL',
    bool active = true,
    int consumptionCycleDays = 0,
  }) async {
    createdCents = priceCents;
    createdCurrency = currency;
    createdCycleDays = consumptionCycleDays;
    return null;
  }

  @override
  Future<String?> updateProduct({
    required String id,
    required String name,
    String description = '',
    int priceCents = 0,
    String currency = 'BRL',
    bool active = true,
    int? consumptionCycleDays,
  }) async {
    updatedCycleDays = consumptionCycleDays;
    return null;
  }
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  Future<_CapturingProducts> pump(WidgetTester tester,
      {List<ProductModel> seeded = const [], String? editId}) async {
    tester.view.physicalSize = const Size(900, 1800);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    final fake = _CapturingProducts()..seeded = seeded;
    final router = GoRouter(
        initialLocation: editId == null
            ? '/sales/products/new'
            : '/sales/products/$editId/edit',
        routes: [
          GoRoute(
              path: '/sales/products/new',
              builder: (_, __) => const ProductFormScreen()),
          GoRoute(
              path: '/sales/products/:id/edit',
              builder: (_, state) =>
                  ProductFormScreen(productId: state.pathParameters['id'])),
          GoRoute(
              path: '/sales/products',
              builder: (_, __) => const Scaffold(body: Text('list'))),
        ]);
    await tester.pumpWidget(ProviderScope(
      overrides: [
        productsListProvider.overrideWith(() => fake),
        dioProvider.overrideWithValue(Dio()),
      ],
      child:
          MaterialApp.router(theme: AppTheme.darkTheme, routerConfig: router),
    ));
    await tester.pumpAndSettle();
    return fake;
  }

  testWidgets('the Brazilian notation saves the amount on screen',
      (tester) async {
    final fake = await pump(tester);

    await tester.enterText(
        find.widgetWithText(TextFormField, '').first, 'Tomógrafo');
    await tester.enterText(
        find.byType(TextFormField).at(2), '1.234,56'); // price field
    await tester.tap(find.text(t.sales.products.save));
    await tester.pumpAndSettle();

    expect(fake.createdCents, 123456,
        reason: 'a notação BR não pode virar zero');
    expect(fake.createdCurrency, 'BRL');
  });

  // The contract changed deliberately. In this market the rep quotes at the
  // moment of the deal, so a REQUIRED price forced a number nobody meant and
  // filled a ninety-product catalogue with "R$ 0,00" — which reads as a broken
  // import, not as "ask me". Empty is now a legible unknown.
  testWidgets('an empty price saves — the rep quotes on the spot',
      (tester) async {
    // Tall surface: the hint under the price sits below the fold at the
    // default 800x600, where a ListView never builds it.
    tester.view.physicalSize = const Size(1200, 2400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    final fake = await pump(tester);

    await tester.enterText(find.byType(TextFormField).first, 'Tomógrafo');

    // The screen says what empty means, instead of leaving 0,00 to be read as
    // free. Asserted BEFORE saving: a successful save leaves this screen.
    expect(find.byKey(const Key('product_price_optional')), findsOneWidget);

    await tester.tap(find.text(t.sales.products.save));
    await tester.pumpAndSettle();

    expect(find.text(t.sales.products.price_required), findsNothing);
    expect(fake.createdCents, 0, reason: 'zero aqui é "não sei", e é salvo');
  });

  // A price that was TYPED still has to parse. Silently turning "R$ dez mil"
  // into zero is worse than no price: it looks answered.
  testWidgets('a typed price that does not parse still blocks the save',
      (tester) async {
    final fake = await pump(tester);

    await tester.enterText(find.byType(TextFormField).first, 'Tomógrafo');
    await tester.enterText(find.byType(TextFormField).at(2), 'dez mil');
    await tester.tap(find.text(t.sales.products.save));
    await tester.pumpAndSettle();

    expect(find.text(t.sales.products.price_invalid), findsOneWidget);
    expect(fake.createdCents, isNull);
  });

  // The cycle is the one number the reorder engine runs on. A product saved
  // without it is one no customer is ever reminded of, and the form used to
  // have no way to say it — the field only existed as an MCP tool.
  testWidgets('the repurchase cycle is saved with a new product',
      (tester) async {
    final fake = await pump(tester);

    await tester.enterText(find.byType(TextFormField).first, 'Luvas');
    await tester.enterText(find.byType(TextFormField).at(2), '49,90');
    await tester.enterText(find.byKey(const Key('product_cycle_days')), '30');
    await tester.tap(find.text(t.sales.products.save));
    await tester.pumpAndSettle();

    expect(fake.createdCycleDays, 30);
  });

  testWidgets('an absurd cycle blocks the save', (tester) async {
    final fake = await pump(tester);

    await tester.enterText(find.byType(TextFormField).first, 'Luvas');
    await tester.enterText(find.byType(TextFormField).at(2), '49,90');
    await tester.enterText(
        find.byKey(const Key('product_cycle_days')), '99999');
    await tester.tap(find.text(t.sales.products.save));
    await tester.pumpAndSettle();

    expect(find.text(t.sales.products.cycle_invalid), findsOneWidget);
    expect(fake.createdCycleDays, isNull);
  });

  testWidgets('editing shows the stored cycle and saves the new one',
      (tester) async {
    final fake = await pump(tester, editId: 'p1', seeded: const [
      ProductModel(
          id: 'p1',
          accountId: 'a1',
          name: 'Luvas',
          priceCents: 4990,
          consumptionCycleDays: 30),
    ]);

    expect(find.widgetWithText(TextFormField, '30'), findsOneWidget);

    await tester.enterText(find.byKey(const Key('product_cycle_days')), '45');
    await tester.tap(find.text(t.sales.products.save));
    await tester.pumpAndSettle();

    expect(fake.updatedCycleDays, 45);
  });

  testWidgets('currency is a closed choice, not free text', (tester) async {
    await pump(tester);
    expect(find.byKey(const Key('product_currency')), findsOneWidget);
    // No text field accepts "reais" any more.
    expect(find.widgetWithText(DropdownButtonFormField<String>, 'BRL'),
        findsOneWidget);
  });
}
