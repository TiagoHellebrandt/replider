import 'package:app/design_system/app_theme.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/settings/presentation/controllers/brand_kit_controller.dart';
import 'package:app/features/settings/presentation/screens/brand_kit_screen.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

// The forbidden list is why this screen exists. In healthcare "aprovado pela
// ANVISA" on a device that is not is a regulatory problem, not a style
// preference — and the salesperson writing it usually does not know that.

class _FakeAuth extends AuthController {
  _FakeAuth(this.role);
  final String role;

  @override
  Future<UserModel?> build() async => UserModel(
        id: 'u1',
        name: 'Teste',
        email: 't@e.com',
        token: 'tok',
        role: role,
      );
}

Future<void> _pump(
  WidgetTester tester, {
  BrandKit kit = const BrandKit(),
  String role = 'admin',
}) async {
  tester.view.physicalSize = const Size(900, 2400);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(ProviderScope(
    overrides: [
      brandKitProvider.overrideWith((ref) async => kit),
      sellerOverlayProvider.overrideWith((ref) async => const SellerOverlay()),
      authControllerProvider.overrideWith(() => _FakeAuth(role)),
    ],
    child: MaterialApp(theme: AppTheme.darkTheme, home: const BrandKitScreen()),
  ));
  await tester.pumpAndSettle();
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  testWidgets('shows both claim lists, forbidden last', (tester) async {
    await _pump(tester);

    expect(find.byKey(const Key('brand_allowed')), findsOneWidget);
    expect(find.byKey(const Key('brand_forbidden')), findsOneWidget);
    // The one with a consequence outside the conversation comes last, so it
    // is the thing the reader leaves the section on.
    final allowed =
        tester.getTopLeft(find.byKey(const Key('brand_allowed'))).dy;
    final forbidden =
        tester.getTopLeft(find.byKey(const Key('brand_forbidden'))).dy;
    expect(forbidden, greaterThan(allowed));
  });

  testWidgets('existing claims render as removable chips', (tester) async {
    await _pump(
      tester,
      kit: const BrandKit(
        forbiddenClaims: ['cura', 'aprovado pela ANVISA'],
      ),
    );

    expect(find.text('cura'), findsOneWidget);
    expect(find.text('aprovado pela ANVISA'), findsOneWidget);
  });

  // A blank rule would match every text ever written.
  testWidgets('a blank claim is not added', (tester) async {
    await _pump(tester);

    final input = find.descendant(
      of: find.byKey(const Key('brand_forbidden')),
      matching: find.byType(TextField),
    );
    await tester.enterText(input, '   ');
    await tester.tap(find.descendant(
      of: find.byKey(const Key('brand_forbidden')),
      matching: find.text(t.brand.add),
    ));
    await tester.pumpAndSettle();

    expect(find.byType(Chip), findsNothing);
  });

  testWidgets('a claim can be added', (tester) async {
    await _pump(tester);

    final input = find.descendant(
      of: find.byKey(const Key('brand_forbidden')),
      matching: find.byType(TextField),
    );
    await tester.enterText(input, 'cura');
    // O botão segue o campo: precisa do quadro em que ele acorda.
    await tester.pump();
    await tester.tap(find.descendant(
      of: find.byKey(const Key('brand_forbidden')),
      matching: find.text(t.brand.add),
    ));
    await tester.pumpAndSettle();

    expect(find.widgetWithText(Chip, 'cura'), findsOneWidget);
  });

  // Deciding what the BUSINESS may claim is not one salesperson's call — but
  // they still need to read the rules they are held to.
  testWidgets('a salesperson reads the rules but cannot change them',
      (tester) async {
    await _pump(
      tester,
      role: 'attendant',
      kit: const BrandKit(forbiddenClaims: ['cura']),
    );

    expect(find.text('cura'), findsOneWidget);
    expect(find.byKey(const Key('brand_save')), findsNothing);
    expect(find.text(t.brand.add), findsNothing);
  });

  // The limit is stated rather than discovered by having a draft rewritten.
  testWidgets('the personal section says it cannot unlock what is forbidden',
      (tester) async {
    await _pump(tester);

    expect(find.byKey(const Key('brand_personal_limit')), findsOneWidget);
    expect(find.text(t.brand.personal_subtitle), findsOneWidget);
  });

  // A salesperson has no company-level edit, but always has their own layer.
  testWidgets('the personal section is available to a salesperson',
      (tester) async {
    await _pump(tester, role: 'attendant');

    expect(find.byKey(const Key('brand_personal_save')), findsOneWidget);
  });
}
