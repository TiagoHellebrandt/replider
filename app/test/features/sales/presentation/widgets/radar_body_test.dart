import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/sales/domain/models/product_model.dart';
import 'package:app/features/sales/presentation/controllers/products_controller.dart';
import 'package:app/features/sales/presentation/controllers/lead_detail_controller.dart'
    show ParkMatch;
import 'package:app/features/sales/presentation/controllers/radar_controller.dart';
import 'package:app/features/sales/presentation/widgets/radar_detail_panel.dart';
import 'package:app/features/sales/presentation/controllers/signals_controller.dart';
import 'package:app/features/sales/presentation/widgets/radar_body.dart';
import 'package:app/i18n/strings.g.dart';

// The Radar (Ofertas phase 2): one feed over tenders, CNES park, reorders
// and dates — each row ending in a next step that lands in the queue.

class _FakeRadarActions extends RadarActions {
  _FakeRadarActions(super.ref);
  final offered = <String>[];
  String? hunted;

  @override
  Future<void> parkOffer(String cnes) async {
    offered.add(cnes);
  }

  final workedSignals = <String>[];

  @override
  Future<void> workSignal(String signalId) async {
    workedSignals.add(signalId);
  }

  @override
  Future<TerritoryHuntResult> hunt(String equipment) async {
    hunted = equipment;
    return _fakeHuntResult;
  }

  int autoHuntCalls = 0;

  @override
  Future<AutoHuntResult> autoHunt() async {
    autoHuntCalls++;
    return _fakeAutoHunt;
  }
}

/// What the fake catalogue hunt answers.
AutoHuntResult _fakeAutoHunt = const AutoHuntResult(found: 9, created: 4);

/// What the fake hunt answers — module-level because the actions fake is
/// created lazily on the provider's first read, after the pump.
TerritoryHuntResult _fakeHuntResult =
    const TerritoryHuntResult(uf: 'MT', holders: []);

class _NoProducts extends ProductsList {
  @override
  Future<List<ProductModel>> build() async => const [];
}

_FakeRadarActions? _actions;

RadarFeed _feed({List<RadarItem>? items, List<String> warnings = const []}) =>
    RadarFeed(
      items: items ??
          [
            RadarItem(
              type: RadarItemType.recompra,
              id: 'reorder-ct1',
              title: 'Hospital Santa Rosa',
              context: 'estimativa: acaba em 2d',
              deadline: DateTime.now().add(const Duration(days: 2)),
              contactId: 'ct1',
            ),
            RadarItem(
              type: RadarItemType.licitacao,
              id: 'sig-1',
              title: 'Pref. São José dos Campos',
              context: 'SP · Luva meio dedo',
              valueCents: 336000,
              deadline: DateTime.now().add(const Duration(days: 5)),
              leadId: 'lead-9',
            ),
            const RadarItem(
              type: RadarItemType.crossSell,
              id: 'xsell-ct9',
              title: 'Clínica Femina',
              context: 'compra luva e ainda não leva avental — 4 clientes '
                  'seus levam juntos',
              contactId: 'ct9',
            ),
            const RadarItem(
              type: RadarItemType.parque,
              id: 'cnes-123',
              title: 'Hospital São Mateus',
              context: '2 Tomógrafo no CNES · no seu território (Cuiabá) · '
                  'dado público, quantidade — não idade',
              cnes: '123',
              competencia: '2026-07',
            ),
          ],
      warnings: warnings,
    );

Future<void> _pump(WidgetTester tester,
    {RadarFeed? feed,
    Size size = const Size(1000, 2200),
    List<SignalSourceHealth> sources = const []}) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  final router = GoRouter(
    initialLocation: '/radar',
    routes: [
      GoRoute(
          path: '/radar',
          builder: (_, __) => const Scaffold(body: RadarBody())),
      GoRoute(
          path: '/prospect',
          builder: (_, __) => const Scaffold(body: Text('queue-screen'))),
      GoRoute(
          path: '/sales/portfolio/:id',
          builder: (_, state) =>
              Scaffold(body: Text('portfolio-${state.pathParameters['id']}'))),
    ],
  );

  await tester.pumpWidget(ProviderScope(
    overrides: [
      radarProvider.overrideWith((ref) async => feed ?? _feed()),
      signalWatchesProvider.overrideWith((ref) async => const [
            SignalWatch(id: 'w1', keywords: ['luva'], states: ['MT'])
          ]),
      signalSourcesProvider.overrideWith((ref) async => sources),
      radarActionsProvider
          .overrideWith((ref) => _actions ??= _FakeRadarActions(ref)),
      productsListProvider.overrideWith(_NoProducts.new),
      dioProvider.overrideWithValue(Dio()),
    ],
    child: MaterialApp.router(theme: AppTheme.darkTheme, routerConfig: router),
  ));
  await tester.pumpAndSettle();
}

