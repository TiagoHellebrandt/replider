import 'package:app/design_system/app_theme.dart';
import 'package:app/features/settings/presentation/controllers/email_suppression_controller.dart';
import 'package:app/features/settings/presentation/screens/email_suppressions_screen.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

// The screen exists so a block is never silent. These tests pin the two
// things that makes true: the address is visible with the reason the
// receiving server gave, and it can be undone.

const _hardBounce = EmailSuppression(
  address: 'dono@clinicafechada.com.br',
  reason: 'hard_bounce',
  diagnostic: '550 5.1.1 User unknown',
);

const _complaint = EmailSuppression(
  address: 'dono@clinica.com.br',
  reason: 'complaint',
);

class _FakeActions implements EmailSuppressionActions {
  String? released;

  @override
  Future<void> release(String address) async {
    released = address;
  }
}

Future<_FakeActions> _pump(
  WidgetTester tester, {
  List<EmailSuppression> items = const [],
}) async {
  final actions = _FakeActions();
  await tester.pumpWidget(ProviderScope(
    overrides: [
      emailSuppressionsProvider.overrideWith((ref) async => items),
      emailSuppressionActionsProvider.overrideWithValue(actions),
    ],
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      home: const EmailSuppressionsScreen(),
    ),
  ));
  await tester.pumpAndSettle();
  return actions;
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  testWidgets('an empty list says so rather than looking broken',
      (tester) async {
    await _pump(tester);

    expect(find.byKey(const Key('suppressions_empty')), findsOneWidget);
  });

  // The question a person arrives with is "why can't I email them?", and the
  // answer is the remote server's own words.
  testWidgets('shows the address and the diagnostic verbatim', (tester) async {
    await _pump(tester, items: [_hardBounce]);

    expect(find.text('dono@clinicafechada.com.br'), findsOneWidget);
    expect(find.text('550 5.1.1 User unknown'), findsOneWidget);
    expect(find.text(t.suppressions.reason_hard_bounce), findsOneWidget);
  });

  // A complaint is not a bounce: the address WORKS, and the person told
  // their provider we are abusive. Reading them as the same thing hides
  // which mistake was made.
  testWidgets('a complaint is labelled as its own thing', (tester) async {
    await _pump(tester, items: [_complaint]);

    expect(find.text(t.suppressions.reason_complaint), findsOneWidget);
    expect(find.text(t.suppressions.reason_hard_bounce), findsNothing);
  });

  // A suppression can be wrong — an address is fixed, a mailbox emptied.
  testWidgets('releasing confirms first, then releases', (tester) async {
    final actions = await _pump(tester, items: [_hardBounce]);

    await tester
        .tap(find.byKey(const Key('release_dono@clinicafechada.com.br')));
    await tester.pumpAndSettle();
    // The dialog names the address, so nobody releases the wrong one.
    expect(find.textContaining('dono@clinicafechada.com.br'), findsWidgets);
    await tester.tap(find.text(t.suppressions.release).last);
    await tester.pumpAndSettle();

    expect(actions.released, 'dono@clinicafechada.com.br');
  });

  // Releasing puts an address a mail server refused back into circulation.
  // Cancelling must leave it blocked.
  testWidgets('cancelling releases nothing', (tester) async {
    final actions = await _pump(tester, items: [_hardBounce]);

    await tester
        .tap(find.byKey(const Key('release_dono@clinicafechada.com.br')));
    await tester.pumpAndSettle();
    await tester.tap(find.text(t.common.cancel));
    await tester.pumpAndSettle();

    expect(actions.released, isNull);
  });
}
