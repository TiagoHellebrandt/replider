import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/sales/domain/models/board_model.dart';
import 'package:app/features/sales/domain/models/funnel_model.dart';
import 'package:app/features/sales/presentation/controllers/board_controller.dart';
import 'package:app/features/sales/presentation/controllers/funnels_controller.dart';
import 'package:app/features/sales/presentation/screens/funnel_branch_screen.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

class _OneFunnel extends FunnelsList {
  @override
  Future<List<FunnelModel>> build() async =>
      [const FunnelModel(id: 'f1', name: 'Funil')];
}

Board _board() => const Board(
      stages: [
        BoardStage(
            id: 's1',
            name: 'Abordados',
            position: 100,
            mapsToStatus: 'contacted',
            isTerminal: false),
      ],
      cards: [
        BoardCard(
            leadId: 'lead1',
            stageId: 's1',
            status: 'contacted',
            score: 3,
            companyName: 'Clínica'),
      ],
    );

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  // US-M024. The inbox chip's link must work even though the branch
  // widget is built WITHOUT the constructor param — GoRouter keys a shell
  // branch's page by path, so a query-only change can reuse the existing
  // widget and never re-run the builder that read `?lead=`. Caught in the
  // browser: the chip navigated, the URL changed, and nothing opened.
  testWidgets('opens the deep-linked lead from the live route, not the ctor',
      (tester) async {
    tester.view.physicalSize = const Size(1400, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    final router = GoRouter(
      initialLocation: '/funnel?lead=lead1',
      routes: [
        GoRoute(
          path: '/funnel',
          // Deliberately NOT forwarding the query param: the screen has
          // to read the live route itself.
          builder: (context, state) => const FunnelBranchScreen(),
        ),
      ],
    );

    await tester.pumpWidget(ProviderScope(
      overrides: [
        funnelsListProvider.overrideWith(_OneFunnel.new),
        boardProvider('f1').overrideWith((ref) async => _board()),
        dioProvider.overrideWithValue(Dio()),
      ],
      child:
          MaterialApp.router(theme: AppTheme.darkTheme, routerConfig: router),
    ));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('lead_detail_title')), findsOneWidget);
  });

  // Com um funil só, o título da barra fica vazio — e uma barra em branco
  // somada à faixa de 44px do seletor empurrava o quadro para um terço da
  // tela. Num Kanban a altura é o recurso escasso.
  testWidgets('o seletor divide a linha do título, não uma faixa própria',
      (tester) async {
    tester.view.physicalSize = const Size(1400, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    final router = GoRouter(
      initialLocation: '/funnel',
      routes: [
        GoRoute(
          path: '/funnel',
          builder: (context, state) => const FunnelBranchScreen(),
        ),
      ],
    );

    await tester.pumpWidget(ProviderScope(
      overrides: [
        funnelsListProvider.overrideWith(_OneFunnel.new),
        boardProvider('f1').overrideWith((ref) async => _board()),
        dioProvider.overrideWithValue(Dio()),
      ],
      child:
          MaterialApp.router(theme: AppTheme.darkTheme, routerConfig: router),
    ));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('funnel_view_toggle')), findsOneWidget);
    // Redesign: no AppBar at all — the switcher shares one custom header
    // row with the title, Ofertas-style.
    expect(find.byType(AppBar), findsNothing,
        reason: 'nada de faixa extra: header é uma linha só');
  });

  // The board's "add a lead by hand" action lives on the Kanban's own
  // Scaffold, and the branch nests that Scaffold inside its own. Verified
  // in the browser first: the FAB was nowhere on the real screen even
  // though it renders fine when the Kanban is pumped on its own.
  testWidgets('the new-lead action survives the branch nesting',
      (tester) async {
    tester.view.physicalSize = const Size(1400, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    final router = GoRouter(
      initialLocation: '/funnel',
      routes: [
        GoRoute(
          path: '/funnel',
          builder: (context, state) => const FunnelBranchScreen(),
        ),
      ],
    );

    await tester.pumpWidget(ProviderScope(
      overrides: [
        funnelsListProvider.overrideWith(_OneFunnel.new),
        boardProvider('f1').overrideWith((ref) async => _board()),
        dioProvider.overrideWithValue(Dio()),
      ],
      child:
          MaterialApp.router(theme: AppTheme.darkTheme, routerConfig: router),
    ));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('board_new_lead')), findsOneWidget);
  });

  // The view toggle NAVIGATES instead of holding local state. The first
  // version kept a `_view` field that, once touched, shadowed `?view=`
  // forever — so /sales/portfolio and the checklist's deep links landed on
  // whatever view was toggled last.
  testWidgets('the view toggle navigates, so ?view= deep links always win',
      (tester) async {
    tester.view.physicalSize = const Size(1400, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    final router = GoRouter(
      initialLocation: '/funnel',
      routes: [
        GoRoute(
          path: '/funnel',
          builder: (context, state) => const FunnelBranchScreen(),
        ),
      ],
    );

    await tester.pumpWidget(ProviderScope(
      overrides: [
        funnelsListProvider.overrideWith(_OneFunnel.new),
        boardProvider('f1').overrideWith((ref) async => _board()),
        dioProvider.overrideWithValue(Dio()),
      ],
      child:
          MaterialApp.router(theme: AppTheme.darkTheme, routerConfig: router),
    ));
    await tester.pumpAndSettle();

    // Toggle by hand to the portfolio…
    await tester.tap(find.text(t.portfolio.title));
    await tester.pumpAndSettle();
    expect(router.state.uri.queryParameters['view'], 'portfolio');

    // …then a deep link back to the pipeline must actually show the board.
    router.go('/funnel?view=pipeline');
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('board_new_lead')), findsOneWidget,
        reason: 'the manual toggle must not shadow the URL');
  });
}