void main() {
  _panelTests();
  _panelContextTests();
  _workByHandTests();
  _sourceDownGroup();
  _sourceBannerWidgetGroup();
  _mainParkPage();
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() {
    LocaleSettings.setLocale(AppLocale.pt);
    _actions = null;
  });

  testWidgets('renders every source with type filters and the watch line',
      (tester) async {
    await _pump(tester);

    expect(find.byKey(const Key('radar_watch_current')), findsOneWidget);
    expect(find.byKey(const Key('radar_item_reorder-ct1')), findsOneWidget);
    expect(find.byKey(const Key('radar_item_sig-1')), findsOneWidget);
    expect(find.byKey(const Key('radar_item_cnes-123')), findsOneWidget);
    // The honesty rule travels in the copy: counts, never age.
    expect(find.textContaining('quantidade — não idade'), findsOneWidget);
    // Cross-sell carries its reason and opens the customer.
    expect(find.byKey(const Key('radar_item_xsell-ct9')), findsOneWidget);
    expect(
        find.byKey(const Key('radar_open_customer_xsell-ct9')), findsOneWidget);

    // Filtering to one type hides the others.
    await tester.tap(find.byKey(const Key('radar_filter_parque')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('radar_item_cnes-123')), findsOneWidget);
    expect(find.byKey(const Key('radar_item_sig-1')), findsNothing);
  });

  testWidgets('preparing a park offer queues the draft and goes to the queue',
      (tester) async {
    await _pump(tester);

    await tester.tap(find.byKey(const Key('radar_park_offer_cnes-123')));
    await tester.pumpAndSettle();

    expect(_actions!.offered, ['123']);
    expect(find.text('queue-screen'), findsOneWidget);
  });

  testWidgets(
      'an auto-offered tender opens the draft, and a failed collector '
      'is named', (tester) async {
    await _pump(tester, feed: _feed(warnings: const ['recompra']));

    expect(find.byKey(const Key('radar_warnings')), findsOneWidget);
    expect(find.textContaining('recompra'), findsWidgets);

    await tester.tap(find.byKey(const Key('radar_open_draft_sig-1')));
    await tester.pumpAndSettle();
    expect(find.text('queue-screen'), findsOneWidget);
  });

  testWidgets('the territory hunt answers with holders and honest flags',
      (tester) async {
    _fakeHuntResult = const TerritoryHuntResult(uf: 'MT', holders: [
      TerritoryHolder(
          cnes: '111',
          total: 2,
          tradeName: 'Hospital Santa Rosa',
          alreadyKnown: true),
      TerritoryHolder(cnes: '222', total: 1, tradeName: 'Hospital São Mateus'),
    ]);
    await _pump(tester, size: const Size(1200, 3600));

    await tester.enterText(
        find.byKey(const Key('territory_hunt_input')), 'tomógrafo');
    await tester.tap(find.byKey(const Key('territory_hunt_search')));
    await tester.pumpAndSettle();

    expect(_actions!.hunted, 'tomógrafo');
    // No cities, no month from the server: the scope line says the state
    // and nothing about a month it does not know.
    expect(find.byKey(const Key('territory_hunt_scope')), findsOneWidget);
    expect(find.text('em MT'), findsOneWidget);
    // A known contact is labeled, not re-offered; a new one gets the CTA.
    expect(find.byKey(const Key('territory_known_111')), findsOneWidget);
    await tester.tap(find.byKey(const Key('territory_offer_222')));
    await tester.pumpAndSettle();
    expect(_actions!.offered, ['222']);
  });

  testWidgets('a park hit names the DATASUS month it is quoting',
      (tester) async {
    await _pump(tester);

    // CNES lags 45–75 days: the card says "07/2026", never "today".
    expect(find.byKey(const Key('radar_park_month_cnes-123')), findsOneWidget);
    expect(find.text('dados CNES de 07/2026'), findsOneWidget);
    // The other item types carry no month line.
    expect(find.byKey(const Key('radar_park_month_sig-1')), findsNothing);
  });

  testWidgets('the hunt says which cities and which month it counted',
      (tester) async {
    _fakeHuntResult = const TerritoryHuntResult(
      uf: 'MT',
      cities: ['Cuiabá', 'Várzea Grande'],
      competencia: '2026-06',
      holders: [
        TerritoryHolder(
            cnes: '222', total: 1, tradeName: 'Hospital São Mateus'),
      ],
    );
    await _pump(tester, size: const Size(1200, 3600));

    await tester.enterText(
        find.byKey(const Key('territory_hunt_input')), 'tomógrafo');
    await tester.tap(find.byKey(const Key('territory_hunt_search')));
    await tester.pumpAndSettle();

    expect(
        find.text('no seu território (Cuiabá, Várzea Grande) · '
            'dados CNES de 06/2026'),
        findsOneWidget);
  });

  // Reorders, cross-sell and dates read from `sales`. Before the first sale
  // the feed cannot carry them, and a chip or a promise for an empty source
  // advertises something the account cannot see yet.
  testWidgets('before the first sale the carteira sources are not advertised',
      (tester) async {
    await _pump(
      tester,
      feed: _feed(items: [
        RadarItem(
          type: RadarItemType.licitacao,
          id: 'sig-1',
          title: 'Pref. São José dos Campos',
          context: 'SP · Luva meio dedo',
          deadline: DateTime.now().add(const Duration(days: 5)),
        ),
        const RadarItem(
          type: RadarItemType.parque,
          id: 'cnes-123',
          title: 'Hospital São Mateus',
          context: '2 Tomógrafo no CNES',
          cnes: '123',
        ),
      ]),
    );

    expect(find.byKey(const Key('radar_filter_licitacao')), findsOneWidget);
    expect(find.byKey(const Key('radar_filter_parque')), findsOneWidget);
    expect(find.byKey(const Key('radar_filter_recompra')), findsNothing);
    expect(find.byKey(const Key('radar_filter_data')), findsNothing);
    expect(find.byKey(const Key('radar_filter_crossSell')), findsNothing);
    expect(find.byKey(const Key('radar_carteira_hint')), findsOneWidget);
    expect(find.text(t.radar.carteira_hint), findsOneWidget);
  });

  testWidgets('a carteira item brings its chip and retires the hint',
      (tester) async {
    await _pump(tester);

    expect(find.byKey(const Key('radar_filter_recompra')), findsOneWidget);
    expect(find.byKey(const Key('radar_filter_crossSell')), findsOneWidget);
    expect(find.byKey(const Key('radar_carteira_hint')), findsNothing);
  });

  testWidgets('an empty feed promises only what can appear before a sale',
      (tester) async {
    await _pump(tester, feed: _feed(items: const []));

    expect(find.byKey(const Key('radar_empty')), findsOneWidget);
    expect(find.text(t.radar.empty_hint_prospecting), findsOneWidget);
    expect(find.text(t.radar.empty_hint), findsNothing);
    expect(find.byKey(const Key('radar_carteira_hint')), findsOneWidget);
  });

  test('competenciaLabel reads a DATASUS month the Brazilian way', () {
    expect(competenciaLabel('2026-07'), '07/2026');
    // Not a month: pass it through rather than invent a date.
    expect(competenciaLabel('202607'), '202607');
  });
  // Typing equipment names is the opposite of a product that goes looking —
  // and it only works if you already know which of your words the CNES
  // registry happens to use. The catalogue button answers for all of them.
  testWidgets('the catalogue hunt runs without a word being typed',
      (tester) async {
    _actions = null;
    await _pump(tester);

    await tester.tap(find.byKey(const Key('territory_auto_hunt')));
    await tester.pumpAndSettle();

    expect(_actions!.autoHuntCalls, 1);
    expect(_actions!.hunted, isNull, reason: 'no word search was needed');
    // It says what it did, in leads — not "pronto".
    expect(find.byKey(const Key('territory_auto_hunt_result')), findsOneWidget);
    expect(find.text(t.radar.auto_hunt_result(created: '4', found: '9')),
        findsOneWidget);
  });

  // An empty territory is an answer, not a silent no-op.
  testWidgets('a hunt that found nothing says so', (tester) async {
    _actions = null;
    _fakeAutoHunt = const AutoHuntResult(found: 0, created: 0);
    addTearDown(
        () => _fakeAutoHunt = const AutoHuntResult(found: 9, created: 4));
    await _pump(tester);

    await tester.tap(find.byKey(const Key('territory_auto_hunt')));
    await tester.pumpAndSettle();

    expect(find.text(t.radar.auto_hunt_none), findsOneWidget);
  });
}

