import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:app/features/auth/data/datasources/public_config_provider.dart';
import 'package:app/features/auth/presentation/screens/login_screen.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/design_system/components/primary_pill_button.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

void main() {
  setUpAll(() {
    FlutterSecureStorage.setMockInitialValues({});
  });

  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
  });

  // US-F001: pin the signup flag so the screen never dials GET /v1/config.
  Widget createWidgetUnderTest({bool registrationEnabled = true}) {
    return ProviderScope(
      overrides: [
        publicConfigProvider.overrideWith(
          (ref) async => PublicConfig(registrationEnabled: registrationEnabled),
        ),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const LoginScreen(),
      ),
    );
  }

  testWidgets(
      'LoginScreen should display all required elements from new reference',
      (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest());
    // The create-account link is FAIL-CLOSED behind the config flag, so it
    // only renders after the flag resolves.
    await tester.pumpAndSettle();

    // 1. Logo and App Name
    expect(find.byType(Image), findsOneWidget); // Logo
    expect(find.text('Replider'), findsOneWidget);

    // 2. Title
    expect(find.text(t.auth.login_title), findsOneWidget);

    // 3. Email and Password inputs
    expect(find.byKey(const Key('email_input')), findsOneWidget);
    expect(find.byKey(const Key('password_input')), findsOneWidget);

    // No "remember me" checkbox — the session is always persisted.
    expect(find.byType(Checkbox), findsNothing);
    expect(find.text(t.auth.forgot_password), findsOneWidget);

    // 4. Login Button (Login)
    final buttonFinder = find.byType(PrimaryPillButton);
    expect(buttonFinder, findsOneWidget);

    // Ensure button is initially disabled because fields are empty
    final PrimaryPillButton button = tester.widget(buttonFinder);
    expect(button.onPressed, isNull);

    // 5. Social Login Buttons (Meta hidden until SDK integration)
    expect(find.byKey(const Key('google_login_button')), findsOneWidget);

    // Bottom text
    expect(find.text(t.auth.create_account), findsOneWidget);
  });

  testWidgets('LoginScreen should enable Login button when fields are filled',
      (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest());

    // Enter text
    await tester.enterText(
        find.byKey(const Key('email_input')), 'test@test.com');
    await tester.enterText(
        find.byKey(const Key('password_input')), 'password123');
    await tester.pumpAndSettle();

    // Ensure button is now enabled
    final buttonFinder = find.byType(PrimaryPillButton);
    final PrimaryPillButton button = tester.widget(buttonFinder);
    expect(button.onPressed, isNotNull);
  });

  testWidgets('LoginScreen should show validation errors',
      (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest());

    // Try to enter invalid email and valid password just to trigger validation
    await tester.enterText(
        find.byKey(const Key('email_input')), 'invalid_email');
    await tester.enterText(find.byKey(const Key('password_input')), 'pass123');
    await tester.pumpAndSettle();

    // Since button is enabled (not empty), let's press it to trigger form validation
    final buttonFinder = find.byType(PrimaryPillButton);
    final PrimaryPillButton button = tester.widget(buttonFinder);

    if (button.onPressed != null) {
      await tester.ensureVisible(buttonFinder);
      await tester.tap(buttonFinder);
      await tester.pumpAndSettle();
    }

    // Expect validation error text (uses current locale's translation)
    final errorFinder = find.text(t.auth.invalid_email);
    expect(errorFinder, findsOneWidget);
  });

  testWidgets('hides the create-account CTA while signup is paused',
      (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest(registrationEnabled: false));
    await tester.pumpAndSettle();

    // Login itself is untouched; only the door to /register closes.
    expect(find.byKey(const Key('email_input')), findsOneWidget);
    expect(find.text(t.auth.dont_have_account), findsNothing);
    expect(find.text(t.auth.create_account), findsNothing);
  });

  // Fail-closed: while the flag is unknown the link stays hidden. In
  // production the door is closed, so failing open made "Criar conta" →
  // "Cadastro em breve" the most likely first experience of the product.
  testWidgets('the create-account CTA stays hidden while the flag loads',
      (WidgetTester tester) async {
    await tester.pumpWidget(ProviderScope(
      overrides: [
        publicConfigProvider.overrideWith(
          // Never resolves — the eternal loading state.
          (ref) => Completer<PublicConfig>().future,
        ),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const LoginScreen(),
      ),
    ));
    await tester.pump();

    expect(find.byKey(const Key('email_input')), findsOneWidget);
    expect(find.text(t.auth.create_account), findsNothing);
  });
}
