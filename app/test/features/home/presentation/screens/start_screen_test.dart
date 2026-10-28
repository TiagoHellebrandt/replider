import 'dart:async';
import 'dart:typed_data';

import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/design_system/components/retryable_error.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/home/domain/models/onboarding_progress_model.dart';
import 'package:app/features/home/presentation/controllers/command_center_controller.dart';
import 'package:app/features/home/presentation/controllers/onboarding_progress_controller.dart';
import 'package:app/features/home/presentation/screens/start_screen.dart';
import 'package:app/features/sales/presentation/controllers/approval_queue_controller.dart';
import 'package:app/features/sales/presentation/controllers/radar_controller.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

// The morning briefing (canvas "Início — Briefing").
//
// One question: what needs me right now? The tests pin that the screen says
// only what is true — the greeting summarizes the loaded payload, the server's
// ranking survives the partition into groups, every row names a customer, a
// section that failed to load says so, and the right rail renders nothing it
// cannot back with data.

PendingAction _action({
  PendingActionKind kind = PendingActionKind.signatureViewed,
  String targetId = 'a1',
  String customerName = 'Clínica Bem Viver',
  int moneyCents = 0,
  DateTime? dueOn,
  DateTime? sinceAt,
  String deepLink = '/settings/signatures',
}) =>
    PendingAction(
      kind: kind,
      targetId: targetId,
      deepLink: deepLink,
      customerName: customerName,
      moneyAtStakeCents: moneyCents,
      currency: 'brl',
      dueOn: dueOn,
      sinceAt: sinceAt,
    );

CommandCenter _center({
  CommandCenterTiles tiles = const CommandCenterTiles(
      activeLeads: 14,
      pendingApproval: 3,
      meetings: 2,
      wonGrossCents: 5000000,
      currency: 'brl',
      periodLabel: '2026-07'),
  List<PendingAction> actions = const [],
  List<CommandCenterStage> funnel = const [
    CommandCenterStage(status: 'new', label: 'Novos', count: 10),
    CommandCenterStage(status: 'contacted', label: 'Abordados', count: 4),
    CommandCenterStage(status: 'won', label: 'Ganhos', count: 1),
  ],
  int? totalActions,
  CommandCenterSuggestion? suggestion,
  OvernightBriefing? briefing,
  bool empty = false,
  List<String> warnings = const [],
}) =>
    CommandCenter(
      tiles: tiles,
      actions: actions,
      funnel: funnel,
      totalActions: totalActions ?? actions.length,
      suggestion: suggestion,
      briefing: briefing,
      empty: empty,
      warnings: warnings,
    );

GoRouter _router() => GoRouter(
      initialLocation: '/start',
      routes: [
        GoRoute(path: '/start', builder: (_, __) => const StartScreen()),
        GoRoute(
            path: '/prospect',
            builder: (_, __) => const Scaffold(body: Text('prospect'))),
        GoRoute(
            path: '/funnel',
            builder: (_, __) => const Scaffold(body: Text('funnel'))),
        GoRoute(
            path: '/home',
            builder: (_, __) => const Scaffold(body: Text('home'))),
        GoRoute(
            path: '/copilot',
            builder: (_, __) => const Scaffold(body: Text('copilot'))),
        GoRoute(
            path: '/sales/signals',
            builder: (_, __) => const Scaffold(body: Text('signals'))),
        GoRoute(
            path: '/settings/signatures',
            builder: (_, __) => const Scaffold(body: Text('signatures'))),
      ],
    );

class _FakeGestor extends AuthController {
  @override
  Future<UserModel?> build() async => const UserModel(
        id: 'u1',
        name: 'Marina Costa',
        email: 'marina@replider.com',
        token: 'tok',
        role: 'gestor',
      );
}

class _FakeVendedor extends AuthController {
  @override
  Future<UserModel?> build() async => const UserModel(
        id: 'u4',
        name: 'Rafa',
        email: 'rafa@replider.com',
        token: 'tok',
        role: 'vendedor',
      );
}

class _NoProgress extends OnboardingProgressController {
  @override
  Future<OnboardingProgressModel?> build() async => null;
}

class _WithProgress extends OnboardingProgressController {
  _WithProgress(this.model);
  final OnboardingProgressModel model;

  @override
  Future<OnboardingProgressModel?> build() async => model;
}

