import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/features/sales/data/datasources/sales_remote_datasource.dart';
import 'package:app/features/sales/domain/models/sales_import.dart';
import 'package:app/features/sales/presentation/controllers/sales_import_controller.dart';

/// The controller binds the two-call contract of `sale_import_handler.go`:
/// preview reads the file and reports rows with per-row problems; confirm
/// sends back the ROWS the person approved and reports created/skipped.

class _FakeSalesImportDataSource extends SalesImportDataSource {
  _FakeSalesImportDataSource() : super(Dio());

  Map<String, dynamic> previewResponse = const {};
  Map<String, dynamic> importResponse = const {};
  Uint8List? sentBytes;
  String? sentFilename;
  List<Map<String, dynamic>>? sentRows;

  @override
  Future<Map<String, dynamic>> previewSalesImport({
    required Uint8List bytes,
    required String filename,
  }) async {
    sentBytes = bytes;
    sentFilename = filename;
    return previewResponse;
  }

  @override
  Future<Map<String, dynamic>> importSales(
      List<Map<String, dynamic>> rows) async {
    sentRows = rows;
    return importResponse;
  }
}

void main() {
  late _FakeSalesImportDataSource fake;
  late ProviderContainer container;

  setUp(() {
    fake = _FakeSalesImportDataSource();
    container = ProviderContainer(overrides: [
      salesImportDataSourceProvider.overrideWithValue(fake),
    ]);
    addTearDown(container.dispose);
  });

  test('preview uploads the picked file and parses rows and counters',
      () async {
    fake.previewResponse = {
      'rows': [
        {
          'line': 2,
          'customer': 'Clínica Aurora',
          'product': 'Soro Fisiológico',
          'date': '10/03/2026',
          'sold_at': '2026-03-10T00:00:00Z',
          'quantity': 20,
          'gross_cents': 150000,
          'contact_id': 'c1',
          'contact_name': 'Clínica Aurora',
          'product_id': 'p1',
        },
        {
          'line': 3,
          'customer': 'Hospital Vale',
          'product': '',
          'date': '99/99/2026',
          // Go marshals its zero time as year 1: "no date" in a timestamp.
          'sold_at': '0001-01-01T00:00:00Z',
          'quantity': 0,
          'gross_cents': 0,
          'problem': 'data não reconhecida (use dd/mm/aaaa)',
        },
        {
          'line': 4,
          'customer': 'Clínica Aurora',
          'date': '10/03/2026',
          'sold_at': '2026-03-10T00:00:00Z',
          'gross_cents': 150000,
          'contact_id': 'c1',
          'contact_name': 'Clínica Aurora',
          'problem': 'venda já registrada',
          'duplicate': true,
        },
      ],
      'importable': 1,
      'problems': 2,
      'already_recorded': 1,
      'unknown_customers': 0,
    };

    final actions = container.read(salesImportActionsProvider);
    final bytes = Uint8List.fromList([1, 2, 3]);
    final preview = await actions.preview(bytes: bytes, filename: 'vendas.csv');

    expect(fake.sentBytes, bytes);
    expect(fake.sentFilename, 'vendas.csv');

    expect(preview.rows, hasLength(3));
    expect(preview.importable, 1);
    expect(preview.problems, 2);
    expect(preview.alreadyRecorded, 1);
    expect(preview.unknownCustomers, 0);

    final good = preview.rows[0];
    expect(good.importable, isTrue);
    expect(good.hasSoldAt, isTrue);
    expect(good.displayName, 'Clínica Aurora');
    expect(good.productId, 'p1');
    expect(good.quantity, 20);

    final bad = preview.rows[1];
    expect(bad.importable, isFalse);
    expect(bad.hasSoldAt, isFalse, reason: 'the Go zero time is not a date');
    expect(bad.problem, isNotEmpty);
    // No resolution came back, so the review falls back to the typed cell.
    expect(bad.displayName, 'Hospital Vale');

    final dup = preview.rows[2];
    expect(dup.duplicate, isTrue);
    expect(dup.importable, isFalse);
  });

  test('confirm sends back the rows as approved and parses the result',
      () async {
    fake.importResponse = {'created': 2, 'skipped': 1};

    final actions = container.read(salesImportActionsProvider);
    final result = await actions.confirm(const [
      SalesImportRow(
        line: 2,
        customer: 'Clínica Aurora',
        product: 'Soro Fisiológico',
        date: '10/03/2026',
        soldAt: '2026-03-10T00:00:00Z',
        quantity: 20,
        grossCents: 150000,
        contactId: 'c1',
        contactName: 'Clínica Aurora',
        productId: 'p1',
      ),
    ]);

    expect(result.created, 2);
    expect(result.skipped, 1);

    final sent = fake.sentRows!;
    expect(sent, hasLength(1));
    // What she approved travels back; the server re-derives resolution, so
    // the ids the preview computed are deliberately not part of the payload.
    expect(sent.first, {
      'line': 2,
      'customer': 'Clínica Aurora',
      'product': 'Soro Fisiológico',
      'date': '10/03/2026',
      'sold_at': '2026-03-10T00:00:00Z',
      'quantity': 20,
      'gross_cents': 150000,
      'problem': '',
    });
  });

  test('an empty body from the server parses to zeros, not a crash', () async {
    fake.previewResponse = const {};
    final actions = container.read(salesImportActionsProvider);
    final preview =
        await actions.preview(bytes: Uint8List(0), filename: 'x.csv');

    expect(preview.rows, isEmpty);
    expect(preview.importable, 0);
    expect(preview.problems, 0);
  });
}