// The Radar answered "5" for a territory holding 2794 matching hospitals,
// and each park card repeated the same catalogue-wide equipment list with
// nothing saying what to sell. These pin both.

void _mainParkPage() {
  group('park page', () {
    testWidgets('says how many hospitals it is NOT showing', (tester) async {
      await _pump(
        tester,
        feed: RadarFeed(
          items: _feed().items,
          warnings: const [],
          park: const RadarParkUniverse(
            establishments: 2794,
            units: 9111,
            shown: 25,
            scope: 'em MT',
            competencia: '2026-07',
          ),
        ),
      );

      expect(find.byKey(const Key('radar_park_universe')), findsOneWidget);
      expect(find.textContaining('2794'), findsOneWidget);
      expect(find.textContaining('25'), findsWidgets);
    });

    testWidgets('stays quiet when the page IS everything', (tester) async {
      await _pump(
        tester,
        feed: RadarFeed(
          items: _feed().items,
          warnings: const [],
          park: const RadarParkUniverse(
            establishments: 4,
            units: 12,
            shown: 4,
            scope: 'em MT',
          ),
        ),
      );

      expect(find.byKey(const Key('radar_park_universe')), findsNothing,
          reason: 'nothing is being missed, so there is nothing to warn about');
    });

    testWidgets('a park card names what to offer', (tester) async {
      await _pump(
        tester,
        feed: RadarFeed(
          items: const [
            RadarItem(
              type: RadarItemType.parque,
              id: 'cnes-123',
              title: 'Hospital São Mateus',
              context: '40 aparelhos no CNES: 40× Incubadora',
              cnes: '123',
              competencia: '2026-07',
              matches: [
                ParkMatch(
                    productName: 'Incubadora neonatal',
                    equipment: 'Incubadora',
                    qtExist: 40),
                ParkMatch(
                    productName: 'Capuz de oxigênio',
                    equipment: 'Usina De Oxigenio',
                    qtExist: 1),
                ParkMatch(
                    productName: 'Mangueira de oxigênio',
                    equipment: 'Usina De Oxigenio',
                    qtExist: 1),
                ParkMatch(
                    productName: 'Berço aquecido',
                    equipment: 'Berço Aquecido',
                    qtExist: 3),
              ],
            ),
          ],
          warnings: const [],
        ),
      );

      expect(find.byKey(const Key('radar_offer_cnes-123_Incubadora neonatal')),
          findsOneWidget);
      expect(find.text('Incubadora neonatal'), findsOneWidget);
      expect(find.text('Berço aquecido'), findsNothing,
          reason: 'only the first few are named; the rest are counted');
      expect(find.text(t.radar.offer_more(count: '1')), findsOneWidget);
    });

    // The chip opens the product, for the same reason the lead panel's line
    // does: nobody knows a hundred-item catalogue by heart, and a product
    // named with nowhere to read it leaves the seller guessing at the spec in
    // front of a buyer.
    testWidgets('a matched product opens its own page', (tester) async {
      await _pump(
        tester,
        feed: RadarFeed(
          items: const [
            RadarItem(
              type: RadarItemType.parque,
              id: 'cnes-123',
              title: 'Hospital São Mateus',
              context: '4 aparelhos no CNES',
              cnes: '123',
              matches: [
                ParkMatch(
                    productId: 'prod-9',
                    productName: 'Incubadora neonatal',
                    equipment: 'Incubadora',
                    qtExist: 4),
              ],
            ),
          ],
          warnings: [],
        ),
      );

      expect(find.byKey(const Key('radar_offer_open_prod-9')), findsOneWidget);
    });

    // An older server sends no id. The chip stays a label rather than becoming
    // a tap that goes nowhere.
    testWidgets('a match with no product id is not tappable', (tester) async {
      await _pump(
        tester,
        feed: RadarFeed(
          items: const [
            RadarItem(
              type: RadarItemType.parque,
              id: 'cnes-123',
              title: 'Hospital São Mateus',
              context: '4 aparelhos no CNES',
              cnes: '123',
              matches: [
                ParkMatch(
                    productName: 'Incubadora neonatal',
                    equipment: 'Incubadora',
                    qtExist: 4),
              ],
            ),
          ],
          warnings: [],
        ),
      );

      expect(find.text('Incubadora neonatal'), findsOneWidget);
      // The card has other tappable parts, so the assertion is about THIS
      // chip: no offer-opening tap was built for a match with no id.
      expect(
        find.byWidgetPredicate((w) =>
            w.key is ValueKey<String> &&
            (w.key! as ValueKey<String>).value.startsWith('radar_offer_open_')),
        findsNothing,
        reason: 'a chip with no product id must not look tappable',
      );
    });
  });
}

