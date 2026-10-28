import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/design_system/components/cached_network_image.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/auth/presentation/widgets/user_account_chip.dart';
import 'package:app/i18n/strings.g.dart';

void main() {
  setUpAll(() {
    FlutterSecureStorage.setMockInitialValues({});
  });

  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
  });

  Widget createWidgetUnderTest({UserModel? user}) {
    return ProviderScope(
      overrides: [
        authControllerProvider.overrideWith(() => _FakeAuthController(user)),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const Scaffold(body: UserAccountChip()),
      ),
    );
  }

  testWidgets('renders nothing when no user is logged in',
      (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pump();

    expect(find.byType(UserAccountChip), findsOneWidget);
    expect(find.byType(CircleAvatar), findsNothing);
  });

  testWidgets('shows initial-letter avatar for non-Google user',
      (WidgetTester tester) async {
    final user = UserModel(
      id: 'user-1',
      name: 'Test User',
      email: 'test@example.com',
      token: 'fake-token',
    );

    await tester.pumpWidget(createWidgetUnderTest(user: user));
    await tester.pump();

    expect(find.byType(AppNetworkImage), findsNothing);
    expect(find.text('T'), findsOneWidget);
  });

  testWidgets('renders Google avatar via cached AppNetworkImage',
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
}

class _FakeAuthController extends AuthController {
  final UserModel? _user;
  _FakeAuthController(this._user);

  @override
  FutureOr<UserModel?> build() async => _user;
}
