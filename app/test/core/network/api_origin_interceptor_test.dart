import 'package:app/core/network/api_origin_interceptor.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

// 2026-09-09: the web app was built pointing at app.replider.com/v1, which
// has no API route, so its SPA fallback answered HTTP 200 with index.html on
// every call. Every instrument said healthy — browser network tab, curl,
// nginx access log — and the failure that reached the screen was
// `DioException [unknown]: null`.

Dio _dioAnswering(String contentType, String body, {int status = 200}) {
  final dio = Dio(BaseOptions(baseUrl: 'https://app.replider.com/v1'));
  dio.interceptors.add(ApiOriginInterceptor());
  dio.httpClientAdapter = _FixedAdapter(contentType, body, status);
  return dio;
}

void main() {
  test(
      'HTML where JSON was expected is refused, and the message names the host',
      () async {
    final dio = _dioAnswering('text/html; charset=utf-8', '<!DOCTYPE html>');

    Object? thrown;
    try {
      await dio.get<Map<String, dynamic>>('/me/command-center');
    } catch (e) {
      thrown = e;
    }

    expect(thrown, isA<DioException>());
    final e = thrown! as DioException;
    expect(e.error, isA<ApiOriginMisconfigured>());
    expect((e.error! as ApiOriginMisconfigured).origin,
        'https://app.replider.com');
    // The sentence a person reads has to name the host and say what is wrong.
    expect(e.message, contains('app.replider.com'));
    expect(e.message, contains('HTML'));
    // The body is kept: a log should be able to show what came back.
    expect(e.response, isNotNull);
  });

  test('a real JSON answer passes through untouched', () async {
    final dio = _dioAnswering('application/json; charset=utf-8', '{"ok":true}');

    final res = await dio.get<Map<String, dynamic>>('/me/command-center');

    expect(res.statusCode, 200);
    expect(res.data, {'ok': true});
  });

  // A CSV export and a signed-file download legitimately answer something
  // other than JSON. The guard fires on what the CALLER asked for, so those
  // are never refused.
  test('a caller that did not ask for JSON is never refused', () async {
    final dio = _dioAnswering('text/html; charset=utf-8', '<html></html>');

    final res = await dio.get<String>(
      '/export.csv',
      options: Options(responseType: ResponseType.plain),
    );

    expect(res.statusCode, 200);
    expect(res.data, '<html></html>');
  });

  // An API error page is still the API. Only the content type decides.
  test('a JSON error body is left for the normal error handling', () async {
    final dio = _dioAnswering(
        'application/json; charset=utf-8', '{"error":"Authorization required"}',
        status: 401);

    Object? thrown;
    try {
      await dio.get<Map<String, dynamic>>('/me');
    } catch (e) {
      thrown = e;
    }

    expect(thrown, isA<DioException>());
    expect(
        (thrown! as DioException).error, isNot(isA<ApiOriginMisconfigured>()));
  });
}

class _FixedAdapter implements HttpClientAdapter {
  _FixedAdapter(this.contentType, this.body, this.status);

  final String contentType;
  final String body;
  final int status;

  @override
  void close({bool force = false}) {}

  @override
  Future<ResponseBody> fetch(RequestOptions options,
      Stream<List<int>>? requestStream, Future<void>? cancelFuture) async {
    return ResponseBody.fromString(
      body,
      status,
      headers: {
        Headers.contentTypeHeader: [contentType],
      },
    );
  }
}
