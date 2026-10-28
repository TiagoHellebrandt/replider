import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/sales/domain/models/funnel_model.dart';
import 'package:app/features/sales/presentation/controllers/approval_queue_controller.dart';
import 'package:app/features/sales/domain/models/product_model.dart';
import 'package:app/features/sales/presentation/controllers/funnels_controller.dart';
import 'package:app/features/sales/presentation/controllers/products_controller.dart';
import 'package:app/features/sales/presentation/controllers/radar_controller.dart';
import 'package:app/features/sales/presentation/controllers/signals_controller.dart';
import 'package:app/features/sales/presentation/controllers/contract_calendar_controller.dart';
import 'package:app/features/sales/presentation/screens/prospect_branch_screen.dart';
import 'package:app/features/sales/presentation/widgets/contract_calendar_body.dart';
import 'package:app/features/sales/presentation/widgets/radar_body.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

// US-M021. Hunting and approving are one sitting, so the destination holds
// both — and the backlog has to be readable from the tab itself, otherwise a
// queue nobody opens is a queue nobody works.

/// No funnels: the prospecting form then renders its "create a funnel first"
/// state, which is enough to prove the hunting tab is what mounts.
class _NoProducts extends ProductsList {
  @override
  Future<List<ProductModel>> build() async => const [];
}

class _NoFunnels extends FunnelsList {
  @override
  Future<List<FunnelModel>> build() async => const [];
}

/// The tabs are role-gated per tab (queue = vendedor+, signals = attendant
/// satisfaction, hunt = admin), so the harness pins a role explicitly.
class _FakeAuth extends AuthController {
  _FakeAuth(this.role);
  final String role;

  @override
  Future<UserModel?> build() async =>
      UserModel(id: 'u1', name: 'U', email: 'u@x.com', token: 't', role: role);
}

const _queue = [
  QueueLead(
    id: 'lead1',
    recipient: 'Clínica Bem Viver',
    recipientAddress: '+5511900000001',
    draft: 'Olá, vi que a clínica atende ortopedia…',
    companyName: 'Clínica Bem Viver',
  ),
  QueueLead(
    id: 'lead2',
    recipient: 'contato@hospital.com',
    recipientAddress: 'contato@hospital.com',
    draft: 'Boa tarde, trabalhamos com agendamento…',
  ),
];

/// Counts how many times the queue was actually fetched, and can answer
/// differently on a refetch — that difference is the whole point of the
/// tab-selection test below.
/// The queue endpoint answers with a page plus the size of the backlog behind
/// it. These tests care about the items, so the total simply matches them.
ApprovalQueuePage _asPage(List<QueueLead> items) =>
    ApprovalQueuePage(items: items, total: items.length, byFunnel: const []);

class _QueueFeed {
  _QueueFeed(this._pages);

  final List<List<QueueLead>> _pages;
  int reads = 0;

  List<QueueLead> next() {
    final page = _pages[reads.clamp(0, _pages.length - 1)];
    reads++;
    return page;
  }
}

/// Quiet fakes for every source the panel reads, so no bare-Dio request timer
/// leaks out of a provider nobody stubbed.
Widget _scope({
  required Widget child,
  List<QueueLead> queue = _queue,
  _QueueFeed? feed,
  String role = 'admin',
}) =>
    ProviderScope(
      overrides: [
        approvalQueueProvider.overrideWith(
            (ref) async => _asPage(feed == null ? queue : feed.next())),
        funnelsListProvider.overrideWith(_NoFunnels.new),
        radarProvider.overrideWith(
            (ref) async => const RadarFeed(items: [], warnings: [])),
        contractCalendarProvider
            .overrideWith((ref) async => const ContractCalendar()),
        signalWatchesProvider
            .overrideWith((ref) async => const <SignalWatch>[]),
        offersOutcomeProvider.overrideWith((ref) async =>
            const OffersOutcome(sent: 0, replies: 0, proposals: 0)),
        productsListProvider.overrideWith(_NoProducts.new),
        authControllerProvider.overrideWith(() => _FakeAuth(role)),
        dioProvider.overrideWithValue(Dio()),
      ],
      child: child,
    );

