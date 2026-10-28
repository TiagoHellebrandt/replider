import 'package:dio/dio.dart';

/// Refuses a "successful" response that is not the API answering.
///
/// This exists because of a deploy on 2026-09-09. The web app was built with
/// `API_BASE_URL=https://app.replider.com/v1` instead of
/// `https://api.replider.com/v1`. The app origin has no `/v1` route, so its
/// SPA fallback answered every API call with **HTTP 200 and index.html**.
///
/// Every instrument agreed the system was healthy. The browser's network tab
/// showed 200. `curl -I` showed 200. The API's own logs showed nothing,
/// because no request ever reached the API. What the client got was a page of
/// HTML where it expected an object, and the failure surfaced as
/// `DioException [unknown]: null` — a sentence that names neither what went
/// wrong nor where.
///
/// A wrong base URL has to fail where it is wrong, saying so. The
/// infrastructure side of this is a JSON 404 on `app.replider.com/v1/*`
/// (nginx/conf.d/30-app.conf); this is the client side, for every other host
/// that might one day answer HTML to an API call — a captive portal, a proxy
/// error page, a CDN interstitial.
class ApiOriginInterceptor extends Interceptor {
  @override
  void onResponse(
      Response<dynamic> response, ResponseInterceptorHandler handler) {
    // Only when the CALLER asked for JSON. A CSV export and a signed-file
    // download legitimately answer something else, and this must never
    // refuse those.
    if (response.requestOptions.responseType != ResponseType.json) {
      handler.next(response);
      return;
    }
    final contentType =
        response.headers.value(Headers.contentTypeHeader)?.toLowerCase() ?? '';
    if (!contentType.contains('text/html')) {
      handler.next(response);
      return;
    }

    final uri = response.requestOptions.uri;
    handler.reject(
      DioException(
        requestOptions: response.requestOptions,
        response: response,
        type: DioExceptionType.badResponse,
        error: ApiOriginMisconfigured(origin: '${uri.scheme}://${uri.host}'),
        // dio prints `error` when the message is null, so the sentence
        // below is what a person actually reads.
        message: 'O endereço ${uri.scheme}://${uri.host} respondeu uma página '
            'HTML onde a API deveria responder. Esta build está apontada para '
            'o servidor errado.',
      ),
      // The response is real HTTP traffic, not a transport failure — keep it
      // attached so a log can show what came back.
      true,
    );
  }
}

/// The typed cause, so a caller can tell this apart from a server error.
///
/// Deliberately NOT a subclass of the app's domain errors: nothing about the
/// account, the plan or the request is wrong. The BUILD is pointed at a host
/// that is not the API.
class ApiOriginMisconfigured implements Exception {
  const ApiOriginMisconfigured({required this.origin});

  /// The host that answered HTML, so the message can name it.
  final String origin;

  @override
  String toString() =>
      'ApiOriginMisconfigured($origin answered HTML where JSON was expected)';
}
