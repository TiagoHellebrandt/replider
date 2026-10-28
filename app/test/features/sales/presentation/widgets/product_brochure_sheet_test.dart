import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/sales/domain/models/product_model.dart';
import 'package:app/features/sales/presentation/widgets/product_brochure_sheet.dart';
import 'package:app/i18n/strings.g.dart';

// The import was already saving brand, model, code and the source page in the
// product's attributes — and the edit screen rendered none of it. Somebody who
// had just imported ninety brochures opened one, saw a name, a short
// description and "R$ 0,00", and concluded the reader had thrown the document
// away. Most of it was there. These pin that it is now on screen.

ProductModel _product({
  Map<String, dynamic>? attributes,
  String? sourceId,
  int sourcePage = 0,
}) =>
    ProductModel(
      id: 'p1',
      accountId: 'acc1',
      name: 'Incubadora Line Duo G',
      attributes: attributes,
      sourceId: sourceId,
      sourcePage: sourcePage,
    );

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  Future<void> pump(WidgetTester tester, ProductModel p,
      {HttpClientAdapter? adapter}) async {
    final dio = Dio(BaseOptions(baseUrl: 'https://api.test/v1'));
    dio.httpClientAdapter = adapter ?? _EmptyImagesAdapter();
    await tester.pumpWidget(ProviderScope(
      overrides: [dioProvider.overrideWithValue(dio)],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: Scaffold(
          body: SingleChildScrollView(child: ProductBrochureSheet(product: p)),
        ),
      ),
    ));
    await tester.pump();
  }

  testWidgets('shows what the brochure said', (tester) async {
    await pump(
      tester,
      _product(attributes: {
        'brand': 'Olidef',
        'model': 'LINE DUO G',
        'sku': '10227189024',
        'specs': 'Alimentação 127/220V. Faixa 20–37 °C. Peso 95 kg.',
        'accessories': ['Suporte de soro', 'Balança integrada'],
        'standards': ['NBR IEC 60601-2-19'],
      }),
    );

    expect(find.byKey(const Key('product_brochure_sheet')), findsOneWidget);
    expect(find.text('Olidef'), findsOneWidget);
    expect(find.text('LINE DUO G'), findsOneWidget);
    expect(find.text('10227189024'), findsOneWidget);
    expect(find.byKey(const Key('product_specs')), findsOneWidget);
    expect(find.textContaining('Suporte de soro'), findsOneWidget);
    expect(find.textContaining('NBR IEC 60601-2-19'), findsOneWidget);
  });

  // A product typed by hand or imported from a spreadsheet has no sheet and no
  // file. An empty bordered box promising "from the manufacturer's brochure"
  // would be a claim about a document that does not exist.
  testWidgets('renders nothing when there is no brochure', (tester) async {
    await pump(tester, _product());
    expect(find.byKey(const Key('product_brochure_sheet')), findsNothing);
  });

  testWidgets('offers the original only when a file is archived',
      (tester) async {
    // Attributes but no source: CSV import carried a brand column, and there
    // is no PDF behind it to open.
    await pump(tester, _product(attributes: {'brand': 'Olidef'}));
    expect(find.byKey(const Key('product_brochure_sheet')), findsOneWidget);
    expect(find.byKey(const Key('product_open_brochure')), findsNothing);

    await pump(
        tester,
        _product(
            attributes: {'brand': 'Olidef'}, sourceId: 's1', sourcePage: 4));
    expect(find.byKey(const Key('product_open_brochure')), findsOneWidget);
    // The page is the point: "open the brochure" without it sends the person
    // scrolling a forty-page catalogue.
    expect(
        find.text(t.sales.products.open_brochure(page: '4')), findsOneWidget);
  });

  test('the accessors survive a malformed attributes bag', () {
    // The bag is free-form JSON from the server; a list of numbers or a null
    // must not throw inside a build.
    final p = _product(attributes: {
      'brand': 42,
      'accessories': [1, null, 'Suporte'],
      'standards': 'não é lista',
    });
    expect(p.brand, '');
    expect(p.accessories, ['Suporte']);
    expect(p.standards, isEmpty);
    expect(p.hasSheet, isTrue); // the one good accessory still counts
  });

  // Three states, and they must look different. A spinner shown as "nothing
  // here" makes a working gallery read as broken for the second it loads.
  testWidgets('the gallery shows the pictures from the brochure',
      (tester) async {
    await pump(
      tester,
      _product(attributes: {'brand': 'Olidef'}, sourceId: 's1', sourcePage: 4),
      adapter: _ImagesAdapter(
          '[{"id":"i1","page":4,"width":600,"height":600,"url":"https://signed/x.jpg"}]'),
    );
    // Before the response lands: a spinner, not an empty state.
    expect(find.byKey(const Key('product_images_loading')), findsOneWidget);

    await tester.pump(const Duration(milliseconds: 50));
    await tester.pump();
    expect(find.byKey(const Key('product_images_strip')), findsOneWidget);
    expect(find.byKey(const Key('product_images_empty')), findsNothing);
  });

  testWidgets('a page with no usable artwork says so', (tester) async {
    await pump(
      tester,
      _product(attributes: {'brand': 'Olidef'}, sourceId: 's1', sourcePage: 4),
    );
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('product_images_empty')), findsOneWidget);
    expect(find.byKey(const Key('product_images_strip')), findsNothing);
  });

  // Pictures are a convenience on top of the catalogue: a failure hides the
  // strip rather than shouting over a screen opened to read a spec sheet.
  testWidgets('a failed image call hides the strip and says nothing',
      (tester) async {
    await pump(
      tester,
      _product(attributes: {'brand': 'Olidef'}, sourceId: 's1', sourcePage: 4),
      adapter:
          _ImagesAdapter('{"code":"PRODUCT_SOURCE_NOT_FOUND"}', status: 404),
    );
    await tester.pump(const Duration(milliseconds: 50));
    expect(find.byKey(const Key('product_images_strip')), findsNothing);
    expect(find.byKey(const Key('product_images_empty')), findsNothing);
    // And the sheet it sits in is still there.
    expect(find.byKey(const Key('product_brochure_sheet')), findsOneWidget);
  });

  // No archived file means no gallery to ask about — and no request.
  testWidgets('a CSV product never asks for pictures', (tester) async {
    await pump(tester, _product(attributes: {'brand': 'Olidef'}));
    await tester.pump(const Duration(milliseconds: 50));
    expect(find.byKey(const Key('product_images_loading')), findsNothing);
    expect(find.byKey(const Key('product_images_empty')), findsNothing);
  });

  test('zero price is unknown, not free', () {
    expect(_product().hasPrice, isFalse);
    expect(
      ProductModel(id: 'p', accountId: 'a', name: 'x', priceCents: 100)
          .hasPrice,
      isTrue,
    );
  });
}

/// Answers the images call with a fixed body.
class _ImagesAdapter implements HttpClientAdapter {
  _ImagesAdapter(this.body, {this.status = 200});
  final String body;
  final int status;

  @override
  Future<ResponseBody> fetch(RequestOptions options,
      Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async {
    return ResponseBody.fromString(body, status, headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType]
    });
  }

  @override
  void close({bool force = false}) {}
}

class _EmptyImagesAdapter extends _ImagesAdapter {
  _EmptyImagesAdapter() : super('[]');
}
