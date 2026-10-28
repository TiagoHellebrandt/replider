import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/billing/presentation/widgets/billing_pill.dart';
import 'package:app/i18n/strings.g.dart';

class _FakeAuth extends AuthController {
  _FakeAuth(this._user);
  final UserModel _user;
  @override
  FutureOr<UserModel?> build() async => _user;
}

UserModel _user({required bool bypass, required bool contract}) => UserModel(
      id: 'u',
      name: 'n',
      email: 'e',
      token: 't',
      isTrialing: true,
      trialDaysRemaining: 0,
      subscriptionStatus: 'trialing',
      planName: 'Enterprise',
      trialBypass: bypass,
      isContractPlan: contract,
    );

Widget _harness(UserModel u) => ProviderScope(
      overrides: [authControllerProvider.overrideWith(() => _FakeAuth(u))],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const Scaffold(body: BillingPill()),
      ),
    );

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.en));

  // Sales-led invite accounts were SOLD their plan in a conversation; a
  // billing pill (least of all a trial chip) contradicts the invite banner
  // that just said the account is born contracted. They get NO pill.
  testWidgets('trial_bypass account shows no billing pill at all',
      (tester) async {
    await tester.pumpWidget(_harness(_user(bypass: true, contract: false)));
    await tester.pumpAndSettle();
    expect(find.text(t.billing.billing_pill_trial_expired), findsNothing);
    expect(find.text('Enterprise'), findsNothing);
  });

  testWidgets('contract account shows no billing pill at all', (tester) async {
    await tester.pumpWidget(_harness(_user(bypass: false, contract: true)));
    await tester.pumpAndSettle();
    expect(find.text(t.billing.billing_pill_trial_expired), findsNothing);
    expect(find.text('Enterprise'), findsNothing);
  });

  testWidgets('self-serve expired trial still shows "trial expired"',
      (tester) async {
    await tester.pumpWidget(_harness(_user(bypass: false, contract: false)));
    await tester.pumpAndSettle();
    expect(find.text(t.billing.billing_pill_trial_expired), findsOneWidget);
  });
}
