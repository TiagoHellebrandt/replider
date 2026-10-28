import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:app/features/auth/data/datasources/public_config_provider.dart';
import 'package:app/features/auth/presentation/screens/register_screen.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

void main() {
  setUpAll(() {
    FlutterSecureStorage.setMockInitialValues({});
  });

  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
  });

  Widget createWidgetUnderTest() {
    return ProviderScope(
      // US-F001: pin the signup flag so the screen renders the form
      // without dialing GET /v1/config.
      overrides: [
        publicConfigProvider.overrideWith(
          (ref) async => const PublicConfig(registrationEnabled: true),
        ),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const RegisterScreen(),
      ),
    );
  }

  testWidgets('RegisterScreen should display all required elements',
      (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest());
    // The signup flag resolves on a microtask; the first frame is the
    // loading spinner (US-F001).
    await tester.pumpAndSettle();

    // Inputs
    expect(find.byKey(const Key('account_name_input')), findsOneWidget);
    expect(find.byKey(const Key('name_input')), findsOneWidget);
    expect(find.byKey(const Key('email_input')), findsOneWidget);
    expect(find.byKey(const Key('password_input')), findsOneWidget);
    expect(find.byKey(const Key('confirm_password_input')), findsOneWidget);

    // Button — uses localized register_title (appears as title and button text)
    expect(find.text(t.auth.register_title), findsWidgets);
  });
  inviteModeTests();
}

// ── Invite mode (sales-led signup, docs/fluxo-criacao-conta.md) ────────────

/// A Dio whose adapter answers the invite probe with a canned response, so
/// the screen's invite path runs without a network.
Dio _probeDio(int status, String body) {
  final dio = Dio(BaseOptions(baseUrl: 'https://api.test/v1'));
  dio.httpClientAdapter = _FakeAdapter(status, body);
  return dio;
}

class _FakeAdapter implements HttpClientAdapter {
  _FakeAdapter(this.status, this.body);
  final int status;
  final String body;

  @override
  Future<ResponseBody> fetch(RequestOptions options,
      Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async {
    return ResponseBody.fromString(body, status, headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    });
  }

  @override
  void close({bool force = false}) {}
}

Widget _inviteWidget(Dio dio,
    {bool registrationEnabled = false, String token = 'sekret'}) {
  return ProviderScope(
    overrides: [
      dioProvider.overrideWithValue(dio),
      publicConfigProvider.overrideWith(
        (ref) async => PublicConfig(registrationEnabled: registrationEnabled),
      ),
    ],
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      home: RegisterScreen(inviteToken: token),
    ),
  );
}

void inviteModeTests() {
  testWidgets(
      'a valid invite opens the CLOSED door, locks the e-mail and says so',
      (tester) async {
    await tester.pumpWidget(_inviteWidget(
      _probeDio(200, '{"valid":true,"email":"compras@clinica.com.br"}'),
    ));
    await tester.pumpAndSettle();

    // Registration is paused, yet the form renders — the invite is the
    // permission.
    expect(find.byKey(const Key('registration_paused')), findsNothing);
    expect(find.byKey(const Key('invite_banner_valid')), findsOneWidget);
    expect(find.text(t.auth.invite_banner(email: 'compras@clinica.com.br')),
        findsOneWidget);

    final email = tester.widget<TextFormField>(
      find.descendant(
        of: find.byKey(const Key('email_input')),
        matching: find.byType(TextFormField),
      ),
    );
    expect(email.controller!.text, 'compras@clinica.com.br');
  });

  testWidgets(
      'a dead invite with the door closed is a dead END — no form to fill',
      (tester) async {
    await tester.pumpWidget(_inviteWidget(_probeDio(200, '{"valid":false}')));
    await tester.pumpAndSettle();

    // Says what happened and where to go…
    expect(find.byKey(const Key('invite_dead_end')), findsOneWidget);
    expect(find.text(t.auth.invite_invalid), findsOneWidget);
    expect(
        find.byKey(const Key('invite_dead_end_login_button')), findsOneWidget);
    // …and never renders five fields that can only be refused at the end.
    expect(find.byKey(const Key('email_input')), findsNothing);
    expect(find.byKey(const Key('register_button')), findsNothing);
  });

  testWidgets(
      'with the door OPEN a dead invite still shows the form, with the warning',
      (tester) async {
    await tester.pumpWidget(_inviteWidget(
      _probeDio(200, '{"valid":false}'),
      registrationEnabled: true,
    ));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('invite_banner_invalid')), findsOneWidget);
    expect(find.byKey(const Key('email_input')), findsOneWidget,
        reason: 'porta aberta: o cadastro normal continua possível');
  });
}
