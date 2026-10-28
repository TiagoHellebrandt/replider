import 'package:flutter_test/flutter_test.dart';
import 'package:app/features/owner/domain/models/owner_account_models.dart';

void main() {
  Map<String, dynamic> listingJson({String? quality}) => {
        'account': {
          'id': 'acc-1',
          'name': 'Acme',
          'subscription_status': 'active',
          'onboarding_completed': true,
          'csat_enabled': true,
          'trial_bypass': false,
          'created_at': '2026-01-01T00:00:00Z',
          'updated_at': '2026-01-01T00:00:00Z',
        },
        'plan_name': 'Pro',
        'tokens_used': 10,
        'plan_max_tokens': 100,
        if (quality != null) 'whatsapp_quality': quality,
      };

  group('OwnerAccountListing.fromJson', () {
    test('parses whatsapp_quality when present', () {
      final l = OwnerAccountListing.fromJson(listingJson(quality: 'RED'));
      expect(l.whatsappQuality, 'RED');
      expect(l.capPercent, 10);
    });

    test('defaults whatsapp_quality to empty when absent', () {
      final l = OwnerAccountListing.fromJson(listingJson());
      expect(l.whatsappQuality, '');
    });
  });
}
