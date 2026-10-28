import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:app/core/network/accept_language_interceptor.dart';
import 'package:app/i18n/strings.g.dart';

void main() {
  group('AcceptLanguageInterceptor', () {
    test('attaches Accept-Language from current Slang locale (en)', () {
      LocaleSettings.setLocale(AppLocale.en);
      final interceptor = AcceptLanguageInterceptor();
      final options = RequestOptions(path: '/anything');
      final handler = _RecordingHandler();

      interceptor.onRequest(options, handler);

      expect(handler.nextCalled, isTrue);
      expect(handler.lastOptions?.headers['Accept-Language'], 'en');
    });

    test('attaches Accept-Language from current Slang locale (pt)', () async {
      // pt is a deferred-loaded locale (non-base); await the load.
      await LocaleSettings.setLocale(AppLocale.pt);
      final interceptor = AcceptLanguageInterceptor();
      final options = RequestOptions(path: '/anything');
      final handler = _RecordingHandler();

      interceptor.onRequest(options, handler);

      expect(handler.nextCalled, isTrue);
      expect(handler.lastOptions?.headers['Accept-Language'], 'pt');
    });

    test('does not overwrite a header already set by an earlier interceptor',
        () {
      LocaleSettings.setLocale(AppLocale.en);
      final interceptor = AcceptLanguageInterceptor();
      final options = RequestOptions(path: '/anything')
        ..headers['Accept-Language'] = 'fr';
      final handler = _RecordingHandler();

      interceptor.onRequest(options, handler);

      expect(handler.lastOptions?.headers['Accept-Language'], 'fr');
    });
  });
}

class _RecordingHandler extends RequestInterceptorHandler {
  bool nextCalled = false;
  RequestOptions? lastOptions;

  @override
  void next(RequestOptions requestOptions) {
    nextCalled = true;
    lastOptions = requestOptions;
  }
}
