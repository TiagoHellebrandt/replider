import 'package:flutter_test/flutter_test.dart';
import 'package:app/features/billing/domain/models/addon_pack_model.dart';

void main() {
  group('AddonPackModel', () {
    final fullJson = <String, dynamic>{
      'id': 'pack-1',
      'name': 'Extra Tokens 50K',
      'tokens': 50000,
      'price': 2990,
      'currency': 'brl',
    };

    test('fromJson should parse all fields correctly', () {
      final pack = AddonPackModel.fromJson(fullJson);

      expect(pack.id, 'pack-1');
      expect(pack.name, 'Extra Tokens 50K');
      expect(pack.tokens, 50000);
      expect(pack.price, 2990);
    });

    test('toJson should produce correct snake_case map', () {
      final pack = AddonPackModel.fromJson(fullJson);
      final json = pack.toJson();

      expect(json['id'], 'pack-1');
      expect(json['name'], 'Extra Tokens 50K');
      expect(json['tokens'], 50000);
      expect(json['price'], 2990);
    });

    test('fromJson and toJson should be symmetric', () {
      final pack = AddonPackModel.fromJson(fullJson);
      final roundTripped = pack.toJson();

      expect(roundTripped, fullJson);
    });

    test('formattedPrice should format price in BRL', () {
      final pack = AddonPackModel.fromJson(fullJson);

      // Exact cents, no rounding — the old toStringAsFixed(0) format
      // displayed 2990 centavos as "R$ 30", overstating the price.
      expect(pack.formattedPrice, r'R$ 29,90');
    });

    test('formattedTokens should format tokens in K', () {
      final pack = AddonPackModel.fromJson(fullJson);

      expect(pack.formattedTokens, '50K');
    });

    test('formattedPrice formats USD packs in dollars', () {
      final pack = AddonPackModel.fromJson(<String, dynamic>{
        'id': 'addon_p',
        'name': 'Top-up S',
        'tokens': 25000,
        'price': 900,
        'currency': 'usd',
      });

      expect(pack.formattedPrice, r'$9.00');
      expect(pack.currency, 'usd');
    });

    test('formattedPrice should handle small amounts', () {
      final pack = AddonPackModel.fromJson(<String, dynamic>{
        'id': 'pack-2',
        'name': 'Starter Pack',
        'tokens': 5000,
        'price': 990,
      });

      expect(pack.formattedPrice, r'R$ 9,90');
      expect(pack.formattedTokens, '5K');
    });
  });
}