// ---- source outage banner ----

// The screen could not tell an outage from a quiet week: it rendered what was
// stored, so a portal refusing every request looked exactly like a portal
// with nothing new. These pin the sentence that closes that gap.
void _sourceDownGroup() {
  group('sourceDownMessage', () {
    final now = DateTime(2026, 9, 10, 13, 0);

    test('a source that never answered says exactly that', () {
      const s = SignalSourceHealth(source: 'pncp', reachable: false);

      expect(
          sourceDownMessage(s, now), t.radar.source_down_never(source: 'PNCP'));
    });

    test('hours, while it is still hours', () {
      final s = SignalSourceHealth(
        source: 'pncp',
        reachable: false,
        lastSuccessAt: now.subtract(const Duration(hours: 7)),
      );

      expect(sourceDownMessage(s, now),
          t.radar.source_down(source: 'PNCP', ago: t.radar.ago_hours(n: 7)));
    });

    // Past two days "51 h" stops meaning anything to a reader.
    test('days, once hours stop being readable', () {
      final s = SignalSourceHealth(
        source: 'pncp',
        reachable: false,
        lastSuccessAt: now.subtract(const Duration(days: 3)),
      );

      expect(sourceDownMessage(s, now),
          t.radar.source_down(source: 'PNCP', ago: t.radar.ago_days(n: 3)));
    });

    // A poll that just failed is still worth saying; "há 0 min" is not.
    test('a fresh failure never reads as zero', () {
      final s = SignalSourceHealth(
        source: 'pncp',
        reachable: false,
        lastSuccessAt: now.subtract(const Duration(seconds: 20)),
      );

      expect(sourceDownMessage(s, now),
          t.radar.source_down(source: 'PNCP', ago: t.radar.ago_minutes(n: 1)));
    });
  });

  group('SignalSourceHealth.fromJson', () {
    // Reading a missing field as "down" would have every screen cry outage
    // the moment a payload changes shape.
    test('an absent reachable flag is not an outage', () {
      final s =
          SignalSourceHealth.fromJson(<String, dynamic>{'source': 'pncp'});

      expect(s.reachable, isTrue);
      expect(s.lastSuccessAt, isNull);
    });

    test('reads the outage the server reported', () {
      final s = SignalSourceHealth.fromJson(<String, dynamic>{
        'source': 'pncp',
        'reachable': false,
        'last_success_at': '2026-09-09T22:13:00Z',
      });

      expect(s.reachable, isFalse);
      expect(s.label, 'PNCP');
      expect(s.lastSuccessAt, isNotNull);
    });
  });
}

// The banner is the whole point: without it the reader takes the last good
// collection for the current state of the market.
void _sourceBannerWidgetGroup() {
  group('source outage banner', () {
    testWidgets('a source that did not answer is named on screen',
        (tester) async {
      await _pump(tester, sources: [
        SignalSourceHealth(
          source: 'pncp',
          reachable: false,
          lastSuccessAt: DateTime.now().subtract(const Duration(hours: 7)),
        ),
      ]);

      expect(find.byKey(const Key('radar_source_down_pncp')), findsOneWidget);
    });

    testWidgets('a healthy source says nothing at all', (tester) async {
      await _pump(tester, sources: const [
        SignalSourceHealth(source: 'pncp', reachable: true),
      ]);

      expect(find.byKey(const Key('radar_source_down_pncp')), findsNothing);
    });

    // The pre-feature behaviour, and what an instance deployed before the
    // stamp exists must keep doing: say nothing rather than something wrong.
    testWidgets('no health data draws no banner', (tester) async {
      await _pump(tester);

      expect(find.byKey(const Key('radar_source_down_pncp')), findsNothing);
    });
  });
}

