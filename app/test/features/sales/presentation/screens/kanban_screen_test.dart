import 'package:app/features/settings/presentation/controllers/team_controller.dart';
import 'package:app/features/settings/domain/models/team_member_model.dart';
import 'package:app/design_system/components/primary_pill_button.dart';
import 'package:app/features/sales/presentation/controllers/funnels_controller.dart';
import 'package:app/features/sales/domain/models/funnel_model.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/sales/domain/models/board_model.dart';
import 'package:app/features/sales/presentation/controllers/board_controller.dart';
import 'package:app/features/sales/presentation/screens/kanban_screen.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:dio/dio.dart';
import 'package:flutter/gestures.dart' show PointerDeviceKind;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

// US-M017/M018. The board's job is to make a pipeline scannable and moveable,
// so the tests pin what a salesperson would notice: the right interaction per
// form factor, the counts, and — most importantly — that a FAILED move does
// not leave the card looking like it worked.

Board _board() => Board(
      stages: const [
        BoardStage(
            id: 's1',
            name: 'Novos',
            position: 100,
            mapsToStatus: 'new',
            isTerminal: false),
        BoardStage(
            id: 's2',
            name: 'Abordados',
            position: 200,
            mapsToStatus: 'contacted',
            isTerminal: false),
        BoardStage(
            id: 's3',
            name: 'Ganhos',
            position: 300,
            mapsToStatus: 'won',
            isTerminal: true),
      ],
      cards: const [
        BoardCard(
          leadId: 'lead1',
          stageId: 's1',
          status: 'new',
          score: 7,
          companyName: 'Hospital São Lucas',
          source: 'speedio',
        ),
        BoardCard(
          leadId: 'lead2',
          stageId: 's1',
          status: 'new',
          score: 0,
          contactLabel: '+5511999990002',
          ownerUserId: 'user-1',
        ),
      ],
    );

/// A funnel big enough that scrolling is not a strategy, with the three
/// signals the header slices on: who replied, who is waiting on a human, and
/// who has been sitting still for weeks.
Board _searchBoard() {
  final now = DateTime.now();
  return Board(
    stages: const [
      BoardStage(
          id: 's1',
          name: 'Novos',
          position: 100,
          mapsToStatus: 'new',
          isTerminal: false),
      BoardStage(
          id: 's2',
          name: 'Abordados',
          position: 200,
          mapsToStatus: 'contacted',
          isTerminal: false),
    ],
    cards: [
      BoardCard(
        leadId: 'vida',
        stageId: 's1',
        status: 'new',
        score: 0,
        companyName: 'Clínica Vida Nova',
        city: 'Sorocaba',
        updatedAt: now.subtract(const Duration(days: 30)),
      ),
      BoardCard(
        leadId: 'aurora',
        stageId: 's1',
        status: 'responded',
        score: 0,
        companyName: 'Clínica Aurora',
        city: 'Itu',
        updatedAt: now.subtract(const Duration(days: 1)),
      ),
      BoardCard(
        leadId: 'sao-lucas',
        stageId: 's1',
        status: 'pending_approval',
        score: 0,
        companyName: 'Hospital São Lucas',
        city: 'Campinas',
        updatedAt: now.subtract(const Duration(days: 3)),
      ),
      BoardCard(
        leadId: 'sem-data',
        stageId: 's2',
        status: 'contacted',
        score: 0,
        companyName: 'Laboratório Central',
        city: 'Sorocaba',
      ),
    ],
  );
}

/// A BoardService whose move either succeeds or throws, so the optimistic
/// path and its rollback are both drivable.
class _FakeService implements BoardService {
  _FakeService(
      {this.fail = false, this.createReturns = true, this.createFails = false});

  final bool fail;

  /// What the server said: true = a new card, false = the phone was
  /// already in this funnel. Mutable so one test can drive a "created"
  /// submission followed by a "already there" one.
  bool createReturns;
  final bool createFails;

  int calls = 0;
  String? lastLeadId;
  String? lastStageId;
  String? lastBeforeId;
  String? lastAfterId;

  int resetCalls = 0;
  String? lastResetFunnelId;
  String? lastResetStageId;

  int createCalls = 0;
  String? lastCreatedName;
  String? lastCreatedCompany;
  String? lastCreatedEmail;
  String? lastCreatedPhone;
  Map<String, String> lastCreatedMetadata = const {};

  List<String> lastBulkIds = const [];
  String? lastBulkOwner;
  int bulkSkipped = 0;

  @override
  Future<({int assigned, int skipped})> bulkAssign({
    required List<String> leadIds,
    required String? ownerUserId,
  }) async {
    lastBulkIds = leadIds;
    lastBulkOwner = ownerUserId;
    return (assigned: leadIds.length - bulkSkipped, skipped: bulkSkipped);
  }

  @override
  Future<bool> createLead({
    required String funnelId,
    required String name,
    required String phone,
    String company = '',
    String role = '',
    String email = '',
    String city = '',
    Map<String, String> metadata = const {},
  }) async {
    createCalls++;
    lastCreatedName = name;
    lastCreatedCompany = company;
    lastCreatedEmail = email;
    lastCreatedPhone = phone;
    lastCreatedMetadata = metadata;
    if (createFails) {
      throw DioException(
        requestOptions: RequestOptions(path: '/funnels/$funnelId/leads'),
        response: Response(
          requestOptions: RequestOptions(path: '/funnels/$funnelId/leads'),
          statusCode: 400,
          data: {'code': 'VALIDATION_ERROR', 'message': 'telefone inválido'},
        ),
      );
    }
    return createReturns;
  }

  @override
  Future<void> resetOrder({
    required String funnelId,
    required String stageId,
  }) async {
    resetCalls++;
    lastResetFunnelId = funnelId;
    lastResetStageId = stageId;
  }

  @override
  Future<void> move({
    required String leadId,
    required String stageId,
    String? beforeLeadId,
    String? afterLeadId,
  }) async {
    calls++;
    lastLeadId = leadId;
    lastStageId = stageId;
    lastBeforeId = beforeLeadId;
    lastAfterId = afterLeadId;
    if (fail) {
      throw DioException(
        requestOptions: RequestOptions(path: '/leads/$leadId/move'),
        response: Response(
          requestOptions: RequestOptions(path: '/leads/$leadId/move'),
          statusCode: 403,
          data: {'code': 'FORBIDDEN', 'message': 'sem permissão'},
        ),
      );
    }
  }
}

