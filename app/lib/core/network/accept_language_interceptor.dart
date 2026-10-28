import 'package:dio/dio.dart';
import 'package:app/i18n/strings.g.dart';

/// Forwards the active app locale to the API via `Accept-Language`.
///
/// Backend uses this header to localise server-seeded copy (e.g. the
/// "Perguntas frequentes" knowledge source created on agent template
/// onboarding). Sent as a simple `pt` / `en` tag — the API only
/// distinguishes those two right now.
class AcceptLanguageInterceptor extends Interceptor {
  @override
  void onRequest(RequestOptions options, RequestInterceptorHandler handler) {
    if (!options.headers.containsKey('Accept-Language')) {
      options.headers['Accept-Language'] =
          LocaleSettings.currentLocale.languageCode;
    }
    handler.next(options);
  }
}
