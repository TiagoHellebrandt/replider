import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:app/features/auth/presentation/screens/forgot_password_screen.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/auth/domain/models/user_model.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// Fake AuthController whose [forgotPassword] resolves only when the test
/// completes [completer] — lets us settle the future AFTER the screen is
/// disposed to prove the `.then`/`.catchError` callbacks no longer touch
/// a dead State (US-101).
class _FakeAuthController extends AuthController {
  _FakeAuthController(this.completer);
  final Completer<void> completer;

  @override
  FutureOr<UserModel?> build() => null;

  @override
  Future<void> forgotPassword(String email) => completer.future;
}

/// Counts forgotPassword calls and resolves immediately — used to prove the
/// post-send cooldown blocks repeat taps from firing extra emails.
class _CountingAuthController extends AuthController {
  int calls = 0;

  @override
  FutureOr<UserModel?> build() => null;

  @override
  Future<void> forgotPassword(String email) async {
    calls++;
  }
}

void main() {
  setUpAll(() {
    FlutterSecureStorage.setMockInitialValues({});
  });

  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
  });

  Widget createWidgetUnderTest() {
    return ProviderScope(
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const ForgotPasswordScreen(),
      ),
    );
  }

  testWidgets(
      'ForgotPasswordScreen should display email input and submit button',
      (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest());

    // Inputs
    expect(find.byKey(const Key('email_input')), findsOneWidget);

    // Button
    // Looking for a button that implies submission, e.g. "Enviar" or "Recuperar"
    expect(find.byKey(const Key('submit_button')), findsOneWidget);
  });

  Widget harness(Completer<void> completer) => ProviderScope(
        overrides: [
          authControllerProvider
              .overrideWith(() => _FakeAuthController(completer)),
        ],
        child: MaterialApp(
          theme: AppTheme.darkTheme,
          home: const ForgotPasswordScreen(),
        ),
      );

  testWidgets(
      'submit success while mounted shows the confirmation (wiring sanity)',
      (WidgetTester tester) async {
    final completer = Completer<void>();
    await tester.pumpWidget(harness(completer));

    await tester.enterText(
        find.byKey(const Key('email_input')), 'user@example.com');
    await tester.pump();
    await tester.tap(find.byKey(const Key('submit_button')));
    await tester.pump();

    // Resolve while still mounted — the `.then` setState must run.
    completer.complete();
    await tester.pump();

    // The honest phrasing: the backend answers 200 for unknown accounts on
    // purpose, so the banner must not assert a send that may not exist.
    expect(find.text(t.auth.forgot_success), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets(
      'submit future resolving after dispose does not setState on dead State (US-101)',
      (WidgetTester tester) async {
    final completer = Completer<void>();
    await tester.pumpWidget(harness(completer));

    await tester.enterText(
        find.byKey(const Key('email_input')), 'user@example.com');
    await tester.pump();
    await tester.tap(find.byKey(const Key('submit_button')));
    await tester.pump();

    // Tear the screen down while the request is still in flight.
    await tester.pumpWidget(
      const MaterialApp(home: Scaffold(body: SizedBox.shrink())),
    );

    // Resolve the future AFTER dispose — the unguarded version would call
    // setState() on a defunct State and throw.
    completer.complete();
    await tester.pump();

    expect(tester.takeException(), isNull);
  });

  testWidgets(
      'after a successful send the button is disabled + counts down, blocking repeat taps',
      (WidgetTester tester) async {
    final fake = _CountingAuthController();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [authControllerProvider.overrideWith(() => fake)],
        child: MaterialApp(
          theme: AppTheme.darkTheme,
          home: const ForgotPasswordScreen(),
        ),
      ),
    );

    await tester.enterText(
        find.byKey(const Key('email_input')), 'user@example.com');
    await tester.pump();
    await tester.tap(find.byKey(const Key('submit_button')));
    await tester.pump(); // flush the immediate forgotPassword + cooldown start

    expect(fake.calls, 1);
    // Cooldown label shows the full window right after a send.
    expect(find.text(t.auth.forgot_resend_in(seconds: 60)), findsOneWidget);

    // Tapping again during cooldown must NOT fire a second email.
    await tester.tap(find.byKey(const Key('submit_button')));
    await tester.pump();
    expect(fake.calls, 1);

    // Dispose the screen so the periodic cooldown timer is cancelled (avoids
    // the "Timer still pending" teardown failure).
    await tester.pumpWidget(const SizedBox.shrink());
  });
}
