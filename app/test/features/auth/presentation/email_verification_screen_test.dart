import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:app/features/auth/presentation/screens/email_verification_screen.dart';
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

  Widget createWidgetUnderTest({String email = 'user@example.com'}) {
    return ProviderScope(
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: EmailVerificationScreen(email: email),
      ),
    );
  }

  testWidgets('EmailVerificationScreen renders logo and app name',
      (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest());

    expect(find.text('Replider'), findsOneWidget);
    expect(find.byType(Image), findsOneWidget);
  });

  testWidgets('EmailVerificationScreen displays title and subtitle',
      (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest());

    expect(find.text(t.email_verification.title), findsOneWidget);
    expect(find.text(t.email_verification.subtitle), findsOneWidget);
  });

  testWidgets('EmailVerificationScreen shows the email address',
      (WidgetTester tester) async {
    const testEmail = 'test@replider.com';
    await tester.pumpWidget(createWidgetUnderTest(email: testEmail));

    expect(find.text(testEmail), findsOneWidget);
  });

  testWidgets('EmailVerificationScreen shows instruction text',
      (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest());

    expect(find.text(t.email_verification.instruction), findsOneWidget);
  });

  testWidgets('EmailVerificationScreen shows back to login button',
      (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest());

    final buttonFinder = find.byType(PrimaryPillButton);
    expect(buttonFinder, findsOneWidget);
    expect(find.text(t.email_verification.back_to_login), findsOneWidget);
  });

  testWidgets('EmailVerificationScreen shows didnt receive hint',
      (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest());

    expect(find.text(t.email_verification.didnt_receive), findsOneWidget);
  });

  testWidgets('EmailVerificationScreen shows email icon',
      (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest());

    expect(find.byIcon(Icons.mark_email_read_outlined), findsOneWidget);
  });
}