Future<void> _pump(
  WidgetTester tester, {
  String? initialTab,
  List<QueueLead> queue = _queue,
  _QueueFeed? feed,
  String role = 'admin',
}) async {
  tester.view.physicalSize = const Size(1000, 1400);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(_scope(
    queue: queue,
    feed: feed,
    role: role,
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      home: ProspectBranchScreen(initialTab: initialTab),
    ),
  ));
  await tester.pumpAndSettle();
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  // Ofertas stopped being four tabs. A seller who has to remember WHICH list
  // holds the good rows works one list and lets the others rot — and the stale
  // queue of half-sent outreach this screen exists to prevent was being
  // produced by the screen's own shape.

  testWidgets('lands on the panel, with no tabs to choose between',
      (tester) async {
    await _pump(tester);

    expect(find.byType(RadarBody), findsOneWidget);
    expect(find.byKey(const Key('prospect_tabs')), findsNothing);
    for (final name in ['queue', 'signals', 'calendar', 'hunt']) {
      expect(find.byKey(Key('prospect_tab_$name')), findsNothing,
          reason: 'the $name tab should be a row in the panel now');
    }
  });

  // These links live in dashboard nudges, e-mails and bookmarks nobody will
  // re-send. A dead link is a worse answer than an old screen.
  testWidgets('?tab=queue still opens the backlog', (tester) async {
    await _pump(tester, initialTab: 'queue');

    expect(find.byKey(const Key('approval_queue_card_lead1')), findsOneWidget);
  });

  testWidgets('?tab=calendar still opens the calendar', (tester) async {
    await _pump(tester, initialTab: 'calendar');

    expect(find.byType(ContractCalendarBody), findsOneWidget);
  });

  // Their content is IN the panel now, so the link resolves to it rather than
  // to a screen that no longer exists.
  testWidgets('?tab=signals and ?tab=hunt resolve to the panel',
      (tester) async {
    for (final tab in ['signals', 'hunt']) {
      await _pump(tester, initialTab: tab);
      expect(find.byType(RadarBody), findsOneWidget, reason: 'for ?tab=$tab');
    }
  });

  // A deep link that opens an old screen with no way out is a trap.
  testWidgets('a legacy view offers the way back; the panel does not',
      (tester) async {
    await _pump(tester, initialTab: 'queue');
    expect(find.byKey(const Key('prospect_back_to_panel')), findsOneWidget);

    await _pump(tester);
    expect(find.byKey(const Key('prospect_back_to_panel')), findsNothing,
        reason: 'nothing to go back to when you are already there');
  });

  // The queue and the calendar 403 below vendedor server-side. Rendering them
  // anyway parked whoever followed the link on a RetryableError whose Retry
  // could never succeed.
  testWidgets('an attendant following a queue link lands on the panel',
      (tester) async {
    await _pump(tester, role: 'attendant', initialTab: 'queue');

    expect(find.byType(RadarBody), findsOneWidget);
    expect(find.byKey(const Key('approval_queue_card_lead1')), findsNothing);
  });

  testWidgets('a vendedor may still follow both legacy links', (tester) async {
    await _pump(tester, role: 'vendedor', initialTab: 'queue');
    expect(find.byKey(const Key('approval_queue_card_lead1')), findsOneWidget);

    await _pump(tester, role: 'vendedor', initialTab: 'calendar');
    expect(find.byType(ContractCalendarBody), findsOneWidget);
  });

  // GoRouter keys a branch page by PATH, so a warm `?tab=` navigation never
  // re-runs the route builder — the briefing's "Revisar" button landed on
  // whatever the operator left the screen on. The URL must win, and reading it
  // in build() is what makes that true without a TabController to nudge.
  testWidgets('a warm ?tab= navigation still follows the URL', (tester) async {
    tester.view.physicalSize = const Size(1000, 1400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    // The panel does not read the queue, so the FIRST read happens on the way
    // in and the second on the way back in — which is the property worth
    // pinning: a queue re-opened must not show what it held last time.
    final feed = _QueueFeed([_queue, _queue]);
    final router = GoRouter(
      initialLocation: '/prospect',
      routes: [
        GoRoute(
          path: '/prospect',
          builder: (_, state) => ProspectBranchScreen(
            initialTab: state.uri.queryParameters['tab'],
          ),
        ),
      ],
    );
    await tester.pumpWidget(_scope(
      feed: feed,
      child:
          MaterialApp.router(theme: AppTheme.darkTheme, routerConfig: router),
    ));
    await tester.pumpAndSettle();
    expect(find.byType(RadarBody), findsOneWidget);

    router.go('/prospect?tab=queue');
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('approval_queue_card_lead1')), findsOneWidget,
        reason: 'the warm navigation did not reach the queue');

    final afterFirstVisit = feed.reads;
    expect(afterFirstVisit, 1, reason: 'the panel must not read the queue');

    await tester.tap(find.byKey(const Key('prospect_back_to_panel')));
    await tester.pumpAndSettle();
    expect(find.byType(RadarBody), findsOneWidget);

    // Re-opening re-reads. This screen's whole job is to stop drafts going
    // stale, so it cannot be the screen that shows a stale list — and the
    // autoDispose provider is what makes that true with no bookkeeping, now
    // that a TabBarView no longer keeps the queue mounted beside the panel.
    router.go('/prospect?tab=queue');
    await tester.pumpAndSettle();

    expect(feed.reads, greaterThan(afterFirstVisit),
        reason: 'the queue served a cached page on re-open');
  });
}
