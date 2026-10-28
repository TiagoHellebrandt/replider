import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import 'package:app/core/utils/currency_format.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/sales/domain/models/sales_import.dart';
import 'package:app/features/sales/presentation/screens/sales_import_screen.dart';
import 'package:app/i18n/strings.g.dart';

/// The review is the whole safeguard — same contract as the customer import:
/// it has to state what it read, refuse to hide a line it could not, and never
/// promise to record more than it will.

class _NoAuth extends AuthController {
  @override
  FutureOr<UserModel?> build() async => null;
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.en));

  SalesImportRow row({
    int line = 2,
    String customer = 'Clínica Aurora',
    String contactId = 'c1',
    String contactName = 'Clínica Aurora',
    String product = '',
    String date = '10/03/2026',
    String soldAt = '2026-03-10T00:00:00Z',
    int quantity = 0,
    int grossCents = 150000,
    String problem = '',
    bool duplicate = false,
  }) =>
      SalesImportRow(
        line: line,
        customer: customer,
        contactId: contactId,
        contactName: contactName,
        product: product,
        date: date,
        soldAt: soldAt,
        quantity: quantity,
        grossCents: grossCents,
        problem: problem,
        duplicate: duplicate,
      );

  SalesImportPreview previewOf(
    List<SalesImportRow> rows, {
    int? importable,
    int? problems,
    int alreadyRecorded = 0,
    int unknownCustomers = 0,
  }) =>
      SalesImportPreview(
        rows: rows,
        // Server-counted in production. Defaulting from the rows keeps the
        // fixtures short; tests that care pass the numbers explicitly.
        importable: importable ?? rows.where((r) => r.importable).length,
        problems: problems ?? rows.where((r) => !r.importable).length,
        alreadyRecorded: alreadyRecorded,
        unknownCustomers: unknownCustomers,
      );

  // A mutable holder, not a returned int: an `int` handed back at pump time
  // is a copy, so a post-tap expect would pass whether or not the button
  // fired — a test that cannot fail.
  Future<List<int>> pump(
    WidgetTester tester,
    SalesImportPreview preview, {
    bool busy = false,
  }) async {
    final confirms = <int>[];
    await tester.pumpWidget(ProviderScope(
      overrides: [authControllerProvider.overrideWith(_NoAuth.new)],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: Scaffold(
          body: SingleChildScrollView(
            child: SalesImportReview(
              preview: preview,
              busy: busy,
              onConfirm: () => confirms.add(1),
            ),
          ),
        ),
      ),
    ));
    return confirms;
  }

  testWidgets('the screen renders the door and the column hint',
      (tester) async {
    final router = GoRouter(
      initialLocation: '/sales/import',
      routes: [
        GoRoute(
            path: '/sales/import',
            builder: (_, __) => const SalesImportScreen()),
        GoRoute(
            path: '/funnel',
            builder: (_, __) => const Scaffold(body: Text('funnel'))),
      ],
    );
    await tester.pumpWidget(ProviderScope(
      overrides: [authControllerProvider.overrideWith(_NoAuth.new)],
      child:
          MaterialApp.router(theme: AppTheme.darkTheme, routerConfig: router),
    ));
    await tester.pumpAndSettle();

    expect(find.text(t.sales_import.title), findsOneWidget);
    expect(find.byKey(const Key('sales_import_help')), findsOneWidget);
    expect(find.byKey(const Key('sales_import_pick')), findsOneWidget);
    // Nothing picked yet: no review, no result.
    expect(find.byKey(const Key('sales_import_summary')), findsNothing);
    expect(find.byKey(const Key('sales_import_result')), findsNothing);
  });

  testWidgets('says what it read before anything is recorded', (tester) async {
    await tester.runAsync(() async {});
    await pump(
        tester,
        previewOf([
          row(line: 2),
          row(line: 3, customer: 'Hospital Vale', contactName: 'Hospital Vale'),
        ]));

    expect(find.byKey(const Key('sales_import_summary')), findsOneWidget);
    expect(find.text(t.sales_import.found(total: '2', importable: '2')),
        findsOneWidget);
    expect(find.text(t.sales_import.confirm(count: '2')), findsOneWidget);
  });

  testWidgets('every row shows the date and the money, not just the name',
      (tester) async {
    await pump(
        tester,
        previewOf([
          row(product: 'Soro Fisiológico', grossCents: 150000),
        ]));

    // A list of names cannot catch the mistake this review exists to catch:
    // a value parsed 100x wrong, or a sale filed on the wrong day.
    final money = formatCurrencyCents(150000, 'brl');
    expect(
      find.text(t.sales_import.line_detail(
          line: '2', detail: '10/03/2026 · $money · Soro Fisiológico')),
      findsOneWidget,
    );
  });

  testWidgets('a row it could not read is shown, not dropped', (tester) async {
    await pump(
        tester,
        previewOf([
          row(line: 2),
          row(
              line: 3,
              customer: 'Clínica Sol',
              contactId: '',
              contactName: '',
              soldAt: '0001-01-01T00:00:00Z',
              problem: 'data não reconhecida (use dd/mm/aaaa)'),
        ]));

    // Both lines are on screen. A line dropped here is a sale she believes
    // she recorded.
    expect(find.byKey(const Key('sales_import_row_2')), findsOneWidget);
    expect(find.byKey(const Key('sales_import_row_3')), findsOneWidget);
    expect(
      find.text(t.sales_import.line_problem(
          line: '3', problem: 'data não reconhecida (use dd/mm/aaaa)')),
      findsOneWidget,
    );
    // And the button promises only what it will actually do.
    expect(find.text(t.sales_import.confirm(count: '1')), findsOneWidget);
  });

  testWidgets('a sale already recorded is a fact, not a fault', (tester) async {
    await pump(
        tester,
        previewOf([
          row(problem: 'venda já registrada', duplicate: true),
        ], alreadyRecorded: 1));

    // Its own line, separate from problems: a re-export of the same ERP
    // report is normal, and what she needs to know is that the recorded
    // sale stays.
    expect(find.byKey(const Key('sales_import_already')), findsOneWidget);
    expect(
        find.text(t.sales_import.already_recorded(count: '1')), findsOneWidget);
  });

  testWidgets('unknown customers point at the customer import first',
      (tester) async {
    await pump(
        tester,
        previewOf([
          row(
              contactId: '',
              contactName: '',
              problem:
                  'cliente não encontrado na base — importe os clientes primeiro'),
        ], unknownCustomers: 1));

    expect(find.byKey(const Key('sales_import_unknown')), findsOneWidget);
    expect(find.text(t.sales_import.unknown_customers(count: '1')),
        findsOneWidget);
  });

  testWidgets('nothing importable disables the button and says why',
      (tester) async {
    final confirms = await pump(
        tester,
        previewOf([
          row(contactId: '', contactName: '', problem: 'sem valor'),
        ]));

    expect(find.byKey(const Key('sales_import_nothing')), findsOneWidget);
    await tester.tap(find.byKey(const Key('sales_import_confirm')));
    await tester.pump();
    expect(confirms, isEmpty, reason: 'a disabled import must not fire');
  });

  testWidgets('confirming fires exactly once', (tester) async {
    final confirms = await pump(tester, previewOf([row()]));

    await tester.tap(find.byKey(const Key('sales_import_confirm')));
    await tester.pump();
    expect(confirms, hasLength(1));
  });

  testWidgets('an upload in flight cannot be confirmed twice', (tester) async {
    final confirms = await pump(tester, previewOf([row()]), busy: true);

    await tester.tap(find.byKey(const Key('sales_import_confirm')));
    await tester.pump();
    expect(confirms, isEmpty);
  });

  testWidgets('a row with no customer at all still has a label',
      (tester) async {
    await pump(
        tester,
        previewOf([
          row(
              customer: '',
              contactId: '',
              contactName: '',
              problem: 'sem cliente'),
        ]));

    // "" in the list is a row nobody can check.
    expect(find.text(t.sales_import.blank_customer), findsOneWidget);
  });

  testWidgets('the result states both numbers and opens the carteira',
      (tester) async {
    final taps = <int>[];
    await tester.pumpWidget(MaterialApp(
      theme: AppTheme.darkTheme,
      home: Scaffold(
        body: SalesImportResultNotice(
          result: const SalesImportResult(created: 5, skipped: 2),
          onViewPortfolio: () => taps.add(1),
        ),
      ),
    ));

    // Both numbers, always: a count that does not match what she confirmed
    // is a question.
    expect(find.text(t.sales_import.skipped(count: '5', skipped: '2')),
        findsOneWidget);

    await tester.tap(find.byKey(const Key('sales_import_view_portfolio')));
    await tester.pump();
    expect(taps, hasLength(1));
  });

  testWidgets('a clean import needs no caveat', (tester) async {
    await tester.pumpWidget(MaterialApp(
      theme: AppTheme.darkTheme,
      home: Scaffold(
        body: SalesImportResultNotice(
          result: const SalesImportResult(created: 3, skipped: 0),
          onViewPortfolio: () {},
        ),
      ),
    ));

    expect(find.text(t.sales_import.result(count: '3')), findsOneWidget);
  });
}