// The seller who wants to chase it themselves.
//
// A tender the auto-offer already drafted opens that draft. One it has NOT
// drafted used to offer only "read the notice" — which leaves the product and
// records nothing — and "not for us". An expiring contract beside it could
// already be worked by hand, and that asymmetry was visible on one screen.
void _workByHandTests() {
  testWidgets('an undrafted tender can be worked by hand', (tester) async {
    await _pump(tester,
        feed: _feed(items: [
          const RadarItem(
            type: RadarItemType.licitacao,
            id: 'sig-sem-rascunho',
            title: 'Município de Sapezal',
            context: 'MT · aquisição de ventiladores pulmonares',
            url: 'https://pncp.gov.br/x',
          ),
        ]));

    final btn = find.byKey(const Key('radar_work_signal_sig-sem-rascunho'));
    expect(btn, findsOneWidget);

    await tester.tap(btn);
    await tester.pumpAndSettle();

    expect(_actions!.workedSignals, ['sig-sem-rascunho'],
        reason: 'the tender was not put in the funnel');
  });

  // Already drafted: the next step is reading what was written, not minting a
  // second card for the same hospital.
  testWidgets('a drafted tender offers the draft, not a second card',
      (tester) async {
    await _pump(tester,
        feed: _feed(items: [
          const RadarItem(
            type: RadarItemType.licitacao,
            id: 'sig-com-rascunho',
            title: 'Município de Sapezal',
            context: 'MT · aquisição de ventiladores pulmonares',
            leadId: 'lead-1',
          ),
        ]));

    expect(find.byKey(const Key('radar_open_draft_sig-com-rascunho')),
        findsOneWidget);
    expect(find.byKey(const Key('radar_work_signal_sig-com-rascunho')),
        findsNothing);
  });
}

// The card is a headline that truncates. Until the panel existed that ellipsis
// was the end of the road — and for a quarter of these tenders there is no
// link either, so the panel is the only place the opportunity can be read.
/// Taps the card where the seller would: on the title, not the centre. The
/// centre of a narrow card can sit on a CTA pill, which absorbs the gesture.
Future<void> _tapCard(WidgetTester tester, String id) async {
  final card = find.byKey(Key('radar_open_$id'));
  await tester.tapAt(tester.getTopLeft(card) + const Offset(40, 24));
  await tester.pumpAndSettle();
}

/// The portal, named — and what the panel says about the buyer.
void _panelContextTests() {
  testWidgets('a notice with no link names PNCP and offers it', (tester) async {
    await _pump(tester,
        size: const Size(1400, 2400),
        feed: _feed(items: [
          const RadarItem(
            type: RadarItemType.licitacao,
            id: 'sig-sem-link',
            title: 'MUNICIPIO DE ITIQUIRA',
            context: 'Itiquira/MT · objeto',
            detail: 'objeto completo',
            reference: '03370251000156-1-000055/2026',
          ),
        ]));
    await _tapCard(tester, 'sig-sem-link');

    // Naming the portal is the point: "search the procurement portal" without
    // saying which is half an instruction.
    expect(find.byKey(const Key('radar_panel_open_portal_sig-sem-link')),
        findsOneWidget);
    expect(find.text(t.radar.panel_open_portal), findsOneWidget);
  });

  testWidgets('a notice WITH a link does not push the portal instead',
      (tester) async {
    await _pump(tester,
        size: const Size(1400, 2400),
        feed: _feed(items: [
          const RadarItem(
            type: RadarItemType.licitacao,
            id: 'sig-link',
            title: 'X',
            context: 'MT · objeto',
            detail: 'objeto',
            reference: 'x-1-000001/2026',
            url: 'https://portal.exemplo/edital',
          ),
        ]));
    await _tapCard(tester, 'sig-link');

    expect(
        find.byKey(const Key('radar_panel_open_url_sig-link')), findsOneWidget);
    expect(find.byKey(const Key('radar_panel_open_portal_sig-link')),
        findsNothing);
  });

  // Writing a stranger's opening line to an existing customer is the kind of
  // mistake that loses one.
  testWidgets('the panel says whether this buyer already knows you',
      (tester) async {
    await _pump(tester,
        size: const Size(1400, 2400),
        feed: _feed(items: [
          RadarItem(
            type: RadarItemType.licitacao,
            id: 'sig-quente',
            title: 'MUNICIPIO X',
            context: 'MT · objeto',
            detail: 'objeto',
            cnpj: '111',
            buyer: BuyerContext(
              relationship: BuyerRelationship.sold,
              contactId: 'c-1',
              contracts: 3,
              lastSupplier: 'Medicalpar',
              lastObject: 'ventiladores pulmonares',
              lastContractAt: DateTime(2025, 3, 10),
            ),
          ),
        ]));
    await _tapCard(tester, 'sig-quente');

    expect(find.byKey(const Key('radar_panel_buyer_known_sig-quente')),
        findsOneWidget);
    expect(find.text(t.radar.panel_buyer_known), findsOneWidget);
    expect(find.byKey(const Key('radar_panel_history_sig-quente')),
        findsOneWidget);
    // The incumbent, named from the published record — the one place a
    // competitor's name may honestly come from.
    expect(find.byKey(const Key('radar_panel_last_supplier_sig-quente')),
        findsOneWidget);
  });

  // The regression for BOTH claims that shipped: a card this product opened by
  // itself is neither a sale nor a conversation. This account had no channel
  // connected and had never sent a message.
  testWidgets('a buyer merely on the board is not reported as contacted',
      (tester) async {
    await _pump(tester,
        size: const Size(1400, 2400),
        feed: _feed(items: [
          const RadarItem(
            type: RadarItemType.licitacao,
            id: 'sig-patris',
            title: 'INSTITUTO PATRIS',
            context: 'MT · objeto',
            detail: 'objeto',
            cnpj: '37678845000140',
            buyer: BuyerContext(
              relationship: BuyerRelationship.listed,
              contactId: 'c-patris',
              scanned: true,
            ),
          ),
        ]));
    await _tapCard(tester, 'sig-patris');

    expect(find.text(t.radar.panel_buyer_listed), findsOneWidget);
    expect(find.text(t.radar.panel_buyer_known), findsNothing,
        reason: 'a contact is not a sale');
    expect(find.text(t.radar.panel_buyer_contacted), findsNothing,
        reason: 'nothing was ever sent from this account');
    // Read, and nothing of ours found — said out loud, because silence would
    // read as "we never looked".
    expect(find.byKey(const Key('radar_panel_history_none_sig-patris')),
        findsOneWidget);
  });

  testWidgets('a buyer we know nothing about gets no block', (tester) async {
    await _pump(tester,
        size: const Size(1400, 2400),
        feed: _feed(items: [
          const RadarItem(
            type: RadarItemType.licitacao,
            id: 'sig-nada',
            title: 'MUNICIPIO Y',
            context: 'MT · objeto',
            detail: 'objeto',
            cnpj: '222',
          ),
        ]));
    await _tapCard(tester, 'sig-nada');

    expect(
        find.byKey(const Key('radar_panel_buyer_known_sig-nada')), findsNothing,
        reason: 'an empty section reads as a failure');
  });
}