OnboardingProgressModel _progress({int done = 1}) => OnboardingProgressModel(
      agentCreated: false,
      previewChatTested: false,
      channelConnected: done >= 1,
      knowledgeAdded: false,
      productsAdded: done >= 2,
      customersAdded: done >= 3,
      proposalMade: done >= 4,
      firstInboundReceived: done >= 5,
      teamInvited: done >= 6,
      onboardingCompleted: false,
      completionPercentage: (done * 100 / 6).round(),
    );

RadarFeed _feed(int count) => RadarFeed(
      items: [
        for (var i = 0; i < count; i++)
          RadarItem(
            type: RadarItemType.values[i % RadarItemType.values.length],
            id: 'r$i',
            title: 'Hospital $i',
            context: 'contexto $i',
            deadline:
                i == 0 ? DateTime.now().add(const Duration(days: 3)) : null,
          ),
      ],
      warnings: const [],
    );

/// Answers every request 204 and records it, so tapping a real action does not
/// reach out — and the test can assert what would have been sent.
class _RecordingAdapter implements HttpClientAdapter {
  final requests = <RequestOptions>[];

  @override
  Future<ResponseBody> fetch(
      RequestOptions options, Stream<Uint8List>? _, Future<void>? __) async {
    requests.add(options);
    return ResponseBody.fromString('', 204);
  }

  @override
  void close({bool force = false}) {}
}

Future<void> _pump(
  WidgetTester tester, {
  CommandCenter? data,
  Object? error,
  bool loading = false,
  Dio? dio,
  Size viewport = const Size(1200, 3000),
  RadarFeed? radar,
  OffersOutcome? outcome,
  OnboardingProgressModel? progress,
  AuthController Function()? auth,
}) async {
  tester.view.physicalSize = viewport;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(ProviderScope(
    retry: (_, __) => null,
    overrides: [
      dioProvider.overrideWithValue(dio ?? Dio()),
      authControllerProvider.overrideWith(auth ?? _FakeGestor.new),
      // The rail cards render NOTHING they cannot back with data — the
      // defaults here keep them out of tests that are not about them.
      radarProvider.overrideWith((ref) =>
          Future.value(radar ?? const RadarFeed(items: [], warnings: []))),
      offersOutcomeProvider.overrideWith((ref) => outcome != null
          ? Future.value(outcome)
          : Future<OffersOutcome>.error(Exception('down'))),
      progress != null
          ? onboardingProgressControllerProvider
              .overrideWith(() => _WithProgress(progress))
          : onboardingProgressControllerProvider.overrideWith(_NoProgress.new),
      commandCenterProvider.overrideWith((ref) {
        if (loading) return Completer<CommandCenter>().future;
        if (error != null) return Future<CommandCenter>.error(error);
        return Future.value(data ?? _center());
      }),
    ],
    child: MaterialApp.router(
      theme: AppTheme.darkTheme,
      routerConfig: _router(),
    ),
  ));
  if (loading) {
    await tester.pump();
  } else {
    await tester.pumpAndSettle();
  }
}

