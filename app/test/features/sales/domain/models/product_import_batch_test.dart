import 'package:flutter_test/flutter_test.dart';

import 'package:app/features/sales/domain/models/product_import.dart';

// Several catalogues read in one go. The contract: the counts are the SUM of
// what the server counted per file (never re-derived from the rows), every row
// remembers which file it came from, and a file that failed does not discard
// the ones that worked.

ProductImportRow _row(String name, {String problem = '', int page = 0}) =>
    ProductImportRow(line: 1, name: name, problem: problem, page: page);

ProductImportPreview _preview(
  List<ProductImportRow> rows, {
  required int importable,
  int problems = 0,
  int already = 0,
}) =>
    ProductImportPreview(
      rows: rows,
      importable: importable,
      problems: problems,
      alreadyInCatalogue: already,
    );

void main() {
  test('merging sums the server counts and tags every row with its file', () {
    final batch = ProductImportBatch.merge(
      [
        (
          filename: 'catalogo-a.pdf',
          preview: _preview([_row('Monitor'), _row('Ventilador')],
              importable: 2, already: 1),
        ),
        (
          filename: 'catalogo-b.pdf',
          preview: _preview([_row('Bomba', problem: 'preço ambíguo')],
              importable: 0, problems: 1),
        ),
      ],
      const [],
    );

    expect(batch.preview.rows.map((r) => r.name),
        ['Monitor', 'Ventilador', 'Bomba']);
    expect(batch.preview.rows.map((r) => r.source),
        ['catalogo-a.pdf', 'catalogo-a.pdf', 'catalogo-b.pdf']);
    expect(batch.preview.importable, 2);
    expect(batch.preview.problems, 1);
    expect(batch.preview.alreadyInCatalogue, 1);
    expect(batch.failed, isEmpty);
  });

  test('a file that failed is named, and the readable ones survive', () {
    final batch = ProductImportBatch.merge(
      [
        (
          filename: 'ok.pdf',
          preview: _preview([_row('Monitor')], importable: 1),
        ),
      ],
      const [
        ProductImportFailure(
            filename: 'corrompido.pdf', reason: 'não deu pra ler o PDF'),
        ProductImportFailure(
            filename: 'gigante.pdf', reason: 'arquivo grande demais'),
      ],
    );

    expect(batch.preview.rows, hasLength(1));
    expect(batch.preview.importable, 1);
    expect(
        batch.failed.map((f) => f.filename), ['corrompido.pdf', 'gigante.pdf']);
    expect(batch.failed.map((f) => f.reason),
        ['não deu pra ler o PDF', 'arquivo grande demais']);
  });

  test('one broken service collapses to ONE reason, not N file names', () {
    // The antivirus being down answered the same 503 for all eighteen files.
    // Listing eighteen names and no cause read like eighteen broken PDFs.
    const down = 'o antivírus não está configurado';
    final batch = ProductImportBatch.merge(
      const [],
      [
        for (final n in ['a.pdf', 'b.pdf', 'c.pdf'])
          ProductImportFailure(filename: n, reason: down),
      ],
    );

    expect(batch.failed, hasLength(3));
    expect(batch.reasons, [down], reason: 'one cause, stated once');
    expect(batch.preview.rows, isEmpty);
  });

  test('the same product in two catalogues is kept twice, never dropped', () {
    // A silently dropped line is a product the person believes they imported.
    // Both copies reach the review with their source so the human decides.
    final batch = ProductImportBatch.merge(
      [
        (
          filename: 'a.pdf',
          preview: _preview([_row('Monitor multiparâmetro')], importable: 1),
        ),
        (
          filename: 'b.pdf',
          preview: _preview([_row('Monitor multiparâmetro')], importable: 1),
        ),
      ],
      const [],
    );

    expect(batch.preview.rows, hasLength(2));
    expect(batch.preview.rows.map((r) => r.source), ['a.pdf', 'b.pdf']);
    expect(batch.preview.importable, 2);
  });

  test('withSource keeps every other field intact', () {
    const original = ProductImportRow(
      line: 7,
      name: 'Bomba de infusão',
      description: 'volumétrica',
      priceCents: 123456,
      currency: 'brl',
      sku: 'SKU-9',
      brand: 'Acme',
      model: 'X1',
      unit: 'un',
      page: 4,
      duplicate: true,
    );
    final tagged = original.withSource('catalogo.pdf');

    expect(tagged.source, 'catalogo.pdf');
    expect(tagged.line, 7);
    expect(tagged.name, 'Bomba de infusão');
    expect(tagged.priceCents, 123456);
    expect(tagged.sku, 'SKU-9');
    expect(tagged.page, 4);
    expect(tagged.duplicate, isTrue);
    // The source is client-side only — confirmation must not carry it.
    expect(tagged.toJson().containsKey('source'), isFalse);
  });
}
