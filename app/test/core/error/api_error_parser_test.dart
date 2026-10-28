import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:app/core/error/api_error_parser.dart';
import 'package:app/i18n/strings.g.dart';

/// Mimics `dart:io`'s SocketException without importing it (the parser
/// matches by runtime type name so web builds don't break on `dart:io`).
class SocketException implements Exception {
  const SocketException(this.message);
  final String message;
}

void main() {
  setUpAll(() {
    // Ensure Slang translations are initialized for tests.
    LocaleSettings.setLocale(AppLocale.en);
  });

  group('parseApiError', () {
    group('with known error codes', () {
      final knownCodes = {
        'AUTH_INVALID_CREDENTIALS': t.errors.AUTH_INVALID_CREDENTIALS,
        'AUTH_EMAIL_ALREADY_EXISTS': t.errors.AUTH_EMAIL_ALREADY_EXISTS,
        'OWNER_EMAIL_EXISTS': t.errors.OWNER_EMAIL_EXISTS,
        'AUTH_UNAUTHORIZED': t.errors.AUTH_UNAUTHORIZED,
        'AUTH_SOCIAL_FAILED': t.errors.AUTH_SOCIAL_FAILED,
        'AUTH_INVALID_RESET_TOKEN': t.errors.AUTH_INVALID_RESET_TOKEN,
        'AGENT_NOT_FOUND': t.errors.AGENT_NOT_FOUND,
        'CONV_NOT_FOUND': t.errors.CONV_NOT_FOUND,
        'CONV_OUTSIDE_24H': t.errors.CONV_OUTSIDE_24H,
        'USAGE_INSUFFICIENT_TOKENS': t.errors.USAGE_INSUFFICIENT_TOKENS,
        'USAGE_NO_CREDITS': t.errors.USAGE_NO_CREDITS,
        'USAGE_LIMIT_EXCEEDED': t.errors.USAGE_LIMIT_EXCEEDED,
        'ACCOUNT_SUSPENDED': t.errors.ACCOUNT_SUSPENDED,
        'TOTP_INVALID_CODE': t.errors.TOTP_INVALID_CODE,
        'TOTP_NOT_STAGED': t.errors.TOTP_NOT_STAGED,
        'TOTP_NOT_ENABLED': t.errors.TOTP_NOT_ENABLED,
        'TOTP_CIPHER_UNCONFIGURED': t.errors.TOTP_CIPHER_UNCONFIGURED,
        // USD billing codes (US-017 review fixes) — used to fall through
        // to the raw backend message / UNKNOWN.
        'PLAN_NOT_FOUND': t.errors.PLAN_NOT_FOUND,
        // US-018 review fixes: PLAN_NOT_PURCHASABLE fell through to the
        // backend's hardcoded pt-BR contract message — a USD/English
        // user saw Portuguese copy.
        'PLAN_NOT_PURCHASABLE': t.errors.PLAN_NOT_PURCHASABLE,
        'PLAN_CURRENCY_MISMATCH': t.errors.PLAN_CURRENCY_MISMATCH,
        'COUPON_EXHAUSTED': t.errors.COUPON_EXHAUSTED,
        'ADDON_CURRENCY_UNSUPPORTED': t.errors.ADDON_CURRENCY_UNSUPPORTED,
        // Setup refusals from the composer/approval flow: each names the fix.
        'LEAD_FUNNEL_NO_CHANNEL': t.errors.LEAD_FUNNEL_NO_CHANNEL,
        'FUNNEL_STAGE_BACKWARD': t.errors.FUNNEL_STAGE_BACKWARD,
        'LEAD_CONTACT_NOT_MESSAGEABLE': t.errors.LEAD_CONTACT_NOT_MESSAGEABLE,
        'LEAD_ALREADY_CONTACTED': t.errors.LEAD_ALREADY_CONTACTED,
        'CONTACT_OPTED_OUT': t.errors.CONTACT_OPTED_OUT,
        'BYOK_NOT_CONFIGURED': t.errors.BYOK_NOT_CONFIGURED,
        'FORBIDDEN': t.errors.FORBIDDEN,
        'LEAD_NO_CONVERSATION': t.errors.LEAD_NO_CONVERSATION,
        'AI_PROVIDER_UNAVAILABLE': t.errors.AI_PROVIDER_UNAVAILABLE,
      };

      for (final entry in knownCodes.entries) {
        test('should return localized message for ${entry.key}', () {
          final error = DioException(
            requestOptions: RequestOptions(path: '/test'),
            response: Response(
              requestOptions: RequestOptions(path: '/test'),
              statusCode: 400,
              data: {'code': entry.key, 'message': 'raw backend message'},
            ),
          );

          expect(parseApiError(error), entry.value);
        });
      }
    });

    test('falls back to backend message when code is unrecognized', () {
      // The parser used to return UNKNOWN here. We changed it to surface
      // the backend's free-form `message` so a new error code shipped
      // server-side (before the client adds a translation) is still
      // readable to the user.
      final error = DioException(
        requestOptions: RequestOptions(path: '/test'),
        response: Response(
          requestOptions: RequestOptions(path: '/test'),
          statusCode: 500,
          data: {'code': 'SOME_FUTURE_ERROR', 'message': 'new error'},
        ),
      );

      expect(parseApiError(error), 'new error');
    });

    test('uses backend message when response has no code field', () {
      // No code → the parser falls through to the backend's `message`
      // field instead of UNKNOWN, which preserves any specific copy the
      // server set (validation messages, etc.).
      final error = DioException(
        requestOptions: RequestOptions(path: '/test'),
        response: Response(
          requestOptions: RequestOptions(path: '/test'),
          statusCode: 422,
          data: {'message': 'validation failed'},
        ),
      );

      expect(parseApiError(error), 'validation failed');
    });

    test('returns SERVER_ERROR for 5xx without a JSON envelope', () {
      // Server crashed and returned a plain string body. parseApiError
      // can't extract a code/message, so it inspects the status code
      // and routes 5xx to SERVER_ERROR rather than the generic UNKNOWN.
      final error = DioException(
        requestOptions: RequestOptions(path: '/test'),
        type: DioExceptionType.badResponse,
        response: Response(
          requestOptions: RequestOptions(path: '/test'),
          statusCode: 500,
          data: 'Internal Server Error',
        ),
      );

      expect(parseApiError(error), t.errors.SERVER_ERROR);
    });

    test('returns SERVER_ERROR for 5xx with null body', () {
      final error = DioException(
        requestOptions: RequestOptions(path: '/test'),
        type: DioExceptionType.badResponse,
        response: Response(
          requestOptions: RequestOptions(path: '/test'),
          statusCode: 502,
          data: null,
        ),
      );

      expect(parseApiError(error), t.errors.SERVER_ERROR);
    });

    test('returns UNKNOWN when both response and type are unknown', () {
      final error = DioException(
        requestOptions: RequestOptions(path: '/test'),
        response: null,
      );

      expect(parseApiError(error), t.errors.UNKNOWN);
    });

    group('network/timeout errors map to specific copy', () {
      final cases = {
        DioExceptionType.connectionError: 'CONNECTION_ERROR',
        DioExceptionType.connectionTimeout: 'CONNECTION_TIMEOUT',
        DioExceptionType.sendTimeout: 'CONNECTION_TIMEOUT',
        DioExceptionType.receiveTimeout: 'CONNECTION_TIMEOUT',
      };

      for (final entry in cases.entries) {
        test('${entry.key.name} → ${entry.value}', () {
          // Each Dio exception type maps to a specific localized key
          // instead of the catch-all UNKNOWN. This is the change that
          // killed "Erro de rede: null" — a null e.message no longer
          // appears in user-visible copy.
          final error = DioException(
            requestOptions: RequestOptions(path: '/test'),
            type: entry.key,
          );
          final expected = entry.value == 'CONNECTION_ERROR'
              ? t.errors.CONNECTION_ERROR
              : t.errors.CONNECTION_TIMEOUT;
          expect(parseApiError(error), expected);
        });
      }
    });

    test('should return UNKNOWN for non-DioException error', () {
      expect(parseApiError(Exception('random')), t.errors.UNKNOWN);
    });

    test('should return UNKNOWN for null error', () {
      expect(parseApiError(null), t.errors.UNKNOWN);
    });

    group('bare (non-Dio) exceptions map to specific copy', () {
      test('TimeoutException → CONNECTION_TIMEOUT', () {
        expect(
          parseApiError(TimeoutException('timed out')),
          t.errors.CONNECTION_TIMEOUT,
        );
      });

      test('SocketException → CONNECTION_ERROR', () {
        expect(
          parseApiError(const SocketException('no route to host')),
          t.errors.CONNECTION_ERROR,
        );
      });

      test('FormatException → BAD_REQUEST', () {
        expect(
          parseApiError(const FormatException('bad json')),
          t.errors.BAD_REQUEST,
        );
      });
    });

    group('DioException wrapping a bare cause is unwrapped', () {
      DioException wrap(Object cause) => DioException(
            requestOptions: RequestOptions(path: '/test'),
            type: DioExceptionType.unknown,
            error: cause,
          );

      test('wrapped TimeoutException → CONNECTION_TIMEOUT', () {
        expect(
          parseApiError(wrap(TimeoutException('t'))),
          t.errors.CONNECTION_TIMEOUT,
        );
      });

      test('wrapped SocketException → CONNECTION_ERROR', () {
        expect(
          parseApiError(wrap(const SocketException('s'))),
          t.errors.CONNECTION_ERROR,
        );
      });

      test('wrapped FormatException → BAD_REQUEST', () {
        expect(
          parseApiError(wrap(const FormatException('f'))),
          t.errors.BAD_REQUEST,
        );
      });

      test('unknown wrapped cause → UNKNOWN', () {
        expect(parseApiError(wrap(Exception('x'))), t.errors.UNKNOWN);
      });
    });
  });
}
