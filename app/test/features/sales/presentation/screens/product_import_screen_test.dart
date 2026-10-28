import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/sales/domain/models/product_import.dart';
import 'package:app/features/sales/presentation/screens/product_import_screen.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

// Importing a price list. The screen's job is to make a machine's reading of
// somebody's spreadsheet CHECKABLE, so these pin what it must never do: hide a
// line it could not read, or promise to import rows it will skip.

const _good = ProductImportRow(
  line: 2,
  name: 'Gaze estéril 10x10',
  priceCents: 4990,
  currency: 'BRL',
);
const _ambiguous = ProductImportRow(
  line: 3,
  name: 'Luva cirúrgica',
  problem: 'preço ambíguo: use duas casas decimais',
);
const _existing = ProductImportRow(
  line: 4,
  name: 'Swab',
  priceCents: 500,
  currency: 'BRL',
  problem: 'já está no catálogo',
);

Future<void> _pumpReview(
  WidgetTester tester,
  ProductImportPreview preview, {
  VoidCallback? onConfirm,
}) async {
  tester.view.physicalSize = const Size(1000, 2000);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(MaterialApp(
    theme: AppTheme.darkTheme,
    home: Scaffold(
      body: SingleChildScrollView(
        child: ProductImportReview(
          preview: preview,
          onConfirm: onConfirm ?? () {},
        ),
      ),
    ),
  ));
  await tester.pumpAndSettle();
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  // Before she goes looking for the file, the screen says what the file has to
  // look like. Two sentences here save the "não funcionou" support thread.
  testWidgets('the screen says what it reads before asking for a file',
      (tester) async {
    await tester.pumpWidget(ProviderScope(
      overrides: [dioProvider.overrideWithValue(Dio())],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const ProductImportScreen(),
      ),
    ));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('product_import_help')), findsOneWidget);
    expect(find.text(t.product_import.what_we_read), findsOneWidget);
    expect(find.byKey(const Key('product_import_pick')), findsOneWidget);
    // Nothing to confirm until a file has been read.
    expect(find.byKey(const Key('product_import_confirm')), findsNothing);
    expect(find.byKey(const Key('product_import_summary')), findsNothing);
    // Rendered outside the shell, so this is the only exit.
    expect(find.byKey(const Key('product_import_back')), findsOneWidget);
  });

  // Every line shows, including the ones that cannot be imported. A line dropped
  // from the list is a product she believes she imported.
  testWidgets('every line is listed, problems included', (tester) async {
    await _pumpReview(
      tester,
      const ProductImportPreview(
        rows: [_good, _ambiguous, _existing],
        importable: 1,
        problems: 2,
        alreadyInCatalogue: 1,
      ),
    );

    expect(find.byKey(const Key('product_import_row_2')), findsOneWidget);
    expect(find.byKey(const Key('product_import_row_3')), findsOneWidget);
    expect(find.byKey(const Key('product_import_row_4')), findsOneWidget);
    expect(find.text('Gaze estéril 10x10'), findsOneWidget);
  });

  // The line number is how she finds the row in the spreadsheet she still has
  // open, and the problem is written for her rather than for a log.
  testWidgets('a problem row names its line and its reason', (tester) async {
    await _pumpReview(
      tester,
      const ProductImportPreview(
        rows: [_ambiguous],
        importable: 0,
        problems: 1,
        alreadyInCatalogue: 0,
      ),
    );

    expect(
      find.text(t.product_import.line_problem(
          line: '3', problem: 'preço ambíguo: use duas casas decimais')),
      findsOneWidget,
    );
  });

  // A good row shows its price WITH the currency, like every other amount in the
  // app: cents with no currency is right until the first account selling in the
  // other one.
  testWidgets('a good row shows the price with its currency', (tester) async {
    await _pumpReview(
      tester,
      const ProductImportPreview(
        rows: [_good],
        importable: 1,
        problems: 0,
        alreadyInCatalogue: 0,
      ),
    );

    expect(
      find.text(t.product_import.line_price(line: '2', price: 'R\$ 49,90')),
      findsOneWidget,
    );
  });

  // The counts come from the server so the screen states the same numbers the
  // import will act on, and "problem" and "already there" stay separate: one
  // needs fixing, the other needs nothing.
  testWidgets('the summary separates problems from what is already there',
      (tester) async {
    await _pumpReview(
      tester,
      const ProductImportPreview(
        rows: [_good, _ambiguous, _existing],
        importable: 1,
        problems: 2,
        alreadyInCatalogue: 1,
      ),
    );

    expect(find.text(t.product_import.found(total: '3', importable: '1')),
        findsOneWidget);
    expect(find.byKey(const Key('product_import_problems')), findsOneWidget);
    expect(find.byKey(const Key('product_import_already')), findsOneWidget);
  });

  testWidgets('a clean file shows neither caveat', (tester) async {
    await _pumpReview(
      tester,
      const ProductImportPreview(
        rows: [_good],
        importable: 1,
        problems: 0,
        alreadyInCatalogue: 0,
      ),
    );

    expect(find.byKey(const Key('product_import_problems')), findsNothing);
    expect(find.byKey(const Key('product_import_already')), findsNothing);
  });

  // "Importar" over a list where half the rows have problems is a promise the
  // import will not keep, so the button counts what it will actually create.
  testWidgets('the button states how many it will create', (tester) async {
    var confirms = 0;
    await _pumpReview(
      tester,
      const ProductImportPreview(
        rows: [_good, _ambiguous],
        importable: 1,
        problems: 1,
        alreadyInCatalogue: 0,
      ),
      onConfirm: () => confirms++,
    );

    expect(find.text(t.product_import.confirm(count: '1')), findsOneWidget);
    await tester.tap(find.byKey(const Key('product_import_confirm')));
    await tester.pumpAndSettle();
    expect(confirms, 1);
  });

  // Nothing importable: the button refuses and says what to do instead of
  // swallowing the tap.
  testWidgets('with nothing importable it refuses and explains',
      (tester) async {
    var confirms = 0;
    await _pumpReview(
      tester,
      const ProductImportPreview(
        rows: [_ambiguous],
        importable: 0,
        problems: 1,
        alreadyInCatalogue: 0,
      ),
      onConfirm: () => confirms++,
    );

    final button = tester
        .widget<FilledButton>(find.byKey(const Key('product_import_confirm')));
    expect(button.onPressed, isNull);
    expect(find.byKey(const Key('product_import_nothing')), findsOneWidget);
    expect(confirms, 0);
  });

  testWidgets('a row with no name still shows, labelled', (tester) async {
    await _pumpReview(
      tester,
      const ProductImportPreview(
        rows: [ProductImportRow(line: 5, name: '', problem: 'sem nome')],
        importable: 0,
        problems: 1,
        alreadyInCatalogue: 0,
      ),
    );

    expect(find.byKey(const Key('product_import_row_5')), findsOneWidget);
    expect(find.text(t.product_import.blank_name), findsOneWidget);
  });

  // The brochure door lives NEXT to the spreadsheet one, on the same screen,
  // feeding the same review — brands ship PDFs, and a separate flow would be
  // a second import to keep honest.
  testWidgets('the PDF door is offered beside the CSV one', (tester) async {
    await tester.pumpWidget(ProviderScope(
      overrides: [dioProvider.overrideWithValue(Dio())],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const ProductImportScreen(),
      ),
    ));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('product_import_pick_pdf')), findsOneWidget);
    expect(find.text(t.product_import.pick_pdf), findsOneWidget);
    expect(find.byKey(const Key('product_import_pdf_help')), findsOneWidget);
  });

  // A brochure row points at the PAGE, not a spreadsheet line: the page is
  // how she leafs back to the folheto and checks what the model read.
  testWidgets('a brochure row shows its page of origin', (tester) async {
    await _pumpReview(
      tester,
      const ProductImportPreview(
        rows: [
          ProductImportRow(
            line: 1,
            name: 'Monitor M-10',
            priceCents: 3250000,
            currency: 'BRL',
            sku: 'M10',
            page: 3,
          ),
          ProductImportRow(
            line: 2,
            name: 'Bomba B-3',
            page: 4,
            problem: 'preço ambíguo: use duas casas decimais',
          ),
        ],
        importable: 1,
        problems: 1,
        alreadyInCatalogue: 0,
      ),
    );

    expect(
      find.textContaining(
          t.product_import.page_price(page: '3', price: '').split('—').first),
      findsOneWidget,
    );
    expect(
      find.text(t.product_import.page_problem(
          page: '4', problem: 'preço ambíguo: use duas casas decimais')),
      findsOneWidget,
    );
  });
}
