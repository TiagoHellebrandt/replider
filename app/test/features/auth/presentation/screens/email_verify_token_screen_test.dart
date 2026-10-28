import 'dart:async';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/auth/presentation/screens/email_verify_token_screen.dart';
import 'package:app/i18n/strings.g.dart';

/// Adapter that returns a fixed status for every request — lets us drive
/// the verify-email GET to success (200) or failure (400) without a server.
class _FakeAdapter implements HttpClientAdapter {
  _FakeAdapter(this.status, [this.body = '']);
  final int status;
  final String body;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async =>
      ResponseBody.fromString(
        body,
        status,
        headers: {
          Headers.contentTypeHeader: [Headers.jsonContentType],
        },
      );

  @override
  void close({bool force = false}) {}
}

Dio _dioWith(int status, [String body = '']) {
  final dio = Dio(BaseOptions(baseUrl: 'http://test.local/v1'));
  dio.httpClientAdapter = _FakeAdapter(status, body);
  return dio;
}

Widget _wrap(Dio dio, String token) => ProviderScope(
      overrides: [dioProvider.overrideWithValue(dio)],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: EmailVerifyTokenScreen(token: token),
      ),
    );

void main() {
  setUpAll(() {
    FlutterSecureStorage.setMockInitialValues({});
  });
  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
  });

  testWidgets('shows success when the token verifies (200)', (tester) async {
    await tester.pumpWidget(_wrap(_dioWith(200, '{"message":"ok"}'), 'tok'));
    await tester.pumpAndSettle();
    expect(find.text(t.email_verification.verified_title), findsOneWidget);
  });

  testWidgets('shows failure when the token is rejected (400)', (tester) async {
    await tester.pumpWidget(_wrap(_dioWith(400, '{"code":"X"}'), 'tok'));
    await tester.pumpAndSettle();
    expect(find.text(t.email_verification.verify_failed_title), findsOneWidget);
  });

  testWidgets('empty token fails fast without a request', (tester) async {
    await tester.pumpWidget(_wrap(_dioWith(200), ''));
    await tester.pumpAndSettle();
    expect(find.text(t.email_verification.verify_failed_title), findsOneWidget);
  });
}
