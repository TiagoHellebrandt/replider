import 'package:app/core/error/api_error_parser.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

// What the app says when a request does not come back.
//
// The copilot failed in production and told the person "Sem conexão com o
// servidor. Verifique sua internet." — while every other screen in the same tab
// was loading fine. The app cannot know whose fault it is, and sending someone
// to check a working router is worse than saying nothing: they stop trusting
// the message that would matter when the connection really is down.

void main() {
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  DioException _err(DioExceptionType type) => DioException(
        requestOptions: RequestOptions(path: '/copilot/ask'),
        type: type,
      );

  test('a connection failure does not blame the user alone', () {
    final message = parseApiError(_err(DioExceptionType.connectionError));

    expect(message, t.errors.CONNECTION_ERROR);
    // It must offer both possibilities, not assert the one it cannot know.
    expect(message.toLowerCase(), contains('instabilidade'));
  });

  test('a timeout is reported as the server being slow, not as no internet',
      () {
    // These are different facts: one means nothing arrived, the other means
    // the server is still thinking. Telling both stories as "check your
    // internet" hides the only actionable difference.
    final message = parseApiError(_err(DioExceptionType.receiveTimeout));

    expect(message, t.errors.CONNECTION_TIMEOUT);
    expect(message.toLowerCase(), contains('demorou'));
  });

  test('the two are not the same sentence', () {
    expect(t.errors.CONNECTION_ERROR, isNot(t.errors.CONNECTION_TIMEOUT));
  });
}
