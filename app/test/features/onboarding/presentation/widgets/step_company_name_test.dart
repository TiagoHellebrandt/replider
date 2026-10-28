import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/onboarding/presentation/controllers/onboarding_controller.dart';
import 'package:app/features/onboarding/presentation/widgets/step_company_name.dart';
import 'package:app/i18n/strings.g.dart';

// The first step is where the brand kit gets filled. Everything the
// outbound composer knows about the business comes from here — with it
// empty the first AI-written approach has the company's NAME and nothing
// else, so it writes around the gap and reads like a template.
void main() {
  setUpAll(() {
    FlutterSecureStorage.setMockInitialValues({});
  });

  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
  });

  late TextEditingController name;
  late TextEditingController about;

  Future<ProviderContainer> pump(WidgetTester tester) async {
    // The step grew two more questions (what the company does, where it
    // sells) and the tone chips now sit below the default 600px surface.
    tester.view.physicalSize = const Size(1000, 1800);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    name = TextEditingController();
    about = TextEditingController();
    addTearDown(name.dispose);
    addTearDown(about.dispose);

    // The controller hydrates over the network on build; a fast-failing
    // adapter keeps that off the wire and leaves no request timer behind.
    final dio = Dio()..httpClientAdapter = _DeadAdapter();
    final container = ProviderContainer(
      overrides: [dioProvider.overrideWithValue(dio)],
    );
    addTearDown(container.dispose);

    await tester.pumpWidget(UncontrolledProviderScope(
      container: container,
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: Scaffold(
          body: StepCompanyName(
            companyNameController: name,
            aboutController: about,
          ),
        ),
      ),
    ));
    await tester.pump();
    return container;
  }

  /// Drains the controller's 800 ms draft-save debounce so the test does
  /// not end with a pending timer.
  Future<void> settleDebounce(WidgetTester tester) =>
      tester.pump(const Duration(seconds: 1));

  testWidgets('asks what the business does, not just its name', (tester) async {
    await pump(tester);

    expect(find.byKey(const Key('onboarding_about_field')), findsOneWidget);
    expect(find.text(t.onboarding.company_about_help), findsOneWidget);

    await settleDebounce(tester);
  });

  testWidgets('the description reaches the controller', (tester) async {
    final container = await pump(tester);

    await tester.enterText(find.byKey(const Key('onboarding_about_field')),
        'Software para clínicas.');
    await tester.pump();

    expect(container.read(onboardingControllerProvider).about,
        'Software para clínicas.');

    await settleDebounce(tester);
  });

  // The stored value goes verbatim into the prompt, so it has to be
  // prose — the chip is a shortcut for writing it, not a code the
  // backend has to understand.
  testWidgets('a tone chip stores the sentence, not the label', (tester) async {
    final container = await pump(tester);

    await tester.tap(find.byKey(const Key('onboarding_tone_direct')));
    await tester.pump();

    final tone = container.read(onboardingControllerProvider).toneOfVoice;
    expect(tone, t.onboarding.company_tone_direct_prose);
    expect(tone, isNot(t.onboarding.company_tone_direct));

    await settleDebounce(tester);
  });

  testWidgets('tapping the selected tone again clears it', (tester) async {
    final container = await pump(tester);

    await tester.tap(find.byKey(const Key('onboarding_tone_warm')));
    await tester.pump();
    await tester.tap(find.byKey(const Key('onboarding_tone_warm')));
    await tester.pump();

    expect(container.read(onboardingControllerProvider).toneOfVoice, '');

    await settleDebounce(tester);
  });

  testWidgets('asks how the company earns and where it sells', (tester) async {
    await pump(tester);

    expect(find.text('${t.onboarding.company_lines_label} *'), findsOneWidget);
    for (final key in ['sell', 'rent', 'service']) {
      expect(find.byKey(Key('onboarding_line_$key')), findsOneWidget);
    }
    expect(
        find.text('${t.onboarding.company_territory_label} *'), findsOneWidget);
    expect(find.byKey(const Key('onboarding_territory_uf')), findsOneWidget);
    expect(
        find.byKey(const Key('onboarding_territory_cities')), findsOneWidget);

    await settleDebounce(tester);
  });

// A set, not a choice: a distributor sells AND services AND rents.
  testWidgets('the business lines accumulate and toggle off', (tester) async {
    final container = await pump(tester);

    await tester.tap(find.byKey(const Key('onboarding_line_sell')));
    await tester.pump();
    await tester.tap(find.byKey(const Key('onboarding_line_service')));
    await tester.pump();
    expect(container.read(onboardingControllerProvider).businessLines,
        {'sell', 'service'});

    await tester.tap(find.byKey(const Key('onboarding_line_sell')));
    await tester.pump();
    expect(container.read(onboardingControllerProvider).businessLines,
        {'service'});

    await tester.pump(const Duration(seconds: 1));
  });

  testWidgets('the state and the cities reach the controller', (tester) async {
    final container = await pump(tester);

    await tester.tap(find.byKey(const Key('onboarding_territory_uf')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('MT').last);
    await tester.pumpAndSettle();
    await tester.enterText(
        find.byKey(const Key('onboarding_territory_cities')), 'Cuiabá, VG');
    await tester.pump();

    final state = container.read(onboardingControllerProvider);
    expect(state.territoryUF, 'MT');
    expect(state.territoryCities, 'Cuiabá, VG');

    await tester.pump(const Duration(seconds: 1));
  });
}

/// Fails every request immediately — no sockets, no pending timers.
class _DeadAdapter implements HttpClientAdapter {
  @override
  void close({bool force = false}) {}

  @override
  Future<ResponseBody> fetch(RequestOptions options,
          Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async =>
      throw DioException(requestOptions: options);
}
