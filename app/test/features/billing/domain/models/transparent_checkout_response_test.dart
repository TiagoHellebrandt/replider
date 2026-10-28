import 'package:app/features/billing/domain/models/transparent_checkout_response.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('TransparentCheckoutResponse', () {
    test('parses a BRL PIX charge', () {
      final json = {
        'provider': 'abacatepay',
        'method': 'pix',
        'pix': {
          'id': 'pix_1',
          'br_code': '000201...',
          'br_code_base64': 'data:image/png;base64,AAA',
          'amount': 9900,
          'status': 'PENDING',
          'expires_at': '2026-07-14T12:00:00Z',
        },
      };
      final res = TransparentCheckoutResponse.fromJson(json);
      expect(res.provider, 'abacatepay');
      expect(res.isPix, isTrue);
      expect(res.isStripe, isFalse);
      expect(res.url, isNull);
      expect(res.pix!.id, 'pix_1');
      expect(res.pix!.brCode, '000201...');
      expect(res.pix!.brCodeBase64, contains('base64'));
      expect(res.pix!.amount, 9900);
    });

    test('parses a Stripe card checkout (url only)', () {
      final json = {
        'provider': 'stripe',
        'method': 'card',
        'url': 'https://checkout.stripe.com/s_1',
      };
      final res = TransparentCheckoutResponse.fromJson(json);
      expect(res.isStripe, isTrue);
      expect(res.isPix, isFalse);
      expect(res.url, 'https://checkout.stripe.com/s_1');
      expect(res.pix, isNull);
    });

    test('parses an AbacatePay hosted card checkout', () {
      final json = {
        'provider': 'abacatepay',
        'method': 'card',
        'url': 'https://app.abacatepay.com/pay/bill_1',
      };
      final res = TransparentCheckoutResponse.fromJson(json);
      expect(res.provider, 'abacatepay');
      expect(res.method, 'card');
      expect(res.isPix, isFalse);
      expect(res.url, contains('abacatepay'));
    });
  });

  group('ChargeStatusResponse', () {
    test('round-trips', () {
      final res =
          ChargeStatusResponse.fromJson({'status': 'PAID', 'paid': true});
      expect(res.status, 'PAID');
      expect(res.paid, isTrue);
      expect(res.toJson()['paid'], isTrue);
    });
  });
}
