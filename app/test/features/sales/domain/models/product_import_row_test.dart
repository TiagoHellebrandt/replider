import 'package:app/features/sales/domain/models/product_import.dart';
import 'package:flutter_test/flutter_test.dart';

// The brochure fields (sku/brand/model/unit/page) travel preview → confirm and
// end up on the product's attributes server-side. A field silently dropped in
// either direction is a fact the person reviewed that never lands.
void main() {
  test('brochure fields round-trip through json', () {
    final row = ProductImportRow.fromJson(const <String, dynamic>{
      'line': 1,
      'name': 'Monitor M-10',
      'price_cents': 3250000,
      'currency': 'BRL',
      'sku': 'M10',
      'brand': 'MedTech',
      'model': 'M-10',
      'unit': 'unidade',
      'page': 3,
    });

    expect(row.sku, 'M10');
    expect(row.brand, 'MedTech');
    expect(row.page, 3);
    expect(row.importable, isTrue);

    final json = row.toJson();
    expect(json['sku'], 'M10');
    expect(json['brand'], 'MedTech');
    expect(json['model'], 'M-10');
    expect(json['unit'], 'unidade');
    expect(json['page'], 3);
  });

  test('csv rows carry no brochure keys', () {
    const row = ProductImportRow(
      line: 2,
      name: 'Gaze estéril',
      priceCents: 4990,
      currency: 'BRL',
    );
    final json = row.toJson();
    // Absent, not empty-string: the server folds these into attributes and an
    // empty key would create a fact nobody stated.
    expect(json.containsKey('sku'), isFalse);
    expect(json.containsKey('page'), isFalse);
    expect(json.containsKey('source_id'), isFalse);
  });

  // The id of the archived PDF makes the round trip untouched.
  //
  // It did not, and the cost was invisible: this model parsed every field of
  // the preview except this one, so a product imported from a brochure landed
  // with a page number and no file. Production carried nineteen archived
  // brochures and zero products pointing at any of them, and the "open the
  // brochure" button — built, routed and wired — could never render.
  test('a brochure row carries the source id back to the server', () {
    final parsed = ProductImportRow.fromJson(<String, dynamic>{
      'line': 1,
      'name': 'Capuz de Oxigênio - Pequeno',
      'currency': 'BRL',
      'source_id': '01KZMVMGFC35QAM2K3W10WD9F1',
      'page': 1,
    });

    expect(parsed.sourceId, '01KZMVMGFC35QAM2K3W10WD9F1');
    expect(parsed.toJson()['source_id'], '01KZMVMGFC35QAM2K3W10WD9F1');
    // And it survives the labelling the review screen does when several
    // brochures are merged into one list.
    expect(
        parsed.withSource('Capuz.pdf').sourceId, '01KZMVMGFC35QAM2K3W10WD9F1');
  });

  test('a page-only row from a brochure with no readable price is importable',
      () {
    final row = ProductImportRow.fromJson(const <String, dynamic>{
      'line': 1,
      'name': 'Luva Nitrílica',
      'page': 2,
    });
    expect(row.importable, isTrue);
    expect(row.priceCents, 0);
    // …but it must SAY the price was not read. Rendering cents=0 as a green
    // "$0.00" claimed a price nobody stated — the exact opposite of "price
    // ambiguous is never guessed".
    expect(row.priceMissing, isTrue);
  });

  test('a CSV row priced at zero is not a brochure miss', () {
    const row = ProductImportRow(line: 3, name: 'Brinde', priceCents: 0);
    expect(row.priceMissing, isFalse, reason: 'page==0 means CSV, not PDF');
  });

  test('a duplicate is a fact, not a fault', () {
    final row = ProductImportRow.fromJson(const <String, dynamic>{
      'line': 4,
      'name': 'Monitor M-10',
      'problem': 'já está no catálogo',
      'duplicate': true,
    });
    expect(row.duplicate, isTrue);
    expect(row.importable, isFalse);
  });
}
