import 'package:flutter_test/flutter_test.dart';
import 'package:app/features/billing/domain/models/checkout_response.dart';

void main() {
  group('CheckoutResponse', () {
    final fullJson = <String, dynamic>{
      'url': 'https://checkout.stripe.com/pay/cs_test_abc123',
    };

    test('fromJson should parse url correctly', () {
      final response = CheckoutResponse.fromJson(fullJson);

      expect(response.url, 'https://checkout.stripe.com/pay/cs_test_abc123');
    });

    test('toJson should produce correct map', () {
      final response = CheckoutResponse.fromJson(fullJson);
      final json = response.toJson();

      expect(json['url'], 'https://checkout.stripe.com/pay/cs_test_abc123');
    });

    test('fromJson and toJson should be symmetric', () {
      final response = CheckoutResponse.fromJson(fullJson);
      final roundTripped = response.toJson();

      expect(roundTripped, fullJson);
    });
  });
}
