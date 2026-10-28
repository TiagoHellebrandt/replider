import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:mocktail/mocktail.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/features/auth/data/datasources/auth_local_datasource.dart';
import 'package:app/features/auth/domain/repositories/auth_repository.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/onboarding/domain/onboarding_variant.dart';
import 'package:app/features/onboarding/presentation/controllers/onboarding_controller.dart';
import 'package:app/i18n/strings.g.dart';

class MockDio extends Mock implements Dio {}

class MockAuthRepository extends Mock implements AuthRepository {}

class MockAuthLocalDataSource extends Mock implements AuthLocalDataSource {}

// Shape of the current onboarding controller (company → price list →
// customers → channel → first offer):
//   step 0 → submitStepAccountName(): PATCH /accounts + brand kit
//   step 4 → submitChannelAndAdvance(next): POST /channels when there is
//            something to post, then goToStep(next)
//   step 7 → completeOnboarding(context)
//
// These tests guard the non-navigation invariants; completeOnboarding
// is covered by the onboarding_screen integration test because it
// needs BuildContext and GoRouter wiring.
void main() {
  late MockDio mockDio;
  late MockAuthRepository mockAuthRepository;
  late MockAuthLocalDataSource mockLocalDataSource;
  late ProviderContainer container;

  setUp(() {
    mockDio = MockDio();
    mockAuthRepository = MockAuthRepository();
    mockLocalDataSource = MockAuthLocalDataSource();
    registerFallbackValue(RequestOptions(path: '/'));
  });

  tearDown(() {
    container.dispose();
  });

  ProviderContainer createContainer({
    OnboardingVariant variant = OnboardingVariant.reduced,
  }) {
    when(() => mockAuthRepository.getCurrentUser())
        .thenAnswer((_) async => null);

    container = ProviderContainer(
      overrides: [
        dioProvider.overrideWithValue(mockDio),
        authRepositoryProvider.overrideWithValue(mockAuthRepository),
        authLocalDataSourceProvider.overrideWithValue(mockLocalDataSource),
        onboardingVariantProvider.overrideWithValue(variant),
      ],
    );
    return container;
  }

  void stubDioGet(MockDio dio, Response<dynamic> response) {
    when(() => dio.get<dynamic>(
          any(),
          queryParameters: any(named: 'queryParameters'),
          options: any(named: 'options'),
          cancelToken: any(named: 'cancelToken'),
          onReceiveProgress: any(named: 'onReceiveProgress'),
        )).thenAnswer((_) async => response);
  }

  void stubDioPost(MockDio dio, Response<dynamic> response) {
    when(() => dio.post<dynamic>(
          any(),
          data: any(named: 'data'),
          queryParameters: any(named: 'queryParameters'),
          options: any(named: 'options'),
          cancelToken: any(named: 'cancelToken'),
          onSendProgress: any(named: 'onSendProgress'),
          onReceiveProgress: any(named: 'onReceiveProgress'),
        )).thenAnswer((_) async => response);
  }

  void stubDioPut(MockDio dio, Response<dynamic> response) {
    when(() => dio.put<dynamic>(
          any(),
          data: any(named: 'data'),
          queryParameters: any(named: 'queryParameters'),
          options: any(named: 'options'),
          cancelToken: any(named: 'cancelToken'),
          onSendProgress: any(named: 'onSendProgress'),
          onReceiveProgress: any(named: 'onReceiveProgress'),
        )).thenAnswer((_) async => response);
  }

  void stubDioPatch(MockDio dio, Response<dynamic> response) {
    when(() => dio.patch<dynamic>(
          any(),
          data: any(named: 'data'),
          queryParameters: any(named: 'queryParameters'),
          options: any(named: 'options'),
          cancelToken: any(named: 'cancelToken'),
          onSendProgress: any(named: 'onSendProgress'),
          onReceiveProgress: any(named: 'onReceiveProgress'),
        )).thenAnswer((_) async => response);
  }

  Map<String, dynamic> capturedPatch(MockDio dio, String path) {
    final data = verify(() => dio.patch<dynamic>(
          path,
          data: captureAny(named: 'data'),
          queryParameters: any(named: 'queryParameters'),
          options: any(named: 'options'),
          cancelToken: any(named: 'cancelToken'),
          onSendProgress: any(named: 'onSendProgress'),
          onReceiveProgress: any(named: 'onReceiveProgress'),
        )).captured;
    return (data.last as Map).cast<String, dynamic>();
  }

  /// What the company does and where — the two step-1 fields added on
  /// 2026-08-18. Every step-1 submit test needs them filled to advance.
  void fillBusinessProfile(OnboardingController notifier) {
    notifier.updateField('businessLines', <String>{'service'});
    notifier.updateField('territoryUF', 'MT');
  }

  Map<String, dynamic> capturedPut(MockDio dio, String path) {
    final data = verify(() => dio.put<dynamic>(
          path,
          data: captureAny(named: 'data'),
          queryParameters: any(named: 'queryParameters'),
          options: any(named: 'options'),
          cancelToken: any(named: 'cancelToken'),
          onSendProgress: any(named: 'onSendProgress'),
          onReceiveProgress: any(named: 'onReceiveProgress'),
        )).captured;
    return (data.last as Map).cast<String, dynamic>();
  }

  void stubDioPostThrows(MockDio dio, Object error) {
    when(() => dio.post<dynamic>(
          any(),
          data: any(named: 'data'),
          queryParameters: any(named: 'queryParameters'),
          options: any(named: 'options'),
          cancelToken: any(named: 'cancelToken'),
          onSendProgress: any(named: 'onSendProgress'),
          onReceiveProgress: any(named: 'onReceiveProgress'),
        )).thenThrow(error);
  }

  group('OnboardingState', () {
    test('default state', () {
      createContainer();
      final state = container.read(onboardingControllerProvider);
      expect(state.currentStep, 0);
      expect(state.channelType, isNull);
      expect(state.channelConnected, isFalse);
      expect(state.isLoading, isFalse);
      expect(state.error, isNull);
    });

    test('copyWith preserves unchanged fields', () {
      const state = OnboardingState(
        currentStep: 1,
        channelName: 'Loja',
        channelConnected: true,
      );
      final updated = state.copyWith(currentStep: 2);
      expect(updated.currentStep, 2);
      expect(updated.channelName, 'Loja');
      expect(updated.channelConnected, isTrue);
    });
  });

  group('updateField', () {
    test('channelName / channelType', () {
      createContainer();
      final notifier = container.read(onboardingControllerProvider.notifier);

      notifier.updateField('channelName', 'Bot 1');
      notifier.updateField('channelType', 'telegram');

      final state = container.read(onboardingControllerProvider);
      expect(state.channelName, 'Bot 1');
      expect(state.channelType, 'telegram');
    });

    test('channelType change resets channelConnected', () {
      createContainer();
      final notifier = container.read(onboardingControllerProvider.notifier);

      notifier.markChannelConnected();
      expect(container.read(onboardingControllerProvider).channelConnected,
          isTrue);

      notifier.updateField('channelType', 'whatsapp');
      expect(container.read(onboardingControllerProvider).channelConnected,
          isFalse);
    });
  });

  // The company step is where the brand kit gets filled. Everything the
  // outbound composer knows about the business comes from here — with it
  // empty, the first AI-written approach is correct and generic, which
  // is the worst first impression for a product sold on writing good
  // ones.
  group('submitStepAccountName (company identity)', () {
    Response<dynamic> ok(String path, [dynamic body]) => Response<dynamic>(
          requestOptions: RequestOptions(path: path),
          statusCode: 200,
          data: body ?? <String, dynamic>{},
        );

    test('a blank description does not advance', () async {
      createContainer();
      final notifier = container.read(onboardingControllerProvider.notifier);
      notifier.updateField('accountName', 'Clínica Sorriso');

      await notifier.submitStepAccountName();

      expect(container.read(onboardingControllerProvider).currentStep, 0);
      verifyNever(() => mockDio.patch<dynamic>(any(),
          data: any(named: 'data'),
          queryParameters: any(named: 'queryParameters'),
          options: any(named: 'options'),
          cancelToken: any(named: 'cancelToken'),
          onSendProgress: any(named: 'onSendProgress'),
          onReceiveProgress: any(named: 'onReceiveProgress')));
    });

    test('the identity reaches the brand kit, not just accounts.name',
        () async {
      stubDioGet(mockDio, ok('/brand-kit'));
      stubDioPatch(mockDio, ok('/accounts'));
      stubDioPut(mockDio, ok('/brand-kit'));

      createContainer();
      final notifier = container.read(onboardingControllerProvider.notifier);
      notifier.updateField('accountName', 'Clínica Sorriso');
      notifier.updateField('about', 'Odontologia para famílias.');
      notifier.updateField('toneOfVoice', 'Escreva de forma direta.');
      fillBusinessProfile(notifier);

      await notifier.submitStepAccountName();

      final body = capturedPut(mockDio, '/brand-kit');
      expect(body['company_name'], 'Clínica Sorriso');
      expect(body['about'], 'Odontologia para famílias.');
      expect(body['tone_of_voice'], 'Escreva de forma direta.');
      // Next is the price list (canonical 5) — the flow, not "step + 1".
      expect(container.read(onboardingControllerProvider).currentStep, 5);
    });

    // PUT /brand-kit is a full replace. An account redoing onboarding
    // must not lose the claims it configured in Settings.
    test('claims already configured survive the write', () async {
      stubDioGet(
        mockDio,
        ok('/brand-kit', <String, dynamic>{
          'company_name': 'Antigo',
          'about': 'antigo',
          'allowed_claims': <String>['ISO 9001'],
          'forbidden_claims': <String>['cura garantida'],
          'logo_url': 'https://cdn/logo.png',
        }),
      );
      stubDioPatch(mockDio, ok('/accounts'));
      stubDioPut(mockDio, ok('/brand-kit'));

      createContainer();
      final notifier = container.read(onboardingControllerProvider.notifier);
      notifier.updateField('accountName', 'Novo Nome');
      notifier.updateField('about', 'Nova descrição.');
      fillBusinessProfile(notifier);

      await notifier.submitStepAccountName();

      final body = capturedPut(mockDio, '/brand-kit');
      expect(body['forbidden_claims'], <String>['cura garantida']);
      expect(body['allowed_claims'], <String>['ISO 9001']);
      expect(body['logo_url'], 'https://cdn/logo.png');
      expect(body['company_name'], 'Novo Nome');
    });

    // Writing a partial kit over one we failed to read would drop exactly
    // the claims the read was there to preserve. Better to stop: both
    // calls are idempotent, so Next retries.
    test('an unreadable kit is never overwritten', () async {
      when(() => mockDio.get<dynamic>(
                any(),
                queryParameters: any(named: 'queryParameters'),
                options: any(named: 'options'),
                cancelToken: any(named: 'cancelToken'),
                onReceiveProgress: any(named: 'onReceiveProgress'),
              ))
          .thenThrow(
              DioException(requestOptions: RequestOptions(path: '/brand-kit')));
      stubDioPatch(mockDio, ok('/accounts'));
      stubDioPut(mockDio, ok('/brand-kit'));

      createContainer();
      final notifier = container.read(onboardingControllerProvider.notifier);
      notifier.updateField('accountName', 'Clínica Sorriso');
      notifier.updateField('about', 'Odontologia para famílias.');
      fillBusinessProfile(notifier);

      await notifier.submitStepAccountName();

      final state = container.read(onboardingControllerProvider);
      expect(state.currentStep, 0);
      expect(state.error, isNotNull);
      verifyNever(() => mockDio.put<dynamic>(
            '/brand-kit',
            data: any(named: 'data'),
            queryParameters: any(named: 'queryParameters'),
            options: any(named: 'options'),
            cancelToken: any(named: 'cancelToken'),
            onSendProgress: any(named: 'onSendProgress'),
            onReceiveProgress: any(named: 'onReceiveProgress'),
          ));
    });

    // The wizard can be closed mid-sentence; the draft is what brings it
    // back.
    test('the description rides the resume draft', () async {
      stubDioPut(mockDio, ok('/me/onboarding-draft'));
      stubDioGet(
        mockDio,
        ok('/me/onboarding-progress', <String, dynamic>{
          'current_step': 0,
          'account_name': 'Clínica Sorriso',
          'draft': <String, dynamic>{
            'about': 'Odontologia para famílias.',
            'tone_of_voice': 'Escreva de forma direta.',
          },
        }),
      );

      createContainer();
      // autoDispose: a bare read is collected before the async hydrate
      // lands, so the listener is what keeps the provider alive.
      container.listen<OnboardingState>(
          onboardingControllerProvider, (_, __) {},
          fireImmediately: true);
      await Future<void>.delayed(const Duration(milliseconds: 20));

      final state = container.read(onboardingControllerProvider);
      expect(state.accountName, 'Clínica Sorriso');
      expect(state.about, 'Odontologia para famílias.');
      expect(state.toneOfVoice, 'Escreva de forma direta.');
    });
  });

  group('the shipped flow (US-114)', () {
    // company → price list → customers → channel → first offer. "Create an
    // agent" used to sit at step two and was the wrong first thing to ask
    // for: it told every new account the product is a chatbot, and drafting
    // an approach needs no agent. What the product needs first is what she
    // sells and a way to send. The full flow that rendered the agent step
    // was deleted on 2026-09-01; the indices it used stay as holes.
    test('drops the agent, ends on the first offer', () {
      // Index 1 is the AI key now, not the deleted "create agent" step: a
      // contract account runs on its own model, and the price list right
      // after it reads brochures with that key. The screen drops the step
      // for a self-serve account (onboardingStepIndicesProvider).
      expect(OnboardingVariant.reduced.stepIndices, [0, 1, 5, 6, 4, 7]);
      expect(OnboardingVariant.reduced.stepIndices.last, 7,
          reason: 'the first offer is the terminal step and the point');
      expect(OnboardingVariant.reduced.stepIndices, isNot(contains(2)),
          reason: 'the agent/knowledge steps are the chatbot product');
      expect(OnboardingVariant.reduced.stepIndices.indexOf(1),
          lessThan(OnboardingVariant.reduced.stepIndices.indexOf(5)),
          reason: 'the key is asked before the step that spends it');
      expect(OnboardingVariant.values, [OnboardingVariant.reduced],
          reason: 'the full flow is gone, not flag-gated');
    });
  });

  // US-012: the SMS config is shape-checked client-side before POST
  // /channels — previously only .trim() ran and garbage went to the API.
  group('submitChannelAndAdvance sms validation (US-012)', () {
    void seedSmsConfig(Map<String, String> config) {
      createContainer();
      final notifier = container.read(onboardingControllerProvider.notifier);
      notifier.updateField('channelType', 'sms');
      notifier.updateField('channelConfig', config);
    }

    Future<void> submitAndExpectError(String expectedError) async {
      final notifier = container.read(onboardingControllerProvider.notifier);
      notifier.goToStep(4);
      await notifier.submitChannelAndAdvance(7);

      final state = container.read(onboardingControllerProvider);
      expect(state.currentStep, 4, reason: 'a rejected config stays put');
      expect(state.error, expectedError);
      expect(state.isLoading, isFalse);
      verifyNever(() => mockDio.post<dynamic>(any(),
          data: any(named: 'data'),
          queryParameters: any(named: 'queryParameters'),
          options: any(named: 'options'),
          cancelToken: any(named: 'cancelToken'),
          onSendProgress: any(named: 'onSendProgress'),
          onReceiveProgress: any(named: 'onReceiveProgress')));
    }

    test('rejects an API-Key style SID before hitting the API', () async {
      seedSmsConfig({
        'account_sid': 'SK0123456789abcdef0123456789abcdef',
        'auth_token': 'tok',
        'from_number': '+15551234567',
      });
      await submitAndExpectError(t.onboarding.invalid_sms_account_sid);
    });

    test('rejects an empty auth_token', () async {
      seedSmsConfig({
        'account_sid': 'AC0123456789abcdef0123456789abcdef',
        'auth_token': '  ',
        'from_number': '+15551234567',
      });
      await submitAndExpectError(t.onboarding.invalid_sms_auth_token);
    });

    test('rejects a national-format from_number', () async {
      seedSmsConfig({
        'account_sid': 'AC0123456789abcdef0123456789abcdef',
        'auth_token': 'tok',
        'from_number': '65999999999',
      });
      await submitAndExpectError(t.onboarding.invalid_sms_from_number);
    });
  });

  // US-012: the draft auto-save must never ship channel credentials —
  // they would be persisted server-side in plaintext.
  group('draft save strips credentials (US-012)', () {
    test('auth_token is filtered out of the PUT body', () async {
      when(() => mockDio.put<dynamic>(
            any(),
            data: any(named: 'data'),
            queryParameters: any(named: 'queryParameters'),
            options: any(named: 'options'),
            cancelToken: any(named: 'cancelToken'),
            onSendProgress: any(named: 'onSendProgress'),
            onReceiveProgress: any(named: 'onReceiveProgress'),
          )).thenAnswer((_) async => Response<dynamic>(
            requestOptions: RequestOptions(path: '/me/onboarding-draft'),
            statusCode: 200,
          ));

      createContainer();
      // Let _hydrate finish so _saveDraft isn't suppressed.
      await Future<void>.delayed(Duration.zero);

      final notifier = container.read(onboardingControllerProvider.notifier);
      notifier.updateField('channelType', 'sms');
      notifier.updateField('channelConfig', {
        'account_sid': 'AC0123456789abcdef0123456789abcdef',
        'auth_token': 'supersecret',
        'from_number': '+15551234567',
      });
      notifier.goToStep(1); // flushes the draft save immediately
      await Future<void>.delayed(Duration.zero);

      final captured = verify(() => mockDio.put<dynamic>(
            '/me/onboarding-draft',
            data: captureAny(named: 'data'),
            queryParameters: any(named: 'queryParameters'),
            options: any(named: 'options'),
            cancelToken: any(named: 'cancelToken'),
            onSendProgress: any(named: 'onSendProgress'),
            onReceiveProgress: any(named: 'onReceiveProgress'),
          )).captured;

      final body = captured.last as Map<String, dynamic>;
      final draft = body['draft'] as Map<String, dynamic>;
      final config = (draft['channel_config'] as Map).cast<String, String>();
      expect(config.containsKey('auth_token'), isFalse,
          reason: 'credentials must not ride the draft');
      expect(config['account_sid'], 'AC0123456789abcdef0123456789abcdef');
      expect(config['from_number'], '+15551234567');
    });
  });

  group('submitStepAccountName (business profile)', () {
    Response<dynamic> ok(String path, [dynamic body]) => Response<dynamic>(
          requestOptions: RequestOptions(path: path),
          statusCode: 200,
          data: body ?? <String, dynamic>{},
        );

    void fillIdentity(OnboardingController notifier) {
      notifier.updateField('accountName', 'Cuiabamed');
      notifier.updateField('about', 'Equipamentos hospitalares em MT.');
    }

    // The two facts every search and every offer key on: without them the
    // step does not advance, exactly like a blank name.
    test('no business line does not advance', () async {
      createContainer();
      final notifier = container.read(onboardingControllerProvider.notifier);
      fillIdentity(notifier);
      notifier.updateField('territoryUF', 'MT');

      expect(notifier.stepCompanyComplete, isFalse);
      await notifier.submitStepAccountName();
      expect(container.read(onboardingControllerProvider).currentStep, 0);
    });

    test('no state does not advance', () async {
      createContainer();
      final notifier = container.read(onboardingControllerProvider.notifier);
      fillIdentity(notifier);
      notifier.updateField('businessLines', <String>{'sell'});

      expect(notifier.stepCompanyComplete, isFalse);
      await notifier.submitStepAccountName();
      expect(container.read(onboardingControllerProvider).currentStep, 0);
    });

    test('the lines and the territory ride the same PATCH as the name',
        () async {
      stubDioGet(mockDio, ok('/brand-kit'));
      stubDioPatch(mockDio, ok('/accounts'));
      stubDioPut(mockDio, ok('/brand-kit'));

      createContainer();
      final notifier = container.read(onboardingControllerProvider.notifier);
      fillIdentity(notifier);
      notifier.updateField('businessLines', <String>{'service', 'rent'});
      notifier.updateField('territoryUF', 'mt');
      notifier.updateField(
          'territoryCities', ' Cuiabá,Várzea Grande ; ; Rondonópolis\n');

      await notifier.submitStepAccountName();

      final body = capturedPatch(mockDio, '/accounts');
      expect(body['name'], 'Cuiabamed');
      expect((body['business_lines'] as List).toSet(), {'service', 'rent'});
      final territory = (body['territory'] as Map).cast<String, dynamic>();
      expect(territory['uf'], 'MT');
      // Split on commas, semicolons and newlines; blanks dropped; the
      // server dedups case-insensitively, the client only trims.
      expect(territory['cities'], ['Cuiabá', 'Várzea Grande', 'Rondonópolis']);
      // Next is the price list (canonical 5) — the flow, not "step + 1".
      expect(container.read(onboardingControllerProvider).currentStep, 5);
    });

    // A resume reads the profile back from the account, not the draft: it
    // is real data from the moment it was saved.
    test('a resume hydrates the profile from /me', () async {
      when(() => mockDio.get<dynamic>(
            '/me',
            queryParameters: any(named: 'queryParameters'),
            options: any(named: 'options'),
            cancelToken: any(named: 'cancelToken'),
            onReceiveProgress: any(named: 'onReceiveProgress'),
          )).thenAnswer((_) async => ok('/me', <String, dynamic>{
            'account': <String, dynamic>{
              'business_lines': <String>['sell', 'service'],
              'territory': <String, dynamic>{
                'uf': 'MT',
                'cities': <String>['Cuiabá', 'Várzea Grande'],
              },
            },
          }));

      createContainer();
      // The provider is built on first read; hydration starts then. Keep a
      // listener so autoDispose does not tear it down between the reads.
      final sub = container.listen(onboardingControllerProvider, (_, __) {});
      addTearDown(sub.close);
      await Future<void>.delayed(const Duration(milliseconds: 50));

      final state = container.read(onboardingControllerProvider);
      expect(state.businessLines, {'sell', 'service'});
      expect(state.territoryUF, 'MT');
      expect(state.territoryCities, 'Cuiabá, Várzea Grande');
    });
  });

  // The channel step in the shipped flow is no longer terminal: it creates
  // the channel when there is something to create and moves on to the first
  // offer. The email tile without SMTP details means "Gmail (via the OAuth
  // callback) or later" — nothing to post, and posting an empty config would
  // be refused for a reason the person cannot act on.
  group('submitChannelAndAdvance (shipped flow)', () {
    Response<dynamic> ok(String path) => Response<dynamic>(
          requestOptions: RequestOptions(path: path),
          statusCode: 201,
          data: <String, dynamic>{'id': 'ch1'},
        );

    test('email with no SMTP typed posts nothing and advances', () async {
      createContainer(variant: OnboardingVariant.reduced);
      final notifier = container.read(onboardingControllerProvider.notifier);
      notifier.goToStep(4);
      notifier.updateField('channelType', 'email');

      await notifier.submitChannelAndAdvance(7);

      expect(container.read(onboardingControllerProvider).currentStep, 7);
      verifyNever(() => mockDio.post<dynamic>(any(),
          data: any(named: 'data'),
          queryParameters: any(named: 'queryParameters'),
          options: any(named: 'options'),
          cancelToken: any(named: 'cancelToken'),
          onSendProgress: any(named: 'onSendProgress'),
          onReceiveProgress: any(named: 'onReceiveProgress')));
    });

    test('email with SMTP typed posts the channel, then advances', () async {
      stubDioPost(mockDio, ok('/channels'));
      createContainer(variant: OnboardingVariant.reduced);
      final notifier = container.read(onboardingControllerProvider.notifier);
      notifier.goToStep(4);
      notifier.updateField('channelType', 'email');
      notifier.updateField('channelConfig', <String, String>{
        'from_address': 'vendas@cuiabamed.com.br',
        'smtp_host': 'smtp.cuiabamed.com.br',
        'smtp_port': '587',
        'smtp_password': 'x',
      });

      await notifier.submitChannelAndAdvance(7);

      expect(container.read(onboardingControllerProvider).currentStep, 7);
      final body = verify(() => mockDio.post<dynamic>('/channels',
          data: captureAny(named: 'data'),
          queryParameters: any(named: 'queryParameters'),
          options: any(named: 'options'),
          cancelToken: any(named: 'cancelToken'),
          onSendProgress: any(named: 'onSendProgress'),
          onReceiveProgress: any(named: 'onReceiveProgress'))).captured;
      expect((body.single as Map)['type'], 'email');
    });

    test('a failed post keeps the person on the step with the error', () async {
      stubDioPostThrows(mockDio,
          DioException(requestOptions: RequestOptions(path: '/channels')));
      createContainer(variant: OnboardingVariant.reduced);
      final notifier = container.read(onboardingControllerProvider.notifier);
      notifier.goToStep(4);
      notifier.updateField('channelType', 'telegram');
      notifier.updateField('channelConfig', <String, String>{'bot_token': 't'});

      await notifier.submitChannelAndAdvance(7);

      final state = container.read(onboardingControllerProvider);
      expect(state.currentStep, 4);
      expect(state.error, isNotNull);
    });
  });
}
