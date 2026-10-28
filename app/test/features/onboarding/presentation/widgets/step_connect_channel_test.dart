import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/design_system/components/channel_tile.dart';
import 'package:app/features/onboarding/presentation/controllers/onboarding_controller.dart';
import 'package:app/features/onboarding/presentation/widgets/step_connect_channel.dart';
import 'package:app/features/settings/domain/models/channel_model.dart';
import 'package:app/i18n/strings.g.dart';

class _MockDio extends Mock implements Dio {}

void main() {
  setUpAll(() {
    FlutterSecureStorage.setMockInitialValues({});
    registerFallbackValue(RequestOptions(path: '/'));
  });

  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
  });

  Widget harness() {
    // The step's controller eagerly hydrates via GET /me/onboarding-progress
    // on build; stub the Dio call so no real request-timeout Timer leaks.
    final dio = _MockDio();
    when(() => dio.get<dynamic>(
          any(),
          queryParameters: any(named: 'queryParameters'),
          options: any(named: 'options'),
          cancelToken: any(named: 'cancelToken'),
          onReceiveProgress: any(named: 'onReceiveProgress'),
        )).thenThrow(DioException(requestOptions: RequestOptions(path: '/')));

    return ProviderScope(
      overrides: [dioProvider.overrideWithValue(dio)],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const Scaffold(body: StepConnectChannel()),
      ),
    );
  }

  Finder instagramTile() => find.byWidgetPredicate(
      (w) => w is ChannelTile && w.type == ChannelType.instagram);

  // The other channels are folded behind "Outros canais" (2026-09-01): the
  // page opens on the Gmail card alone. Tests that pick another channel
  // unfold them first, the way a person would.
  Future<void> openOtherChannels(WidgetTester tester) async {
    await tester.tap(find.byKey(const Key('onboarding_other_channels_toggle')));
    await tester.pump();
  }

  testWidgets('Instagram tile is enabled (no coming-soon badge)',
      (tester) async {
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(harness());
    await tester.pump();
    await openOtherChannels(tester);

    expect(instagramTile(), findsOneWidget);
    final tile = tester.widget<ChannelTile>(instagramTile());
    expect(tile.disabled, isFalse);
    expect(
      find.text(t.onboarding.step_channel_badge_coming_soon),
      findsNothing,
    );

    // Drain the debounced draft-save timer scheduled by hydration state.
    await tester.pump(const Duration(milliseconds: 900));
  });

  testWidgets(
      'selecting Instagram shows Page ID + Access Token and builds '
      '{page_id, access_token} config', (tester) async {
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(harness());
    await tester.pump();
    await openOtherChannels(tester);

    await tester.tap(instagramTile());
    await tester.pump();

    expect(find.text(t.onboarding.instagram_page_id), findsOneWidget);
    expect(find.text(t.onboarding.instagram_access_token), findsOneWidget);
    expect(find.text(t.onboarding.step_channel_instagram_hint), findsOneWidget);
    expect(find.text(t.settings.channel_instagram_help_link), findsOneWidget);

    await tester.enterText(find.byType(TextField).at(0), '17841400000000000');
    await tester.enterText(find.byType(TextField).at(1), 'IGAA-token-123');
    await tester.pump();

    final container =
        ProviderScope.containerOf(tester.element(instagramTile()));
    final state = container.read(onboardingControllerProvider);
    expect(state.channelType, 'instagram');
    expect(state.channelConfig, {
      'page_id': '17841400000000000',
      'access_token': 'IGAA-token-123',
    });

    // Drain the debounced draft-save timer before teardown.
    await tester.pump(const Duration(milliseconds: 900));
  });

  Finder smsTile() => find
      .byWidgetPredicate((w) => w is ChannelTile && w.type == ChannelType.sms);

  testWidgets(
      'selecting SMS shows the Twilio fields (Auth Token obscured) and '
      'builds {account_sid, auth_token, from_number} config', (tester) async {
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(harness());
    await tester.pump();
    await openOtherChannels(tester);

    expect(smsTile(), findsOneWidget);
    await tester.tap(smsTile());
    await tester.pump();

    expect(find.text(t.onboarding.sms_account_sid), findsOneWidget);
    expect(find.text(t.onboarding.sms_auth_token), findsOneWidget);
    expect(find.text(t.onboarding.sms_from_number), findsOneWidget);
    expect(find.text(t.onboarding.step_channel_sms_hint), findsOneWidget);
    expect(find.text(t.settings.channel_sms_help_link), findsOneWidget);

    // The Auth Token credential must never render in clear text.
    final obscured = tester
        .widgetList<TextField>(find.byType(TextField))
        .where((f) => f.obscureText);
    expect(obscured, hasLength(1));

    final sid = 'AC${'a' * 32}';
    await tester.enterText(find.byType(TextField).at(0), sid);
    await tester.enterText(find.byType(TextField).at(1), 'twilio-auth-token');
    await tester.enterText(find.byType(TextField).at(2), '+15551234567');
    await tester.pump();

    final container = ProviderScope.containerOf(tester.element(smsTile()));
    final state = container.read(onboardingControllerProvider);
    expect(state.channelType, 'sms');
    expect(state.channelConfig, {
      'account_sid': sid,
      'auth_token': 'twilio-auth-token',
      'from_number': '+15551234567',
    });

    // Drain the debounced draft-save timer before teardown.
    await tester.pump(const Duration(milliseconds: 900));
  });

  // Email is the door (2026-08-18): the tile is selected before the person
  // touches anything, the one-click Gmail card is the first thing on it,
  // and the SMTP fields — the graduation path — are folded until asked for.
  testWidgets('opens on email with the Gmail card first and SMTP folded',
      (tester) async {
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(harness());
    await tester.pump(); // build
    await tester.pump(); // the post-frame default lands

    final container = ProviderScope.containerOf(
        tester.element(find.byType(StepConnectChannel)));
    expect(container.read(onboardingControllerProvider).channelType, 'email');
    expect(find.byKey(const Key('gmail_connect_card')), findsOneWidget);
    // Only the door: no channel tiles until asked for.
    expect(find.byType(ChannelTile), findsNothing);
    expect(smsTile(), findsNothing);
    expect(find.text(t.onboarding.email_smtp_host), findsNothing,
        reason: 'SMTP is folded until asked for');

    await tester.tap(find.byKey(const Key('onboarding_email_smtp_toggle')));
    await tester.pump();
    expect(find.text(t.onboarding.email_smtp_host), findsOneWidget);
    // The Gmail card stays above it — the fold adds, it does not replace.
    expect(find.byKey(const Key('gmail_connect_card')), findsOneWidget);

    await tester.pump(const Duration(milliseconds: 900));
  });

  // A resume that already picked another channel is left alone: the default
  // only fills an empty choice.
  testWidgets('the email default does not override a channel already chosen',
      (tester) async {
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(harness());
    final container = ProviderScope.containerOf(
        tester.element(find.byType(StepConnectChannel)));
    // Chosen before the post-frame callback runs.
    container
        .read(onboardingControllerProvider.notifier)
        .updateField('channelType', 'sms');
    await tester.pump();
    await tester.pump();

    expect(container.read(onboardingControllerProvider).channelType, 'sms');

    await tester.pump(const Duration(milliseconds: 900));
  });

  // "Outros canais" unfolds the four other tiles (and the email tile, so the
  // person can come back); folding them again leaves the email door alone.
  testWidgets('the other channels sit behind an expander, closed by default',
      (tester) async {
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(harness());
    await tester.pump();
    await tester.pump();

    expect(find.text(t.onboarding.step_channel_other_channels), findsOneWidget);
    expect(find.byType(ChannelTile), findsNothing);
    // The SMTP path is a link under the Gmail card, and it says so.
    expect(find.text(t.onboarding.email_smtp_toggle), findsOneWidget);

    await openOtherChannels(tester);
    expect(find.byType(ChannelTile), findsNWidgets(5));
    expect(find.text(t.onboarding.step_channel_other_channels_hide),
        findsOneWidget);

    await tester.tap(find.byKey(const Key('onboarding_other_channels_toggle')));
    await tester.pump();
    expect(find.byType(ChannelTile), findsNothing);
    expect(find.byKey(const Key('gmail_connect_card')), findsOneWidget);

    await tester.pump(const Duration(milliseconds: 900));
  });

  // A resume that already picked WhatsApp must not hide the tile it picked.
  testWidgets('a resumed non-email choice opens the tiles by itself',
      (tester) async {
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(harness());
    final container = ProviderScope.containerOf(
        tester.element(find.byType(StepConnectChannel)));
    container
        .read(onboardingControllerProvider.notifier)
        .updateField('channelType', 'whatsapp');
    await tester.pump();
    await tester.pump();

    // No expander to tap: a non-email choice keeps its tiles on screen.
    expect(find.byKey(const Key('onboarding_other_channels_toggle')),
        findsNothing);
    expect(find.byType(ChannelTile), findsNWidgets(5));
    final wa = tester.widget<ChannelTile>(find.byWidgetPredicate(
        (w) => w is ChannelTile && w.type == ChannelType.whatsapp));
    expect(wa.isSelected, isTrue);

    await tester.pump(const Duration(milliseconds: 900));
  });
}
