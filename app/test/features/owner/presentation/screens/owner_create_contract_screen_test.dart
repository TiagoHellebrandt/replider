import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/features/owner/presentation/screens/owner_create_contract_screen.dart';
import 'package:app/i18n/strings.g.dart';

GoRouter _router() {
  return GoRouter(routes: [
    GoRoute(path: '/', builder: (_, __) => const OwnerCreateContractScreen()),
    GoRoute(path: '/owner/accounts', builder: (_, __) => const Scaffold()),
  ]);
}

Widget _harness() {
  return ProviderScope(
    child: MaterialApp.router(
      theme: AppTheme.darkTheme,
      routerConfig: _router(),
    ),
  );
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.en));

  group('OwnerCreateContractScreen', () {
    testWidgets('renders title + form labels', (tester) async {
      await tester.pumpWidget(_harness());
      await tester.pumpAndSettle();

      expect(find.text(t.owner.create_title), findsOneWidget);
      expect(find.text(t.owner.create_company), findsOneWidget);
      expect(find.text(t.owner.create_admin_email), findsOneWidget);
      expect(find.text(t.owner.create_admin_name), findsOneWidget);
      expect(find.text(t.owner.create_plan), findsOneWidget);
    });

    testWidgets('renders override fields after scroll', (tester) async {
      await tester.pumpWidget(_harness());
      await tester.pumpAndSettle();

      // Form is a ListView; the override fields + submit live below
      // the visible viewport on a 600px test screen.
      await tester.drag(find.byType(ListView), const Offset(0, -800));
      await tester.pumpAndSettle();

      expect(find.text(t.owner.cap_max_agents), findsOneWidget);
      expect(find.text(t.owner.cap_max_channels), findsOneWidget);
      expect(find.text(t.owner.cap_max_conversations), findsOneWidget);
      expect(find.text(t.owner.cap_max_tokens), findsOneWidget);
    });

    testWidgets('switching to existing-account mode swaps to the picker',
        (tester) async {
      await tester.pumpWidget(_harness());
      await tester.pumpAndSettle();

      // Defaults to the new-account form.
      expect(find.text(t.owner.create_company), findsOneWidget);

      await tester.tap(find.text(t.owner.mode_existing_account));
      await tester.pumpAndSettle();

      // New-account fields gone; the account picker takes over.
      expect(find.text(t.owner.create_company), findsNothing);
      expect(find.text(t.owner.pick_account_label), findsOneWidget);

      // Submit CTA flips to "Apply contract".
      await tester.drag(find.byType(ListView), const Offset(0, -1500));
      await tester.pumpAndSettle();
      expect(find.text(t.owner.apply_contract_submit), findsOneWidget);
    });

    testWidgets('renders trial bypass switch + submit CTA', (tester) async {
      await tester.pumpWidget(_harness());
      await tester.pumpAndSettle();

      await tester.drag(find.byType(ListView), const Offset(0, -1500));
      await tester.pumpAndSettle();

      expect(find.text(t.owner.create_trial_bypass), findsOneWidget);
      expect(find.text(t.owner.create_submit), findsOneWidget);
      // SwitchListTile present.
      expect(find.byType(SwitchListTile), findsOneWidget);
    });
  });
}