void _panelTests() {
  const itiquira = RadarItem(
    type: RadarItemType.licitacao,
    id: 'sig-itiquira',
    title: 'MUNICIPIO DE ITIQUIRA',
    context: 'Itiquira/MT · AQUISIÇÃO DE PEÇAS DE REPOSIÇÃO ORIGINAIS...',
    detail: 'AQUISIÇÃO DE PEÇAS DE REPOSIÇÃO ORIGINAIS PARA ATENDER ÀS '
        'NECESSIDADES DO LABORATÓRIO MUNICIPAL, GARANTINDO A CONTINUIDADE '
        'DOS EXAMES.',
    cnpj: '03370251000156',
    reference: '03370251000156-1-000055/2026',
    city: 'Itiquira',
    uf: 'MT',
  );

  testWidgets('the desk layout opens the panel beside the list',
      (tester) async {
    await _pump(tester,
        feed: _feed(items: [itiquira]), size: const Size(1400, 2400));
    expect(find.byKey(const Key('radar_detail_panel')), findsNothing);

    await _tapCard(tester, 'sig-itiquira');
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('radar_detail_panel')), findsOneWidget);
    // The whole object, not the truncated headline. This exact tender was
    // dismissed as out-of-trade on the strength of the cut text — the word
    // that decides it, LABORATÓRIO, sat past the ellipsis.
    expect(
      find.byKey(const Key('radar_panel_detail_sig-itiquira')),
      findsOneWidget,
    );
    final detail = tester.widget<SelectableText>(
        find.byKey(const Key('radar_panel_detail_sig-itiquira')));
    expect(detail.data, contains('LABORATÓRIO MUNICIPAL'));
  });

  // A notice with no link is not a dead end, and must not look like one: the
  // control number is the only address that exists, so the panel says so.
  testWidgets('a notice with no link shows the number and says why',
      (tester) async {
    await _pump(tester,
        feed: _feed(items: [itiquira]), size: const Size(1400, 2400));
    await _tapCard(tester, 'sig-itiquira');
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('radar_panel_reference_sig-itiquira')),
        findsOneWidget);
    expect(find.byKey(const Key('radar_panel_no_link_sig-itiquira')),
        findsOneWidget);
    expect(find.byKey(const Key('radar_panel_open_url_sig-itiquira')),
        findsNothing);
    // The buyer's CNPJ is the thread a seller actually pulls, since a public
    // notice publishes no contact at all.
    expect(
        find.byKey(const Key('radar_panel_cnpj_sig-itiquira')), findsOneWidget);
  });

  testWidgets('a notice WITH a link offers it instead of the excuse',
      (tester) async {
    tester.view.physicalSize = const Size(1400, 1600);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await _pump(tester,
        feed: _feed(items: [
          const RadarItem(
            type: RadarItemType.licitacao,
            id: 'sig-com-link',
            title: 'Município X',
            context: 'MT · objeto',
            detail: 'objeto completo',
            reference: 'x-1-000001/2026',
            url: 'https://pncp.gov.br/x',
          ),
        ]));
    await _tapCard(tester, 'sig-com-link');
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('radar_panel_open_url_sig-com-link')),
        findsOneWidget);
    expect(find.byKey(const Key('radar_panel_no_link_sig-com-link')),
        findsNothing);
  });

  // Narrow: no room beside the list, so the same content arrives as a sheet.
  // A phone does not get less of the opportunity than a desk.
  testWidgets('a narrow screen gets the same content in a sheet',
      (tester) async {
    await _pump(tester,
        feed: _feed(items: [itiquira]), size: const Size(700, 2400));
    await _tapCard(tester, 'sig-itiquira');
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('radar_detail_panel')), findsNothing,
        reason: 'the side panel does not fit here');
    expect(find.byType(RadarDetailPanel), findsOneWidget);
    expect(find.byKey(const Key('radar_panel_detail_sig-itiquira')),
        findsOneWidget);
  });

  // The registry publishes a phone, an address and an e-mail for most of the
  // territory, and the panel rendered the device counts alone — so the card
  // that finally had a way to reach the hospital showed none of it.
  testWidgets('the panel says how to reach the hospital', (tester) async {
    await _pump(tester,
        size: const Size(1400, 2400),
        feed: _feed(items: [
          const RadarItem(
            type: RadarItemType.parque,
            id: 'cnes-9209352',
            title: 'HOSPITAL MUNICIPAL DE CUIABA',
            context: '872 aparelhos no CNES',
            detail: '402x Bomba De Infusao',
            phone: '(65) 3617-7876',
            email: 'ca.hpsmc@cuiaba.mt.gov.br',
            address: 'Rua General Valle, 192',
          ),
        ]));
    await _tapCard(tester, 'cnes-9209352');

    expect(find.byKey(const Key('radar_panel_phone_cnes-9209352')),
        findsOneWidget);
    expect(find.byKey(const Key('radar_panel_email_cnes-9209352')),
        findsOneWidget);
    expect(find.text('Rua General Valle, 192'), findsOneWidget);
  });

  // A buyer the account already holds needs a way IN, not just a label. "Já
  // está no seu quadro" used to leave the seller hunting a name across the
  // board — and the label alone is not a position: a card in Ganhos and one
  // nobody touched are both "no quadro".
  testWidgets('the panel opens the card and names the column it sits in',
      (tester) async {
    await _pump(tester,
        size: const Size(1400, 2400),
        feed: _feed(items: [
          const RadarItem(
            type: RadarItemType.parque,
            id: 'cnes-1',
            title: 'HOSPITAL X',
            context: 'MT',
            detail: 'parque',
            buyer: BuyerContext(
              relationship: BuyerRelationship.listed,
              contactId: 'c-1',
              leadId: 'lead-9',
              stage: 'Clientes',
            ),
          ),
        ]));
    await _tapCard(tester, 'cnes-1');

    expect(find.byKey(const Key('radar_panel_stage_cnes-1')), findsOneWidget);
    expect(find.text(t.radar.panel_stage(stage: 'Clientes')), findsOneWidget);
    expect(
        find.byKey(const Key('radar_panel_open_board_cnes-1')), findsOneWidget);
  });

  // A buyer with no card must not get a button that opens an empty board.
  testWidgets('a buyer with no card gets no board link', (tester) async {
    await _pump(tester,
        size: const Size(1400, 2400),
        feed: _feed(items: [
          const RadarItem(
            type: RadarItemType.parque,
            id: 'cnes-2',
            title: 'HOSPITAL Y',
            context: 'MT',
            detail: 'parque',
            buyer: BuyerContext(relationship: BuyerRelationship.cold),
          ),
        ]));
    await _tapCard(tester, 'cnes-2');

    expect(
        find.byKey(const Key('radar_panel_open_board_cnes-2')), findsNothing);
    expect(find.byKey(const Key('radar_panel_stage_cnes-2')), findsNothing);
  });

  // The card's chips already opened the product. The panel — the place where
  // somebody actually decides — listed the same names as dead text.
  testWidgets('each offered product opens its product page', (tester) async {
    await _pump(tester,
        size: const Size(1400, 2400),
        feed: _feed(items: [
          const RadarItem(
            type: RadarItemType.parque,
            id: 'cnes-3',
            title: 'HOSPITAL Z',
            context: 'MT',
            detail: 'parque',
            matches: [
              ParkMatch(
                  productId: 'p-1',
                  productName: 'Suporte bomba de infusão',
                  equipment: 'Bomba De Infusao',
                  qtExist: 402),
              // Named by the matcher, not identified: nowhere to go, so no
              // link — the same rule the card's chips follow.
              ParkMatch(
                  productId: '',
                  productName: 'Reanimador manual',
                  equipment: 'Reanimador',
                  qtExist: 243),
            ],
          ),
        ]));
    await _tapCard(tester, 'cnes-3');

    expect(find.byKey(const Key('radar_panel_offer_open_p-1')), findsOneWidget);
    // The unidentified one still renders — it just gets no link, so the
    // seller never taps something that goes nowhere.
    expect(find.byKey(const Key('radar_panel_offer_open_')), findsNothing);
  });

  // A public hospital rarely signs anything — its secretaria does, and 11 of
  // MT's 20 biggest parks publish only the secretaria's CNPJ. Reading that
  // record is the only way to know who supplies the place; saying it was the
  // HOSPITAL that contracted would be false, and identical on the seven
  // hospitals that share one secretaria.
  testWidgets(
      'the panel names who buys for the hospital, and whose record it is',
      (tester) async {
    await _pump(tester,
        size: const Size(1400, 2400),
        feed: _feed(items: [
          const RadarItem(
            type: RadarItemType.parque,
            id: 'cnes-5558638',
            title: 'HOSPITAL CENTRAL DE ALTA COMPLEXIDADE',
            context: 'MT',
            detail: 'parque',
            buyerEntityName: 'SECRETARIA ESTADUAL DE SAUDE DE MATO GROSSO',
            buyer: BuyerContext(
              relationship: BuyerRelationship.listed,
              contracts: 3,
              scanned: true,
              lastSupplier: 'Medicalpar',
              historyVia: 'SECRETARIA ESTADUAL DE SAUDE DE MATO GROSSO',
            ),
          ),
        ]));
    await _tapCard(tester, 'cnes-5558638');

    expect(find.byKey(const Key('radar_panel_purchaser_cnes-5558638')),
        findsOneWidget);
    expect(
        find.text(t.radar.panel_history_via(
            n: 3, org: 'SECRETARIA ESTADUAL DE SAUDE DE MATO GROSSO')),
        findsOneWidget);
    // And never the unqualified claim, which would be about the hospital.
    expect(find.text(t.radar.panel_history_count(n: 3)), findsNothing);
  });

  // A hospital that answers for itself gets the plain sentence — no "via"
  // caveat explaining away a record that IS its own.
  testWidgets('its own record carries no caveat', (tester) async {
    await _pump(tester,
        size: const Size(1400, 2400),
        feed: _feed(items: [
          const RadarItem(
            type: RadarItemType.parque,
            id: 'cnes-2396866',
            title: 'SANTA CASA DE RONDONOPOLIS',
            context: 'MT',
            detail: 'parque',
            cnpj: '03099157000104',
            buyer: BuyerContext(
              relationship: BuyerRelationship.listed,
              contracts: 2,
              scanned: true,
            ),
          ),
        ]));
    await _tapCard(tester, 'cnes-2396866');

    expect(find.text(t.radar.panel_history_count(n: 2)), findsOneWidget);
    expect(find.byKey(const Key('radar_panel_purchaser_cnes-2396866')),
        findsNothing);
  });
  // The park says what the hospital owns; the census says how much care it is
  // built to deliver, and that is what sizes an order. Twenty neonatal cots
  // against six incubators in the park IS the opening — and the product could
  // not see a single bed until the LT file was read.
  testWidgets('the panel shows the beds the hospital runs', (tester) async {
    await _pump(tester,
        size: const Size(1400, 2400),
        feed: _feed(items: [
          const RadarItem(
            type: RadarItemType.parque,
            id: 'cnes-2396866',
            title: 'SANTA CASA DE RONDONOPOLIS',
            context: 'MT',
            detail: 'parque',
            beds: [
              BedCount(
                  bedCode: '81',
                  name: 'UTI NEONATAL - TIPO II',
                  qtExist: 20,
                  isIcu: true,
                  isNeonatal: true),
              // Intensive care, NOT neonatal — the code that was miscounted
              // as neonatal in the audit that produced this feature.
              BedCount(
                  bedCode: '86',
                  name: 'UTI CORONARIANA TIPO III - UCO TIPO III',
                  qtExist: 6,
                  isIcu: true),
            ],
          ),
        ]));
    await _tapCard(tester, 'cnes-2396866');

    expect(find.byKey(const Key('radar_panel_bed_cnes-2396866_81')),
        findsOneWidget);
    expect(find.byKey(const Key('radar_panel_bed_cnes-2396866_86')),
        findsOneWidget);
    expect(find.text('20×'), findsOneWidget);
  });

  // A row with no census renders no section — an empty heading reads as a
  // hospital with no beds, which is a claim about the hospital.
  testWidgets('no census, no section', (tester) async {
    await _pump(tester,
        size: const Size(1400, 2400),
        feed: _feed(items: [
          const RadarItem(
            type: RadarItemType.parque,
            id: 'cnes-sem',
            title: 'HOSPITAL SEM CENSO',
            context: 'MT',
            detail: 'parque',
          ),
        ]));
    await _tapCard(tester, 'cnes-sem');

    expect(find.text(t.radar.panel_beds), findsNothing);
  });

  // A habilitation is capability the Ministry FUNDS — an ordinance pays for
  // it and requires the equipment that entitles it. Harder than anything the
  // hospital declares about itself, and never read until the audit.
  testWidgets('the panel shows what the Ministry funds and what it performs',
      (tester) async {
    await _pump(tester,
        size: const Size(1400, 2600),
        feed: _feed(items: [
          const RadarItem(
            type: RadarItemType.parque,
            id: 'cnes-2396866',
            title: 'SANTA CASA DE RONDONOPOLIS',
            context: 'MT',
            detail: 'parque',
            habilitations: [
              Habilitation(
                  code: '2602',
                  name: 'UTI II NEONATAL',
                  beds: 10,
                  isIcu: true,
                  isNeonatal: true),
              // Not bed-based: zero must not read as "no capacity", so no
              // count is printed at all.
              Habilitation(
                  code: '1408', name: 'REFERENCIA EM TRIAGEM NEONATAL'),
            ],
            services: [
              CnesService(
                  code: '122011', name: 'POTENCIAIS EVOCADOS', sus: true),
            ],
          ),
        ]));
    await _tapCard(tester, 'cnes-2396866');

    expect(find.byKey(const Key('radar_panel_hab_cnes-2396866_2602')),
        findsOneWidget);
    expect(
        find.text(
            t.radar.panel_habilitation_beds(name: 'UTI II NEONATAL', n: 10)),
        findsOneWidget);
    // The bed-less one renders its name alone.
    expect(find.text('REFERENCIA EM TRIAGEM NEONATAL'), findsOneWidget);
    expect(find.byKey(const Key('radar_panel_srv_cnes-2396866_122011')),
        findsOneWidget);
  });

  // No habilitation, no section — an empty heading would read as "the
  // Ministry funds nothing here", which is a claim about the hospital.
  testWidgets('no habilitation, no section', (tester) async {
    await _pump(tester,
        size: const Size(1400, 2400),
        feed: _feed(items: [
          const RadarItem(
            type: RadarItemType.parque,
            id: 'cnes-nada',
            title: 'HOSPITAL SEM HABILITACAO',
            context: 'MT',
            detail: 'parque',
          ),
        ]));
    await _tapCard(tester, 'cnes-nada');

    expect(find.text(t.radar.panel_habilitations), findsNothing);
    expect(find.text(t.radar.panel_services), findsNothing);
  });
}
