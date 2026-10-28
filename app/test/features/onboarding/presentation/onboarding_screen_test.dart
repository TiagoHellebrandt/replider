import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:app/features/onboarding/domain/onboarding_variant.dart';
import 'package:app/features/onboarding/presentation/screens/onboarding_screen.dart';
import 'package:app/features/onboarding/presentation/controllers/onboarding_controller.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'dart:typed_data';
import 'package:dio/dio.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

void main() {
  setUpAll(() {
    FlutterSecureStorage.setMockInitialValues({});
  });

  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
  });

  Widget createWidgetUnderTest({
    OnboardingVariant variant = OnboardingVariant.reduced,
  }) {
    return ProviderScope(
      overrides: [
        onboardingControllerProvider
            .overrideWith(() => _FakeOnboardingController()),
        onboardingVariantProvider.overrideWithValue(variant),
        // The prefill awaits the auth future; without an override these
        // tests would boot the REAL AuthController (network timers).
        authControllerProvider.overrideWith(() => _FakeAuthWithAccount('', '')),
        dioProvider.overrideWithValue(_instantFailDio()),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const OnboardingScreen(),
      ),
    );
  }

  testWidgets('the AI key step lets you move on — with or without a key',
      (tester) async {
    // A step added to stepIndices but missing from _isNextEnabled falls into
    // the default and disables Next forever: the key saved, and the wizard
    // would not let go.
    await tester.pumpWidget(ProviderScope(
      overrides: [
        onboardingControllerProvider.overrideWith(
            () => _FakeOnboardingController(step: aiKeyStepIndex)),
        onboardingVariantProvider.overrideWithValue(OnboardingVariant.reduced),
        authControllerProvider.overrideWith(() => _FakeContractAuth()),
        dioProvider.overrideWithValue(_instantFailDio()),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const OnboardingScreen(),
      ),
    ));
    await tester.pumpAndSettle();

    final next = tester.widget<ElevatedButton>(
      find.ancestor(
        of: find.text(t.onboarding.next),
        matching: find.byType(ElevatedButton),
      ),
    );
    expect(next.onPressed, isNotNull,
        reason: 'Next must be live on the key step');
  });

  testWidgets('OnboardingScreen renders logo and app name',
      (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pump();

    expect(find.text('Replider'), findsOneWidget);
  });

  testWidgets('OnboardingScreen renders with PageView',
      (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pump();

    expect(find.byType(PageView), findsOneWidget);
  });

  testWidgets('OnboardingScreen shows next button',
      (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pump();

    expect(find.text(t.onboarding.next), findsOneWidget);
  });

  testWidgets('OnboardingScreen shows step indicator',
      (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pump();

    // 5 steps: Company, Agent, Knowledge, Preview, Channel.
    expect(find.text(t.onboarding.step_of(current: '1', total: '5')),
        findsOneWidget);
  });

  testWidgets('OnboardingScreen does not show back button on first step',
      (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pump();

    expect(find.text(t.onboarding.back), findsNothing);
  });

  testWidgets('OnboardingScreen renders without errors',
      (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pump();

    expect(find.byType(OnboardingScreen), findsOneWidget);
    expect(find.byType(Scaffold), findsWidgets);
  });

  testWidgets('constrains content width on wide screens (US-117)',
      (WidgetTester tester) async {
    // Simulate a desktop/tablet viewport wider than the 500px cap.
    tester.view.physicalSize = const Size(1400, 1000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pump();

    // Both the page content and the bottom nav are capped at 500px so
    // fields/buttons don't stretch edge-to-edge on desktop.
    final constrained = tester
        .widgetList<ConstrainedBox>(find.byType(ConstrainedBox))
        .where((b) => b.constraints.maxWidth == 500)
        .toList();
    expect(constrained.length, greaterThanOrEqualTo(2));

    // The rendered PageView must not exceed the cap on a wide screen.
    expect(tester.getSize(find.byType(PageView)).width, lessThanOrEqualTo(500));
  });

  testWidgets('the shipped variant shows a 5-step flow',
      (WidgetTester tester) async {
    await tester.pumpWidget(
      createWidgetUnderTest(variant: OnboardingVariant.reduced),
    );
    await tester.pump();

    // company → price list → customers → channel → first offer (knowledge +
    // preview stay post-onboarding nudges).
    expect(
      find.text(t.onboarding.step_of(current: '1', total: '5')),
      findsOneWidget,
    );
    expect(
      find.text(t.onboarding.step_of(current: '1', total: '3')),
      findsNothing,
    );
  });

  companyPrefillTests();
}

class _FakeOnboardingController extends OnboardingController {
  _FakeOnboardingController({this.step = 0});
  final int step;

  @override
  OnboardingState build() {
    return OnboardingState(currentStep: step);
  }
}

/// A contract account whose key is already saved — the exact state that used
/// to leave Next dead.
class _FakeContractAuth extends AuthController {
  @override
  Future<UserModel?> build() async => const UserModel(
        id: 'u1',
        name: 'Tiago',
        email: 'x@y.com.br',
        token: 't',
        isContractPlan: true,
        geminiKeyMasked: 'AIza****4f2a',
      );
}

// ── Company-name prefill (register already asked; don't ask twice) ─────────

class _FakeAuthWithAccount extends AuthController {
  _FakeAuthWithAccount(this.accountName, this.personName);
  final String accountName;
  final String personName;

  @override
  Future<UserModel?> build() async => UserModel(
        id: 'u1',
        name: personName,
        email: 'x@y.com.br',
        token: 't',
        accountName: accountName,
      );
}

void companyPrefillTests() {
  Widget harness(String accountName, String personName) {
    return ProviderScope(
      overrides: [
        onboardingControllerProvider
            .overrideWith(() => _FakeOnboardingController()),
        onboardingVariantProvider.overrideWithValue(OnboardingVariant.reduced),
        authControllerProvider
            .overrideWith(() => _FakeAuthWithAccount(accountName, personName)),
        dioProvider.overrideWithValue(_instantFailDio()),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const OnboardingScreen(),
      ),
    );
  }

  testWidgets('a real company name from the register form is prefilled',
      (tester) async {
    await tester.pumpWidget(harness('Clínica Jornada', 'Tiago'));
    await tester.pump();
    await tester.pump();

    expect(find.text('Clínica Jornada'), findsWidgets,
        reason: 'a pergunta já foi respondida no cadastro');

    // O prefill chama updateField, que agenda o auto-save do draft com
    // debounce (comportamento real) — drena o timer antes do teardown.
    await tester.pump(const Duration(seconds: 1));
  });

  testWidgets(
      'a social signup (account name == person name) still asks for the company',
      (tester) async {
    await tester.pumpWidget(harness('Tiago', 'Tiago'));
    await tester.pump();
    await tester.pump();

    // "Tiago" não é uma clínica: o campo fica vazio para a pessoa responder.
    final field = tester.widget<TextField>(find
        .descendant(
          of: find.byType(PageView),
          matching: find.byType(TextField),
        )
        .first);
    expect(field.controller?.text ?? '', isEmpty);
  });
}

/// Fails every network call instantly so widgets that fetch on build
/// (BillingPill etc.) settle without leaving request timers pending.
class _InstantFailAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(RequestOptions options,
      Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async {
    return ResponseBody.fromString('{}', 404);
  }

  @override
  void close({bool force = false}) {}
}

Dio _instantFailDio() {
  final dio = Dio(BaseOptions(baseUrl: 'https://api.test/v1'));
  dio.httpClientAdapter = _InstantFailAdapter();
  return dio;
}