/// A funnel with the SDR's six stages — the shape the board is actually used in,
/// and the one that overflowed the window's width.
Board _sixStageBoard() => Board(
      stages: const [
        BoardStage(
            id: 's1',
            name: 'Novos',
            position: 100,
            mapsToStatus: 'new',
            isTerminal: false),
        BoardStage(
            id: 's2',
            name: 'Abordados',
            position: 200,
            mapsToStatus: 'contacted',
            isTerminal: false),
        BoardStage(
            id: 's3',
            name: 'Responderam',
            position: 300,
            mapsToStatus: 'responded',
            isTerminal: false),
        BoardStage(
            id: 's4',
            name: 'Qualificados',
            position: 400,
            mapsToStatus: 'qualified',
            isTerminal: false),
        BoardStage(
            id: 's5',
            name: 'Ganhos',
            position: 500,
            mapsToStatus: 'won',
            isTerminal: true),
        BoardStage(
            id: 's6',
            name: 'Perdidos',
            position: 600,
            mapsToStatus: 'lost',
            isTerminal: true),
      ],
      cards: const [
        BoardCard(
          leadId: 'unico',
          stageId: 's1',
          status: 'new',
          score: 0,
          companyName: 'Clínica de Imagem Paulista',
        ),
      ],
    );

/// A stage with more cards than the screen's height.
Board _tallColumnBoard(int cards) => Board(
      stages: const [
        BoardStage(
            id: 's1',
            name: 'Novos',
            position: 100,
            mapsToStatus: 'new',
            isTerminal: false),
        BoardStage(
            id: 's2',
            name: 'Ganhos',
            position: 200,
            mapsToStatus: 'won',
            isTerminal: true),
      ],
      cards: [
        for (var i = 0; i < cards; i++)
          BoardCard(
            leadId: 'lead-$i',
            stageId: 's1',
            status: 'new',
            score: 0,
            companyName: 'Clínica $i',
          ),
      ],
    );

Future<_FakeService> _pumpBoard(
  WidgetTester tester, {
  required Size size,
  bool failMove = false,
  String? openLeadId,
  bool createReturns = true,
  bool createFails = false,
  Board? board,
  List<FunnelModel> funnels = const [],
  List<TeamMemberModel> team = const [],
}) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  final service = _FakeService(
      fail: failMove, createReturns: createReturns, createFails: createFails);
  await tester.pumpWidget(ProviderScope(
    overrides: [
      boardProvider('f1').overrideWith((ref) async => board ?? _board()),
      funnelsListProvider.overrideWith(() => _FakeFunnels(funnels)),
      teamMembersProvider.overrideWith(() => _FakeTeam(team)),
      boardServiceProvider.overrideWithValue(service),
      // Never dial the network from a widget test.
      dioProvider.overrideWithValue(Dio()),
    ],
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      home: KanbanScreen(funnelId: 'f1', openLeadId: openLeadId),
    ),
  ));
  await tester.pumpAndSettle();
  return service;
}

