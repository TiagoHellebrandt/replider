import 'package:app/design_system/app_theme.dart';
import 'package:app/features/auth/data/datasources/public_config_provider.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/auth/presentation/screens/register_screen.dart';
import 'package:app/features/auth/presentation/widgets/google_sign_in_button.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  setUpAll(() {
    FlutterSecureStorage.setMockInitialValues({});
  });

  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
  });

  // US-F001: the screen reads GET /v1/config before deciding whether to
  // render the form. Tests pin the flag so no widget test ever dials the
  // network.
  Widget underTest({bool registrationEnabled = true}) => ProviderScope(
        overrides: [
          authControllerProvider.overrideWith(() => _IdleAuth()),
          publicConfigProvider.overrideWith(
            (ref) async =>
                PublicConfig(registrationEnabled: registrationEnabled),
          ),
        ],
        child: MaterialApp(
          theme: AppTheme.darkTheme,
          home: const RegisterScreen(),
        ),
      );

  testWidgets('register screen offers Google sign-up alongside the form',
      (tester) async {
    // Tall viewport so the button (below the form fold) is built.
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(underTest());
    await tester.pumpAndSettle();

    expect(find.byType(GoogleSignInButton), findsOneWidget);
    // The email/password register CTA is still present.
    expect(find.byKey(const Key('register_button')), findsOneWidget);
  });

  testWidgets('registration paused hides the form and offers login only',
      (tester) async {
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(underTest(registrationEnabled: false));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('registration_paused')), findsOneWidget);
    expect(find.byKey(const Key('paused_go_to_login_button')), findsOneWidget);
    // No form, no social sign-up: every account-creating affordance is gone.
    expect(find.byKey(const Key('register_button')), findsNothing);
    expect(find.byKey(const Key('email_input')), findsNothing);
    expect(find.byType(GoogleSignInButton), findsNothing);
  });
}

class _IdleAuth extends AuthController {
  @override
  Future<UserModel?> build() async => null;
}
