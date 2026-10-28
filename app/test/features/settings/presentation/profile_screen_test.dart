import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:app/features/settings/presentation/screens/profile_screen.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/design_system/components/cached_network_image.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

void main() {
  setUpAll(() {
    FlutterSecureStorage.setMockInitialValues({});
  });

  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
  });

  final testUser = UserModel(
    id: 'user-1',
    name: 'Test User',
    email: 'test@example.com',
    token: 'fake-token',
  );

  Widget createWidgetUnderTest({UserModel? user}) {
    return ProviderScope(
      overrides: [
        authControllerProvider
            .overrideWith(() => _FakeAuthController(user ?? testUser)),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const ProfileScreen(),
      ),
    );
  }

  testWidgets('ProfileScreen renders app bar with title',
      (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pumpAndSettle();

    expect(find.text(t.profile.title), findsOneWidget);
  });

  testWidgets('ProfileScreen shows user avatar with initial',
      (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pumpAndSettle();

    expect(find.byType(CircleAvatar), findsOneWidget);
    expect(find.text('T'), findsOneWidget); // First letter of "Test User"
  });

  testWidgets('ProfileScreen renders Google avatar via cached AppNetworkImage',
      (WidgetTester tester) async {
    final googleUser = UserModel(
      id: 'user-2',
      name: 'Google User',
      email: 'google@example.com',
      token: 'fake-token',
      picture: 'https://example.com/avatar.png',
    );

    await tester.pumpWidget(createWidgetUnderTest(user: googleUser));
    await tester.pump(); // avoid network fetch in pumpAndSettle

    // Avatar goes through the cached wrapper, not a bare NetworkImage.
    expect(find.byType(AppNetworkImage), findsOneWidget);
  });

  testWidgets('ProfileScreen shows user name and email',
      (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pumpAndSettle();

    expect(find.text('Test User'),
        findsWidgets); // Shown in avatar area and name field
    expect(find.text('test@example.com'),
        findsWidgets); // Shown in avatar area and email field
  });

  testWidgets('ProfileScreen shows personal info section',
      (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pumpAndSettle();

    expect(find.text(t.profile.personal_info), findsOneWidget);
    expect(find.text(t.profile.save_info), findsOneWidget);
  });

  testWidgets('ProfileScreen shows change password section',
      (WidgetTester tester) async {
    // The signature field (US-018) pushed this section below the default
    // 800x600 test viewport — enlarge so it's on screen.
    tester.view.physicalSize = const Size(1200, 1800);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pumpAndSettle();

    expect(find.text(t.profile.change_password), findsOneWidget);
    expect(find.text(t.profile.change_password_button), findsOneWidget);
  });

  testWidgets('ProfileScreen shows name and email text fields pre-filled',
      (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pumpAndSettle();

    // Check that text fields with labels exist
    expect(find.text(t.profile.name), findsOneWidget);
    expect(find.text(t.profile.email), findsOneWidget);
  });

  testWidgets('ProfileScreen shows signature field pre-filled (US-018)',
      (WidgetTester tester) async {
    final signedUser = UserModel(
      id: 'user-3',
      name: 'Signed User',
      email: 'signed@example.com',
      token: 'fake-token',
      signature: 'Abraços, Signed',
    );

    await tester.pumpWidget(createWidgetUnderTest(user: signedUser));
    await tester.pumpAndSettle();

    expect(find.text(t.profile.signature_label), findsOneWidget);
    expect(find.text('Abraços, Signed'), findsOneWidget);
  });

  testWidgets('ProfileScreen shows password fields',
      (WidgetTester tester) async {
    tester.view.physicalSize = const Size(1200, 1800);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pumpAndSettle();

    expect(find.text(t.profile.current_password), findsOneWidget);
    expect(find.text(t.profile.new_password), findsOneWidget);
    expect(find.text(t.profile.confirm_new_password), findsOneWidget);
  });

  testWidgets('ProfileScreen shows back button', (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pumpAndSettle();

    expect(find.byIcon(Icons.arrow_back), findsOneWidget);
  });

  testWidgets('ProfileScreen shows not found when user is null',
      (WidgetTester tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authControllerProvider.overrideWith(() => _FakeAuthController(null)),
        ],
        child: MaterialApp(
          theme: AppTheme.darkTheme,
          home: const ProfileScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text(t.profile.user_not_found), findsOneWidget);
  });
}

class _FakeAuthController extends AuthController {
  final UserModel? _user;
  _FakeAuthController(this._user);

  @override
  FutureOr<UserModel?> build() async => _user;
}