String _textOf(WidgetTester tester, String key) =>
    tester.widget<Text>(find.byKey(Key(key))).data!;

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  testWidgets('loading shows the screen\'s own shape', (tester) async {
    await _pump(tester, loading: true);

    expect(find.byKey(const Key('start_skeleton')), findsOneWidget);
  });

  testWidgets('an error offers a retry', (tester) async {
    await _pump(tester, error: Exception('boom'));

    expect(find.byType(RetryableError), findsOneWidget);
  });

  // ── Header ───────────────────────────────────────────────────────────────

  testWidgets('the greeting names the person and matches the hour',
      (tester) async {
    await _pump(tester);

    final h = DateTime.now().hour;
    final expected = h < 12
        ? t.start.greeting_morning(name: 'Marina')
        : h < 18
            ? t.start.greeting_afternoon(name: 'Marina')
            : t.start.greeting_evening(name: 'Marina');
    expect(_textOf(tester, 'start_greeting'), expected);
  });

  testWidgets('the summary is derived from the loaded payload', (tester) async {
    await _pump(tester,
        data: _center(actions: [
          _action(kind: PendingActionKind.waitingReply, targetId: 'w1'),
          _action(
              kind: PendingActionKind.contractAlert,
              targetId: 'c1',
              dueOn: DateTime.now().subtract(const Duration(days: 1))),
        ]));

    // pendingApproval=3 from the default tiles, one waiting reply, one due.
    expect(find.text(t.start.summary_ready(count: 3)), findsOneWidget);
    expect(find.text(t.start.summary_waiting(count: 1)), findsOneWidget);
    expect(find.text(t.start.summary_due_today(count: 1)), findsOneWidget);
    expect(find.text(t.start.summary_due_none), findsNothing);
  });

  testWidgets('nothing due today says so in gray, not with a zero',
      (tester) async {
    await _pump(tester);

    expect(find.text(t.start.summary_due_none), findsOneWidget);
    expect(find.text(t.start.summary_waiting(count: 0)), findsNothing);
  });

  // ── Stat strip ───────────────────────────────────────────────────────────

  testWidgets('the strip is counted and the money cells name their period',
      (tester) async {
    await _pump(tester);

    // The strip carries an ACTION and a scoreboard. "Leads ativos" was cut:
    // it names no action, the board is one tap below and the mini-funnel
    // already breaks the same number down by stage.
    expect(find.byKey(const Key('start_tile_active')), findsNothing);
    expect(_textOf(tester, 'start_tile_pending'), '3');
    expect(_textOf(tester, 'start_tile_meetings'), '2');
    expect(_textOf(tester, 'start_tile_won'), 'R\$ 50.000,00');
    // "Ganhos" with no window is a number nobody can check — and "2026-07"
    // is how a database says July, not how a person reads a footnote.
    final july = t.command_center
        .month_of_year(month: t.command_center.months[6], year: '2026');
    expect(_textOf(tester, 'start_tile_won_period'), july);
    expect(_textOf(tester, 'start_tile_meetings_period'), july);

    // "Nunca abordados" was cut from the strip entirely: the Radar's own
    // banner already says "N hospitais em MT declaram equipamento que seu
    // catálogo atende", in the place where you can act on it. The tile was a
    // louder duplicate of a sentence that reads better in context.
    expect(find.byKey(const Key('start_tile_untouched')), findsNothing);
  });

  // Walking the product as a salesperson, "2790 nunca abordados" is the size
  // of Mato Grosso: a number nobody can act on in three seconds, sitting in
  // the loudest place on the screen. It says the same thing the Radar banner
  // says, one tap away, where the hospitals it counts are listed.
  testWidgets('the size of the ground left is not a headline number',
      (tester) async {
    await _pump(
      tester,
      data: _center(
        tiles: const CommandCenterTiles(
          activeLeads: 1,
          untouchedTargets: 480,
          periodLabel: '2026-07',
        ),
      ),
    );

    expect(find.byKey(const Key('start_tile_untouched')), findsNothing);
    // The hospitals themselves still reach the screen — as named rows in
    // "Precisa de você", which IS a phone call, not a statistic.
    expect(find.byKey(const Key('start_tile_pending')), findsOneWidget);
  });

  // A meeting on the calendar is an action. Zero of them is not.
  testWidgets('no meetings, no meetings tile', (tester) async {
    await _pump(tester,
        data: _center(
            tiles: const CommandCenterTiles(
                pendingApproval: 3, meetings: 0, periodLabel: '2026-07')));

    expect(find.byKey(const Key('start_tile_meetings')), findsNothing);
  });

  testWidgets('a meeting on the calendar earns its tile', (tester) async {
    await _pump(tester,
        data: _center(
            tiles: const CommandCenterTiles(
                pendingApproval: 3, meetings: 2, periodLabel: '2026-07')));

    expect(_textOf(tester, 'start_tile_meetings'), '2');
  });

  // Zero offers waiting is worth reading — "nothing is waiting on you" is an
  // answer. This one stays at zero where the others hide.
  testWidgets('the pending tile stays at zero, because zero is an answer',
      (tester) async {
    await _pump(tester,
        data: _center(
            tiles: const CommandCenterTiles(
                pendingApproval: 0, periodLabel: '2026-07')));
    expect(_textOf(tester, 'start_tile_pending'), '0');
  });

  // The row is a phone call, not a statistic: it names the hospital. And it
  // ranks below a customer who is already talking — the server orders, the
  // screen never re-sorts, so this pins the payload order surviving.
  testWidgets('an untouched hospital is named and sits below a live customer',
      (tester) async {
    await _pump(
      tester,
      data: _center(actions: [
        _action(
          kind: PendingActionKind.waitingReply,
          targetId: 'conv1',
          customerName: 'Clínica Bem Viver',
        ),
        _action(
          kind: PendingActionKind.untouchedTarget,
          targetId: 'cnes:111',
          customerName: 'FEMINA HOSPITAL INFANTIL',
        ),
      ]),
    );

    expect(find.text('FEMINA HOSPITAL INFANTIL'), findsOneWidget);

    final live = tester.getTopLeft(find.text('Clínica Bem Viver')).dy;
    final cold = tester.getTopLeft(find.text('FEMINA HOSPITAL INFANTIL')).dy;
    expect(cold, greaterThan(live),
        reason: 'ground nobody promised anything to outranked a live customer');
  });

  testWidgets('an unexpected period is shown as it came', (tester) async {
    await _pump(tester,
        data: _center(
            tiles: const CommandCenterTiles(
                activeLeads: 1,
                pendingApproval: 0,
                meetings: 0,
                wonGrossCents: 100,
                currency: 'brl',
                periodLabel: 'ontem')));

    expect(_textOf(tester, 'start_tile_won_period'), 'ontem');
  });

  testWidgets('every cell leads to where its number becomes work',
      (tester) async {
    for (final (key, landing) in [
      ('start_tile_pending_open', 'prospect'),
      ('start_tile_meetings_open', 'funnel'),
      ('start_tile_won_open', 'funnel'),
    ]) {
      await _pump(tester);
      await tester.tap(find.byKey(Key(key)));
      await tester.pumpAndSettle();

      expect(find.text(landing), findsOneWidget,
          reason: '$key foi para lugar errado');
    }
  });

  testWidgets('zero ganhos shows the way in, not a bold zero', (tester) async {
    await _pump(tester,
        data: _center(
            tiles: const CommandCenterTiles(
                activeLeads: 1,
                pendingApproval: 0,
                meetings: 0,
                wonGrossCents: 0,
                currency: 'brl',
                periodLabel: '2026-07')));

    expect(_textOf(tester, 'start_register_sale'), t.start.register_sale_link);
    expect(find.byKey(const Key('start_tile_won_period')), findsNothing);
  });

  // The unowned footnote went with the "leads ativos" tile it hung off. The
  // leads themselves are not lost: an unowned lead still reaches the screen
  // as a named row in "Precisa de você" — below the offers, which is where a
  // chore belongs.
  testWidgets('unowned leads still reach the screen, as named rows',
      (tester) async {
    await _pump(tester,
        data: _center(actions: [
          _action(kind: PendingActionKind.unownedLead, targetId: 'u1'),
          _action(kind: PendingActionKind.unownedLead, targetId: 'u2'),
        ]));

    expect(find.byKey(const Key('start_tile_active_unowned')), findsNothing);
    expect(find.byKey(const Key('start_action_u1')), findsOneWidget);
  });

  // ── Precisa de você ──────────────────────────────────────────────────────

  // The rule the screen rests on: the server ranks, the client renders. The
  // partition into groups must keep payload order inside a group AND order the
  // groups by the rank of their first item.
  testWidgets('groups follow the server order and never re-sort within',
      (tester) async {
    await _pump(tester,
        data: _center(actions: [
          _action(
              kind: PendingActionKind.approveDraft,
              targetId: 'd-old',
              moneyCents: 500000),
          _action(kind: PendingActionKind.waitingReply, targetId: 'w1'),
          _action(
              kind: PendingActionKind.approveDraft,
              targetId: 'd-big',
              moneyCents: 10000000),
        ]));

    final yOld =
        tester.getTopLeft(find.byKey(const Key('start_action_d-old'))).dy;
    final yBig =
        tester.getTopLeft(find.byKey(const Key('start_action_d-big'))).dy;
    final yW1 = tester.getTopLeft(find.byKey(const Key('start_action_w1'))).dy;

    // d-old came first in the payload, so it stays first — a client that
    // re-sorted by amount would put d-big on top and disagree with the
    // copilot. And w1's group starts where the server ranked its first item:
    // after the drafts.
    expect(yOld, lessThan(yBig));
    expect(yBig, lessThan(yW1));
  });

  testWidgets('chips filter the loaded list, never a new request',
      (tester) async {
    await _pump(tester,
        data: _center(actions: [
          _action(kind: PendingActionKind.approveDraft, targetId: 'd1'),
          _action(kind: PendingActionKind.waitingReply, targetId: 'w1'),
        ]));

    expect(find.byKey(const Key('start_chip_all')), findsOneWidget);
    await tester.tap(find.byKey(const Key('start_chip_waiting_reply')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('start_action_w1')), findsOneWidget);
    expect(find.byKey(const Key('start_action_d1')), findsNothing);

    await tester.tap(find.byKey(const Key('start_chip_all')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('start_action_d1')), findsOneWidget);
  });

  testWidgets('a row names the customer and shows what it is worth',
      (tester) async {
    await _pump(tester,
        data: _center(actions: [_action(moneyCents: 10000000)]));

    // "3 assinaturas pendentes" is a statistic; a name is a phone call.
    expect(find.text('Clínica Bem Viver'), findsOneWidget);
    expect(_textOf(tester, 'start_action_money_a1'), 'R\$ 100.000,00');
  });

  testWidgets('a row with no amount shows no money, not zero', (tester) async {
    await _pump(tester,
        data: _center(actions: [
          _action(kind: PendingActionKind.approveDraft, moneyCents: 0),
        ]));

    // Zero means "this action carries no amount", not "it is worth nothing".
    expect(find.byKey(const Key('start_action_money_a1')), findsNothing);
  });

  testWidgets('the age chip says how late an overdue deadline is',
      (tester) async {
    final due = DateTime.now().subtract(const Duration(days: 3));
    await _pump(tester,
        data: _center(actions: [
          _action(kind: PendingActionKind.contractAlert, dueOn: due),
        ]));

    expect(_textOf(tester, 'start_action_age_a1'),
        t.command_center.overdue_days(count: 3));
  });

  testWidgets('the age chip says how long a row has waited', (tester) async {
    await _pump(tester,
        data: _center(actions: [
          _action(
              kind: PendingActionKind.approveDraft,
              sinceAt: DateTime.now().subtract(const Duration(days: 4))),
        ]));

    expect(_textOf(tester, 'start_action_age_a1'),
        t.command_center.waiting_days(count: 4));
  });

  testWidgets('tapping a row goes where the work happens', (tester) async {
    await _pump(tester,
        data: _center(actions: [_action(deepLink: '/settings/signatures')]));

    await tester.tap(find.byKey(const Key('start_action_a1')));
    await tester.pumpAndSettle();

    expect(find.text('signatures'), findsOneWidget);
  });

  testWidgets('a waiting customer gets the loud pill, and it goes there',
      (tester) async {
    await _pump(tester,
        data: _center(actions: [
          _action(kind: PendingActionKind.waitingReply, deepLink: '/home'),
        ]));

    expect(find.text(t.start.action_reply), findsOneWidget);
    await tester.tap(find.byKey(const Key('start_action_go_a1')));
    await tester.pumpAndSettle();
    expect(find.text('home'), findsOneWidget);
  });

  // "Assumir" only for who the server lets in: PATCH /leads/:id/owner is
  // RequireRole(admin/owner/gestor). Below that, the verb is "open".
  testWidgets('a gestor can take an unowned lead in one tap', (tester) async {
    final adapter = _RecordingAdapter();
    final dio = Dio()..httpClientAdapter = adapter;
    await _pump(tester,
        dio: dio,
        data: _center(actions: [
          _action(kind: PendingActionKind.unownedLead, targetId: 'lead-9'),
        ]));

    await tester.tap(find.byKey(const Key('start_action_go_lead-9')));
    await tester.pumpAndSettle();

    final req =
        adapter.requests.where((r) => r.path == '/leads/lead-9/owner').single;
    expect(req.method, 'PATCH');
    expect((req.data as Map<String, dynamic>)['owner_user_id'], 'u1');
  });

  testWidgets('a vendedor sees "abrir" on an unowned lead, not "assumir"',
      (tester) async {
    await _pump(tester,
        auth: _FakeVendedor.new,
        data: _center(actions: [
          _action(kind: PendingActionKind.unownedLead, targetId: 'lead-9'),
        ]));

    expect(find.text(t.start.action_assume), findsNothing);
    expect(find.text(t.shell.needs_open), findsOneWidget);
  });

  testWidgets('the unowned group header carries the distribute action',
      (tester) async {
    await _pump(tester,
        data: _center(actions: [
          _action(kind: PendingActionKind.unownedLead, targetId: 'u1'),
          _action(kind: PendingActionKind.unownedLead, targetId: 'u2'),
        ]));

    expect(find.byKey(const Key('start_distribute_unowned')), findsOneWidget);
    expect(find.text(t.start.distribute_team), findsOneWidget);
  });

  testWidgets('a vendedor gets no distribute action — the server would 403',
      (tester) async {
    await _pump(tester,
        auth: _FakeVendedor.new,
        data: _center(actions: [
          _action(kind: PendingActionKind.unownedLead, targetId: 'u1'),
        ]));

    expect(find.byKey(const Key('start_distribute_unowned')), findsNothing);
  });

  // "Volta em alguns dias" was vague about a number the client itself picks:
  // it sends snooze_hours=72.
  testWidgets('snoozing says exactly how long, and sends that same number',
      (tester) async {
    final adapter = _RecordingAdapter();
    final dio = Dio()..httpClientAdapter = adapter;
    await _pump(tester,
        dio: dio, data: _center(actions: [_action(targetId: 'lead-1')]));

    await tester.tap(find.byKey(const Key('start_action_snooze_lead-1')));
    await tester.pumpAndSettle();

    expect(
        (adapter.requests.single.data as Map<String, dynamic>)['snooze_hours'],
        CommandCenterActions.snoozeHours);
    expect(
        find.text(t.command_center
            .snoozed(days: '${CommandCenterActions.snoozeHours ~/ 24}')),
        findsOneWidget);
  });

  // Six rows with no total would read as "that is all of it".
  testWidgets('a truncated list says how many are hidden', (tester) async {
    await _pump(tester,
        data: _center(
          actions: [_action(), _action(targetId: 'a2')],
          totalActions: 9,
        ));

    expect(_textOf(tester, 'start_more_actions'),
        t.command_center.more_actions(count: 7));
  });

  // An empty list is a CLAIM.
  testWidgets('a section that failed to load is disclosed', (tester) async {
    await _pump(tester, data: _center(warnings: const ['contracts']));

    expect(find.byKey(const Key('start_partial')), findsOneWidget);
  });

  testWidgets('nothing waiting says so plainly', (tester) async {
    await _pump(tester, data: _center());

    expect(find.byKey(const Key('start_needs_clear')), findsOneWidget);
    expect(find.byKey(const Key('start_partial')), findsNothing);
  });

  // ── Copilot chip ─────────────────────────────────────────────────────────

  testWidgets('exactly one copilot chip, and it carries its prompt',
      (tester) async {
    await _pump(tester,
        data: _center(
          actions: [_action(moneyCents: 100)],
          suggestion: const CommandCenterSuggestion(
              label: 'Retomar Clínica Bem Viver',
              prompt: 'Sugira uma mensagem de retomada'),
        ));

    // One, never a list: three suggestions means all three get ignored.
    expect(find.byKey(const Key('start_suggestion')), findsOneWidget);

    await tester.tap(find.byKey(const Key('start_suggestion')));
    await tester.pumpAndSettle();
    expect(find.text('copilot'), findsOneWidget);
  });

  testWidgets('no suggestion means no chip at all', (tester) async {
    await _pump(tester, data: _center(actions: [_action()]));

    expect(find.byKey(const Key('start_suggestion')), findsNothing);
  });

  // ── Mini-funnel ──────────────────────────────────────────────────────────

  testWidgets('the mini-funnel shows the shape and opens the board',
      (tester) async {
    await _pump(tester);

    expect(_textOf(tester, 'start_funnel_new'), '10');
    expect(_textOf(tester, 'start_funnel_won'), '1');

    await tester.tap(find.byKey(const Key('start_mini_funnel')));
    await tester.pumpAndSettle();
    expect(find.text('funnel'), findsOneWidget);
  });

  // "Needs you" is the question the screen is named after; the scoreboard
  // (mini-funnel) stays below it.
  testWidgets('work comes before the scoreboard', (tester) async {
    await _pump(tester);

    final needsYou = tester.getTopLeft(find.text(t.shell.needs_you_title)).dy;
    final funnel =
        tester.getTopLeft(find.byKey(const Key('start_mini_funnel'))).dy;

    expect(needsYou, lessThan(funnel));
  });

  // ── Activation ───────────────────────────────────────────────────────────

  testWidgets('an empty account gets something to do, not a wall of zeros',
      (tester) async {
    await _pump(tester, data: _center(empty: true));

    expect(find.byKey(const Key('start_activation')), findsOneWidget);
    // And no strip: "0 · 0 · 0 · R$ 0" teaches a new user that the product
    // does nothing.
    expect(find.byKey(const Key('start_tile_active')), findsNothing);

    await tester.tap(find.byKey(const Key('start_activation_prospect')));
    await tester.pumpAndSettle();
    expect(find.text('prospect'), findsOneWidget);
  });

  // Both CTAs open the Radar, whose tab (and server routes) admit attendant
  // and above; for a vendedor they fell through to her queue — a button that
  // goes somewhere else than it says.
  testWidgets('a vendedor gets no Radar buttons on the activation card',
      (tester) async {
    await _pump(tester, auth: _FakeVendedor.new, data: _center(empty: true));

    expect(find.byKey(const Key('start_activation')), findsOneWidget);
    expect(find.byKey(const Key('start_activation_prospect')), findsNothing);
    expect(find.byKey(const Key('start_activation_signals')), findsNothing);
  });

  // ── Overnight hero (K2) ──────────────────────────────────────────────────

  testWidgets('the overnight briefing leads the screen', (tester) async {
    await _pump(tester,
        data: _center(
          briefing: const OvernightBriefing(id: 'run-1', drafted: 40),
        ));

    expect(find.byKey(const Key('start_briefing')), findsOneWidget);
    expect(find.text(t.overnight.drafted(count: 40)), findsOneWidget);
    // A full night needs no explanation.
    expect(find.byKey(const Key('start_briefing_budget')), findsNothing);
  });

  testWidgets('a night the allowance cut short explains the missing drafts',
      (tester) async {
    await _pump(tester,
        data: _center(
          briefing: const OvernightBriefing(
              id: 'run-1', drafted: 40, skippedBudget: 60),
        ));

    // "40 prontas" when 100 were asked for is a number that does not add up
    // without this line.
    expect(find.text(t.overnight.budget_hit(count: 60)), findsOneWidget);
  });

  testWidgets('no briefing renders no card', (tester) async {
    await _pump(tester, data: _center());

    // Most mornings there was nothing to draft, and a daily "0 prontas" is how
    // a good habit becomes a dismissed banner.
    expect(find.byKey(const Key('start_briefing')), findsNothing);
  });

  testWidgets('the hero pills review the queue and open the Radar',
      (tester) async {
    final adapter = _RecordingAdapter();
    for (final (key, landing) in [
      ('start_briefing_review', 'prospect'),
      ('start_briefing_radar', 'prospect'),
    ]) {
      await _pump(tester,
          dio: Dio()..httpClientAdapter = adapter,
          data: _center(
            briefing: const OvernightBriefing(id: 'run-1', drafted: 12),
          ));

      await tester.tap(find.byKey(Key(key)));
      await tester.pumpAndSettle();
      expect(find.text(landing), findsOneWidget,
          reason: '$key foi para lugar errado');
    }
    // Both taps acknowledged the briefing — reviewing IS reading it.
    expect(
        adapter.requests
            .where((r) => r.path.contains('/me/overnight-drafting/run-1/seen'))
            .length,
        2);
  });

  testWidgets('a vendedor gets no "Ver Radar" pill on the hero',
      (tester) async {
    await _pump(tester,
        auth: _FakeVendedor.new,
        data: _center(
          briefing: const OvernightBriefing(id: 'run-1', drafted: 12),
        ));

    expect(find.byKey(const Key('start_briefing_review')), findsOneWidget);
    expect(find.byKey(const Key('start_briefing_radar')), findsNothing);
  });

  // ── Right rail ───────────────────────────────────────────────────────────

  testWidgets('the wide screen gets the rail: radar, week and engine',
      (tester) async {
    await _pump(tester,
        viewport: const Size(1600, 1200),
        radar: _feed(4),
        outcome: const OffersOutcome(sent: 12, replies: 3, proposals: 1),
        progress: _progress(done: 2));

    expect(find.byKey(const Key('start_rail')), findsOneWidget);
    // Radar: first 3 of the feed, never more.
    expect(find.byKey(const Key('start_rail_radar')), findsOneWidget);
    expect(find.byKey(const Key('start_rail_radar_r0')), findsOneWidget);
    expect(find.byKey(const Key('start_rail_radar_r2')), findsOneWidget);
    expect(find.byKey(const Key('start_rail_radar_r3')), findsNothing);
    // Week: the outcome numbers.
    expect(_textOf(tester, 'start_week_sent'), '12');
    expect(_textOf(tester, 'start_week_replies'), '3');
    expect(_textOf(tester, 'start_week_proposals'), '1');
    // Engine: "2 de 6".
    expect(_textOf(tester, 'start_engine_count'),
        t.start.of_total(current: '2', total: '6'));
  });

  testWidgets('an empty or failed radar feed renders no card at all',
      (tester) async {
    await _pump(tester, viewport: const Size(1600, 1200));

    // Empty feed + errored outcome + no progress: an empty card would be a
    // claim the data has not earned.
    expect(find.byKey(const Key('start_rail_radar')), findsNothing);
    expect(find.byKey(const Key('start_rail_week')), findsNothing);
    expect(find.byKey(const Key('start_rail_engine')), findsNothing);
  });

  testWidgets('a vendedor never fetches the attendant-gated radar',
      (tester) async {
    await _pump(tester,
        viewport: const Size(1600, 1200),
        auth: _FakeVendedor.new,
        radar: _feed(3),
        outcome: const OffersOutcome(sent: 1, replies: 0, proposals: 0));

    // The feed HAS items; the card still does not render for a role the
    // server's tab would refuse.
    expect(find.byKey(const Key('start_rail_radar')), findsNothing);
    expect(find.byKey(const Key('start_rail_week')), findsOneWidget);
  });

  testWidgets('below 1180 the rail folds under the main column',
      (tester) async {
    await _pump(tester,
        viewport: const Size(1000, 2600),
        radar: _feed(3),
        outcome: const OffersOutcome(sent: 12, replies: 3, proposals: 1),
        progress: _progress(done: 2));

    expect(find.byKey(const Key('start_rail')), findsNothing);
    // Stacked: engine first, radar and week still present at this width.
    expect(find.byKey(const Key('start_rail_engine')), findsOneWidget);
    expect(find.byKey(const Key('start_rail_radar')), findsOneWidget);
    expect(find.byKey(const Key('start_rail_week')), findsOneWidget);
    final engine =
        tester.getTopLeft(find.byKey(const Key('start_rail_engine'))).dy;
    final radar =
        tester.getTopLeft(find.byKey(const Key('start_rail_radar'))).dy;
    expect(engine, lessThan(radar));
  });

  testWidgets('below 900 only the engine card survives the fold',
      (tester) async {
    await _pump(tester,
        viewport: const Size(800, 2600),
        radar: _feed(3),
        outcome: const OffersOutcome(sent: 12, replies: 3, proposals: 1),
        progress: _progress(done: 2));

    expect(find.byKey(const Key('start_rail_engine')), findsOneWidget);
    expect(find.byKey(const Key('start_rail_radar')), findsNothing);
    expect(find.byKey(const Key('start_rail_week')), findsNothing);
  });

  testWidgets('a finished engine renders no card', (tester) async {
    await _pump(tester,
        viewport: const Size(1600, 1200), progress: _progress(done: 6));

    expect(find.byKey(const Key('start_rail_engine')), findsNothing);
  });

  testWidgets('dismissing the engine hides it for the day', (tester) async {
    final adapter = _RecordingAdapter();
    await _pump(tester,
        dio: Dio()..httpClientAdapter = adapter,
        viewport: const Size(1600, 1200),
        progress: _progress(done: 2));

    await tester.tap(find.byKey(const Key('start_engine_dismiss')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('start_rail_engine')), findsNothing);
    // The 24h snooze reaches the server too, so other devices agree.
    expect(
        adapter.requests
            .where((r) => r.path.contains('/me/onboarding-checklist/dismiss'))
            .length,
        1);
  });

  // ── Models ───────────────────────────────────────────────────────────────

  test('a briefing that fitted does not claim it was cut', () {
    expect(const OvernightBriefing(id: 'r', drafted: 40).budgetHit, isFalse);
    expect(
      const OvernightBriefing(id: 'r', drafted: 40, skippedBudget: 1).budgetHit,
      isTrue,
    );
  });

  test('an unknown action kind still renders something tappable', () {
    // A server that grows a seventh kind must not crash the first screen — the
    // deep link comes from the server, so the row stays actionable.
    final action = PendingAction.fromJson(<String, dynamic>{
      'kind': 'something_new',
      'target_id': 'x',
      'deep_link': '/funnel',
    });

    expect(action.kind, PendingActionKind.unknown);
    expect(action.deepLink, '/funnel');
  });

  test('hiddenActions never goes negative', () {
    final center = _center(actions: [_action()], totalActions: 0);

    expect(center.hiddenActions, 0);
  });
}
