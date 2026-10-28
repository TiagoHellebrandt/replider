import 'package:app/features/sales/presentation/controllers/board_controller.dart';
import 'package:app/features/settings/presentation/controllers/team_controller.dart';
import 'package:app/features/settings/domain/models/team_member_model.dart';
import 'package:app/features/sales/presentation/controllers/funnels_controller.dart';
import 'package:app/features/sales/domain/models/funnel_model.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/sales/domain/models/board_model.dart';
import 'package:app/features/sales/domain/models/approach_review.dart';
import 'package:app/features/sales/presentation/controllers/lead_detail_controller.dart';
import 'package:app/features/sales/presentation/widgets/lead_detail_panel.dart';
import 'package:app/features/settings/presentation/controllers/email_template_controller.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

// US-M019. The panel exists so a lead can be ACTED on, and the action is
// sending a message to a real person — so the tests that matter are the ones
// pinning the human gate: nothing leaves without a confirmation that shows
// the exact text and the recipient, and a viewer who may not act is not
// offered the button at all.

const _card = BoardCard(
  leadId: 'lead1',
  stageId: 's1',
  status: 'enriched',
  score: 8,
  companyName: 'Clínica Bem Viver',
);

class _FakeActions implements LeadActions {
  _FakeActions({this.failSend = false});

  final String draftText = 'rascunho novo';
  final bool failSend;

  int drafts = 0;
  int sends = 0;
  final savedFields = <String, String>{};
  String? sentText;
  String? sentSubject;
  String? sentTemplateId;
  int sentTemplateVersion = 0;

  @override
  Future<void> assignOwner({
    required String leadId,
    required String? ownerUserId,
  }) async {
    assignedOwner = ownerUserId;
    assignCalls++;
  }

  String? assignedOwner;
  int assignCalls = 0;
  String? setVisibilityTo;

  @override
  Future<void> setVisibility({
    required String leadId,
    required String visibility,
  }) async {
    setVisibilityTo = visibility;
  }

  @override
  Future<void> saveMetadataField({
    required String leadId,
    required String fieldKey,
    required String value,
  }) async {
    savedFields[fieldKey] = value;
  }

  @override
  Future<String?> draft({required String leadId}) async {
    drafts++;
    return draftText;
  }

  @override
  Future<DateTime?> approveSend({
    required String leadId,
    required String text,
    String? subject,
    String? templateName,
    String? templateLanguage,
    String? templateCategory,
    List<Map<String, dynamic>>? templateButtons,
    String? emailTemplateId,
    int emailTemplateVersion = 0,
    bool viaBulk = false,
  }) async {
    sends++;
    sentText = text;
    sentSubject = subject;
    sentTemplateId = emailTemplateId;
    sentTemplateVersion = emailTemplateVersion;
    if (failSend) {
      throw DioException(
        requestOptions: RequestOptions(path: '/leads/$leadId/approve-send'),
        response: Response(
          requestOptions: RequestOptions(path: '/leads/$leadId/approve-send'),
          statusCode: 403,
          data: {'code': 'FORBIDDEN', 'message': 'sem permissão'},
        ),
      );
    }
    return null;
  }

  @override
  Future<ApproachReview> review({
    required String leadId,
    required String text,
  }) async =>
      const ApproachReview(safe: true, checked: true, risks: []);

  final savedDrafts = <String>[];

  @override
  Future<void> saveDraft({
    required String leadId,
    required String text,
  }) async {
    savedDrafts.add(text);
  }

  @override
  Future<void> discard({required String leadId}) async {}
}

LeadDetail _detail({
  String draft = 'Olá, tudo bem?',
  List<LeadFact>? facts,
  String? conversationId,
  String address = '+5511988770001',
  // pending_approval: the send path is only live in that status now.
  String status = 'pending_approval',
  String? researchedAt,
  String? researchBlockedReason,
  List<ApproachFinding> findings = const [],
  String ownerUserId = '',
  List<String> sources = const [],
  String? linkedinUrl,
  String? instagramUrl,
}) =>
    LeadDetail(
      id: 'lead1',
      status: status,
      researchedAt: researchedAt,
      researchBlockedReason: researchBlockedReason,
      findings: findings,
      ownerUserId: ownerUserId,
      sources: sources,
      linkedinUrl: linkedinUrl,
      instagramUrl: instagramUrl,
      approachDraft: draft,
      recipient: 'Clínica Bem Viver',
      recipientAddress: address,
      facts: facts ??
          const [
            LeadFact(label: 'Empresa', value: 'Clínica Bem Viver'),
            LeadFact(label: 'Cargo', value: 'Diretora', fromResearch: true),
          ],
      timeline: const ['Criado em 01/07/2026 10:00'],
      conversationId: conversationId,
    );

