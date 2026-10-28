import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/owner/presentation/widgets/owner_2fa_step_up.dart';
import 'package:app/i18n/strings.g.dart';

// The owner panel's 2FA proof expires every 12h. The contract under test: the
// operator re-proves IN PLACE and the original call is retried, so a
// half-filled form is never thrown away — and a wrong code is reported inside
// the dialog rather than closing it.

/// Answers /me/2fa/reverify with [reverifyStatus]; anything else 404s.
class _ReverifyAdapter implements HttpClientAdapter {
  _ReverifyAdapter(this.reverifyStatus);

  final int reverifyStatus;
  int calls = 0;

  @override
  Future<ResponseBody> fetch(RequestOptions options,
      Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async {
    if (options.path.contains('/me/2fa/reverify')) {
      calls++;
      if (reverifyStatus >= 400) {
        return ResponseBody.fromString(
          '{"code":"TOTP_INVALID_CODE","message":"bad code"}',
          reverifyStatus,
          headers: {
            Headers.contentTypeHeader: [Headers.jsonContentType],
          },
        );
      }
      return ResponseBody.fromString('', reverifyStatus);
    }
    return ResponseBody.fromString('{}', 404);
  }

  @override
  void close({bool force = false}) {}
}

DioException _stale() => DioException(
      requestOptions: RequestOptions(path: '/owner/invites'),
      response: Response<dynamic>(
        requestOptions: RequestOptions(path: '/owner/invites'),
        statusCode: 401,
        // The real middleware body: flat code + message.
        data: <String, dynamic>{
          'code': 'OWNER_2FA_STALE',
          'message': 'Re-enter your 2FA code to continue',
        },
      ),
    );

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.en));

  /// Host that runs [action] through withOwnerStepUp when the button is tapped,
  /// recording the outcome so the test can assert what the caller saw.
  Widget harness({
    required Future<String> Function() action,
    required _ReverifyAdapter adapter,
    required List<String> results,
    required List<Object> errors,
  }) {
    final dio = Dio(BaseOptions(baseUrl: 'https://api.test/v1'));
    dio.httpClientAdapter = adapter;
    return ProviderScope(
      overrides: [dioProvider.overrideWithValue(dio)],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: Consumer(
          builder: (context, ref, _) => Scaffold(
            body: Center(
              child: ElevatedButton(
                key: const Key('go'),
                onPressed: () async {
                  try {
                    results.add(await withOwnerStepUp(context, ref, action));
                  } catch (e) {
                    errors.add(e);
                  }
                },
                child: const Text('go'),
              ),
            ),
          ),
        ),
      ),
    );
  }

  testWidgets('a call that succeeds never asks for a code', (tester) async {
    final adapter = _ReverifyAdapter(204);
    final results = <String>[];
    final errors = <Object>[];
    var calls = 0;

    await tester.pumpWidget(harness(
      action: () async {
        calls++;
        return 'ok';
      },
      adapter: adapter,
      results: results,
      errors: errors,
    ));
    await tester.tap(find.byKey(const Key('go')));
    await tester.pumpAndSettle();

    expect(find.text(t.owner.stepup_title), findsNothing);
    expect(calls, 1);
    expect(results, ['ok']);
    expect(errors, isEmpty);
  });

  testWidgets('an expired proof is refreshed in place and the call retried',
      (tester) async {
    final adapter = _ReverifyAdapter(204);
    final results = <String>[];
    final errors = <Object>[];
    var calls = 0;

    await tester.pumpWidget(harness(
      action: () async {
        calls++;
        // Stale only the first time — exactly what the server does once the
        // proof is refreshed.
        if (calls == 1) throw _stale();
        return 'ok';
      },
      adapter: adapter,
      results: results,
      errors: errors,
    ));
    await tester.tap(find.byKey(const Key('go')));
    await tester.pumpAndSettle();

    expect(find.text(t.owner.stepup_title), findsOneWidget,
        reason: 'the stale proof asks for a code instead of dead-ending');

    await tester.enterText(
        find.byKey(const Key('owner_stepup_code')), '123456');
    // The Confirm button is gated on a non-empty code — let it re-enable.
    await tester.pump();
    await tester.tap(find.byKey(const Key('owner_stepup_confirm')));
    await tester.pumpAndSettle();

    expect(adapter.calls, 1, reason: 'the code went to /me/2fa/reverify');
    expect(calls, 2, reason: 'the original action ran again after the proof');
    expect(results, ['ok']);
    expect(errors, isEmpty);
    expect(find.text(t.owner.stepup_title), findsNothing);
  });

  testWidgets('a wrong code is reported inside the dialog, which stays open',
      (tester) async {
    final adapter = _ReverifyAdapter(400);
    final results = <String>[];
    final errors = <Object>[];
    var calls = 0;

    await tester.pumpWidget(harness(
      action: () async {
        calls++;
        throw _stale();
      },
      adapter: adapter,
      results: results,
      errors: errors,
    ));
    await tester.tap(find.byKey(const Key('go')));
    await tester.pumpAndSettle();
    await tester.enterText(
        find.byKey(const Key('owner_stepup_code')), '000000');
    await tester.pump();
    await tester.tap(find.byKey(const Key('owner_stepup_confirm')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('owner_stepup_error')), findsOneWidget);
    expect(find.text(t.owner.stepup_title), findsOneWidget,
        reason: 'a bad code must not close the dialog and lose the form');
    expect(calls, 1, reason: 'no retry until the proof actually refreshes');
  });

  testWidgets('cancelling surfaces the original error to the caller',
      (tester) async {
    final adapter = _ReverifyAdapter(204);
    final results = <String>[];
    final errors = <Object>[];
    var calls = 0;

    await tester.pumpWidget(harness(
      action: () async {
        calls++;
        throw _stale();
      },
      adapter: adapter,
      results: results,
      errors: errors,
    ));
    await tester.tap(find.byKey(const Key('go')));
    await tester.pumpAndSettle();
    await tester.tap(find.text(t.common.cancel));
    await tester.pumpAndSettle();

    expect(calls, 1);
    expect(results, isEmpty);
    expect(errors, hasLength(1),
        reason: 'the caller still sees the refusal and shows its own message');
    expect(adapter.calls, 0);
  });
}
