import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/settings/presentation/widgets/api_keys_settings_card.dart';
import 'package:app/i18n/strings.g.dart';

class _FakeAuth extends AuthController {
  _FakeAuth(this.user);
  final UserModel? user;

  @override
  FutureOr<UserModel?> build() async => user;
}

UserModel _contractUser() => const UserModel(
      id: 'u',
      name: 'N',
      email: 'e@x.com',
      token: 't',
      role: 'admin',
      planName: 'Enterprise',
      isContractPlan: true,
    );

Widget _wrap(UserModel? user) => ProviderScope(
      overrides: [
        authControllerProvider.overrideWith(() => _FakeAuth(user)),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const Scaffold(
          body: SingleChildScrollView(child: ApiKeysSettingsCard()),
        ),
      ),
    );

void main() {
  setUpAll(() {
    FlutterSecureStorage.setMockInitialValues({});
  });
  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
  });

  testWidgets('card hidden for a self-serve (non-contract) account',
      (tester) async {
    await tester.pumpWidget(_wrap(const UserModel(
      id: 'u',
      name: 'N',
      email: 'e@x.com',
      token: 't',
    )));
    await tester.pumpAndSettle();

    expect(find.text(t.api_keys.title), findsNothing);
  });

  testWidgets('Save enables after typing a key (regression: onChanged rebuild)',
      (tester) async {
    await tester.pumpWidget(_wrap(_contractUser()));
    await tester.pumpAndSettle();

    // Card visible for a contract account.
    expect(find.text(t.api_keys.title), findsOneWidget);

    // Open the inline editor on the first provider row (no key yet → "Add").
    await tester.tap(find.text(t.api_keys.add).first);
    await tester.pumpAndSettle();

    // Save starts disabled — the field is empty.
    FilledButton saveBtn() => tester.widget<FilledButton>(
          find.widgetWithText(FilledButton, t.api_keys.save),
        );
    expect(saveBtn().onPressed, isNull);

    // Type a key. Without the onChanged→setState rebuild the button would
    // stay disabled even though _ctrl.text is now non-empty.
    await tester.enterText(find.byType(TextField), 'sk-test-1234567890');
    await tester.pump();

    expect(saveBtn().onPressed, isNotNull);
  });
}