Future<_FakeActions> _pump(
  WidgetTester tester, {
  bool canAct = true,
  bool failSend = false,
  LeadDetail? detail,
  List<EmailTemplate>? templates,
  List<FunnelModel> funnels = const [],
  String? funnelId,
  List<TeamMemberModel> team = const [],
  VoidCallback? onBoardBuild,
  LeadPark? park,
}) async {
  tester.view.physicalSize = const Size(500, 1400);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  final actions = _FakeActions(failSend: failSend);
  await tester.pumpWidget(ProviderScope(
    overrides: [
      leadDetailProvider('lead1')
          .overrideWith((ref) async => detail ?? _detail()),
      leadActionsProvider.overrideWithValue(actions),
      dioProvider.overrideWithValue(Dio()),
      // The CNES card would otherwise hit the bare Dio above.
      leadParkProvider.overrideWith((ref, id) async => park),
      emailTemplatesProvider.overrideWith((ref) async => templates ?? []),
      funnelsListProvider.overrideWith(() => _FakeFunnels(funnels)),
      teamMembersProvider.overrideWith(() => _FakeTeam(team)),
      boardProvider('f1').overrideWith((ref) async {
        onBoardBuild?.call();
        return const Board(stages: [], cards: []);
      }),
    ],
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      home: Scaffold(
        body: Column(
          children: [
            // A real listener for the board: invalidating a provider nobody
            // watches rebuilds nothing, and the test would pass without proving
            // anything.
            Consumer(builder: (context, ref, _) {
              ref.watch(boardProvider('f1'));
              return const SizedBox.shrink();
            }),
            Expanded(
              child: LeadDetailPanel(
                card: _card,
                stageName: 'Novos',
                funnelId: funnelId,
                canAct: canAct,
              ),
            ),
          ],
        ),
      ),
    ),
  ));
  await tester.pumpAndSettle();
  return actions;
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  testWidgets('shows the lead, its profile and the existing draft',
      (tester) async {
    await _pump(tester);

    expect(find.byKey(const Key('lead_detail_title')), findsOneWidget);
    // Label and value are styled differently now — the label is muted so the
    // eye lands on the fact, not on the field name — so the row is one
    // RichText rather than one string.
    expect(find.textContaining('Clínica Bem Viver', findRichText: true),
        findsWidgets);
    // The draft lands in the editor, not in read-only text: the human is
    // expected to change it before it goes anywhere.
    final field = tester
        .widget<TextField>(find.byKey(const Key('lead_detail_approach_field')));
    expect(field.controller!.text, 'Olá, tudo bem?');
  });

  testWidgets('the CNES box names the month its counts are from',
      (tester) async {
    await _pump(
      tester,
      park: const LeadPark(
        cnes: '2655411',
        tradeName: 'Hospital São Mateus',
        matchedBy: 'phone',
        items: ['2x Tomógrafo Computadorizado', '240x código CNES 0565'],
        competencia: '2026-07',
      ),
    );

    expect(find.byKey(const Key('lead_panel_cnes_park')), findsOneWidget);
    // DATASUS lags 45–75 days: "07/2026", never "hoje".
    expect(find.byKey(const Key('lead_panel_cnes_month')), findsOneWidget);
    expect(find.text('dados CNES de 07/2026'), findsOneWidget);
    // An unnamed code stays a code on screen — never a guessed name.
    expect(find.textContaining('240x código CNES 0565'), findsOneWidget);
  });

  // "Onde mostra o que posso oferecer pra cada cliente?" — the answer used to
  // be "nowhere, until you spend a draft". The pairing is a word match the
  // server does for free, so it belongs on the panel.
  testWidgets('the panel says what to offer, before any draft', (tester) async {
    await _pump(
      tester,
      park: const LeadPark(
        cnes: '5558638',
        tradeName: 'HOSPITAL CENTRAL',
        matchedBy: 'cnes',
        items: ['4x Incubadora', '2x Equipamento De Fototerapia'],
        competencia: '2026-07',
        matches: [
          ParkMatch(
              productId: 'prod-1',
              productName: 'Incubadora Line Duo G',
              equipment: 'Incubadora',
              qtExist: 4),
        ],
      ),
    );

    expect(find.byKey(const Key('lead_panel_offer_Incubadora Line Duo G')),
        findsOneWidget);
    // It names the product, the count and what they own — not "há oportunidade".
    expect(find.textContaining('Incubadora Line Duo G'), findsWidgets);
    expect(find.textContaining('4 aparelhos'), findsWidgets);
    // And the product NAME opens its page. Nobody knows a hundred-item
    // catalogue by heart — that is the whole reason the crossing exists, and
    // naming something with nowhere to read it leaves the seller guessing at
    // the specification in front of a buyer.
    expect(
      find.bySemanticsLabel(
          t.board.panel_offer_open(product: 'Incubadora Line Duo G')),
      findsOneWidget,
    );
  });

  // An older server sends no product id. The line stays a sentence rather than
  // becoming a link to nowhere.
  testWidgets('a match with no product id is plain text', (tester) async {
    await _pump(
      tester,
      park: const LeadPark(
        cnes: '5558638',
        tradeName: 'HOSPITAL CENTRAL',
        matchedBy: 'cnes',
        items: ['4x Incubadora'],
        matches: [
          ParkMatch(
              productName: 'Incubadora Line Duo G',
              equipment: 'Incubadora',
              qtExist: 4),
        ],
      ),
    );

    expect(find.textContaining('Incubadora Line Duo G'), findsWidgets);
    expect(
      find.bySemanticsLabel(
          t.board.panel_offer_open(product: 'Incubadora Line Duo G')),
      findsNothing,
    );
  });

  // Nothing matching means no block, not an empty promise.
  testWidgets('no matching product renders no offer block', (tester) async {
    await _pump(
      tester,
      park: const LeadPark(
        cnes: '5558638',
        tradeName: 'HOSPITAL CENTRAL',
        matchedBy: 'cnes',
        items: ['2x Tomógrafo Computadorizado'],
      ),
    );
    expect(find.byKey(const Key('lead_panel_cnes_park')), findsOneWidget);
    expect(find.textContaining(t.board.panel_offer_title), findsNothing);
  });

  test('a park code the dictionary does not name renders as the raw code', () {
    expect(
        leadParkItemLabel(
            {'equip_code': '0111', 'name': 'Tomógrafo Computadorizado'}),
        'Tomógrafo Computadorizado');
    expect(
        leadParkItemLabel({'equip_code': '1103', 'name': 'código CNES 1103'}),
        'código CNES 1103');
    // A 000219 group-only row: the official group rides along, the name
    // is still the code.
    expect(
        leadParkItemLabel({
          'equip_code': '0565',
          'name': 'Código 0565',
          'group_name': 'Equipamentos De Manutenção Da Vida',
        }),
        'código CNES 0565 (Equipamentos De Manutenção Da Vida)');
  });

  testWidgets('regenerating replaces the draft in place', (tester) async {
    final actions = await _pump(tester);

    await tester.tap(find.byKey(const Key('lead_detail_regenerate')));
    await tester.pumpAndSettle();

    expect(actions.drafts, 1);
    final field = tester
        .widget<TextField>(find.byKey(const Key('lead_detail_approach_field')));
    expect(field.controller!.text, 'rascunho novo');
  });

  // The one that must never regress: approving opens a confirmation that
  // quotes the message AND names the recipient.
  testWidgets('sending confirms with the exact text and the recipient',
      (tester) async {
    final actions = await _pump(tester);

    await tester.enterText(
        find.byKey(const Key('lead_detail_approach_field')), 'Texto revisado');
    await tester.tap(find.byKey(const Key('lead_detail_approve')));
    await tester.pumpAndSettle();

    // Nothing sent yet — the dialog is the gate.
    expect(actions.sends, 0);
    expect(find.textContaining('Texto revisado'), findsWidgets);
    expect(find.textContaining('Clínica Bem Viver'), findsWidgets);
    // ...and the number. A name cannot catch a wrong recipient: two
    // clinics can share one, and a mistyped number keeps whatever name
    // was typed alongside it.
    expect(find.textContaining('+5511988770001'), findsWidgets);
  });

  // Prospected contacts often have no name at all. The confirmation must
  // still say where the message is going, and must not print a separator
  // with nothing on one side of it.
  testWidgets('a nameless lead confirms with just the address', (tester) async {
    await _pump(
      tester,
      detail: const LeadDetail(
        id: 'lead1',
        status: 'enriched',
        approachDraft: 'Olá',
        recipient: '+5511988770001',
        recipientAddress: '+5511988770001',
        facts: <LeadFact>[],
        timeline: <String>[],
      ),
    );

    await tester.tap(find.byKey(const Key('lead_detail_approve')));
    await tester.pumpAndSettle();

    expect(find.textContaining('+5511988770001'), findsWidgets);
    expect(find.textContaining('—'), findsNothing);
  });

  testWidgets('cancelling the confirmation sends nothing', (tester) async {
    final actions = await _pump(tester);

    await tester.tap(find.byKey(const Key('lead_detail_approve')));
    await tester.pumpAndSettle();
    await tester.tap(find.text(t.common.cancel));
    await tester.pumpAndSettle();

    expect(actions.sends, 0);
  });

  testWidgets('confirming sends the EDITED text, not the AI draft',
      (tester) async {
    final actions = await _pump(tester);

    await tester.enterText(
        find.byKey(const Key('lead_detail_approach_field')), 'Texto revisado');
    await tester.tap(find.byKey(const Key('lead_detail_approve')));
    await tester.pumpAndSettle();
    await tester.tap(find.text(t.lead_detail.approve_send).last);
    await tester.pumpAndSettle();

    expect(actions.sends, 1);
    expect(actions.sentText, 'Texto revisado');
  });

  testWidgets('a failed send says so instead of pretending it went out',
      (tester) async {
    await _pump(tester, failSend: true);

    await tester.tap(find.byKey(const Key('lead_detail_approve')));
    await tester.pumpAndSettle();
    await tester.tap(find.text(t.lead_detail.approve_send).last);
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('lead_detail_send_error')), findsOneWidget);
  });

  testWidgets('an empty draft cannot be sent', (tester) async {
    final actions = await _pump(tester, detail: _detail(draft: ''));

    await tester.tap(find.byKey(const Key('lead_detail_approve')));
    await tester.pumpAndSettle();

    expect(actions.sends, 0);
    // No confirmation either: there is nothing to confirm.
    expect(find.text(t.lead_detail.confirm_title), findsNothing);
  });

  // The button used to look ready over an empty box and then swallow the
  // click — `_approveSend` returned on the spot and said nothing. A live
  // button that does nothing reads as a broken product, and this one carries
  // the highest-stakes action on the screen.
  testWidgets('the send button is dead until there is something to send',
      (tester) async {
    await _pump(tester, detail: _detail(draft: ''));

    FilledButton approve() => tester
        .widget<FilledButton>(find.byKey(const Key('lead_detail_approve')));

    expect(approve().onPressed, isNull);

    await tester.enterText(
        find.byKey(const Key('lead_detail_approach_field')), 'oi');
    await tester.pump();

    expect(approve().onPressed, isNotNull);

    // Whitespace is not a message.
    await tester.enterText(
        find.byKey(const Key('lead_detail_approach_field')), '   ');
    await tester.pump();

    expect(approve().onPressed, isNull);
  });

  // "Gerar outra" in front of an empty box names a draft that was never
  // written, and hides the one button that produces the first one.
  testWidgets('the generate button says which one it is', (tester) async {
    await _pump(tester, detail: _detail(draft: ''));

    expect(find.text(t.lead_detail.generate), findsOneWidget);
    expect(find.text(t.lead_detail.regenerate), findsNothing);

    await tester.enterText(
        find.byKey(const Key('lead_detail_approach_field')), 'oi');
    await tester.pump();

    expect(find.text(t.lead_detail.regenerate), findsOneWidget);
    expect(find.text(t.lead_detail.generate), findsNothing);
  });

  // A vendedor looking at a teammate's lead: the server refuses anyway, so
  // showing the button would only teach them to expect an error.
  // The approval queue shows the text's check; this panel, which ALSO sends with
  // the same button, sent without it. Two paths to one action, one with a net and
  // one without — and the netless one is what somebody reaches for first: open the
  // lead, generate, send.
  testWidgets('the draft comes with the same check as the queue',
      (tester) async {
    await _pump(tester,
        detail: _detail(findings: const [
          ApproachFinding(kind: 'promise', quote: 'Garantimos retorno em 24h'),
        ]));

    expect(find.byKey(const Key('lead_detail_verdict')), findsOneWidget);
    expect(find.textContaining('Garantimos retorno em 24h'), findsOneWidget);
  });

  // "No pattern found" is not "reviewed and approved": the check is of known
  // patterns, and promising a reading nobody did would be worse than saying
  // nothing.
  testWidgets('with no finding, the text says what the check did NOT see',
      (tester) async {
    await _pump(tester);

    expect(find.text(t.lead_detail.check_clean), findsOneWidget);
  });

  // With no draft there is nothing to check — and a verdict over emptiness would
  // be a seal of approval on nothing.
  testWidgets('an empty box gets no verdict', (tester) async {
    await _pump(tester, detail: _detail(draft: ''));

    expect(find.byKey(const Key('lead_detail_verdict')), findsNothing);
  });

  // `findings` is the reading of the SAVED draft, and the condition to show it
  // was the LOCAL text: writing a fresh promise into the field left the block
  // asserting "no pattern found" over text nobody checked. A green seal that
  // lies is what this block exists to prevent.
  testWidgets('edited text does not inherit the saved draft verdict',
      (tester) async {
    await _pump(tester);
    expect(find.text(t.lead_detail.check_clean), findsOneWidget);

    await tester.enterText(find.byKey(const Key('lead_detail_approach_field')),
        'Garantimos retorno em 24h');
    await tester.pump();

    expect(find.byKey(const Key('lead_detail_verdict')), findsNothing);
    expect(find.text(t.lead_detail.check_clean), findsNothing);
    expect(find.byKey(const Key('lead_detail_verdict_edited')), findsOneWidget);
  });

  // Returning to the saved text gives the check back: it holds for THIS text
  // again, and hiding it then would hide a real finding.
  testWidgets('returning to the saved text restores the verdict',
      (tester) async {
    await _pump(tester,
        detail: _detail(draft: 'Olá, tudo bem?', findings: const [
          ApproachFinding(kind: 'promise', quote: 'Garantimos retorno em 24h'),
        ]));

    await tester.enterText(
        find.byKey(const Key('lead_detail_approach_field')), 'outro texto');
    await tester.pump();
    expect(find.byKey(const Key('lead_detail_verdict')), findsNothing);

    await tester.enterText(
        find.byKey(const Key('lead_detail_approach_field')), 'Olá, tudo bem?');
    await tester.pump();

    expect(find.byKey(const Key('lead_detail_verdict')), findsOneWidget);
    expect(find.byKey(const Key('lead_detail_verdict_edited')), findsNothing);
  });

  // The funnel's required fields are what makes a lead qualified, and only the AI
  // could answer them, during a conversation: no screen called
  // PATCH /leads/:id/metadata. A lead that arrived by referral could be qualified
  // by nobody.
  group('funnel fields in the panel', () {
    const schema = [
      MetadataFieldModel(key: 'convenio', label: 'Convênio'),
    ];

    testWidgets('os campos aparecem com o rótulo do funil', (tester) async {
      await _pump(tester, funnelId: 'f1', funnels: const [
        FunnelModel(id: 'f1', name: 'Funil', metadataSchema: schema)
      ]);

      expect(find.byKey(const Key('lead_funnel_fields')), findsOneWidget);
      expect(find.text('Convênio'), findsOneWidget);
    });

    testWidgets('filling one saves that field', (tester) async {
      final actions = await _pump(tester, funnelId: 'f1', funnels: const [
        FunnelModel(id: 'f1', name: 'Funil', metadataSchema: schema)
      ]);

      await tester.enterText(
          find.byKey(const Key('metadata_field_convenio')), 'Unimed');
      await tester.pumpAndSettle();

      expect(actions.savedFields['convenio'], 'Unimed');
    });

    // Someone who may only read is not given a form the server will refuse.
    testWidgets('a reader gets no form', (tester) async {
      await _pump(tester, canAct: false, funnelId: 'f1', funnels: const [
        FunnelModel(id: 'f1', name: 'Funil', metadataSchema: schema)
      ]);

      expect(find.byKey(const Key('lead_funnel_fields')), findsNothing);
    });

    // Opened from somewhere that does not know the funnel, the section does not
    // appear — rather than appearing empty or with the wrong funnel's fields.
    testWidgets('with no known funnel, no section', (tester) async {
      await _pump(tester);

      expect(find.byKey(const Key('lead_funnel_fields')), findsNothing);
    });
  });

  // The board marked every card "no owner" in red and nothing could change that:
  // AssignOwner existed in the use case and had no route. An alarm nobody can
  // switch off.
  // The board behind the panel only refreshed on CLOSE, so assigning somebody
  // left the card still reading "no owner" beside a picker already showing the
  // name. One screen asserting both.
  testWidgets('changing the owner reloads the board, not just the panel',
      (tester) async {
    var boardBuilds = 0;
    final actions = await _pump(
      tester,
      funnelId: 'f1',
      team: [_member('u1', 'Ana')],
      onBoardBuild: () => boardBuilds++,
    );
    final before = boardBuilds;

    await tester.tap(find.byKey(const Key('lead_owner_picker')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Ana').last);
    await tester.pumpAndSettle();

    expect(actions.assignedOwner, 'u1');
    expect(boardBuilds, greaterThan(before),
        reason: 'o quadro atrás precisa saber que o dono mudou');
  });

  // The top read the CARD — the snapshot the board held when the panel OPENED.
  // Assigning somebody left a red "No owner" contradicting the picker just below
  // it, on the same screen, at the same time.
  testWidgets('the top does not repeat the missing owner in red',
      (tester) async {
    await _pump(tester, team: [_member('u1', 'Ana')]);

    expect(find.text(t.board.unassigned), findsNothing);
    expect(find.byKey(const Key('lead_owner_picker')), findsOneWidget);
  });

  group('responsável', () {
    testWidgets('a teammate can be picked', (tester) async {
      await _pump(tester, team: [_member('u1', 'Ana'), _member('u2', 'Bruno')]);

      expect(find.byKey(const Key('lead_owner_picker')), findsOneWidget);
      await tester.tap(find.byKey(const Key('lead_owner_picker')));
      await tester.pumpAndSettle();

      expect(find.text('Ana'), findsWidgets);
      expect(find.text('Bruno'), findsWidgets);
    });

    testWidgets('escolher grava o dono', (tester) async {
      final actions = await _pump(tester, team: [_member('u1', 'Ana')]);

      await tester.tap(find.byKey(const Key('lead_owner_picker')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Ana').last);
      await tester.pumpAndSettle();

      expect(actions.assignedOwner, 'u1');
    });

    // Assigning work to somebody who left the company is the same as losing it —
    // and the endpoint returns deactivated members alongside active ones.
    testWidgets('a deactivated member is not in the list', (tester) async {
      await _pump(tester, team: [
        _member('u1', 'Ana'),
        _member('u2', 'Saiu', active: false),
      ]);

      await tester.tap(find.byKey(const Key('lead_owner_picker')));
      await tester.pumpAndSettle();

      expect(find.text('Saiu'), findsNothing);
    });

    // Without that option, a lead assigned by mistake stays assigned forever.
    testWidgets('it can be taken from everybody', (tester) async {
      await _pump(tester, team: [_member('u1', 'Ana')]);

      await tester.tap(find.byKey(const Key('lead_owner_picker')));
      await tester.pumpAndSettle();

      expect(find.text(t.board.owner_none), findsWidgets);
    });

    testWidgets('a reader does not pick an owner', (tester) async {
      await _pump(tester, canAct: false, team: [_member('u1', 'Ana')]);

      expect(find.byKey(const Key('lead_owner_block')), findsNothing);
    });
  });

  // The `private` filter has existed since the migration and ran over a column
  // that was always 'team', because nothing in the product could write it.
  group('visibilidade', () {
    testWidgets('with no owner there is nothing to hide', (tester) async {
      await _pump(tester, team: [_member('u1', 'Ana')]);

      expect(find.byKey(const Key('lead_visibility_toggle')), findsNothing);
    });

    testWidgets('with an owner, it can be made private', (tester) async {
      final actions = await _pump(
        tester,
        team: [_member('u1', 'Ana')],
        detail: _detail(ownerUserId: 'u1'),
      );

      await tester.tap(find.byKey(const Key('lead_visibility_toggle')));
      await tester.pumpAndSettle();

      expect(actions.setVisibilityTo, 'private');
    });
  });

  // A partner's name read off a page and one the model inferred looked alike.
  // Whoever is about to cite it needs to know which of the two it is.
  group('procedência do perfil', () {
    testWidgets('the sources show by domain', (tester) async {
      await _pump(tester,
          detail: _detail(sources: const [
            'https://www.linkedin.com/in/fulano',
            'https://clinicaexemplo.com.br/sobre',
          ]));

      expect(find.byKey(const Key('lead_detail_sources')), findsOneWidget);
      expect(find.text('linkedin.com'), findsOneWidget);
      expect(find.text('clinicaexemplo.com.br'), findsOneWidget);
    });

    testWidgets('with no sources, no section', (tester) async {
      await _pump(tester);

      expect(find.byKey(const Key('lead_detail_sources')), findsNothing);
    });
  });

  testWidgets('a read-only viewer gets no send button', (tester) async {
    await _pump(tester, canAct: false);

    expect(find.byKey(const Key('lead_detail_approve')), findsNothing);
    expect(find.byKey(const Key('lead_detail_regenerate')), findsNothing);
    expect(find.byKey(const Key('lead_detail_read_only')), findsOneWidget);
  });

  // ── What the panel was missing ──────────────────────────────────────────
  //
  // Reported from the screen: "when I click it comes up to talk about the
  // approach, but that's all — it says nothing else about the customer".

  // The address was in the model and rendered nowhere, so deciding whether to
  // phone someone meant starting a send just to read the confirmation dialog.
  testWidgets('the panel shows where the message would go', (tester) async {
    await _pump(tester);

    expect(find.byKey(const Key('lead_detail_address')), findsOneWidget);
    expect(find.byKey(const Key('lead_detail_call')), findsOneWidget);
  });

  // An email address is not something to dial.
  testWidgets('an email contact is not offered a call button', (tester) async {
    await _pump(
      tester,
      detail: const LeadDetail(
        id: 'lead1',
        status: 'enriched',
        approachDraft: 'Olá',
        recipient: 'compras@clinica.com.br',
        recipientAddress: 'compras@clinica.com.br',
        facts: <LeadFact>[],
        timeline: <String>[],
      ),
    );

    expect(find.byKey(const Key('lead_detail_address')), findsOneWidget);
    expect(find.byKey(const Key('lead_detail_call')), findsNothing);
  });

  // The old panel dropped the whole profile section for an unenriched lead:
  // no heading, no explanation, no way to fill it — even though the endpoint
  // that fills it already existed.
  testWidgets('an unresearched lead says so, and offers to look it up',
      (tester) async {
    await _pump(tester, detail: _detail(facts: const []));

    expect(find.byKey(const Key('lead_detail_profile_empty')), findsOneWidget);
    expect(find.text(t.lead_detail.profile_empty_hint), findsOneWidget);
    expect(find.byKey(const Key('lead_detail_enrich')), findsOneWidget);
  });

  // The research button stays available on a researched lead too: enrichment
  // goes stale, and a company that moved is worth looking up again.
  testWidgets('a researched lead still offers to look it up again',
      (tester) async {
    await _pump(tester);

    expect(find.byKey(const Key('lead_detail_profile_empty')), findsNothing);
    expect(find.byKey(const Key('lead_detail_enrich')), findsOneWidget);
  });

  // I collapsed this section by default reasoning that "whoever opens a lead to
  // act does not need the facts in front of the button". The real use is the
  // opposite: you open a lead to find out WHO IT IS. Collapsed, the panel
  // answered "3 facts about the company" to someone asking which.
  testWidgets('the profile starts open, with the facts in view',
      (tester) async {
    await _pump(tester);

    expect(find.textContaining('Diretora', findRichText: true), findsWidgets);
    expect(find.byKey(const Key('lead_detail_enrich')), findsOneWidget);
  });

  // Enrichment writes linkedin_url/instagram_url and no funnel screen read them
  // — only the old lead list did. The search found the decider's LinkedIn and
  // the panel made it disappear.
  testWidgets('the decision-maker profile shows when the search found it',
      (tester) async {
    await _pump(tester,
        detail: _detail(
          linkedinUrl: 'https://linkedin.com/in/diretora',
          instagramUrl: 'https://instagram.com/clinica',
        ));

    expect(find.byKey(const Key('lead_detail_linkedin')), findsOneWidget);
    expect(find.byKey(const Key('lead_detail_instagram')), findsOneWidget);
  });

  // `company_name` is the prospecting SEED — `withCompanyName` injects it
  // before the search runs. Measuring "researched" by `facts` made every
  // prospected lead start out enriched: `researchedNothing` was never true, the
  // sentence "we searched and found nothing" never appeared, and the lead
  // changed column with the screen saying nothing more.
  testWidgets('a search that only found the seed says it found nothing',
      (tester) async {
    await _pump(tester,
        detail: _detail(
          facts: const [
            LeadFact(label: 'Empresa', value: 'Clínica Bem Viver'),
            LeadFact(label: 'Endereço', value: 'Rua X, 123'),
          ],
          researchedAt: '2026-08-01T10:00:00Z',
        ));

    expect(find.byKey(const Key('lead_detail_profile_found_nothing')),
        findsOneWidget);
    // And the facts already known stay on screen: saying "found nothing" must
    // not cost hiding the name and the address.
    expect(find.textContaining('Clínica Bem Viver', findRichText: true),
        findsWidgets);
  });

  // The only writers of the draft were the AI and the send: whoever typed their
  // own text and closed the panel lost all of it, because the words lived in a
  // screen field and nowhere else.
  testWidgets('what was written is stored without waiting for a send',
      (tester) async {
    final actions = await _pump(tester);

    await tester.enterText(find.byKey(const Key('lead_detail_approach_field')),
        'Oi, vi que vocês atendem convênio');
    await tester.pump(const Duration(seconds: 2));

    expect(actions.savedDrafts, contains('Oi, vi que vocês atendem convênio'));
  });

  // Closing was the gesture that lost the text — that is what the test has to
  // cover.
  testWidgets('closing the panel writes the last stretch', (tester) async {
    final actions = await _pump(tester);

    await tester.enterText(
        find.byKey(const Key('lead_detail_approach_field')), 'quase pronto');
    // Without waiting for the debounce: leaving cannot depend on the timer
    // having run. Taking the panel out of the tree IS what closing does — and
    // `dispose` is the last point at which the text still exists.
    await tester.pumpWidget(const SizedBox());
    await tester.pump();

    expect(actions.savedDrafts, contains('quase pronto'));
  });

  // Text identical to what is stored is not worth a write: opening and closing
  // without editing is the common case.
  testWidgets('opening and closing without editing writes nothing',
      (tester) async {
    final actions = await _pump(tester);

    await tester.pumpWidget(const SizedBox());
    await tester.pump();

    expect(actions.savedDrafts, isEmpty);
  });

  // Someone who may only read does not write — not even via autosave.
  testWidgets('read-only does not store a draft', (tester) async {
    final actions = await _pump(tester, canAct: false);
    await tester.pump(const Duration(seconds: 2));

    expect(actions.savedDrafts, isEmpty);
  });

  // Archaeology: whoever acts does not need it, whoever inherited the lead
  // needs it badly.
  testWidgets('the history starts folded, counting its events', (tester) async {
    await _pump(tester);

    expect(find.text('Criado em 01/07/2026 10:00'), findsNothing);
    expect(find.text(t.lead_detail.timeline_events(count: 1)), findsOneWidget);

    await tester.tap(find.byKey(const Key('lead_section_timeline')));
    await tester.pumpAndSettle();

    expect(find.text('Criado em 01/07/2026 10:00'), findsOneWidget);
  });

  // Four different facts used to render one sentence. Found live: the board
  // had already moved the lead to "Pesquisados" while the panel still said
  // nobody had ever looked it up.
  testWidgets('a research that found nothing does not read as untouched',
      (tester) async {
    await _pump(
      tester,
      detail: _detail(facts: const [], researchedAt: '2026-07-31T10:40:00Z'),
    );

    expect(find.byKey(const Key('lead_detail_profile_found_nothing')),
        findsOneWidget);
    expect(find.byKey(const Key('lead_detail_profile_empty')), findsNothing);
    // And the button no longer offers work the product already did.
    expect(find.text(t.lead_detail.enrich_again), findsOneWidget);
  });

  testWidgets('a lead with nothing to search for says what is missing',
      (tester) async {
    await _pump(
      tester,
      detail: _detail(
          facts: const [], researchBlockedReason: 'no_company_or_website'),
    );

    expect(
        find.byKey(const Key('lead_detail_profile_blocked')), findsOneWidget);
    expect(find.text(t.lead_detail.profile_blocked_hint), findsOneWidget);
  });

  testWidgets('a research in flight says so and cannot be started twice',
      (tester) async {
    await _pump(
      tester,
      detail: _detail(facts: const [], status: 'enriching'),
    );

    expect(find.byKey(const Key('lead_detail_profile_researching')),
        findsOneWidget);
    final button = tester
        .widget<OutlinedButton>(find.byKey(const Key('lead_detail_enrich')));
    expect(button.onPressed, isNull);
  });

  testWidgets('a lead with no conversation says so instead of dead-ending',
      (tester) async {
    await _pump(tester);

    expect(
        find.byKey(const Key('lead_detail_no_conversation')), findsOneWidget);
    expect(
        find.byKey(const Key('lead_detail_open_conversation')), findsNothing);
  });

  testWidgets('a lead with a thread can reach it', (tester) async {
    await _pump(tester, detail: _detail(conversationId: 'conv-1'));

    expect(
        find.byKey(const Key('lead_detail_open_conversation')), findsOneWidget);
  });

  // ── Email path ──────────────────────────────────────────────────────────
  //
  // These pin the send-side half of the tracking loop: a reply is only
  // countable if the send said which template it came from.

  const _emailLead = 'dono@clinicabemviver.com.br';

  final _templates = [
    const EmailTemplate(
      id: 'tpl1',
      name: 'Primeiro contato — clínicas',
      subject: 'Uma ideia para a Clínica Bem Viver',
      body: 'Bom dia! Vi que vocês atendem convênios.',
      version: 3,
      stats: [
        TemplateStats(
            version: 3,
            sent: 40,
            opened: 20,
            clicked: 4,
            replied: 6,
            bounced: 0),
      ],
    ),
  ];

  testWidgets('an email lead gets a subject field and a template picker',
      (tester) async {
    await _pump(tester,
        detail: _detail(address: _emailLead), templates: _templates);

    expect(find.byKey(const Key('lead_detail_subject_field')), findsOneWidget);
    expect(
        find.byKey(const Key('lead_detail_template_picker')), findsOneWidget);
  });

  // A phone lead has neither: a subject would be meaningless and an email
  // template would send the wrong words.
  testWidgets('a phone lead gets neither', (tester) async {
    await _pump(tester, templates: _templates);

    expect(find.byKey(const Key('lead_detail_subject_field')), findsNothing);
    expect(find.byKey(const Key('lead_detail_template_picker')), findsNothing);
  });

  // An empty dropdown is a dead control that makes the panel look broken.
  testWidgets('no templates means no picker at all', (tester) async {
    await _pump(tester, detail: _detail(address: _emailLead));

    expect(find.byKey(const Key('lead_detail_subject_field')), findsOneWidget);
    expect(find.byKey(const Key('lead_detail_template_picker')), findsNothing);
  });

  testWidgets('picking a template fills the subject and the body',
      (tester) async {
    await _pump(tester,
        detail: _detail(address: _emailLead), templates: _templates);

    await tester.tap(find.byKey(const Key('lead_detail_template_picker')));
    await tester.pumpAndSettle();
    await tester.tap(find.textContaining('Primeiro contato').last);
    await tester.pumpAndSettle();

    expect(
      tester
          .widget<TextField>(find.byKey(const Key('lead_detail_subject_field')))
          .controller!
          .text,
      'Uma ideia para a Clínica Bem Viver',
    );
    expect(
      tester
          .widget<TextField>(
              find.byKey(const Key('lead_detail_approach_field')))
          .controller!
          .text,
      'Bom dia! Vi que vocês atendem convênios.',
    );
  });

  // The whole point of the picker: the send carries the template VERSION, so
  // a reply days later credits the exact words that earned it.
  testWidgets('the send carries the template id and version', (tester) async {
    final actions = await _pump(tester,
        detail: _detail(address: _emailLead), templates: _templates);

    await tester.tap(find.byKey(const Key('lead_detail_template_picker')));
    await tester.pumpAndSettle();
    await tester.tap(find.textContaining('Primeiro contato').last);
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('lead_detail_approve')));
    await tester.pumpAndSettle();
    await tester.tap(find.text(t.lead_detail.approve_send).last);
    await tester.pumpAndSettle();

    expect(actions.sends, 1);
    expect(actions.sentTemplateId, 'tpl1');
    expect(actions.sentTemplateVersion, 3);
    expect(actions.sentSubject, 'Uma ideia para a Clínica Bem Viver');
  });

  // Editing the text keeps the attribution. An edited template is still that
  // template's idea, and dropping the id on the first keystroke would leave
  // essentially every real send unattributed.
  testWidgets('editing the text keeps the template attribution',
      (tester) async {
    final actions = await _pump(tester,
        detail: _detail(address: _emailLead), templates: _templates);

    await tester.tap(find.byKey(const Key('lead_detail_template_picker')));
    await tester.pumpAndSettle();
    await tester.tap(find.textContaining('Primeiro contato').last);
    await tester.pumpAndSettle();
    await tester.enterText(
        find.byKey(const Key('lead_detail_approach_field')), 'Texto meu');
    await tester.tap(find.byKey(const Key('lead_detail_approve')));
    await tester.pumpAndSettle();
    await tester.tap(find.text(t.lead_detail.approve_send).last);
    await tester.pumpAndSettle();

    expect(actions.sentText, 'Texto meu');
    expect(actions.sentTemplateId, 'tpl1');
  });

  // A message written from scratch has no template, and must still send.
  testWidgets('sending without a template sends nothing to attribute',
      (tester) async {
    final actions = await _pump(tester,
        detail: _detail(address: _emailLead), templates: _templates);

    await tester.enterText(
        find.byKey(const Key('lead_detail_approach_field')), 'Escrito na hora');
    await tester.tap(find.byKey(const Key('lead_detail_approve')));
    await tester.pumpAndSettle();
    await tester.tap(find.text(t.lead_detail.approve_send).last);
    await tester.pumpAndSettle();

    expect(actions.sends, 1);
    expect(actions.sentTemplateId, isNull);
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

TeamMemberModel _member(String id, String name, {bool active = true}) =>
    TeamMemberModel(
      id: id,
      name: name,
      email: '$id@x.com',
      role: 'attendant',
      isActive: active,
      isEmailVerified: true,
      createdAt: '2026-01-01T00:00:00Z',
      updatedAt: '2026-01-01T00:00:00Z',
    );
