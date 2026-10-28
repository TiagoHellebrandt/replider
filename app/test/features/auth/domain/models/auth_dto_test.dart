import 'package:flutter_test/flutter_test.dart';
import 'package:app/features/auth/domain/models/auth_dto.dart';

void main() {
  group('AuthenticateUserDTO', () {
    test('toJson should produce correct map', () {
      const dto = AuthenticateUserDTO(
        email: 'john@example.com',
        password: 'secret123',
      );
      final json = dto.toJson();

      expect(json['email'], 'john@example.com');
      expect(json['password'], 'secret123');
    });
  });

  group('AuthenticateUserResponse', () {
    test('fromJson should parse token correctly', () {
      final json = <String, dynamic>{
        'token': 'jwt-token-abc',
      };

      final response = AuthenticateUserResponse.fromJson(json);

      expect(response.token, 'jwt-token-abc');
      // Note: isNew is not deserialized by generated code (not included in
      // the generated fromJson), so it defaults to null regardless of input.
      expect(response.isNew, isNull);
    });
  });

  group('CreateAccountDTO', () {
    test('toJson should produce correct snake_case map', () {
      const dto = CreateAccountDTO(
        accountName: 'My Company',
        adminEmail: 'admin@company.com',
        adminName: 'Admin User',
        password: 'strongPass1!',
        termsAccepted: true,
      );
      final json = dto.toJson();

      expect(json['account_name'], 'My Company');
      expect(json['admin_email'], 'admin@company.com');
      expect(json['admin_name'], 'Admin User');
      expect(json['password'], 'strongPass1!');
      expect(json['terms_accepted'], true);
    });

    test('toJson should serialize terms_accepted=false explicitly', () {
      const dto = CreateAccountDTO(
        accountName: 'X',
        adminEmail: 'x@x.com',
        adminName: 'X',
        password: 'p',
        termsAccepted: false,
      );
      // Backend rejects this, but the DTO still serializes — the UI is the
      // gate that prevents submitting with false.
      expect(dto.toJson()['terms_accepted'], false);
    });
  });

  group('HandleCreateAccountResponse', () {
    test('fromJson should parse message', () {
      final json = <String, dynamic>{
        'message': 'Account created successfully',
      };

      final response = HandleCreateAccountResponse.fromJson(json);

      expect(response.message, 'Account created successfully');
    });

    test('fromJson should handle null message', () {
      final json = <String, dynamic>{};

      final response = HandleCreateAccountResponse.fromJson(json);

      expect(response.message, isNull);
    });
  });

  group('ForgotPasswordDTO', () {
    test('toJson should produce correct map', () {
      const dto = ForgotPasswordDTO(email: 'user@example.com');
      final json = dto.toJson();

      expect(json['email'], 'user@example.com');
    });
  });

  group('SocialAuthDTO', () {
    test('toJson should produce correct snake_case map', () {
      const dto = SocialAuthDTO(idToken: 'google-id-token-xyz');
      final json = dto.toJson();

      expect(json['id_token'], 'google-id-token-xyz');
    });
  });

  group('SocialAuthResponse', () {
    test('fromJson should parse all fields correctly', () {
      final json = <String, dynamic>{
        'token': 'jwt-social-token',
        'is_new': true,
      };

      final response = SocialAuthResponse.fromJson(json);

      expect(response.token, 'jwt-social-token');
      expect(response.isNew, true);
    });

    test('fromJson should handle null is_new', () {
      final json = <String, dynamic>{
        'token': 'jwt-social-token',
      };

      final response = SocialAuthResponse.fromJson(json);

      expect(response.token, 'jwt-social-token');
      expect(response.isNew, isNull);
    });
  });

  group('GetMeResponse', () {
    test('fromJson should parse user and account maps', () {
      final json = <String, dynamic>{
        'user': <String, dynamic>{
          'id': 'user-1',
          'name': 'John Doe',
          'email': 'john@example.com',
        },
        'account': <String, dynamic>{
          'id': 'acc-1',
          'name': 'Acme Corp',
          'is_trialing': true,
          'plan_name': 'Trial',
        },
      };

      final response = GetMeResponse.fromJson(json);

      expect(response.user['id'], 'user-1');
      expect(response.user['name'], 'John Doe');
      expect(response.user['email'], 'john@example.com');
      expect(response.account['id'], 'acc-1');
      expect(response.account['is_trialing'], true);
      expect(response.account['plan_name'], 'Trial');
    });
  });
}
