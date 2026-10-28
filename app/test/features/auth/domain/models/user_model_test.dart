import 'package:flutter_test/flutter_test.dart';
import 'package:app/features/auth/domain/models/user_model.dart';

void main() {
  group('UserModel.isPaidActive', () {
    UserModel u({
      String? status,
      int? days,
      bool bypass = false,
      bool contract = false,
    }) =>
        UserModel(
          id: 'u',
          name: 'n',
          email: 'e',
          token: 't',
          subscriptionStatus: status,
          trialDaysRemaining: days,
          isTrialing: status == 'trialing',
          trialBypass: bypass,
          isContractPlan: contract,
        );

    test('trial_bypass account is paid-active even with an expired trial', () {
      expect(u(status: 'trialing', days: 0, bypass: true).isPaidActive, isTrue);
    });
    test('contract account is paid-active regardless of status', () {
      expect(u(status: 'past_due', contract: true).isPaidActive, isTrue);
    });
    test('expired self-serve trial is NOT paid-active', () {
      expect(u(status: 'trialing', days: 0).isPaidActive, isFalse);
    });
    test('active self-serve is paid-active', () {
      expect(u(status: 'active').isPaidActive, isTrue);
    });
  });

  group('UserModel', () {
    final fullJson = <String, dynamic>{
      'id': 'user-1',
      'name': 'John Doe',
      'email': 'john@example.com',
      'account_name': '',
      'token': 'jwt-token-abc123',
      'is_new': true,
      'is_trialing': false,
      'plan_name': 'Trial',
      'role': 'attendant',
      'picture': null,
      'trial_days_remaining': null,
      'subscription_status': null,
      'is_contract_plan': false,
      'openai_key_masked': '',
      'gemini_key_masked': '',
      'impersonating_actor': null,
      'totp_enabled': true,
      'trial_bypass': false,
      'is_email_verified': true,
      'account_currency': 'usd',
      'is_platform_account': false,
      'is_seller': false,
      'signature': null,
    };

    test('fromJson should parse all fields correctly', () {
      final user = UserModel.fromJson(fullJson);

      expect(user.id, 'user-1');
      expect(user.name, 'John Doe');
      expect(user.email, 'john@example.com');
      expect(user.token, 'jwt-token-abc123');
      expect(user.isNew, true);
      expect(user.totpEnabled, true);
      expect(user.accountCurrency, 'usd');
    });

    test('accountCurrency defaults to brl for legacy cached rows', () {
      final legacy = Map<String, dynamic>.from(fullJson)
        ..remove('account_currency');
      expect(UserModel.fromJson(legacy).accountCurrency, 'brl');
    });

    // US-014: is_seller parses and defaults to false for cached rows
    // written before the flag existed — regular users must never flash
    // seller controls.
    test('isSeller parses and defaults to false when absent', () {
      final withFlag = Map<String, dynamic>.from(fullJson)
        ..['is_seller'] = true;
      expect(UserModel.fromJson(withFlag).isSeller, isTrue);
      expect(UserModel.fromJson(withFlag).toJson()['is_seller'], isTrue);

      expect(UserModel.fromJson(fullJson).isSeller, isFalse);
      final legacy = Map<String, dynamic>.from(fullJson)..remove('is_seller');
      expect(UserModel.fromJson(legacy).isSeller, isFalse);
    });

    test('toJson should produce correct map', () {
      final user = UserModel.fromJson(fullJson);
      final json = user.toJson();

      expect(json['id'], 'user-1');
      expect(json['name'], 'John Doe');
      expect(json['email'], 'john@example.com');
      expect(json['token'], 'jwt-token-abc123');
      expect(json['is_new'], true);
    });

    test('fromJson should use default value for is_new when absent', () {
      final minimalJson = <String, dynamic>{
        'id': 'user-2',
        'name': 'Jane Doe',
        'email': 'jane@example.com',
        'token': 'jwt-token-xyz789',
      };

      final user = UserModel.fromJson(minimalJson);

      expect(user.id, 'user-2');
      expect(user.name, 'Jane Doe');
      expect(user.email, 'jane@example.com');
      expect(user.token, 'jwt-token-xyz789');
      expect(user.isNew, false);
    });

    test('fromJson and toJson should be symmetric', () {
      final user = UserModel.fromJson(fullJson);
      final roundTripped = user.toJson();

      expect(roundTripped, fullJson);
    });

    test('equality should compare by id, name, email, and token', () {
      final user1 = UserModel.fromJson(fullJson);
      final user2 = UserModel.fromJson(fullJson);

      expect(user1, equals(user2));
    });

    test('equality should return false for different users', () {
      final user1 = UserModel.fromJson(fullJson);
      final user2 = UserModel.fromJson(<String, dynamic>{
        'id': 'user-other',
        'name': 'John Doe',
        'email': 'john@example.com',
        'token': 'jwt-token-abc123',
        'is_new': true,
      });

      expect(user1, isNot(equals(user2)));
    });

    test('hashCode should be consistent with equality', () {
      final user1 = UserModel.fromJson(fullJson);
      final user2 = UserModel.fromJson(fullJson);

      expect(user1.hashCode, equals(user2.hashCode));
    });

    test('picture defaults to null when absent', () {
      final user = UserModel.fromJson(<String, dynamic>{
        'id': 'u',
        'name': 'N',
        'email': 'e@x.com',
        'token': 't',
      });
      expect(user.picture, isNull);
      expect(user.isGoogleUser, isFalse);
    });

    test('picture is parsed when present', () {
      final user = UserModel.fromJson(<String, dynamic>{
        'id': 'u',
        'name': 'N',
        'email': 'e@x.com',
        'token': 't',
        'picture': 'https://photo.example/me.jpg',
      });
      expect(user.picture, 'https://photo.example/me.jpg');
      expect(user.isGoogleUser, isTrue);
    });

    test('isEmailVerified defaults to true when absent (no false banner)', () {
      final user = UserModel.fromJson(<String, dynamic>{
        'id': 'u',
        'name': 'N',
        'email': 'e@x.com',
        'token': 't',
      });
      expect(user.isEmailVerified, isTrue);
    });

    test('isEmailVerified parses an explicit false from /me', () {
      final user = UserModel.fromJson(<String, dynamic>{
        'id': 'u',
        'name': 'N',
        'email': 'e@x.com',
        'token': 't',
        'is_email_verified': false,
      });
      expect(user.isEmailVerified, isFalse);
    });

    test('isGoogleUser is false for empty picture string', () {
      final user = UserModel.fromJson(<String, dynamic>{
        'id': 'u',
        'name': 'N',
        'email': 'e@x.com',
        'token': 't',
        'picture': '',
      });
      expect(user.isGoogleUser, isFalse);
    });

    test('equality includes picture', () {
      const a = UserModel(
        id: '1',
        name: 'A',
        email: 'a@x.com',
        token: 't',
        picture: 'https://p.example/1.jpg',
      );
      const b = UserModel(
        id: '1',
        name: 'A',
        email: 'a@x.com',
        token: 't',
        picture: 'https://p.example/2.jpg',
      );
      expect(a, isNot(equals(b)));
    });
  });
}