/// Mirrors _columnMinWidth: below it the column stops being readable.
const _readableColumn = 280.0;

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  // Six stages at a fixed 280px made 1740px. In a 1440 window — an ordinary
  // laptop — the first column sat behind the navigation, and since the account's
  // only lead was in it, a freshly imported board looked empty: the chips at the
  // top said "1" and every visible column said "No leads here".
  group('the whole funnel fits the window', () {
    // Six stages still fit in 1440 with a readable column; eight do not, and the
    // board then scrolls (with auto-scroll while dragging).
    testWidgets('as seis etapas aparecem em 1440px', (tester) async {
      await _pumpBoard(tester,
          size: const Size(1440, 900), board: _sixStageBoard());

      // Populated stages keep their full column; empty ones collapse into
      // 44px rulers — but every stage stays ON SCREEN and droppable.
      expect(find.byKey(const Key('board_column_title_s1')), findsOneWidget);
      for (final id in ['s2', 's3', 's4', 's5', 's6']) {
        expect(find.byKey(Key('board_stub_$id')), findsOneWidget,
            reason: 'a etapa $id não está na tela');
      }
    });

    // Squeezing to fit would trade one unreachable column for eight unreadable
    // ones: a company name broken across two lines, tags wrapping.
    testWidgets('a column never gets narrower than readable', (tester) async {
      await _pumpBoard(tester,
          size: const Size(900, 900), board: _sixStageBoard());

      final first = tester.getRect(find.byKey(const Key('board_column_s1')));
      expect(first.width, greaterThanOrEqualTo(280));
    });

    // What the user saw: they imported a lead, clicked "View funnel", and the
    // card was in the hidden column.
    testWidgets('the first stage\'s lead is visible', (tester) async {
      await _pumpBoard(tester,
          size: const Size(1440, 900), board: _sixStageBoard());

      expect(find.text('Clínica de Imagem Paulista'), findsOneWidget);
    });

    // The bug was the FIRST column behind the navigation, with the account's only
    // lead inside it. The last one being out of view is the board scrolling, which
    // is expected with eight stages — what must not come back is the board's
    // beginning starting out unreachable.
    testWidgets('the board starts at the beginning', (tester) async {
      await _pumpBoard(tester,
          size: const Size(900, 900), board: _sixStageBoard());

      final first = tester.getRect(find.byKey(const Key('board_column_s1')));
      expect(first.left, greaterThanOrEqualTo(0));
      expect(first.width, greaterThanOrEqualTo(_readableColumn),
          reason: 'inteira, não uma fatia');
    });

    // Squeezing six columns into a narrow window would trade one unreachable
    // column for six unreadable ones: below the minimum, the board scrolls.
    testWidgets('in a narrow window the column stops at the readable minimum',
        (tester) async {
      await _pumpBoard(tester,
          size: const Size(900, 900), board: _sixStageBoard());

      final first = tester.getRect(find.byKey(const Key('board_column_s1')));
      expect(first.width, greaterThanOrEqualTo(_readableColumn));
      expect(find.byKey(const Key('board_desktop_scroll')), findsOneWidget);
    });
  });

  // The column grew with its content inside a parent that only scrolled
  // horizontally. Past the height of the screen, the rest of the stage could not
  // be reached — and the first stage is the one that receives everything
  // prospecting imports.
  group('the card stack scrolls inside the column', () {
    testWidgets('each stage has its own vertical scroll', (tester) async {
      await _pumpBoard(tester,
          size: const Size(1440, 900), board: _tallColumnBoard(30));

      expect(find.byKey(const Key('board_column_list_s1')), findsOneWidget);
      // s2 has no cards: it collapses to the ruler (still a drop target).
      expect(find.byKey(const Key('board_stub_s2')), findsOneWidget);
    });

    testWidgets('the card at the end of the stack is reachable',
        (tester) async {
      await _pumpBoard(tester,
          size: const Size(1440, 900), board: _tallColumnBoard(30));

      expect(find.text('Clínica 29'), findsNothing,
          reason: 'começa fora da vista');

      // Drags the stack itself up until the last card appears.
      for (var i = 0;
          i < 20 && find.text('Clínica 29').evaluate().isEmpty;
          i++) {
        await tester.drag(find.byKey(const Key('board_column_list_s1')),
            const Offset(0, -400));
        await tester.pump();
      }
      expect(find.text('Clínica 29'), findsOneWidget);
    });

    // A board that does not fit the screen and only scrolls with the mouse wheel
    // excludes anyone using the mouse one-handed.
    testWidgets('the board background can be dragged with the mouse',
        (tester) async {
      await _pumpBoard(tester,
          size: const Size(1440, 900), board: _sixStageBoard());

      final behavior = tester
          .widget<ScrollConfiguration>(find
              .ancestor(
                of: find.byKey(const Key('board_desktop_scroll')),
                matching: find.byType(ScrollConfiguration),
              )
              .first)
          .behavior;
      expect(behavior.dragDevices, contains(PointerDeviceKind.mouse));
    });
  });

  // The floating button hovered over the last column. With the board scrolled it
  // covered the cards the person was trying to read — a create button hiding what
  // already exists.
  group('where the new-lead button sits', () {
    testWidgets('on desktop it is in the header, not over the cards',
        (tester) async {
      await _pumpBoard(tester,
          size: const Size(1440, 900), board: _sixStageBoard());

      expect(find.byKey(const Key('board_new_lead')), findsOneWidget);
      expect(find.byType(FloatingActionButton), findsNothing);
    });

    // On a phone the thumb reaches the corner and there is no spare header row —
    // there the floating button is still the right shape.
    testWidgets('on a phone it stays floating', (tester) async {
      await _pumpBoard(tester, size: const Size(420, 800));

      expect(find.byType(FloatingActionButton), findsOneWidget);
    });
  });

  // Taking a lead to an off-screen stage meant: drop, scroll, find the card
  // again, pick it up again. Three steps for the gesture a board exists to
  // provide.
  group('dragging to the edge scrolls the board', () {
    testWidgets('hovering over the right edge pans the board', (tester) async {
      // Every stage populated: with the empty ones collapsing into rulers,
      // a one-lead six-stage board no longer overflows 900px at all.
      final base = _sixStageBoard();
      final wide = Board(stages: base.stages, cards: [
        ...base.cards,
        for (var i = 0; i < base.stages.length; i++)
          BoardCard(
            leadId: 'fill-$i',
            stageId: base.stages[i].id,
            status: 'new',
            score: 0,
            companyName: 'Filler $i',
          ),
      ]);
      await _pumpBoard(tester, size: const Size(900, 800), board: wide);

      final scroll = tester.widget<SingleChildScrollView>(
          find.byKey(const Key('board_desktop_scroll')));
      final before = scroll.controller!.offset;
      expect(before, 0);

      // Picks the card up and takes it to the right-hand strip without dropping
      // it. Horizontally only: the card responds to that axis (affinity), and a
      // vertical jump would let the column's list win the gesture arena.
      final card = tester.getCenter(find.text('Clínica de Imagem Paulista'));
      final gesture = await tester.startGesture(card);
      for (var x = card.dx; x < 870; x += 40) {
        await gesture.moveTo(Offset(x, card.dy));
        await tester.pump(const Duration(milliseconds: 16));
      }
      await gesture.moveTo(Offset(870, card.dy));
      for (var i = 0; i < 20; i++) {
        await tester.pump(const Duration(milliseconds: 16));
      }

      expect(scroll.controller!.offset, greaterThan(before),
          reason: 'o quadro anda enquanto o card espera na borda');

      await gesture.up();
      await tester.pumpAndSettle();
    });
  });

  // Sixty-four unowned leads, one at a time, is work nobody does — and a field
  // nobody fills in becomes a field nobody trusts.
  group('atribuição em massa', () {
    Board boardWithOwnerless(int without, {int owned = 0}) => Board(
          stages: _sixStageBoard().stages,
          cards: [
            for (var i = 0; i < without; i++)
              BoardCard(
                leadId: 'sem-$i',
                stageId: 's1',
                status: 'new',
                score: 0,
                companyName: 'Sem dono $i',
              ),
            for (var i = 0; i < owned; i++)
              BoardCard(
                leadId: 'com-$i',
                stageId: 's1',
                status: 'new',
                score: 0,
                companyName: 'Com dono $i',
                ownerUserId: 'u9',
              ),
          ],
        );

    testWidgets('the button counts how many have no owner', (tester) async {
      await _pumpBoard(tester,
          size: const Size(1440, 900), board: boardWithOwnerless(7, owned: 3));

      expect(find.byKey(const Key('board_bulk_assign')), findsOneWidget);
      expect(find.textContaining('7'), findsWidgets);
    });

    // An action that would do nothing does not deserve header space.
    testWidgets('with nobody orphaned, the button does not exist',
        (tester) async {
      await _pumpBoard(tester,
          size: const Size(1440, 900), board: boardWithOwnerless(0, owned: 3));

      expect(find.byKey(const Key('board_bulk_assign')), findsNothing);
    });

    testWidgets('only the ownerless ones travel', (tester) async {
      final service = await _pumpBoard(tester,
          size: const Size(1440, 900),
          board: boardWithOwnerless(2, owned: 2),
          team: [_teamMember('u1', 'Ana')]);

      await tester.tap(find.byKey(const Key('board_bulk_assign')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('bulk_assign_owner')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Ana').last);
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('bulk_assign_submit')));
      await tester.pumpAndSettle();

      expect(service.lastBulkIds, ['sem-0', 'sem-1']);
      expect(service.lastBulkOwner, 'u1');
    });

    // This dialog exists to GIVE an owner. Confirming with nobody picked would
    // send a bulk unassignment nobody asked for.
    testWidgets('with no person picked, it cannot be confirmed',
        (tester) async {
      await _pumpBoard(tester,
          size: const Size(1440, 900),
          board: boardWithOwnerless(2),
          team: [_teamMember('u1', 'Ana')]);

      await tester.tap(find.byKey(const Key('board_bulk_assign')));
      await tester.pumpAndSettle();

      final button = tester.widget<PrimaryPillButton>(
          find.byKey(const Key('bulk_assign_submit')));
      expect(button.onPressed, isNull);
    });

    // "Assigned 64" and "assigned 61, three vanished" are different answers, and
    // the second shows up when somebody touched the funnel with the screen open.
    testWidgets('what was no longer there is counted separately',
        (tester) async {
      final service = await _pumpBoard(tester,
          size: const Size(1440, 900),
          board: boardWithOwnerless(3),
          team: [_teamMember('u1', 'Ana')]);
      service.bulkSkipped = 1;

      await tester.tap(find.byKey(const Key('board_bulk_assign')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('bulk_assign_owner')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Ana').last);
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('bulk_assign_submit')));
      await tester.pumpAndSettle();

      expect(find.text(t.board.bulk_assign_partial(count: '2', skipped: '1')),
          findsOneWidget);
    });

    // With a search active the count is not "the whole funnel", and the dialog
    // says so.
    testWidgets(
        'with a search, the dialog says the count is of what is visible',
        (tester) async {
      await _pumpBoard(tester,
          size: const Size(1440, 900),
          board: boardWithOwnerless(3),
          team: [_teamMember('u1', 'Ana')]);

      await tester.enterText(
          find.byKey(const Key('board_search')), 'Sem dono 1');
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('board_bulk_assign')));
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('bulk_assign_filtered')), findsOneWidget);
    });
  });

  // The funnel declares the fields it needs to know, and the create form asked
  // for a name and a phone. Worse: the required ones are what makes a lead
  // qualified, and they had nowhere to be filled in.
  group('the create form asks what the funnel asks for', () {
    testWidgets('os campos do funil aparecem no diálogo', (tester) async {
      final service =
          await _pumpBoard(tester, size: const Size(1440, 900), funnels: [
        const FunnelModel(id: 'f1', name: 'Funil', metadataSchema: [
          MetadataFieldModel(key: 'convenio', label: 'Convênio'),
        ]),
      ]);

      await tester.tap(find.byKey(const Key('board_new_lead')));
      await tester.pumpAndSettle();

      expect(find.text(t.board.funnel_fields), findsOneWidget);
      expect(find.text('Convênio'), findsOneWidget);
      expect(service.createCalls, 0);
    });

    testWidgets('what was answered travels with the create call',
        (tester) async {
      final service =
          await _pumpBoard(tester, size: const Size(1440, 900), funnels: [
        const FunnelModel(id: 'f1', name: 'Funil', metadataSchema: [
          MetadataFieldModel(key: 'convenio', label: 'Convênio'),
        ]),
      ]);

      await tester.tap(find.byKey(const Key('board_new_lead')));
      await tester.pumpAndSettle();
      await tester.enterText(
          find.byKey(const Key('new_lead_company')), 'Clínica X');
      await tester.enterText(
          find.byKey(const Key('new_lead_phone')), '+5511999990001');
      await tester.enterText(
          find.byKey(const Key('metadata_field_convenio')), 'Unimed');
      await tester.tap(find.byKey(const Key('new_lead_submit')));
      await tester.pumpAndSettle();

      expect(service.lastCreatedMetadata['convenio'], 'Unimed');
    });

    // A funnel with no declared fields gets no empty titled section.
    testWidgets('a funnel with no fields keeps the form short', (tester) async {
      await _pumpBoard(tester, size: const Size(1440, 900));

      await tester.tap(find.byKey(const Key('board_new_lead')));
      await tester.pumpAndSettle();

      expect(find.text(t.board.funnel_fields), findsNothing);
    });
  });

  // "0 of 0 · most idle first" is a sort caption over nothing.
  group('the column caption', () {
    testWidgets('an empty column announces no ordering', (tester) async {
      await _pumpBoard(tester,
          size: const Size(1440, 900), board: _sixStageBoard());

      expect(find.byKey(const Key('board_column_showing_s6')), findsNothing);
      expect(find.byKey(const Key('board_column_showing_s1')), findsOneWidget);
    });
  });

  testWidgets('desktop renders every column with its count', (tester) async {
    await _pumpBoard(tester, size: const Size(1400, 900));

    expect(find.byKey(const Key('board_column_s1')), findsOneWidget);
    // Empty stages collapse to rulers — present, countable, droppable.
    expect(find.byKey(const Key('board_stub_s2')), findsOneWidget);
    expect(find.byKey(const Key('board_stub_s3')), findsOneWidget);

    // Counts are what tell a manager where the pipeline is stuck.
    expect(
      tester.widget<Text>(find.byKey(const Key('board_column_count_s1'))).data,
      '2',
    );

    // The card leads with the COMPANY: they are selling to an organization,
    // and a phone number is not a name.
    expect(find.text('Hospital São Lucas'), findsOneWidget);
    // ...falling back to whatever identifies the contact when there is none.
    expect(find.text('+5511999990002'), findsOneWidget);
  });

  testWidgets('an unowned lead is flagged — it is nobody\'s job yet',
      (tester) async {
    await _pumpBoard(tester, size: const Size(1400, 900));
    expect(find.text(t.board.unassigned), findsOneWidget);
  });

  // Sixty-nine cards all saying "Sem dono" is one repetition per lead of a
  // fact that names no action — which is every board of a single-seat
  // account, because nobody there ever assigns. The chip turns itself on the
  // moment ownership starts to mean something.
  testWidgets('a board where nobody owns anything does not say so, per card',
      (tester) async {
    final noOwners = Board(
      stages: _board().stages,
      cards: const [
        BoardCard(
            leadId: 'lead1',
            stageId: 's1',
            status: 'new',
            score: 7,
            companyName: 'Hospital São Lucas'),
        BoardCard(
            leadId: 'lead2',
            stageId: 's1',
            status: 'new',
            score: 3,
            companyName: 'Santa Casa'),
      ],
    );

    await _pumpBoard(tester, size: const Size(1400, 900), board: noOwners);

    expect(find.text(t.board.unassigned), findsNothing);
    // The cards themselves are all there — only the label nobody can act on
    // is gone.
    expect(find.byKey(const Key('lead_card_lead1')), findsOneWidget);
    expect(find.byKey(const Key('lead_card_lead2')), findsOneWidget);
  });

  testWidgets('desktop drag moves the card to the target column',
      (tester) async {
    final service = await _pumpBoard(tester, size: const Size(1400, 900));

    final card = find.byKey(const Key('lead_card_lead1'));
    // s2 is empty, so the drop target is its collapsed ruler.
    final target = find.byKey(const Key('board_stub_s2'));

    final gesture = await tester.startGesture(tester.getCenter(card));
    await tester.pump(const Duration(milliseconds: 200));
    await gesture.moveTo(tester.getCenter(target));
    await tester.pump();
    await gesture.up();
    await tester.pumpAndSettle();

    expect(service.calls, 1);
    expect(service.lastLeadId, 'lead1');
    expect(service.lastStageId, 's2');
  });

  // The important one: a refused move must not leave the card sitting in the
  // new column, or the salesperson believes a deal advanced when it did not.
  // A card that moved because someone TAPPED it would change a deal's stage
  // — and its canonical status — without anyone deciding to. Opening the
  // lead and moving it are different intents.
  testWidgets('tapping a card opens it and moves nothing', (tester) async {
    final service = await _pumpBoard(tester, size: const Size(1400, 900));

    await tester.tap(find.byKey(const Key('lead_card_lead1')));
    await tester.pumpAndSettle();

    expect(service.calls, 0, reason: 'a tap is not a drag');
    expect(find.byKey(const Key('lead_detail_title')), findsOneWidget);
  });

  testWidgets('a failed move rolls the card back and says why', (tester) async {
    await _pumpBoard(tester, size: const Size(1400, 900), failMove: true);

    final gesture = await tester.startGesture(
        tester.getCenter(find.byKey(const Key('lead_card_lead1'))));
    await tester.pump(const Duration(milliseconds: 200));
    await gesture
        .moveTo(tester.getCenter(find.byKey(const Key('board_stub_s2'))));
    await tester.pump();
    await gesture.up();
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('board_move_error')), findsOneWidget);
    // Column counts are back where they started.
    expect(
      tester.widget<Text>(find.byKey(const Key('board_column_count_s1'))).data,
      '2',
    );
    expect(find.byKey(const Key('board_stub_s2')), findsOneWidget);
  });

  // US-M024: the inbox's pipeline chip links here with ?lead=, and the
  // board opens that lead's detail instead of leaving the user to hunt
  // for the card.
  testWidgets('a deep-linked lead opens its detail on arrival', (tester) async {
    await _pumpBoard(tester, size: const Size(1400, 900), openLeadId: 'lead1');

    expect(find.byKey(const Key('lead_detail_title')), findsOneWidget);
  });

  // A lead that is not on this board must not open an empty panel.
  testWidgets('an unknown deep-linked lead is ignored', (tester) async {
    await _pumpBoard(tester,
        size: const Size(1400, 900), openLeadId: 'not-on-this-board');

    expect(find.byKey(const Key('lead_detail_title')), findsNothing);
  });

  // The product is not about clinics. This text is not an example inside an
  // empty field — it is the label saying WHAT the search looks through, and it
  // asserted a vertical. The search spans company name, city and contact;
  // saying "clinic" promised it only finds clinics.
  testWidgets('the search does not claim to look up one vertical',
      (tester) async {
    await _pumpBoard(tester, size: const Size(1200, 800));

    final field =
        tester.widget<TextField>(find.byKey(const Key('board_search')));
    final hint = field.decoration!.hintText!;

    expect(hint, t.board.search_hint);
    expect(hint.toLowerCase(), isNot(contains('clínic')));
    expect(hint.toLowerCase(), isNot(contains('clinic')));
  });

  testWidgets('mobile shows one column at a time with stage chips',
      (tester) async {
    await _pumpBoard(tester, size: const Size(390, 800));

    expect(find.byKey(const Key('board_pageview')), findsOneWidget);
    expect(find.byKey(const Key('board_stage_chips')), findsOneWidget);
    // No drag targets on a phone: dragging across a paging viewport is two
    // gestures fighting.
    expect(find.byKey(const Key('board_column_s1')), findsNothing);
  });

  // The floating button covered the end of the list. The stack is ordered by
  // idle time, so the hidden card was always the most idle one — the very card
  // the ordering exists to show first.
  testWidgets('the stage list leaves room for the floating button',
      (tester) async {
    await _pumpBoard(tester, size: const Size(390, 800));

    final list = tester.widget<ListView>(find
        .descendant(
          of: find.byKey(const Key('board_pageview')),
          matching: find.byType(ListView),
        )
        .first);
    final padding = (list.padding as EdgeInsets?) ?? EdgeInsets.zero;
    expect(padding.bottom, greaterThanOrEqualTo(72),
        reason: 'the extended FAB is ~56px and floats over the list');
  });

  testWidgets('mobile moves a card through the sheet', (tester) async {
    final service = await _pumpBoard(tester, size: const Size(390, 800));

    await tester.tap(find.byKey(const Key('lead_card_move_lead1')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('board_move_sheet')), findsOneWidget);

    await tester.tap(find.byKey(const Key('board_move_option_s2')));
    await tester.pumpAndSettle();

    expect(service.calls, 1);
    expect(service.lastStageId, 's2');
  });

  testWidgets('picking the CURRENT column is a no-op, not a wasted request',
      (tester) async {
    final service = await _pumpBoard(tester, size: const Size(390, 800));

    await tester.tap(find.byKey(const Key('lead_card_move_lead1')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('board_move_option_s1')));
    await tester.pumpAndSettle();

    expect(service.calls, 0);
  });

  // A funnel could only be filled by the Places import or an inbound
  // message, so a referral or a card from a conference had no way in.
  testWidgets('a lead can be typed in by hand', (tester) async {
    final service = await _pumpBoard(tester, size: const Size(1400, 900));

    await tester.tap(find.byKey(const Key('board_new_lead')));
    await tester.pumpAndSettle();

    await tester.enterText(
        find.byKey(const Key('new_lead_company')), 'Clínica Aurora');
    await tester.enterText(
        find.byKey(const Key('new_lead_phone')), '+5511999900101');
    await tester.tap(find.byKey(const Key('new_lead_submit')));
    await tester.pumpAndSettle();

    expect(service.createCalls, 1);
    expect(service.lastCreatedCompany, 'Clínica Aurora');
    expect(service.lastCreatedPhone, '+5511999900101');
    expect(find.byKey(const Key('new_lead_created')), findsOneWidget);
  });

  // The server is idempotent. Saying "added" when nothing was added sends
  // the user hunting for a second card that will never appear.
  testWidgets('a repeated phone says it was already in the funnel',
      (tester) async {
    await _pumpBoard(tester, size: const Size(1400, 900), createReturns: false);

    await tester.tap(find.byKey(const Key('board_new_lead')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('new_lead_company')), 'Aurora');
    await tester.enterText(
        find.byKey(const Key('new_lead_phone')), '+5511999900101');
    await tester.tap(find.byKey(const Key('new_lead_submit')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('new_lead_existed')), findsOneWidget);
    expect(find.byKey(const Key('new_lead_created')), findsNothing);
  });

  // Two submissions in a row: the second answer must replace the first, not
  // queue behind it. Found in the browser — adding a lead twice showed the
  // previous message, which reads as the reply to the wrong submission.
  testWidgets('a second answer replaces the first instead of queueing',
      (tester) async {
    final service = await _pumpBoard(tester, size: const Size(1400, 900));

    Future<void> addOnce() async {
      await tester.tap(find.byKey(const Key('board_new_lead')));
      await tester.pumpAndSettle();
      await tester.enterText(
          find.byKey(const Key('new_lead_company')), 'Aurora');
      await tester.enterText(
          find.byKey(const Key('new_lead_phone')), '+5511999900101');
      await tester.tap(find.byKey(const Key('new_lead_submit')));
      await tester.pumpAndSettle();
    }

    // First one lands as a genuinely new card...
    await addOnce();
    expect(find.byKey(const Key('new_lead_created')), findsOneWidget);

    // ...and the second, submitted while that message is still on screen,
    // must show ITS own answer, not wait behind the previous one.
    service.createReturns = false;
    await addOnce();

    expect(find.byKey(const Key('new_lead_existed')), findsOneWidget);
    expect(find.byKey(const Key('new_lead_created')), findsNothing,
        reason: 'the previous answer is still on screen');
  });

  // Phone OR e-mail is the messaging address, so a lead nobody can contact
  // must not land on the board looking actionable.
  testWidgets('the form refuses to submit without a phone or an e-mail',
      (tester) async {
    final service = await _pumpBoard(tester, size: const Size(1400, 900));

    await tester.tap(find.byKey(const Key('board_new_lead')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('new_lead_company')), 'Aurora');
    await tester.tap(find.byKey(const Key('new_lead_submit')));
    await tester.pumpAndSettle();

    expect(service.createCalls, 0);
    expect(find.text(t.board.new_lead_reach_required), findsOneWidget);
  });

  // A lead in this product is a company with somebody in it; a purchasing
  // manager reached by e-mail is as real as a clinic owner on WhatsApp.
  testWidgets('an e-mail is enough to reach, and the company is the lead',
      (tester) async {
    final service = await _pumpBoard(tester, size: const Size(1400, 900));

    await tester.tap(find.byKey(const Key('board_new_lead')));
    await tester.pumpAndSettle();
    await tester.enterText(
        find.byKey(const Key('new_lead_company')), 'Hospital Regional');
    await tester.enterText(find.byKey(const Key('new_lead_name')), 'Maria');
    await tester.enterText(
        find.byKey(const Key('new_lead_email')), 'compras@hospital.test');
    await tester.tap(find.byKey(const Key('new_lead_submit')));
    await tester.pumpAndSettle();

    expect(service.createCalls, 1);
    expect(service.lastCreatedCompany, 'Hospital Regional');
    expect(service.lastCreatedName, 'Maria');
    expect(service.lastCreatedEmail, 'compras@hospital.test');
    expect(service.lastCreatedPhone, '');
  });

  testWidgets('the company is required', (tester) async {
    final service = await _pumpBoard(tester, size: const Size(1400, 900));

    await tester.tap(find.byKey(const Key('board_new_lead')));
    await tester.pumpAndSettle();
    await tester.enterText(
        find.byKey(const Key('new_lead_phone')), '+5565999990000');
    await tester.tap(find.byKey(const Key('new_lead_submit')));
    await tester.pumpAndSettle();

    expect(service.createCalls, 0);
    expect(find.text(t.board.new_lead_company_required), findsOneWidget);
  });

  // A rejected phone keeps the dialog open with what the user typed: making
  // them retype the name to fix a digit is how forms get abandoned.
  testWidgets('a rejected lead keeps the form and shows why', (tester) async {
    await _pumpBoard(tester, size: const Size(1400, 900), createFails: true);

    await tester.tap(find.byKey(const Key('board_new_lead')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('new_lead_company')), 'Aurora');
    await tester.enterText(find.byKey(const Key('new_lead_phone')), '123');
    await tester.tap(find.byKey(const Key('new_lead_submit')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('new_lead_error')), findsOneWidget);
    expect(find.text('Aurora'), findsOneWidget);
  });

  // The dialog is where a lead gets typed in, and an SDR types leads in
  // between appointments — on a phone. A form that overflows there is a
  // form nobody finishes.
  testWidgets('the new-lead form fits a phone', (tester) async {
    final service = await _pumpBoard(tester, size: const Size(390, 800));

    await tester.tap(find.byKey(const Key('board_new_lead')));
    await tester.pumpAndSettle();

    await tester.enterText(
        find.byKey(const Key('new_lead_company')), 'Clínica Aurora');
    await tester.enterText(
        find.byKey(const Key('new_lead_phone')), '+5511999900101');
    // Six fields stack on a phone; the form scrolls, and so does the thumb.
    await tester.ensureVisible(find.byKey(const Key('new_lead_submit')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('new_lead_submit')));
    await tester.pumpAndSettle();

    expect(service.createCalls, 1);
  });

  // ── Search, filters and order ────────────────────────────────────────────
  //
  // Sixty leads in a column was blind scrolling: no search, no order, no way
  // to tell which deal is dying. These pin the handles that fix that.

  testWidgets('typing a clinic name narrows the columns to it', (tester) async {
    await _pumpBoard(tester,
        size: const Size(1400, 900), board: _searchBoard());

    expect(find.text('Clínica Vida Nova'), findsOneWidget);
    expect(find.text('Clínica Aurora'), findsOneWidget);

    await tester.enterText(find.byKey(const Key('board_search')), 'vida');
    await tester.pumpAndSettle();

    expect(find.text('Clínica Vida Nova'), findsOneWidget);
    expect(find.text('Clínica Aurora'), findsNothing);
  });

  // Found live: "clínica" returned nothing while a card named "Clinica
  // Ficticia" sat on the board. Most of this text was typed by somebody else,
  // so the accent is a coin flip either way.
  testWidgets('the accent does not decide whether a lead is findable',
      (tester) async {
    await _pumpBoard(tester,
        size: const Size(1400, 900), board: _searchBoard());

    await tester.enterText(find.byKey(const Key('board_search')), 'clinica');
    await tester.pumpAndSettle();

    expect(find.text('Clínica Vida Nova'), findsOneWidget);
    expect(find.text('Hospital São Lucas'), findsNothing);

    // ...and the other direction: an accented query over unaccented data.
    await tester.enterText(find.byKey(const Key('board_search')), 'são lucas');
    await tester.pumpAndSettle();
    expect(find.text('Hospital São Lucas'), findsOneWidget);

    await tester.enterText(find.byKey(const Key('board_search')), 'sao lucas');
    await tester.pumpAndSettle();
    expect(find.text('Hospital São Lucas'), findsOneWidget);
  });

  // The salesperson remembers "aquela de Sorocaba" as often as the name.
  testWidgets('searching by city works too', (tester) async {
    await _pumpBoard(tester,
        size: const Size(1400, 900), board: _searchBoard());

    await tester.enterText(find.byKey(const Key('board_search')), 'sorocaba');
    await tester.pumpAndSettle();

    expect(find.text('Clínica Vida Nova'), findsOneWidget);
    expect(find.text('Laboratório Central'), findsOneWidget);
    expect(find.text('Hospital São Lucas'), findsNothing);
  });

  testWidgets('a column with no match says where else to look', (tester) async {
    await _pumpBoard(tester,
        size: const Size(1400, 900), board: _searchBoard());

    await tester.enterText(find.byKey(const Key('board_search')), 'zzzz');
    await tester.pumpAndSettle();

    // Not the same as an empty column: this one has a next move.
    expect(find.text(t.board.column_no_match(query: 'zzzz')), findsWidgets);
    expect(find.text(t.board.column_no_match_hint), findsWidgets);
    expect(find.text(t.board.empty_column), findsNothing);
  });

  testWidgets('clearing the search brings every card back', (tester) async {
    await _pumpBoard(tester,
        size: const Size(1400, 900), board: _searchBoard());

    await tester.enterText(find.byKey(const Key('board_search')), 'vida');
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('board_search_clear')));
    await tester.pumpAndSettle();

    expect(find.text('Clínica Aurora'), findsOneWidget);
    expect(find.text('Hospital São Lucas'), findsOneWidget);
  });

  testWidgets('the filters count their pile before anyone clicks',
      (tester) async {
    await _pumpBoard(tester,
        size: const Size(1400, 900), board: _searchBoard());

    expect(find.text(t.board.filter_responded(count: 1)), findsOneWidget);
    expect(find.text(t.board.filter_awaiting(count: 1)), findsOneWidget);
    // One lead 30 days idle; the 1-day and 3-day ones are not stalled, and
    // the timestamp-less one is unknown, not stalled.
    expect(find.text(t.board.filter_stalled(count: 1)), findsOneWidget);
  });

  testWidgets('picking "responderam" leaves only the replies', (tester) async {
    await _pumpBoard(tester,
        size: const Size(1400, 900), board: _searchBoard());

    await tester.tap(find.byKey(const Key('board_filter_responded')));
    await tester.pumpAndSettle();

    expect(find.text('Clínica Aurora'), findsOneWidget);
    expect(find.text('Clínica Vida Nova'), findsNothing);
    expect(find.text('Hospital São Lucas'), findsNothing);
  });

  testWidgets('a column header says how much of itself it is showing',
      (tester) async {
    await _pumpBoard(tester,
        size: const Size(1400, 900), board: _searchBoard());

    expect(
      tester
          .widget<Text>(find.byKey(const Key('board_column_showing_s1')))
          .data,
      t.board.column_showing(showing: 3, total: 3),
    );

    await tester.enterText(find.byKey(const Key('board_search')), 'vida');
    await tester.pumpAndSettle();

    expect(
      tester
          .widget<Text>(find.byKey(const Key('board_column_showing_s1')))
          .data,
      t.board.column_showing(showing: 1, total: 3),
      reason: 'the total must keep counting what the filter hid',
    );
  });

  testWidgets('a card says how long it has been sitting still', (tester) async {
    await _pumpBoard(tester,
        size: const Size(1400, 900), board: _searchBoard());

    expect(
      tester.widget<Text>(find.byKey(const Key('lead_card_idle_vida'))).data,
      t.board.idle_days(count: 30),
    );
    // No timestamp is not "moved today".
    expect(
      tester
          .widget<Text>(find.byKey(const Key('lead_card_idle_sem-data')))
          .data,
      t.board.idle_unknown,
    );
  });

  testWidgets('a truncated board admits the search is partial', (tester) async {
    final many = Board(
      truncated: true,
      stages: _searchBoard().stages,
      cards: _searchBoard().cards,
    );
    await _pumpBoard(tester, size: const Size(1400, 900), board: many);

    expect(find.byKey(const Key('board_truncated')), findsOneWidget);
  });

  testWidgets('a complete board says nothing about truncation', (tester) async {
    await _pumpBoard(tester,
        size: const Size(1400, 900), board: _searchBoard());

    expect(find.byKey(const Key('board_truncated')), findsNothing);
  });

  testWidgets('the phone stage chips count what the filter leaves',
      (tester) async {
    await _pumpBoard(tester, size: const Size(390, 900), board: _searchBoard());

    expect(find.text('Novos (3)'), findsOneWidget);

    await tester.tap(find.byKey(const Key('board_filter_responded')));
    await tester.pumpAndSettle();

    expect(find.text('Novos (1)'), findsOneWidget,
        reason: 'a chip promising cards that the filter removed is a trap');
  });

  // ── Reordering inside a column ───────────────────────────────────────────
  //
  // Dragging a card within its own column used to do nothing: the only drop
  // target was the whole column, so there was no slot to aim at and no
  // feedback about where the card would land.

  /// Claims the drag (horizontal affinity) and parks the card over the gap
  /// with [gapKey].
  ///
  /// The gaps are ZERO-height at rest (invisible slots) and only grow once a
  /// drag is in flight, so the helper claims the drag first, lets the gaps
  /// finish growing, and only THEN measures where to aim — a target taken
  /// from the resting layout points at a slot that has since moved. The
  /// trailing nudge matters too: drag hover only re-hit-tests on pointer
  /// MOVEMENT, never on layout changes under a still pointer. `skipOffstage:
  /// false` because a zero-size sliver child reads as offstage to the
  /// default finder.
  Future<TestGesture> dragOnto(
      WidgetTester tester, Finder card, Key gapKey) async {
    final gesture = await tester.startGesture(tester.getCenter(card));
    await tester.pump(const Duration(milliseconds: 200));
    // The card only grabs the arena horizontally; once claimed, it moves
    // freely — including vertically, which is what a reorder is.
    await gesture.moveBy(const Offset(60, 0));
    await tester.pump(const Duration(milliseconds: 250));
    await gesture.moveBy(const Offset(0, 1));
    await tester.pump(const Duration(milliseconds: 250));
    await gesture
        .moveTo(tester.getCenter(find.byKey(gapKey, skipOffstage: false)));
    await tester.pump(const Duration(milliseconds: 100));
    await gesture.moveBy(const Offset(0, 1));
    await tester.pump(const Duration(milliseconds: 250));
    await tester.pump(const Duration(milliseconds: 250));
    return gesture;
  }

  group('reordenar dentro da coluna', () {
    testWidgets('hovering a gap opens the placeholder', (tester) async {
      await _pumpBoard(tester, size: const Size(1400, 900));

      final gap = find.byKey(const Key('board_gap_s1_0'), skipOffstage: false);
      expect(tester.getSize(gap).height, 0,
          reason: 'at rest the slots are invisible');

      final gesture = await dragOnto(
          tester,
          find.byKey(const Key('lead_card_lead2')),
          const Key('board_gap_s1_0'));

      expect(tester.getSize(gap).height, greaterThanOrEqualTo(70),
          reason: 'the hovered slot opens into the placeholder');
      // The other gaps grew too, so the next slot is hittable mid-flight.
      expect(
          tester
              .getSize(
                  find.byKey(const Key('board_gap_s1_1'), skipOffstage: false))
              .height,
          greaterThan(0));

      await gesture.up();
      await tester.pumpAndSettle();
    });

    testWidgets('dropping on a gap sends the slot neighbors', (tester) async {
      final service = await _pumpBoard(tester, size: const Size(1400, 900));

      final gesture = await dragOnto(
          tester,
          find.byKey(const Key('lead_card_lead2')),
          const Key('board_gap_s1_0'));
      await gesture.up();
      await tester.pumpAndSettle();

      expect(service.calls, 1);
      expect(service.lastLeadId, 'lead2');
      expect(service.lastStageId, 's1');
      expect(service.lastBeforeId, isNull,
          reason: 'dropped at the very top: nothing above');
      expect(service.lastAfterId, 'lead1',
          reason: 'the dragged card itself is never its own neighbor');
    });

    testWidgets('a same-column drop reorders optimistically', (tester) async {
      await _pumpBoard(tester, size: const Size(1400, 900));

      var y1 = tester.getTopLeft(find.byKey(const Key('lead_card_lead1'))).dy;
      var y2 = tester.getTopLeft(find.byKey(const Key('lead_card_lead2'))).dy;
      expect(y1, lessThan(y2), reason: 'starting order: lead1 above lead2');

      final gesture = await dragOnto(
          tester,
          find.byKey(const Key('lead_card_lead2')),
          const Key('board_gap_s1_0'));
      await gesture.up();
      await tester.pumpAndSettle();

      y1 = tester.getTopLeft(find.byKey(const Key('lead_card_lead1'))).dy;
      y2 = tester.getTopLeft(find.byKey(const Key('lead_card_lead2'))).dy;
      expect(y2, lessThan(y1),
          reason: 'the card follows the finger before the server answers');

      // ...and the caption stops claiming the automatic order.
      expect(
        tester
            .widget<Text>(find.byKey(const Key('board_column_showing_s1')))
            .data,
        t.board.column_showing_manual(showing: 2, total: 2),
      );
    });

    testWidgets('dropping a card back around itself asks nothing',
        (tester) async {
      final service = await _pumpBoard(tester, size: const Size(1400, 900));

      // Gap 1 sits between lead1 and lead2 — for lead2, that is where it
      // already is.
      final gesture = await dragOnto(
          tester,
          find.byKey(const Key('lead_card_lead2')),
          const Key('board_gap_s1_1'));
      await gesture.up();
      await tester.pumpAndSettle();

      expect(service.calls, 0, reason: 'a no-op drop is not a request');
    });
  });

  group('ordem manual e o caminho de volta', () {
    Board manualBoard() => Board(
          stages: _board().stages,
          cards: const [
            BoardCard(
              leadId: 'lead1',
              stageId: 's1',
              status: 'new',
              score: 0,
              companyName: 'Hospital São Lucas',
              boardPos: 10,
            ),
            BoardCard(
              leadId: 'lead2',
              stageId: 's1',
              status: 'new',
              score: 0,
              companyName: 'Clínica Aurora',
            ),
          ],
        );

    testWidgets('a placed card flips the caption to ordem manual',
        (tester) async {
      await _pumpBoard(tester,
          size: const Size(1400, 900), board: manualBoard());

      expect(
        tester
            .widget<Text>(find.byKey(const Key('board_column_showing_s1')))
            .data,
        t.board.column_showing_manual(showing: 2, total: 2),
      );
      expect(find.byKey(const Key('board_order_reset_s1')), findsOneWidget);
      // An untouched column keeps the automatic caption and no reset.
      expect(find.byKey(const Key('board_order_reset_s2')), findsNothing);
    });

    testWidgets('the reset calls the route and restores the caption',
        (tester) async {
      final service = await _pumpBoard(tester,
          size: const Size(1400, 900), board: manualBoard());

      await tester.tap(find.byKey(const Key('board_order_reset_s1')));
      await tester.pumpAndSettle();

      expect(service.resetCalls, 1);
      expect(service.lastResetFunnelId, 'f1');
      expect(service.lastResetStageId, 's1');
      // Optimistically back to the automatic order.
      expect(
        tester
            .widget<Text>(find.byKey(const Key('board_column_showing_s1')))
            .data,
        t.board.column_showing(showing: 2, total: 2),
      );
    });

    testWidgets('placed cards sort first, by their chosen position',
        (tester) async {
      // lead2 has no manual position: it renders AFTER the placed lead1
      // regardless of idle time — and a second placed card obeys its number.
      final board = Board(
        stages: _board().stages,
        cards: [
          BoardCard(
            leadId: 'auto',
            stageId: 's1',
            status: 'new',
            score: 0,
            companyName: 'Automática',
            updatedAt: DateTime.now().subtract(const Duration(days: 90)),
          ),
          const BoardCard(
            leadId: 'second',
            stageId: 's1',
            status: 'new',
            score: 0,
            companyName: 'Segunda',
            boardPos: 20,
          ),
          const BoardCard(
            leadId: 'first',
            stageId: 's1',
            status: 'new',
            score: 0,
            companyName: 'Primeira',
            boardPos: 10,
          ),
        ],
      );
      await _pumpBoard(tester, size: const Size(1400, 900), board: board);

      final yFirst =
          tester.getTopLeft(find.byKey(const Key('lead_card_first'))).dy;
      final ySecond =
          tester.getTopLeft(find.byKey(const Key('lead_card_second'))).dy;
      final yAuto =
          tester.getTopLeft(find.byKey(const Key('lead_card_auto'))).dy;
      expect(yFirst, lessThan(ySecond));
      expect(ySecond, lessThan(yAuto),
          reason: 'the most stalled card still cannot outrank a placed one');
    });
  });
}

class _FakeFunnels extends FunnelsList {
  _FakeFunnels(this._funnels);
  final List<FunnelModel> _funnels;

  @override
  Future<List<FunnelModel>> build() async => _funnels;
}

class _FakeTeam extends TeamMembers {
  _FakeTeam(this._members);
  final List<TeamMemberModel> _members;

  @override
  Future<List<TeamMemberModel>> build() async => _members;
}

TeamMemberModel _teamMember(String id, String name) => TeamMemberModel(
      id: id,
      name: name,
      email: '$id@x.com',
      role: 'attendant',
      isActive: true,
      isEmailVerified: true,
      createdAt: '2026-01-01T00:00:00Z',
      updatedAt: '2026-01-01T00:00:00Z',
    );
