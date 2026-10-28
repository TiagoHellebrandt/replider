import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/auth/presentation/screens/reset_password_screen.dart';
import 'package:app/i18n/strings.g.dart';

/// Captures the reset args so a test can assert the token + new password were
/// forwarded; optionally throws to exercise the error path.
class _FakeAuthController extends AuthController {
  String? token;
  String? password;
  bool fail = false;

  @override
  FutureOr<UserModel?> build() => null;

  @override
  Future<void> resetPassword(String t, String p) async {
    token = t;
    password = p;
    if (fail) throw Exception('boom');
  }
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.en));

  Widget harness(String token, {_FakeAuthController? fake}) => ProviderScope(
        overrides: [
          authControllerProvider
              .overrideWith(() => fake ?? _FakeAuthController()),
        ],
        child: MaterialApp(
          theme: AppTheme.darkTheme,
          home: ResetPasswordScreen(token: token),
        ),
      );

  testWidgets('blank token shows the invalid-link state, no inputs',
      (tester) async {
    await tester.pumpWidget(harness(''));
    await tester.pump();

    expect(find.text(t.auth.reset_invalid_link), findsOneWidget);
    expect(find.byKey(const Key('password_input')), findsNothing);
  });

  testWidgets('with a token renders both password fields + submit',
      (tester) async {
    await tester.pumpWidget(harness('tok123'));
    await tester.pump();

    expect(find.byKey(const Key('password_input')), findsOneWidget);
    expect(find.byKey(const Key('confirm_input')), findsOneWidget);
    expect(find.byKey(const Key('submit_button')), findsOneWidget);
  });

  testWidgets('a password shorter than 8 chars is rejected, no call',
      (tester) async {
    final fake = _FakeAuthController();
    await tester.pumpWidget(harness('tok123', fake: fake));
    await tester.pump();

    await tester.enterText(find.byKey(const Key('password_input')), 'short');
    await tester.enterText(find.byKey(const Key('confirm_input')), 'short');
    await tester.pump();
    await tester.tap(find.byKey(const Key('submit_button')));
    await tester.pump();

    expect(find.text(t.auth.reset_min_length), findsOneWidget);
    expect(fake.token, isNull);
  });

  testWidgets('mismatched confirmation is rejected, no call', (tester) async {
    final fake = _FakeAuthController();
    await tester.pumpWidget(harness('tok123', fake: fake));
    await tester.pump();

    await tester.enterText(
        find.byKey(const Key('password_input')), 'longenough1');
    await tester.enterText(
        find.byKey(const Key('confirm_input')), 'different1');
    await tester.pump();
    await tester.tap(find.byKey(const Key('submit_button')));
    await tester.pump();

    expect(find.text(t.auth.reset_mismatch), findsOneWidget);
    expect(fake.token, isNull);
  });

  testWidgets('valid matching password forwards token + password and confirms',
      (tester) async {
    final fake = _FakeAuthController();
    await tester.pumpWidget(harness('tok123', fake: fake));
    await tester.pump();

    await tester.enterText(
        find.byKey(const Key('password_input')), 'longenough1');
    await tester.enterText(
        find.byKey(const Key('confirm_input')), 'longenough1');
    await tester.pump();
    await tester.tap(find.byKey(const Key('submit_button')));
    await tester.pump();

    expect(fake.token, 'tok123');
    expect(fake.password, 'longenough1');
    expect(find.text(t.auth.reset_success), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}
