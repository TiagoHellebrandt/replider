import 'package:flutter_test/flutter_test.dart';
import 'package:app/core/analytics/utm_bootstrap.dart';
import 'package:app/features/auth/domain/models/auth_dto.dart';

void main() {
  group('UtmBootstrap', () {
    tearDown(UtmBootstrap.reset);

    test('trims values and nil-ifies blanks', () {
      UtmBootstrap.setForTest(
        source: '  marketing  ',
        medium: 'hero',
        campaign: '   ',
      );
      expect(UtmBootstrap.source, 'marketing');
      expect(UtmBootstrap.medium, 'hero');
      expect(UtmBootstrap.campaign, isNull);
    });

    test('reset clears all values', () {
      UtmBootstrap.setForTest(
          source: 'x', medium: 'y', campaign: 'z', referralCode: 'CODE1234');
      UtmBootstrap.reset();
      expect(UtmBootstrap.source, isNull);
      expect(UtmBootstrap.medium, isNull);
      expect(UtmBootstrap.campaign, isNull);
      expect(UtmBootstrap.referralCode, isNull);
    });

    test('captures and trims the referral code (US-174)', () {
      UtmBootstrap.setForTest(referralCode: '  FRIEND01  ');
      expect(UtmBootstrap.referralCode, 'FRIEND01');

      UtmBootstrap.setForTest(referralCode: '   ');
      expect(UtmBootstrap.referralCode, isNull);
    });
  });

  group('CreateAccountDTO UTM serialization', () {
    test('forwards utm_* keys when present', () {
      final dto = CreateAccountDTO(
        accountName: 'Acme',
        adminEmail: 'a@acme.com',
        adminName: 'Admin',
        password: 'securepassword123',
        termsAccepted: true,
        utmSource: 'marketing',
        utmMedium: 'hero',
        utmCampaign: 'home',
      );
      final json = dto.toJson();
      expect(json['utm_source'], 'marketing');
      expect(json['utm_medium'], 'hero');
      expect(json['utm_campaign'], 'home');
    });

    test('forwards referral_code when present (US-174)', () {
      final dto = CreateAccountDTO(
        accountName: 'Acme',
        adminEmail: 'a@acme.com',
        adminName: 'Admin',
        password: 'securepassword123',
        termsAccepted: true,
        referralCode: 'FRIEND01',
      );
      expect(dto.toJson()['referral_code'], 'FRIEND01');
    });

    test('emits null utm_* when absent', () {
      const dto = CreateAccountDTO(
        accountName: 'Acme',
        adminEmail: 'a@acme.com',
        adminName: 'Admin',
        password: 'securepassword123',
        termsAccepted: true,
      );
      final json = dto.toJson();
      expect(json['utm_source'], isNull);
      expect(json['utm_medium'], isNull);
      expect(json['utm_campaign'], isNull);
    });
  });
}
