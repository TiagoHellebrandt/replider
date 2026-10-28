import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/features/sales/domain/models/customer_import.dart';
import 'package:app/features/sales/presentation/screens/customer_import_screen.dart';
import 'package:app/i18n/strings.g.dart';

/// The review is the whole safeguard. It has to state what it read, refuse to
/// hide a line it could not, and never promise to import more than it will.

void main() {
  setUp(() => LocaleSettings.setLocale(AppLocale.en));

  CustomerImportPreview previewOf(
    List<CustomerImportRow> rows, {
    int? importable,
    int? problems,
    int alreadyInBase = 0,
  }) =>
      CustomerImportPreview(
        rows: rows,
        // Server-counted in production. Defaulting from the rows here keeps the
        // fixtures short, and the tests that care pass the numbers explicitly.
        importable: importable ?? rows.where((r) => r.importable).length,
        problems: problems ?? rows.where((r) => !r.importable).length,
        alreadyInBase: alreadyInBase,
      );

  // A mutable holder, not a returned int: an `int` handed back at pump time is
  // a copy, so `expect(confirms, 0)` after a tap would pass whether or not the
  // button fired — a test that cannot fail.
  Future<List<int>> pump(
    WidgetTester tester,
    CustomerImportPreview preview, {
    bool busy = false,
  }) async {
    final confirms = <int>[];
    await tester.pumpWidget(MaterialApp(
      theme: AppTheme.darkTheme,
      home: Scaffold(
        body: SingleChildScrollView(
          child: CustomerImportReview(
            preview: preview,
            busy: busy,
            onConfirm: () => confirms.add(1),
          ),
        ),
      ),
    ));
    return confirms;
  }

  testWidgets('says what it read before anything is created', (tester) async {
    await pump(
        tester,
        previewOf([
          const CustomerImportRow(
              line: 2,
              company: 'Clínica Aurora',
              name: 'Marina',
              phone: '+5551988887766'),
          const CustomerImportRow(
              line: 3, company: 'Hospital Vale', phone: '+5551988887755'),
        ]));

    expect(find.byKey(const Key('customer_import_summary')), findsOneWidget);
    expect(find.text(t.customer_import.found(total: '2', importable: '2')),
        findsOneWidget);
    expect(find.text(t.customer_import.confirm(count: '2')), findsOneWidget);
  });

  testWidgets('every row shows the address, not just the name', (tester) async {
    await pump(
        tester,
        previewOf([
          const CustomerImportRow(
              line: 2, company: 'Clínica Aurora', phone: '+5551988887766'),
        ]));

    // A list of names cannot catch the mistake the review exists to catch: two
    // clinics with similar names, or a number typed wrong.
    expect(
      find.text(
          t.customer_import.line_address(line: '2', address: '+5551988887766')),
      findsOneWidget,
    );
  });

  testWidgets('a row it could not read is shown, not dropped', (tester) async {
    await pump(
        tester,
        previewOf([
          const CustomerImportRow(
              line: 2, company: 'Clínica Aurora', phone: '+5551988887766'),
          const CustomerImportRow(
              line: 3,
              company: 'Clínica Sol',
              problem: 'telefone não reconhecido'),
        ]));

    // Both lines are on screen. A line dropped here is a customer she believes
    // she imported.
    expect(find.byKey(const Key('customer_import_row_2')), findsOneWidget);
    expect(find.byKey(const Key('customer_import_row_3')), findsOneWidget);
    expect(
      find.text(t.customer_import
          .line_problem(line: '3', problem: 'telefone não reconhecido')),
      findsOneWidget,
    );
    // And the button promises only what it will actually do.
    expect(find.text(t.customer_import.confirm(count: '1')), findsOneWidget);
  });

  testWidgets('customers already in the base are a fact, not a fault',
      (tester) async {
    await pump(
        tester,
        previewOf([
          const CustomerImportRow(
              line: 2,
              company: 'Clínica Aurora',
              problem: 'já está na sua base'),
        ], alreadyInBase: 1));

    // Its own line, separate from problems: re-importing an updated list is
    // normal, and what she needs to know is that the existing contact stays.
    expect(find.byKey(const Key('customer_import_already')), findsOneWidget);
    expect(
        find.text(t.customer_import.already_here(count: '1')), findsOneWidget);
  });

  testWidgets('nothing importable disables the button and says why',
      (tester) async {
    final confirms = await pump(
        tester,
        previewOf([
          const CustomerImportRow(
              line: 2,
              company: 'Clínica Sol',
              problem: 'sem telefone nem e-mail'),
        ]));

    expect(find.byKey(const Key('customer_import_nothing')), findsOneWidget);
    await tester.tap(find.byKey(const Key('customer_import_confirm')));
    await tester.pump();
    expect(confirms, isEmpty, reason: 'a disabled import must not fire');
  });

  testWidgets('confirming fires exactly once', (tester) async {
    final confirms = await pump(
        tester,
        previewOf([
          const CustomerImportRow(
              line: 2, company: 'Clínica Aurora', phone: '+5551988887766'),
        ]));

    await tester.tap(find.byKey(const Key('customer_import_confirm')));
    await tester.pump();
    expect(confirms, hasLength(1));
  });

  testWidgets('an upload in flight cannot be confirmed twice', (tester) async {
    final confirms = await pump(
      tester,
      previewOf([
        const CustomerImportRow(
            line: 2, company: 'Clínica Aurora', phone: '+5551988887766'),
      ]),
      busy: true,
    );

    await tester.tap(find.byKey(const Key('customer_import_confirm')));
    await tester.pump();
    expect(confirms, isEmpty);
  });

  testWidgets('a row with no name at all still has a label', (tester) async {
    await pump(
        tester,
        previewOf([
          const CustomerImportRow(line: 2, phone: '+5551988887766'),
        ]));

    // "" in the list is a row nobody can check.
    expect(find.text(t.customer_import.blank_name), findsOneWidget);
  });
}
