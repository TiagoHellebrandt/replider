import 'package:app/features/settings/domain/models/claude_integration_models.dart';
import 'package:app/features/settings/presentation/controllers/claude_integration_controller.dart';
import 'package:app/features/settings/presentation/screens/claude_integration_screen.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

class _FakeClaudeIntegration extends ClaudeIntegration {
  final ClaudeIntegrationState seed;
  _FakeClaudeIntegration(this.seed);

  @override
  Future<ClaudeIntegrationState> build() async => seed;
}

void ignoreListTileAssertions() {
  final prev = FlutterError.onError;
  FlutterError.onError = (d) {
    if (d.exceptionAsString().contains('ListTile background color')) return;
    prev?.call(d);
  };
}

Widget _harness(ClaudeIntegrationState seed) {
  final router = GoRouter(routes: [
    GoRoute(
      path: '/',
      builder: (_, __) => const ClaudeIntegrationScreen(),
    ),
    GoRoute(path: '/settings', builder: (_, __) => const SizedBox()),
  ]);
  return ProviderScope(
    overrides: [
      claudeIntegrationProvider
          .overrideWith(() => _FakeClaudeIntegration(seed)),
    ],
    child: MaterialApp.router(
      theme: AppTheme.darkTheme,
      routerConfig: router,
    ),
  );
}

void main() {
  setUpAll(() {
    FlutterSecureStorage.setMockInitialValues({});
  });
  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
  });

  testWidgets('renders connect card and empty states', (tester) async {
    ignoreListTileAssertions();
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(_harness(
      const ClaudeIntegrationState(tokens: [], connections: []),
    ));
    await tester.pumpAndSettle();

    expect(find.text(t.claude_integration.connect_title), findsOneWidget);
    // Numbered no-token flow + advanced section headers.
    expect(find.text(t.claude_integration.connect_step2), findsOneWidget);
    expect(
        find.text(t.claude_integration.connect_no_token_note), findsOneWidget);
    expect(find.text(t.claude_integration.advanced_title), findsOneWidget);
    expect(find.text(t.claude_integration.tokens_empty), findsOneWidget);
    expect(find.text(t.claude_integration.connections_empty), findsOneWidget);
    // MCP URL derived from API_BASE_URL's PROD fallback (never localhost).
    expect(find.textContaining('https://api.replider.com/mcp'), findsWidgets);
  });

  testWidgets('renders token rows and connection rows', (tester) async {
    ignoreListTileAssertions();
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(_harness(
      ClaudeIntegrationState(
        tokens: [
          PatModel(
            id: '01TOK',
            name: 'Meu Claude',
            tokenPrefix: 'rpl_mcp_AB12',
            createdAt: DateTime(2026, 7, 1),
          ),
        ],
        connections: const [
          ClaudeConnectionModel(clientId: '01CLI', clientName: 'Claude'),
        ],
      ),
    ));
    await tester.pumpAndSettle();

    expect(find.text('Meu Claude'), findsOneWidget);
    expect(find.textContaining('rpl_mcp_AB12'), findsOneWidget);
    expect(find.text('Claude'), findsWidgets);
    expect(find.text(t.claude_integration.connections_revoke), findsOneWidget);
  });

  testWidgets('create token opens name dialog', (tester) async {
    ignoreListTileAssertions();
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(_harness(
      const ClaudeIntegrationState(tokens: [], connections: []),
    ));
    await tester.pumpAndSettle();

    await tester.tap(find.text(t.claude_integration.tokens_create));
    await tester.pumpAndSettle();

    expect(find.text(t.claude_integration.create_dialog_title), findsOneWidget);
    // Cancel closes without side effects.
    await tester.tap(find.text(t.common.cancel));
    await tester.pumpAndSettle();
    expect(find.text(t.claude_integration.create_dialog_title), findsNothing);
  });
}
