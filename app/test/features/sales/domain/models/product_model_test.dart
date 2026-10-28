import 'package:flutter_test/flutter_test.dart';
import 'package:app/features/sales/domain/models/product_model.dart';

void main() {
  group('ProductModel', () {
    final fullJson = <String, dynamic>{
      'id': 'prod-1',
      'account_id': 'acc-1',
      'name': 'Premium Plan',
      'description': 'Yearly subscription',
      'price_cents': 1990,
      'currency': 'BRL',
      'attributes': {'sku': 'PREM-1'},
      'active': true,
      'created_at': '2026-01-01T00:00:00Z',
      'updated_at': '2026-01-02T00:00:00Z',
    };

    test('fromJson parses all snake_case fields', () {
      final p = ProductModel.fromJson(fullJson);

      expect(p.id, 'prod-1');
      expect(p.accountId, 'acc-1');
      expect(p.name, 'Premium Plan');
      expect(p.description, 'Yearly subscription');
      expect(p.priceCents, 1990);
      expect(p.currency, 'BRL');
      expect(p.attributes, {'sku': 'PREM-1'});
      expect(p.active, true);
      expect(p.createdAt, '2026-01-01T00:00:00Z');
      expect(p.updatedAt, '2026-01-02T00:00:00Z');
    });

    test('price converts cents to major units', () {
      final p = ProductModel.fromJson(fullJson);
      expect(p.price, 19.90);
    });

    test('toJson round-trips snake_case keys', () {
      final p = ProductModel.fromJson(fullJson);
      final json = p.toJson();
      expect(json['account_id'], 'acc-1');
      expect(json['price_cents'], 1990);
      expect(json['created_at'], '2026-01-01T00:00:00Z');
    });

    test('tolerates minimal payload with defaults', () {
      final p = ProductModel.fromJson(<String, dynamic>{
        'id': 'prod-2',
        'account_id': 'acc-1',
        'name': 'Basic',
      });
      expect(p.description, '');
      expect(p.priceCents, 0);
      expect(p.currency, 'BRL');
      expect(p.active, true);
      expect(p.attributes, isNull);
    });
  });
}
