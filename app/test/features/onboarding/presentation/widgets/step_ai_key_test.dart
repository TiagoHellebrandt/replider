import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/onboarding/domain/onboarding_variant.dart';
import 'package:app/features/onboarding/presentation/widgets/step_ai_key.dart';
import 'package:app/i18n/strings.g.dart';

// A contract account runs the AI on its OWN key, and nothing in the wizard used
// to ask for one — so the next step (reading a catalogue PDF) refused with an
// error that blamed the environment. This step is that question.

class _CaptureAdapter implements HttpClientAdapter {
  Map<String, dynamic>? lastBody;

  @override
  Future<ResponseBody> fetch(RequestOptions options,
      Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async {
    if (options.path.contains('/me/api-keys')) {
      lastBody = (options.data as Map).cast<String, dynamic>();
      return ResponseBody.fromString('', 204);
    }
    return ResponseBody.fromString('{}', 404);
  }

  @override
  void close({bool force = false}) {}
}

class _FakeAuth extends AuthController {
  _FakeAuth(this._user);
  final UserModel? _user;
  int refreshes = 0;

  @override
  Future<UserModel?> build() async => _user;

  @override
  Future<void> refreshMe() async => refreshes++;
}

UserModel _user({
  bool contract = true,
  String gemini = '',
}) =>
    UserModel(
      id: 'u1',
      name: 'Tiago',
      email: 't@x.com',
      token: 'jwt',
      isContractPlan: contract,
      geminiKeyMasked: gemini,
    );

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.en));

  Widget harness(UserModel user, _CaptureAdapter adapter) {
    final dio = Dio(BaseOptions(baseUrl: 'https://api.test/v1'));
    dio.httpClientAdapter = adapter;
    return ProviderScope(
      overrides: [
        dioProvider.overrideWithValue(dio),
        authControllerProvider.overrideWith(() => _FakeAuth(user)),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const Scaffold(body: StepAIKey()),
      ),
    );
  }

  testWidgets('saving sends gemini_key — the name the API actually reads',
      (tester) async {
    final adapter = _CaptureAdapter();
    await tester.pumpWidget(harness(_user(), adapter));
    await tester.pumpAndSettle();

    await tester.enterText(
        find.byKey(const Key('onboarding_aikey_input')), 'AIza-secret');
    await tester.pump();
    await tester.tap(find.byKey(const Key('onboarding_aikey_save')));
    await tester.pumpAndSettle();

    // A wrong field name is accepted by the handler and saves NOTHING, which
    // is the failure this pins: the person reads success over a dead key.
    expect(adapter.lastBody, isNotNull);
    expect(adapter.lastBody!['gemini_key'], 'AIza-secret');
    expect(adapter.lastBody!.containsKey('openai_key'), isFalse,
        reason: 'an omitted provider keeps whatever it had');
  });

  testWidgets('the Save button stays dead until something is typed',
      (tester) async {
    await tester.pumpWidget(harness(_user(), _CaptureAdapter()));
    await tester.pumpAndSettle();

    final btn = tester
        .widget<FilledButton>(find.byKey(const Key('onboarding_aikey_save')));
    expect(btn.onPressed, isNull);
  });

  testWidgets('an account that already has a key sees it, not a blank form',
      (tester) async {
    await tester
        .pumpWidget(harness(_user(gemini: 'AIza****4f2a'), _CaptureAdapter()));
    await tester.pumpAndSettle();

    expect(
        find.byKey(const Key('onboarding_aikey_configured')), findsOneWidget);
    expect(find.byKey(const Key('onboarding_aikey_input')), findsNothing);
    expect(find.textContaining('AIza****4f2a'), findsOneWidget);
  });

  testWidgets('skipping is offered in words, because CSV needs no model',
      (tester) async {
    await tester.pumpWidget(harness(_user(), _CaptureAdapter()));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('onboarding_aikey_skip_note')), findsOneWidget);
  });

  group('step order', () {
    test('a contract account is asked for a key before the price list', () {
      final steps = OnboardingVariant.reduced.stepIndices;
      expect(steps.contains(aiKeyStepIndex), isTrue);
      expect(steps.indexOf(aiKeyStepIndex), lessThan(steps.indexOf(5)),
          reason: 'the key must come before the step that spends it');
    });

    test('a self-serve account is never asked — it runs on the platform key',
        () {
      final container = ProviderContainer(overrides: [
        authControllerProvider
            .overrideWith(() => _FakeAuth(_user(contract: false))),
      ]);
      addTearDown(container.dispose);
      // Resolve the auth future before reading the derived list.
      container.read(authControllerProvider);
      final steps = container.read(onboardingStepIndicesProvider);
      expect(steps.contains(aiKeyStepIndex), isFalse);
    });
  });
}
