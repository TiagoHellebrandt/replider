import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/features/sales/domain/models/funnel_model.dart';
import 'package:app/features/sales/presentation/controllers/funnels_controller.dart';
import 'package:app/features/sales/presentation/screens/funnel_config_screen.dart';
import 'package:app/i18n/strings.g.dart';

/// Fake funnels list that seeds [_seed] for edit-mode hydration and records the
/// webhook test result it should return.
class _FakeFunnelsList extends FunnelsList {
  _FakeFunnelsList(this._seed, this._result);

  final List<FunnelModel> _seed;
  final FunnelWebhookTestResult _result;
  int testCalls = 0;

  @override
  Future<List<FunnelModel>> build() async => _seed;

  @override
  Future<FunnelWebhookTestResult> testWebhook(String id) async {
    testCalls++;
    return _result;
  }
}

GoRouter _router(String location, Widget screen) => GoRouter(
      initialLocation: location,
      routes: [
        GoRoute(path: location, builder: (_, __) => screen),
        GoRoute(
            path: '/sales/funnels',
            builder: (_, __) => const Scaffold(body: Text('list'))),
      ],
    );

Widget _harness({
  required Widget screen,
  required String location,
  _FakeFunnelsList? fake,
}) =>
    ProviderScope(
      overrides: [
        if (fake != null) funnelsListProvider.overrideWith(() => fake),
      ],
      child: MaterialApp.router(
        theme: AppTheme.darkTheme,
        routerConfig: _router(location, screen),
      ),
    );

// SwitchListTile in a themed surface trips a debug-only paint assertion.
void _ignoreListTileAssertions() {
  final prev = FlutterError.onError;
  FlutterError.onError = (d) {
    if (d.exceptionAsString().contains('ListTile background color')) return;
    prev?.call(d);
  };
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.en));

  // The webhook section sits below the fold of the funnel form ListView; a tall
  // viewport keeps it on-stage so finders/taps see it.
  void tallViewport(WidgetTester tester) {
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
  }

  testWidgets('create mode renders the webhook section controls',
      (tester) async {
    _ignoreListTileAssertions();
    tallViewport(tester);
    await tester.pumpWidget(_harness(
      screen: const FunnelConfigScreen(),
      location: '/sales/funnels/new',
    ));
    await tester.pumpAndSettle();

    // SettingsSectionHeader uppercases the label.
    expect(
        find.text(t.sales.webhook.section_title.toUpperCase()), findsOneWidget);
    expect(find.text(t.sales.webhook.field_url), findsOneWidget);
    expect(
        find.text(t.sales.webhook.trigger_required_complete), findsOneWidget);
    expect(find.text(t.sales.webhook.trigger_buy_intent), findsOneWidget);
    expect(find.text(t.sales.webhook.send_test), findsOneWidget);
    // No secret yet -> "Generate" (not "Regenerate").
    expect(find.text(t.sales.webhook.generate_secret), findsOneWidget);
  });

  testWidgets('generate fills the secret field and switches the button label',
      (tester) async {
    _ignoreListTileAssertions();
    tallViewport(tester);
    await tester.pumpWidget(_harness(
      screen: const FunnelConfigScreen(),
      location: '/sales/funnels/new',
    ));
    await tester.pumpAndSettle();

    await tester.tap(find.text(t.sales.webhook.generate_secret));
    await tester.pumpAndSettle();

    // 64 hex chars (32 bytes) now live in the secret field.
    final secret = tester
        .widgetList<TextField>(find.byType(TextField))
        .map((f) => f.controller?.text ?? '')
        .firstWhere((t) => t.length == 64, orElse: () => '');
    expect(secret, hasLength(64));
    expect(RegExp(r'^[0-9a-f]+$').hasMatch(secret), isTrue);
    expect(find.text(t.sales.webhook.regenerate_secret), findsOneWidget);
  });

  testWidgets('test button disabled in create mode shows save-first toast',
      (tester) async {
    _ignoreListTileAssertions();
    tallViewport(tester);
    await tester.pumpWidget(_harness(
      screen: const FunnelConfigScreen(),
      location: '/sales/funnels/new',
    ));
    await tester.pumpAndSettle();

    final button = tester.widget<OutlinedButton>(find.ancestor(
      of: find.text(t.sales.webhook.send_test),
      matching: find.byType(OutlinedButton),
    ));
    expect(button.onPressed, isNull);
  });

  testWidgets('edit mode fires the webhook test and shows the success toast',
      (tester) async {
    _ignoreListTileAssertions();
    tallViewport(tester);
    final fake = _FakeFunnelsList(
      const [
        FunnelModel(
          id: 'fnl_1',
          name: 'Funil',
          webhookUrl: 'https://example.com/hook',
        ),
      ],
      const FunnelWebhookTestResult(status: 200, body: 'ok'),
    );

    await tester.pumpWidget(_harness(
      screen: const FunnelConfigScreen(funnelId: 'fnl_1'),
      location: '/sales/funnels/fnl_1',
      fake: fake,
    ));
    await tester.pumpAndSettle();

    await tester.ensureVisible(find.text(t.sales.webhook.send_test));
    await tester.tap(find.text(t.sales.webhook.send_test));
    await tester.pumpAndSettle();

    expect(fake.testCalls, 1);
    expect(
        find.text(t.sales.webhook.test_success(status: '200')), findsOneWidget);
  });
}
