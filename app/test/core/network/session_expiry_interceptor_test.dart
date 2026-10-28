import 'dart:convert';
import 'dart:typed_data';

import 'package:app/core/network/session_expiry_interceptor.dart';
import 'package:dio/dio.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

/// Returns a fixed HTTP status for every request so we can drive the
/// interceptor's onError path deterministically.
class _StatusAdapter implements HttpClientAdapter {
  _StatusAdapter(this.status);
  final int status;

  @override
  Future<ResponseBody> fetch(RequestOptions options,
      Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async {
    return ResponseBody.fromString(
      '{}',
      status,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

Dio _dioReturning(
  int status, {
  required Future<void> Function() onExpired,
  // Por padrão o servidor confirma que a sessão morreu — os testes antigos
  // descrevem justamente esse caminho.
  Future<bool> Function(String token)? isSessionDead,
}) {
  final dio = Dio()..httpClientAdapter = _StatusAdapter(status);
  dio.interceptors.add(
    SessionExpiryInterceptor(
      onSessionExpired: onExpired,
      isSessionDead: isSessionDead ?? (_) async => true,
      storage: const FlutterSecureStorage(),
    ),
  );
  return dio;
}

void main() {
  setUp(() {
    FlutterSecureStorage.setMockInitialValues({});
  });

  void seedSession() {
    FlutterSecureStorage.setMockInitialValues({
      'cached_user_model': json.encode({'token': 'live-jwt', 'name': 'A'}),
    });
  }

  test('401 with a stored session forces logout exactly once', () async {
    seedSession();
    var calls = 0;
    final dio = _dioReturning(401, onExpired: () async => calls++);

    await expectLater(
        dio.get<dynamic>('/anything'), throwsA(isA<DioException>()));

    expect(calls, 1);
  });

  test('401 without a stored session does NOT logout (login failure)',
      () async {
    // No cached_user_model seeded — a bad-credentials 401 on the login
    // screen must not clear anything.
    var calls = 0;
    final dio = _dioReturning(401, onExpired: () async => calls++);

    await expectLater(dio.get<dynamic>('/auth'), throwsA(isA<DioException>()));

    expect(calls, 0);
  });

  test('non-401 error never triggers logout', () async {
    seedSession();
    var calls = 0;
    final dio = _dioReturning(403, onExpired: () async => calls++);

    await expectLater(
        dio.get<dynamic>('/anything'), throwsA(isA<DioException>()));

    expect(calls, 0);
  });

  // Two tabs share one storage.
  //
  // A tab left open holds the OLD token in memory. After the session is
  // renewed anywhere, that tab's next request 401s legitimately — and the
  // interceptor used to answer by wiping the storage and revoking the token,
  // killing the session the other tab had just obtained. The person is thrown
  // out of a working session by a tab they were not even looking at.

  test('a 401 on a superseded token does not kill the current session',
      () async {
    seedSession(); // storage holds 'live-jwt'
    var calls = 0;
    final dio = _dioReturning(401, onExpired: () async => calls++);

    await expectLater(
      dio.get<dynamic>('/anything',
          options: Options(headers: {'Authorization': 'Bearer stale-jwt'})),
      throwsA(isA<DioException>()),
    );

    expect(calls, 0,
        reason: 'the request carried a token we no longer hold — that is '
            'another tab catching up, not this session ending');
  });

  test('a 401 on the token we actually hold ends the session', () async {
    seedSession();
    var calls = 0;
    final dio = _dioReturning(401, onExpired: () async => calls++);

    await expectLater(
      dio.get<dynamic>('/anything',
          options: Options(headers: {'Authorization': 'Bearer live-jwt'})),
      throwsA(isA<DioException>()),
    );

    expect(calls, 1);
  });

  test('a 401 with no readable Authorization still ends the session', () async {
    // Impersonation sends and any future path that signs elsewhere must still
    // be able to end a dead session; erring toward logout is safe for a real
    // expiry, and the unsafe case is only the stale-token one above.
    seedSession();
    var calls = 0;
    final dio = _dioReturning(401, onExpired: () async => calls++);

    await expectLater(
      dio.get<dynamic>('/anything',
          options: Options(headers: {'Authorization': 'Basic whatever'})),
      throwsA(isA<DioException>()),
    );

    expect(calls, 1);
  });

  // Um endpoint que responde 401 onde devia responder 403 não pode expulsar
  // ninguém do produto. Aconteceu: /analytics/attendants/live negava o papel
  // com 401, era consultado de dez em dez segundos, e a pessoa caía no login
  // — de onde voltava para a mesma tela, para cair de novo.
  test('um 401 solto não derruba a sessão se /me continua valendo', () async {
    seedSession();
    var expired = 0;
    final dio = _dioReturning(401,
        onExpired: () async => expired++, isSessionDead: (_) async => false);

    await expectLater(
      dio.get<dynamic>('/analytics/attendants/live',
          options: Options(headers: {'Authorization': 'Bearer live-jwt'})),
      throwsA(isA<DioException>()),
    );

    expect(expired, 0);
  });

  // E quando a checagem em si não responde, ninguém é expulso: errar para o
  // lado de manter a pessoa dentro é o certo, porque o outro lado já custou
  // um laço de login.
  test('checagem que falha não conta como prova', () async {
    seedSession();
    var expired = 0;
    final dio = _dioReturning(401,
        onExpired: () async => expired++,
        isSessionDead: (_) async => throw Exception('rede caiu'));

    await expectLater(
      dio.get<dynamic>('/qualquer',
          options: Options(headers: {'Authorization': 'Bearer live-jwt'})),
      throwsA(isA<DioException>()),
    );

    expect(expired, 0);
  });
}
