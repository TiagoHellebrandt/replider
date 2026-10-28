import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/design_system/components/gradient_pill.dart';
import 'package:app/features/sales/domain/models/approach_review.dart';
import 'package:app/features/sales/domain/models/funnel_model.dart';
import 'package:app/features/sales/presentation/controllers/approval_queue_controller.dart';
import 'package:app/features/sales/presentation/controllers/funnels_controller.dart';
import 'package:app/features/sales/presentation/controllers/lead_detail_controller.dart';
import 'package:app/features/sales/presentation/screens/approval_queue_screen.dart';
import 'package:app/features/settings/domain/models/channel_model.dart';
import 'package:app/features/settings/domain/models/whatsapp_template_model.dart';
import 'package:app/features/settings/presentation/controllers/channels_controller.dart';
import 'package:app/features/settings/presentation/controllers/whatsapp_config_controller.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

// US-M020. The queue is the human gate on cold outbound, so the tests pin
// both halves of its promise: it is FAST (shortcuts, counter, undo) and it
// never sends without an explicit confirmation showing text + recipient.

const _risky = [
  QueueLead(
    id: 'lead1',
    recipient: 'Clínica Bem Viver',
    recipientAddress: '+5511900000001',
    draft: 'Garantimos atendimento em até 24 horas.',
    companyName: 'Clínica Bem Viver',
    source: 'google_places',
    findings: [
      ApproachFinding(kind: 'promise', quote: 'Garantimos'),
      ApproachFinding(kind: 'promise', quote: 'em até 24 horas'),
    ],
  ),
];

const _queue = [
  QueueLead(
    id: 'lead1',
    recipient: 'Clínica Bem Viver',
    recipientAddress: '+5511900000001',
    draft: 'Olá, vi que a clínica atende ortopedia…',
    companyName: 'Clínica Bem Viver',
    source: 'speedio',
  ),
  QueueLead(
    id: 'lead2',
    recipient: 'contato@hospital.com',
    recipientAddress: 'contato@hospital.com',
    draft: 'Boa tarde, trabalhamos com agendamento…',
    companyName: 'Hospital São Lucas',
  ),
];

/// The batch-action fake shared with the harness override; reset per test.
_FakeBatch? _batch;

class _FakeBatch extends QueueBatchActions {
  _FakeBatch(super.ref);
  List<String> discarded = const [];
  List<String> redrafted = const [];

  @override
  Future<QueueBatchOutcome> discardAll(List<String> leadIds) async {
    discarded = leadIds;
    return QueueBatchOutcome(processed: leadIds.length, failed: 0);
  }

  @override
  Future<QueueBatchOutcome> redraftAll(List<String> leadIds,
      {void Function(int done, int total)? onProgress}) async {
    redrafted = leadIds;
    return QueueBatchOutcome(processed: leadIds.length, failed: 0);
  }
}

/// One queue entry, for the tests that need a specific draft or finding.
QueueLead _lead(
  String id, {
  String draft = 'texto',
  List<ApproachFinding> findings = const [],
}) =>
    QueueLead(
      id: id,
      recipient: 'Clínica $id',
      recipientAddress: '+551190000000$id',
      draft: draft,
      findings: findings,
    );

class _NoFunnelsList extends FunnelsList {
  @override
  Future<List<FunnelModel>> build() async => const [];
}

class _FakeActions implements LeadActions {
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
  }) async {}

  _FakeActions({this.discardDelay = Duration.zero, this.failDiscard = false});

  /// Lets a test hold the discard in flight — the window where a card used
  /// to pop back into the queue after its undo expired.
  final Duration discardDelay;
  final bool failDiscard;

  int sends = 0;
  int discards = 0;
  final discarded = <String>[];

  @override
  Future<String?> draft({required String leadId}) async => null;

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
    sentTexts.add(text);
    sentSubject = subject;
    sentTemplate = templateName;
    sentViaBulk.add(viaBulk);
    return null;
  }

  /// Whether each send claimed the bulk route — batch approvals must, and
  /// one-by-one approvals must not.
  final sentViaBulk = <bool>[];

  /// Every text that actually went out, so a test can prove WHICH ones did —
  /// a count alone cannot tell "sent both" from "sent the wrong one".
  final sentTexts = <String>[];

  /// What rode along with the last approval — the confirmation dialog's
  /// whole reason to exist.
  String? sentSubject;
  String? sentTemplate;

  /// What the AI pass will answer. Null makes the call fail, which is the
  /// interesting case: the screen must say the review did not run, never fall
  /// back to a green "clean".
  ApproachReview? reviewResult;
  int reviews = 0;

  @override
  Future<ApproachReview> review({
    required String leadId,
    required String text,
  }) async {
    reviews++;
    final r = reviewResult;
    if (r == null) throw StateError('review failed');
    return r;
  }

  final savedDrafts = <String>[];

  @override
  Future<void> saveDraft({
    required String leadId,
    required String text,
  }) async {
    savedDrafts.add(text);
  }

  @override
  Future<void> discard({required String leadId}) async {
    discards++;
    discarded.add(leadId);
    if (discardDelay > Duration.zero) await Future<void>.delayed(discardDelay);
    if (failDiscard) throw StateError('discard failed');
  }
}

/// The queue endpoint answers with a page plus the size of the backlog behind
/// it. Tests that only care about the cards get a page whose total matches what
/// they passed; the ones testing the filter build their own.
ApprovalQueuePage _asPage(List<QueueLead> items) =>
    ApprovalQueuePage(items: items, total: items.length, byFunnel: const []);

Future<_FakeActions> _pump(
  WidgetTester tester, {
  List<QueueLead> queue = _queue,
  ApprovalQueuePage? page,
  Duration discardDelay = Duration.zero,
  bool failDiscard = false,
  Size size = const Size(900, 1600),
  ApproachReview? reviewResult,
}) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  final actions =
      _FakeActions(discardDelay: discardDelay, failDiscard: failDiscard);
  actions.reviewResult = reviewResult;
  await tester.pumpWidget(ProviderScope(
    overrides: [
      approvalQueueProvider.overrideWith((ref) async => page ?? _asPage(queue)),
      leadActionsProvider.overrideWithValue(actions),
      // The channel-resolution helper reads funnels; a bare Dio here would
      // leave a real request Timer pending at teardown.
      funnelsListProvider.overrideWith(_NoFunnelsList.new),
      dioProvider.overrideWithValue(Dio()),
      // The outcome line and the batch actions would otherwise hit the bare
      // Dio above and leave request timers pending at teardown.
      offersOutcomeProvider.overrideWith((ref) async =>
          const OffersOutcome(sent: 12, replies: 3, proposals: 1)),
      queueBatchActionsProvider
          .overrideWith((ref) => _batch ??= _FakeBatch(ref)),
    ],
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      home: const ApprovalQueueScreen(),
    ),
  ));
  await tester.pumpAndSettle();
  return actions;
}

void main() {
  group('send blocked and the phone bar', _blockedQueueTests);
  group('rankable cards', _rankableCardTests);
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() {
    LocaleSettings.setLocale(AppLocale.pt);
    _batch = null;
  });

  _groupingTests();
  _scheduleTests();

  _filterTests();

  // WHICH product and WHY, above the text: an approver who disagrees with
  // the pick must not have to reverse-engineer it from the prose.
  testWidgets('a card names the matched product and the reason',
      (tester) async {
    await _pump(tester, queue: const [
      QueueLead(
        id: 'lead9',
        recipient: 'Hospital Regional',
        recipientAddress: 'compras@hospital.test',
        draft: 'Bom dia…',
        companyName: 'Hospital Regional',
        matchedProducts: [
          MatchedProduct(
              productId: 'p2',
              name: 'Monitor de ECG',
              reason: 'UTI nova pede monitor.'),
        ],
      ),
    ]);

    final row = find.byKey(const Key('approval_queue_product_lead9_p2'));
    expect(row, findsOneWidget);
    // Name + reason are spans of one RichText.
    // Name + reason are spans of one RichText (the Icon renders its own).
    final plain = tester
        .widgetList<RichText>(
            find.descendant(of: row, matching: find.byType(RichText)))
        .map((r) => r.text.toPlainText())
        .join(' ');
    expect(plain, contains('Monitor de ECG'));
    expect(plain, contains('UTI nova pede monitor.'));
  });

  test('matched_products parses from a list or a JSON string, dropping junk',
      () {
    final fromList = MatchedProduct.listFrom([
      {'product_id': 'p1', 'name': 'A', 'reason': 'r'},
      {'product_id': 'p2', 'name': ''},
      'garbage',
    ]);
    expect(fromList.map((p) => p.productId), ['p1']);
    final fromString =
        MatchedProduct.listFrom('[{"product_id":"p3","name":"C","reason":""}]');
    expect(fromString.single.name, 'C');
    expect(MatchedProduct.listFrom(null), isEmpty);
    expect(MatchedProduct.listFrom('not json'), isEmpty);
  });

  testWidgets('lists every pending draft with its recipient and full text',
      (tester) async {
    await _pump(tester);

    expect(find.byKey(const Key('approval_queue_card_lead1')), findsOneWidget);
    expect(find.byKey(const Key('approval_queue_card_lead2')), findsOneWidget);
    // The whole message, not a preview: approving a truncated text is not
    // approving what gets sent.
    expect(
        find.text('Olá, vi que a clínica atende ortopedia…'), findsOneWidget);
    expect(find.textContaining('+5511900000001'), findsOneWidget);
  });

  testWidgets('the counter says how much backlog is left', (tester) async {
    await _pump(tester);

    expect(
      tester.widget<Text>(find.byKey(const Key('approval_queue_counter'))).data,
      t.lead_detail.queue_remaining(count: 2),
    );
  });

  testWidgets('an empty queue explains how to fill it', (tester) async {
    await _pump(tester, queue: const []);

    expect(find.byKey(const Key('approval_queue_empty')), findsOneWidget);
    expect(find.text(t.lead_detail.queue_empty_hint), findsOneWidget);
    expect(find.byKey(const Key('approval_queue_list')), findsNothing);
  });

  // The gate: a click on Approve is not a send.
  testWidgets('approving confirms with the exact text and the recipient',
      (tester) async {
    final actions = await _pump(tester);

    await tester.tap(find.byKey(const Key('approval_queue_approve_lead1')));
    await tester.pumpAndSettle();

    expect(actions.sends, 0);
    expect(find.text(t.lead_detail.confirm_title), findsOneWidget);
    expect(find.textContaining('Olá, vi que a clínica atende ortopedia…'),
        findsWidgets);
    expect(find.textContaining('+5511900000001'), findsWidgets);

    await tester.tap(find.text(t.lead_detail.approve_send).last);
    await tester.pumpAndSettle();
    expect(actions.sends, 1);
    // One-by-one approvals go through the customer's own mailbox — never
    // the bulk provider (hybrid model).
    expect(actions.sentViaBulk, [false]);
  });

  // A card whose draft came back empty (generation failed, or the server
  // queued it before writing one) used to show a live Approve that returned
  // on the spot without a word.
  testWidgets('a card with no text to send has a dead approve button',
      (tester) async {
    await _pump(tester, queue: const [
      QueueLead(
        id: 'lead1',
        recipient: 'Clínica Bem Viver',
        recipientAddress: '+5511900000001',
        draft: '   ',
      ),
    ]);

    final button = tester.widget<GradientPill>(
        find.byKey(const Key('approval_queue_approve_lead1')));
    expect(button.onTap, isNull);
  });

  // Regression: a Container(alignment:) inside the pill once expanded it to
  // the card's full width, shoving Editar/Descartar onto their own line.
  testWidgets('the approve pill hugs its label instead of the card width',
      (tester) async {
    await _pump(tester);

    final pill =
        tester.getSize(find.byKey(const Key('approval_queue_approve_lead1')));
    final edit =
        tester.getSize(find.byKey(const Key('approval_queue_edit_lead1')));
    expect(pill.width, lessThan(360));
    expect(pill.height, 42);
    expect(edit.height, 42);
  });

  // Speed without removing the gate: the shortcut opens the SAME dialog.
  testWidgets('the A shortcut approves the highlighted card, still confirming',
      (tester) async {
    final actions = await _pump(tester);

    await tester.sendKeyEvent(LogicalKeyboardKey.keyA);
    await tester.pumpAndSettle();

    expect(actions.sends, 0, reason: 'a keystroke must never send by itself');
    expect(find.text(t.lead_detail.confirm_title), findsOneWidget);
  });

  testWidgets('arrow keys move the highlight to the next card', (tester) async {
    await _pump(tester);

    await tester.sendKeyEvent(LogicalKeyboardKey.arrowDown);
    await tester.pumpAndSettle();
    await tester.sendKeyEvent(LogicalKeyboardKey.keyX);
    await tester.pumpAndSettle();

    // The SECOND card left the list, so the highlight really moved.
    expect(find.byKey(const Key('approval_queue_card_lead2')), findsNothing);
    expect(find.byKey(const Key('approval_queue_card_lead1')), findsOneWidget);

    // Let the undo window close so no timer outlives the test.
    await tester.pump(ApprovalQueueScreen.undoWindow);
    await tester.pumpAndSettle();
  });

  testWidgets('discarding removes the card and offers an undo', (tester) async {
    final actions = await _pump(tester);

    await tester.tap(find.byKey(const Key('approval_queue_discard_lead1')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('approval_queue_card_lead1')), findsNothing);
    expect(
        find.byKey(const Key('approval_queue_discard_undo')), findsOneWidget);
    // Nothing has been told to the server yet — that is what makes undo free.
    expect(actions.discards, 0);
    expect(
      tester.widget<Text>(find.byKey(const Key('approval_queue_counter'))).data,
      t.lead_detail.queue_remaining(count: 1),
    );

    await tester.tap(find.text(t.lead_detail.undo));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('approval_queue_card_lead1')), findsOneWidget);
    expect(actions.discards, 0, reason: 'an undone discard never happened');

    // And it stays undone: the cancelled timer never fires.
    await tester.pump(ApprovalQueueScreen.undoWindow);
    await tester.pumpAndSettle();
    expect(actions.discards, 0);
  });

  // Caught in the browser, not by the earlier tests: once the undo window
  // expired the card reappeared until the refreshed queue arrived — long
  // enough to act on a lead that was already dropped.
  testWidgets('a card stays gone while its discard is still in flight',
      (tester) async {
    await _pump(tester, discardDelay: const Duration(seconds: 2));

    await tester.tap(find.byKey(const Key('approval_queue_discard_lead2')));
    await tester.pump();

    // Undo window closes; the request is now in flight.
    await tester.pump(ApprovalQueueScreen.undoWindow);
    await tester.pump();

    expect(find.byKey(const Key('approval_queue_card_lead2')), findsNothing);
    expect(
      tester.widget<Text>(find.byKey(const Key('approval_queue_counter'))).data,
      t.lead_detail.queue_remaining(count: 1),
    );

    await tester.pump(const Duration(seconds: 2));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('approval_queue_card_lead2')), findsNothing);
  });

  // Failing the other way round would be worse: a lead that vanished from
  // the queue without the server ever agreeing.
  testWidgets('a failed discard puts the card back and says why',
      (tester) async {
    await _pump(tester, failDiscard: true);

    await tester.tap(find.byKey(const Key('approval_queue_discard_lead2')));
    await tester.pump();
    await tester.pump(ApprovalQueueScreen.undoWindow);
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('approval_queue_card_lead2')), findsOneWidget);
    expect(
        find.byKey(const Key('approval_queue_discard_error')), findsOneWidget);
  });

  testWidgets('a discard left alone commits once the window closes',
      (tester) async {
    final actions = await _pump(tester);

    await tester.tap(find.byKey(const Key('approval_queue_discard_lead2')));
    await tester.pump();

    await tester.pump(ApprovalQueueScreen.undoWindow);
    await tester.pumpAndSettle();

    expect(actions.discards, 1);
    expect(actions.discarded, ['lead2']);
    // The undo offer is gone with it — a "Desfazer" that no longer works is
    // worse than no button at all.
    expect(find.text(t.lead_detail.undo), findsNothing);
  });

  // This queue is the product's main send surface. Approving while seeing
  // only a name is not approval: two clinics can share one, and a mistyped
  // number keeps whatever name was typed alongside it.
  testWidgets('the card names the lead AND shows the address', (tester) async {
    await _pump(tester);

    expect(
      find.byKey(const Key('approval_queue_recipient_lead1')),
      findsOneWidget,
    );
    expect(find.textContaining('Clínica Bem Viver'), findsWidgets);
    expect(find.textContaining('+5511900000001'), findsWidgets);
  });

  // When the label IS the address there is nothing to join, and printing a
  // separator with the same value twice reads as two different recipients.
  testWidgets('an address-only lead is not printed twice', (tester) async {
    await _pump(tester);
    // Only the OPEN card prints the recipient line now — select lead2.
    await tester.tap(find.byKey(const Key('approval_queue_card_lead2')));
    await tester.pumpAndSettle();

    final line = tester
        .widget<Text>(find.byKey(const Key('approval_queue_recipient_lead2')))
        .data!;
    expect(line, contains('contato@hospital.com'));
    expect(line, isNot(contains('—')));
  });

  // Approving happens between appointments, on a phone. The card packs a
  // long draft, a recipient line that now carries name AND number, and
  // three actions in a row — the shape most likely to overflow once the
  // width halves. An overflow is a paint-time error, so it fails here.
  testWidgets('the queue works at phone width', (tester) async {
    await _pump(tester, size: const Size(390, 900));

    expect(find.byKey(const Key('approval_queue_list')), findsOneWidget);
    expect(
      find.byKey(const Key('approval_queue_recipient_lead1')),
      findsOneWidget,
    );

    // And the actions are still reachable, not pushed off the edge. The
    // discard stays undoable, so what proves the tap landed is the undo
    // offer, not a committed call.
    await tester.tap(find.byKey(const Key('approval_queue_discard_lead1')));
    await tester.pumpAndSettle();
    expect(
      find.byKey(const Key('approval_queue_discard_undo')),
      findsOneWidget,
    );
    expect(find.byKey(const Key('approval_queue_card_lead1')), findsNothing);
  });

  // The promise detector already ran server-side and cost nothing. Showing
  // its verdict on every card is the difference between a safety feature that
  // exists and one the operator can act on.
  testWidgets('a clean draft says so, without claiming an AI reviewed it',
      (tester) async {
    await _pump(tester);

    expect(
        find.byKey(const Key('approval_queue_verdict_lead1')), findsOneWidget);
    expect(
      tester
          .widget<Text>(
              find.byKey(const Key('approval_queue_verdict_line_lead1')))
          .data,
      t.lead_detail.check_clean,
      reason: 'the pattern pass speaks only for the patterns it knows',
    );
    // The AI half has not run, so it must not have said anything yet.
    expect(find.byKey(const Key('approval_queue_review_line_lead1')),
        findsNothing);
  });

  testWidgets('a risky draft quotes the sentence and names what it is',
      (tester) async {
    await _pump(tester, queue: _risky);

    expect(
      tester
          .widget<Text>(
              find.byKey(const Key('approval_queue_verdict_line_lead1')))
          .data,
      t.lead_detail.check_found(count: 2),
    );
    // Verbatim, so the reader finds it without re-reading the message.
    expect(find.text('“Garantimos”'), findsOneWidget);
    expect(find.text('“em até 24 horas”'), findsOneWidget);
    expect(find.text(t.lead_detail.check_kind_promise), findsWidgets);
  });

  testWidgets('the draft says where it came from', (tester) async {
    await _pump(tester);

    // The origin line lives on the OPEN card — one at a time now.
    expect(find.text(t.lead_detail.written_by_ai(source: 'speedio')),
        findsOneWidget);
    await tester.tap(find.byKey(const Key('approval_queue_card_lead2')));
    await tester.pumpAndSettle();
    // lead2 has no source: it still says the AI wrote it, without inventing
    // an origin.
    expect(find.text(t.lead_detail.written_by_ai_unknown), findsOneWidget);
  });

  testWidgets('the AI review runs on demand and reports a clean verdict',
      (tester) async {
    final actions = await _pump(
      tester,
      reviewResult: const ApproachReview(safe: true, checked: true, risks: []),
    );

    await tester.tap(find.byKey(const Key('approval_queue_review_lead1')));
    await tester.pumpAndSettle();

    expect(actions.reviews, 1);
    expect(
      tester
          .widget<Text>(
              find.byKey(const Key('approval_queue_review_line_lead1')))
          .data,
      t.lead_detail.review_clean,
    );
  });

  // The one that matters: a failed review must never read as an approval.
  testWidgets('a review that did not run says so, never "clean"',
      (tester) async {
    await _pump(tester);

    await tester.tap(find.byKey(const Key('approval_queue_review_lead1')));
    await tester.pumpAndSettle();

    expect(
      tester
          .widget<Text>(
              find.byKey(const Key('approval_queue_review_line_lead1')))
          .data,
      t.lead_detail.review_unavailable,
    );
  });

  // Same rule for a reachable model that answered without checking anything.
  testWidgets('checked=false is reported as not-run even when safe is true',
      (tester) async {
    await _pump(
      tester,
      reviewResult: const ApproachReview(safe: true, checked: false, risks: []),
    );

    await tester.tap(find.byKey(const Key('approval_queue_review_lead1')));
    await tester.pumpAndSettle();

    expect(
      tester
          .widget<Text>(
              find.byKey(const Key('approval_queue_review_line_lead1')))
          .data,
      t.lead_detail.review_unavailable,
    );
  });

  // An AI risk already caught by the regex must not be listed twice: the same
  // sentence under two names makes the reader hunt twice.
  testWidgets('an AI risk duplicating a pattern finding is not repeated',
      (tester) async {
    await _pump(
      tester,
      queue: _risky,
      reviewResult: const ApproachReview(
        safe: false,
        checked: true,
        risks: [ApproachFinding(kind: 'promise', quote: 'Garantimos')],
      ),
    );

    await tester.tap(find.byKey(const Key('approval_queue_review_lead1')));
    await tester.pumpAndSettle();

    expect(find.text('“Garantimos”'), findsOneWidget);
    expect(
      tester
          .widget<Text>(
              find.byKey(const Key('approval_queue_verdict_line_lead1')))
          .data,
      t.lead_detail.check_found(count: 2),
      reason: 'the duplicate must not inflate the count either',
    );
  });

  testWidgets('the keyboard shortcuts are written down on desktop',
      (tester) async {
    await _pump(tester);

    expect(find.byKey(const Key('approval_queue_shortcuts')), findsOneWidget);
    expect(find.text(t.lead_detail.shortcut_approve), findsOneWidget);
    expect(find.text(t.lead_detail.shortcut_discard), findsOneWidget);
  });

  testWidgets('the shortcut bar stays off a phone, where there is no keyboard',
      (tester) async {
    await _pump(tester, size: const Size(390, 1600));

    expect(find.byKey(const Key('approval_queue_shortcuts')), findsNothing);
  });

  // Reported from the screen: "antes de enviar tem que verificar se não tem
  // um {{name}} ou algo do tipo, segue anexo sem anexo e etc".

  // An unfilled placeholder is never intentional, unlike a promise the
  // approver may have meant. It is the one finding that also blocks the send.
  testWidgets('an unfilled placeholder is flagged on the card', (tester) async {
    await _pump(tester, queue: const [
      QueueLead(
        id: 'lead1',
        recipient: 'Clínica Bem Viver',
        recipientAddress: '+5511900000001',
        draft: 'Olá {{nome}}, tudo bem?',
        findings: [ApproachFinding(kind: 'placeholder', quote: '{{nome}}')],
      ),
    ]);

    expect(find.text('“{{nome}}”'), findsOneWidget);
    expect(find.text(t.lead_detail.check_kind_placeholder), findsOneWidget);
  });

  // "Segue em anexo o termo" with nothing attached gets answered with "não
  // veio nada", and the thread starts on an apology.
  testWidgets('a promised attachment that is not there is flagged',
      (tester) async {
    await _pump(tester, queue: const [
      QueueLead(
        id: 'lead1',
        recipient: 'Clínica Bem Viver',
        recipientAddress: '+5511900000001',
        draft: 'Segue em anexo o termo de referência.',
        findings: [
          ApproachFinding(
              kind: 'missing_attachment',
              quote: 'Segue em anexo o termo de referência.'),
        ],
      ),
    ]);

    expect(
        find.text(t.lead_detail.check_kind_missing_attachment), findsOneWidget);
    // The finding "quotes" the whole message, and repeating it under the
    // bubble it came from would be noise.
    expect(find.text('“Segue em anexo o termo de referência.”'), findsNothing);
  });

  // ── Phone: swipe (D5) ────────────────────────────────────────────────────
  //
  // A phone is where this screen is actually worked, and a phone has no
  // keyboard to press A/E/X on.

  // The important part is what a right swipe does NOT do. Making the fastest
  // gesture on the screen also the one that puts a message in front of a
  // stranger, with no reading step, is exactly the accident this product
  // exists to prevent — a thumb brushing a list while scrolling is not
  // consent.
  testWidgets('swiping right asks before sending, it does not send',
      (tester) async {
    final actions = await _pump(tester, size: const Size(420, 900));

    await tester.drag(
        find.byKey(const Key('dismissible_lead1')), const Offset(500, 0));
    await tester.pumpAndSettle();

    expect(find.text(t.lead_detail.confirm_title), findsOneWidget);
    expect(actions.sends, 0, reason: 'nothing sent until the human confirms');
  });

  testWidgets('confirming after a right swipe sends', (tester) async {
    final actions = await _pump(tester, size: const Size(420, 900));

    await tester.drag(
        find.byKey(const Key('dismissible_lead1')), const Offset(500, 0));
    await tester.pumpAndSettle();
    await tester.tap(find.text(t.lead_detail.approve_send).last);
    await tester.pumpAndSettle();

    expect(actions.sends, 1);
  });

  // A discard is allowed to be immediate because it is undoable for five
  // seconds and nothing has been sent.
  testWidgets('swiping left discards without a dialog', (tester) async {
    await _pump(tester, size: const Size(420, 900));

    await tester.drag(
        find.byKey(const Key('dismissible_lead1')), const Offset(-500, 0));
    await tester.pumpAndSettle();

    expect(find.text(t.lead_detail.confirm_title), findsNothing);
    expect(find.byKey(const Key('approval_queue_card_lead1')), findsNothing);
  });

  // On a desktop the keyboard is the fast path; a swipe target there would
  // just make dragging to select text discard a lead.
  testWidgets('desktop has no swipe wrapper', (tester) async {
    await _pump(tester);

    expect(find.byKey(const Key('dismissible_lead1')), findsNothing);
  });

  // ── Desktop: bulk send (D5) ──────────────────────────────────────────────

  testWidgets('the bulk bar only appears once something is ticked',
      (tester) async {
    await _pump(tester);
    expect(find.byKey(const Key('approval_queue_bulk_bar')), findsNothing);

    await tester.tap(find.byKey(const Key('queue_check_lead1')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('approval_queue_bulk_bar')), findsOneWidget);
  });

  // A bulk action that asks "send 12 approaches?" is not a review — it is a
  // number, and nobody has read anything. Every message is quoted, each on its
  // own card, with its own recipient.
  testWidgets('a bulk send quotes every message before sending',
      (tester) async {
    final actions = await _pump(tester);

    await tester.tap(find.byKey(const Key('queue_check_lead1')));
    await tester.tap(find.byKey(const Key('queue_check_lead2')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('approval_queue_bulk_send')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('bulk_approval_dialog')), findsOneWidget);
    expect(find.textContaining('Olá, vi que a clínica atende ortopedia'),
        findsWidgets);
    expect(find.textContaining('Boa tarde, trabalhamos com agendamento'),
        findsWidgets);
    expect(actions.sends, 0, reason: 'still nothing sent before confirming');
  });

  testWidgets('confirming a bulk send sends every ticked lead', (tester) async {
    final actions = await _pump(tester);

    await tester.tap(find.byKey(const Key('queue_check_lead1')));
    await tester.tap(find.byKey(const Key('queue_check_lead2')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('approval_queue_bulk_send')));
    await tester.pumpAndSettle();
    // Emails in the batch require one subject before the button arms.
    await tester.enterText(
        find.byKey(const Key('bulk_approval_subject')), 'Proposta');
    await tester.pump();
    await tester.tap(find.byKey(const Key('bulk_approval_send')));
    await tester.pumpAndSettle();

    expect(actions.sends, 2);
    // Hybrid model: every BATCH approval claims the bulk route — the API
    // refuses it until volume sending is configured, and it must never fall
    // back to the customer's warming mailbox.
    expect(actions.sentViaBulk, [true, true]);
  });

  // The old dialog concatenated everything into one string: the only two moves
  // were send all twelve or cancel all twelve. Reading one and dropping it was
  // impossible, which made the review a formality.
  testWidgets('dropping one in the review sends only the rest', (tester) async {
    final actions = await _pump(tester);

    await tester.tap(find.byKey(const Key('queue_check_lead1')));
    await tester.tap(find.byKey(const Key('queue_check_lead2')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('approval_queue_bulk_send')));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('bulk_approval_check_lead2')));
    await tester.pumpAndSettle();
    // Emails in the batch require one subject before the button arms.
    await tester.enterText(
        find.byKey(const Key('bulk_approval_subject')), 'Proposta');
    await tester.pump();
    await tester.tap(find.byKey(const Key('bulk_approval_send')));
    await tester.pumpAndSettle();

    expect(actions.sends, 1);
    expect(actions.sentTexts.single,
        contains('Olá, vi que a clínica atende ortopedia'));
  });

  // The count on the button is what will actually be sent — otherwise the
  // number is back to being a number nobody checked.
  testWidgets('the button counts what survived the review', (tester) async {
    await _pump(tester);

    await tester.tap(find.byKey(const Key('queue_check_lead1')));
    await tester.tap(find.byKey(const Key('queue_check_lead2')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('approval_queue_bulk_send')));
    await tester.pumpAndSettle();
    await tester.enterText(
        find.byKey(const Key('bulk_approval_subject')), 'Proposta');
    await tester.pump();
    expect(find.text(t.lead_detail.bulk_review_send(count: 2)), findsOneWidget);

    await tester.tap(find.byKey(const Key('bulk_approval_check_lead2')));
    await tester.pumpAndSettle();

    expect(find.text(t.lead_detail.bulk_review_send(count: 1)), findsOneWidget);
  });

  // Untick everything and there is nothing to send: a live button there would
  // open the send path and do nothing.
  testWidgets('with nothing kept, the send button is dead', (tester) async {
    await _pump(tester);

    await tester.tap(find.byKey(const Key('queue_check_lead1')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('approval_queue_bulk_send')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('bulk_approval_check_lead1')));
    await tester.pumpAndSettle();

    expect(find.text(t.lead_detail.bulk_review_none), findsOneWidget);
  });

  // A draft that promises a deadline has to be findable in a batch, or the
  // review is faster only because it looks at less.
  testWidgets('the automatic check travels with each card', (tester) async {
    await _pump(tester, queue: [
      _lead('lead1', draft: 'Garantimos retorno em 24h', findings: const [
        ApproachFinding(kind: 'promise', quote: 'Garantimos retorno em 24h'),
      ]),
    ]);

    await tester.tap(find.byKey(const Key('queue_check_lead1')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('approval_queue_bulk_send')));
    await tester.pumpAndSettle();

    expect(
        find.byKey(const Key('bulk_approval_findings_lead1')), findsOneWidget);
  });

  testWidgets('clearing the selection hides the bar and sends nothing',
      (tester) async {
    final actions = await _pump(tester);

    await tester.tap(find.byKey(const Key('queue_check_lead1')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('approval_queue_bulk_clear')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('approval_queue_bulk_bar')), findsNothing);
    expect(actions.sends, 0);
  });

  // The confirmation is where the channel's needs are collected. Approving
  // used to POST only the text: e-mail went out on the default subject and
  // WhatsApp bounced off Meta with a generic error.
  testWidgets("an e-mail lead's confirmation collects the subject",
      (tester) async {
    final actions = await _pump(tester); // lead2 is the e-mail lead
    // Approve lives only on the open card: select it first.
    await tester.tap(find.byKey(const Key('approval_queue_card_lead2')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('approval_queue_approve_lead2')));
    await tester.pumpAndSettle();

    await tester.enterText(find.byKey(const Key('approach_send_subject')),
        'Proposta para o hospital');
    await tester.tap(find.byKey(const Key('approach_send_confirm')));
    await tester.pumpAndSettle();

    expect(actions.sends, 1);
    expect(actions.sentSubject, 'Proposta para o hospital');
  });

  testWidgets('a WhatsApp lead cannot be confirmed without a template',
      (tester) async {
    tester.view.physicalSize = const Size(1200, 2000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    final actions = _FakeActions();
    await tester.pumpWidget(ProviderScope(
      overrides: [
        approvalQueueProvider.overrideWith((ref) async => _asPage(const [
              QueueLead(
                id: 'wa1',
                recipient: 'Clínica Zap',
                recipientAddress: '+5511900000009',
                draft: 'Olá, tudo bem?',
                funnelId: 'fnwa',
              ),
            ])),
        leadActionsProvider.overrideWithValue(actions),
        funnelsListProvider.overrideWith(_WaFunnel.new),
        channelsListProvider.overrideWith(_WaChannel.new),
        whatsAppTemplatesProvider.overrideWith(_NoTemplates.new),
        dioProvider.overrideWithValue(Dio()),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const ApprovalQueueScreen(),
      ),
    ));
    await tester.pumpAndSettle();

    // The card names the door the message leaves through.
    expect(find.textContaining('WhatsApp'), findsWidgets);

    await tester.tap(find.byKey(const Key('approval_queue_approve_wa1')));
    await tester.pumpAndSettle();

    final confirm = tester
        .widget<FilledButton>(find.byKey(const Key('approach_send_confirm')));
    expect(confirm.onPressed, isNull,
        reason: 'sem template aprovado o envio não pode ser confirmado');
    expect(actions.sends, 0);
  });
}

class _WaFunnel extends FunnelsList {
  @override
  Future<List<FunnelModel>> build() async =>
      [const FunnelModel(id: 'fnwa', name: 'Zap', channelId: 'chwa')];
}

class _WaChannel extends ChannelsList {
  @override
  Future<List<ChannelModel>> build() async => [
        ChannelModel.fromJson(const {
          'id': 'chwa',
          'account_id': 'acc',
          'name': 'WhatsApp',
          'type': 'whatsapp',
          'is_active': true,
        }),
      ];
}

class _NoTemplates extends WhatsAppTemplates {
  @override
  Future<List<WhatsAppTemplateModel>> build(String channelId) async => const [];
}

// --- Slicing a backlog of sixty -------------------------------------------

/// Two funnels, one draft each: the shape a filter actually has to survive.
ApprovalQueuePage _twoFunnels({
  int total = 2,
  bool flagOne = false,
  DateTime? firstWaitingSince,
}) =>
    ApprovalQueuePage(
      total: total,
      byFunnel: const [
        QueueFunnelCount(funnelId: 'fn1', funnelName: 'Clínicas', total: 1),
        QueueFunnelCount(funnelId: 'fn2', funnelName: 'Academias', total: 1),
      ],
      items: [
        QueueLead(
          id: 'lead1',
          recipient: 'Clínica Bem Viver',
          recipientAddress: '+5511900000001',
          draft: 'Olá, vi que a clínica atende ortopedia…',
          companyName: 'Clínica Bem Viver',
          funnelId: 'fn1',
          waitingSince: firstWaitingSince,
          findings: flagOne
              ? const [ApproachFinding(kind: 'promise', quote: 'Garantimos')]
              : const [],
        ),
        const QueueLead(
          id: 'lead2',
          recipient: 'Academia Forte',
          recipientAddress: '+5511900000002',
          draft: 'Bom dia, posso mandar duas opções?',
          companyName: 'Academia Forte',
          funnelId: 'fn2',
        ),
      ],
    );

void _filterTests() {
  // Sixty drafts in a flat list is a pile, not a queue. The chips are the map:
  // each carries a count read server-side over the WHOLE queue.
  testWidgets('the chips say how much of the backlog sits in each funnel',
      (tester) async {
    await _pump(tester, page: _twoFunnels());

    expect(find.byKey(const Key('approval_queue_filter_bar')), findsOneWidget);
    expect(find.byKey(const Key('approval_queue_funnel_fn1')), findsOneWidget);
    expect(find.byKey(const Key('approval_queue_funnel_fn2')), findsOneWidget);
    expect(find.text('Clínicas · 1'), findsOneWidget);
    expect(find.text('${t.lead_detail.queue_filter_all} · 2'), findsOneWidget);
  });

  // One funnel is nothing to choose between, and a lone chip beside "All" is
  // two buttons for one list.
  testWidgets('a single-funnel backlog gets no funnel chips', (tester) async {
    await _pump(
      tester,
      page: ApprovalQueuePage(
        items: _queue,
        total: _queue.length,
        byFunnel: const [
          QueueFunnelCount(funnelId: 'fn1', funnelName: 'Clínicas', total: 2),
        ],
      ),
    );

    expect(find.byKey(const Key('approval_queue_funnel_all')), findsNothing);
    // The order and flag toggles still render: those are useful on one funnel.
    expect(find.byKey(const Key('approval_queue_sort')), findsOneWidget);
  });

  // The funnel narrow is SERVER-side, so picking one has to re-read the queue.
  // Filtering the loaded page instead would show whichever of the funnel's 40
  // drafts happened to arrive, and call that the funnel.
  testWidgets('picking a funnel re-reads the queue with the narrow',
      (tester) async {
    final asked = <Map<String, dynamic>>[];
    await tester.pumpWidget(ProviderScope(
      overrides: [
        approvalQueueProvider.overrideWith((ref) async {
          final f = ref.watch(approvalQueueFilterProvider);
          asked.add(<String, dynamic>{
            'funnel': f.funnelId,
            'newest': f.newestFirst,
          });
          return _twoFunnels();
        }),
        leadActionsProvider.overrideWithValue(_FakeActions()),
        funnelsListProvider.overrideWith(_NoFunnelsList.new),
        dioProvider.overrideWithValue(Dio()),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const ApprovalQueueScreen(),
      ),
    ));
    await tester.pumpAndSettle();
    expect(asked, [
      {'funnel': null, 'newest': false}
    ]);

    await tester.tap(find.byKey(const Key('approval_queue_funnel_fn2')));
    await tester.pumpAndSettle();
    expect(asked.last['funnel'], 'fn2');

    // Tapping the selected chip is the way back — the operator should not have
    // to hunt for "All" to undo their own tap.
    await tester.tap(find.byKey(const Key('approval_queue_funnel_fn2')));
    await tester.pumpAndSettle();
    expect(asked.last['funnel'], isNull);

    // The order is server-side too: the stalest draft is only first if the SQL
    // says so.
    await tester.tap(find.byKey(const Key('approval_queue_sort')));
    await tester.pumpAndSettle();
    expect(asked.last['newest'], isTrue);
  });

  // "Flagged only" runs over what is loaded, so it must NOT spend a round-trip.
  testWidgets('the flagged filter hides the clean drafts without re-reading',
      (tester) async {
    var reads = 0;
    await tester.pumpWidget(ProviderScope(
      overrides: [
        approvalQueueProvider.overrideWith((ref) async {
          ref.watch(approvalQueueFilterProvider
              .select((f) => (f.funnelId, f.newestFirst)));
          reads++;
          return _twoFunnels(flagOne: true);
        }),
        leadActionsProvider.overrideWithValue(_FakeActions()),
        funnelsListProvider.overrideWith(_NoFunnelsList.new),
        dioProvider.overrideWithValue(Dio()),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const ApprovalQueueScreen(),
      ),
    ));
    await tester.pumpAndSettle();
    expect(reads, 1);

    await tester.tap(find.byKey(const Key('approval_queue_flagged_only')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('approval_queue_card_lead1')), findsOneWidget);
    expect(find.byKey(const Key('approval_queue_card_lead2')), findsNothing);
    expect(reads, 1, reason: 'a client-side filter must not cost a request');
  });

  // An empty list under a filter is not an empty queue. Saying "nothing to
  // approve" here sends the operator home with work still waiting.
  testWidgets('a filter that empties the list says the drafts are still there',
      (tester) async {
    await _pump(tester, page: _twoFunnels());

    await tester.tap(find.byKey(const Key('approval_queue_flagged_only')));
    await tester.pumpAndSettle();

    expect(
        find.byKey(const Key('approval_queue_filtered_empty')), findsOneWidget);
    expect(find.byKey(const Key('approval_queue_empty')), findsNothing);
    // And the way out of the filter is still on screen.
    expect(find.byKey(const Key('approval_queue_filter_bar')), findsOneWidget);
  });

  // The counter measures the WORK, not the view: a number that drops when you
  // pick a funnel is reporting the filter.
  testWidgets('the counter keeps counting the whole backlog under a filter',
      (tester) async {
    await _pump(tester, page: _twoFunnels(total: 340));

    expect(
      tester.widget<Text>(find.byKey(const Key('approval_queue_counter'))).data,
      t.lead_detail.queue_remaining(count: 340),
    );

    await tester.tap(find.byKey(const Key('approval_queue_funnel_fn1')));
    await tester.pumpAndSettle();

    expect(
      tester.widget<Text>(find.byKey(const Key('approval_queue_counter'))).data,
      t.lead_detail.queue_remaining(count: 340),
    );
  });

  // Every filter on this screen is honest only about the drafts that arrived,
  // so a capped page announces itself instead of being inferred from a count.
  testWidgets('a page that could not hold the queue says so', (tester) async {
    await _pump(tester, page: _twoFunnels(total: 340));

    expect(
      tester.widget<Text>(find.byKey(const Key('approval_queue_partial'))).data,
      t.lead_detail.queue_partial(shown: '2', total: '340'),
    );
  });

  testWidgets('a complete page claims nothing about what it left out',
      (tester) async {
    await _pump(tester, page: _twoFunnels());
    expect(find.byKey(const Key('approval_queue_partial')), findsNothing);
  });

  // Staleness is the one thing a queue has to show. A list where yesterday's
  // draft looks exactly like the one written a minute ago cannot answer the
  // operator's real question.
  testWidgets('the card says how long the draft has been waiting',
      (tester) async {
    await _pump(
      tester,
      page: _twoFunnels(
          firstWaitingSince:
              DateTime.now().subtract(const Duration(days: 3, hours: 2))),
    );

    final line = tester
        .widget<Text>(find.byKey(const Key('approval_queue_context_lead1')))
        .data;
    expect(line, contains(t.lead_detail.queue_waiting_days(count: '3')));
    // In the cross-funnel view the card also names its campaign, so the same
    // words can be read in the right context.
    expect(line, contains('Clínicas'));
  });

  // No timestamp is no claim: a missing date must not render as 1970.
  testWidgets('a draft with no timestamp shows no age', (tester) async {
    await _pump(
      tester,
      page: ApprovalQueuePage(
        items: _queue,
        total: _queue.length,
        byFunnel: const [],
      ),
    );
    expect(find.byKey(const Key('approval_queue_context_lead1')), findsNothing);
  });

  // The bulk bar counts what would be SENT. A tick left behind a filter would
  // make the bar say 2 and send 1 — the exact silence this screen exists to
  // prevent, applied to our own UI.
  testWidgets('filtering drops ticks whose card left the screen',
      (tester) async {
    final actions = await _pump(tester, page: _twoFunnels(flagOne: true));

    await tester.tap(find.byKey(const Key('queue_check_lead2')));
    await tester.pumpAndSettle();
    expect(find.text(t.lead_detail.bulk_selected(count: 1)), findsOneWidget);

    // lead2 is clean, so the flagged filter takes it off screen.
    await tester.tap(find.byKey(const Key('approval_queue_flagged_only')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('approval_queue_bulk_bar')), findsNothing);
    expect(actions.sentTexts, isEmpty);
  });
}

// Ofertas → Hoje phase 1: the queue is grouped by source (book > signal >
// cold) and the near-identical cold drafts fold behind one header with batch
// actions, so the two drafts that pay are read first.
void _groupingTests() {
  QueueLead src(String id, String? source) => QueueLead(
        id: id,
        recipient: 'Clínica $id',
        recipientAddress: '+55119000000$id',
        draft: 'texto $id',
        source: source,
      );

  final mixed = [
    for (var i = 0; i < 5; i++) src('f$i', 'google_places'),
    src('sig', 'pncp_signal'),
    src('book', null),
  ];

  testWidgets('groups by source with the book first and cold folded',
      (tester) async {
    // Tall viewport: the cold group is LAST, and a lazy ListView never
    // builds what is below the fold.
    await _pump(tester, queue: mixed, size: const Size(1000, 4000));

    expect(find.byKey(const Key('queue_group_carteira')), findsOneWidget);
    expect(find.byKey(const Key('queue_group_sinal')), findsOneWidget);
    expect(find.byKey(const Key('queue_group_frio')), findsOneWidget);

    // Folded: 5 cold drafts, only 2 on screen, the rest behind the unfold.
    expect(find.text('Clínica f0'), findsOneWidget);
    expect(find.text('Clínica f2'), findsNothing);
    await tester.tap(find.byKey(const Key('queue_frio_show_all')));
    await tester.pumpAndSettle();
    expect(find.text('Clínica f2'), findsOneWidget);
  });

  testWidgets('discard-all sweeps the whole cold group after one confirm',
      (tester) async {
    await _pump(tester, queue: mixed, size: const Size(1000, 4000));

    await tester.tap(find.byKey(const Key('queue_frio_discard_all')));
    await tester.pumpAndSettle();
    await tester.tap(find.text(t.sales.leads.discard).last);
    await tester.pumpAndSettle();

    // All 5 cold ids went in the batch — including the 3 folded off screen.
    expect(_batch!.discarded.length, 5);
    expect(_batch!.discarded, isNot(contains('book')));
  });

  testWidgets('the weekly outcome line renders sent, replies and proposals',
      (tester) async {
    await _pump(tester);
    expect(find.byKey(const Key('queue_outcome_line')), findsOneWidget);
    expect(
        find.text(t.lead_detail
            .queue_outcome_line(sent: '12', replies: '3', proposals: '1')),
        findsOneWidget);
  });
}

// Phase 1b: approving past the daily cap schedules the send; the card is
// labeled instead of hidden.
void _scheduleTests() {
  testWidgets('a scheduled card carries the badge with its send time',
      (tester) async {
    final at = DateTime(2026, 8, 30, 8, 0);
    await _pump(tester, queue: [
      QueueLead(
        id: 'lead-s',
        recipient: 'Clínica Agendada',
        recipientAddress: 'compras@agendada.com',
        draft: 'texto aprovado',
        scheduledFor: at,
      ),
    ]);

    expect(
        find.byKey(const Key('queue_scheduled_badge_lead-s')), findsOneWidget);
    expect(
        find.text(
            t.lead_detail.queue_scheduled_badge(when: formatScheduledFor(at))),
        findsOneWidget);
  });
}

// Production, 2026-09-09: the CuiabáMed account had 41 finished offers and
// ZERO channels. The queue rendered a live blue "Aprovar e enviar" on every
// card and the failure only arrived after the click.
void _blockedQueueTests() {
  ApprovalQueuePage blocked(List<QueueLead> items) => ApprovalQueuePage(
        items: items,
        total: items.length,
        byFunnel: const [],
        sendBlockedReason: 'NO_CHANNEL',
      );

  testWidgets('with nowhere to send, the send button is dead and says why',
      (tester) async {
    await _pump(tester, page: blocked(_queue));

    expect(find.byKey(const Key('queue_send_blocked_banner')), findsOneWidget);

    final approve = tester.widget<GradientPill>(
        find.byKey(Key('approval_queue_approve_${_queue.first.id}')));
    expect(approve.onTap, isNull,
        reason: 'a button that cannot succeed must not offer to try');
  });

  testWidgets('a queue that can send shows no banner and a live button',
      (tester) async {
    await _pump(tester, page: _asPage(_queue));

    expect(find.byKey(const Key('queue_send_blocked_banner')), findsNothing);

    final approve = tester.widget<GradientPill>(
        find.byKey(Key('approval_queue_approve_${_queue.first.id}')));
    expect(approve.onTap, isNotNull);
  });

  // On a phone the card's own buttons sit more than a screen below the fold,
  // past the products, the objections and the whole offer text — on every one
  // of forty cards.
  testWidgets('on a phone the action bar follows the card', (tester) async {
    await _pump(tester, size: const Size(390, 900));

    expect(
        find.byKey(const Key('approval_queue_phone_actions')), findsOneWidget);
    final approve = tester.widget<GradientPill>(
        find.byKey(const Key('approval_queue_phone_approve')));
    expect(approve.onTap, isNotNull);
  });

  testWidgets('the phone bar is not on the desktop, which has shortcuts',
      (tester) async {
    await _pump(tester, size: const Size(1200, 1000));
    expect(find.byKey(const Key('approval_queue_phone_actions')), findsNothing);
  });

  testWidgets('the phone bar respects the same block as the card',
      (tester) async {
    await _pump(tester, page: blocked(_queue), size: const Size(390, 900));
    final approve = tester.widget<GradientPill>(
        find.byKey(const Key('approval_queue_phone_approve')));
    expect(approve.onTap, isNull);
  });
}

/// A queue of fifty look-alike cards has no first item, and a list with no
/// first item is a list nobody starts. These pin the two things that let an
/// operator choose: what closes soonest, and what cannot be sent at all.
void _rankableCardTests() {
  // Ordering is pure and testable on its own — no screen to pump.
  test('the nearest deadline leads, and an expired tender sinks', () {
    final now = DateTime(2026, 9, 10);
    QueueLead at(String id, DateTime? due) => QueueLead(
          id: id,
          recipient: id,
          recipientAddress: 'a@b.c',
          draft: 'oi',
          tenderDeadline: due,
        );

    final ordered = deadlineFirst(<QueueLead>[
      at('no-date-1', null),
      at('expired', DateTime(2026, 9, 8)),
      at('far', DateTime(2026, 9, 30)),
      at('soon', DateTime(2026, 9, 12)),
      at('no-date-2', null),
    ], now);

    expect(ordered.map((l) => l.id).toList(),
        ['soon', 'far', 'expired', 'no-date-1', 'no-date-2']);
  });

  testWidgets('a card answering a tender shows the clock and the size',
      (tester) async {
    await _pump(tester, queue: <QueueLead>[
      QueueLead(
        id: 'lead1',
        recipient: 'Município de Sinop',
        recipientAddress: 'compras@sinop.mt.gov.br',
        draft: 'Sobre o pregão 124/2026…',
        companyName: 'Município de Sinop',
        tenderValue: '250.000,00 BRL',
        tenderDeadline: DateTime.now().add(const Duration(days: 2)),
      ),
    ]);

    expect(find.byKey(const Key('queue_tender_strip_lead1')), findsOneWidget);
    expect(find.textContaining('250.000,00 BRL'), findsWidgets);
  });

  // Finished, correct, and addressed to nobody. Said on the card because the
  // repair is per-card work — find this buyer's e-mail — not a screen-wide
  // "you have no channel", which would be a different and untrue claim.
  testWidgets('a card with no address says so without blocking the queue',
      (tester) async {
    await _pump(tester, queue: <QueueLead>[
      QueueLead(
        id: 'lead1',
        recipient: 'Estado de Mato Grosso',
        recipientAddress: '',
        draft: 'Olá!',
        companyName: 'Estado de Mato Grosso',
        sendBlockedReason: 'NO_ADDRESS',
      ),
      const QueueLead(
        id: 'lead2',
        recipient: 'Clínica Bem Viver',
        recipientAddress: 'contato@bemviver.com.br',
        draft: 'Olá!',
        companyName: 'Clínica Bem Viver',
      ),
    ]);

    expect(find.byKey(const Key('queue_no_address_lead1')), findsOneWidget);
    // The reachable card beside it carries no such warning, and the
    // account-level banner never appears: the account CAN send.
    expect(find.byKey(const Key('queue_no_address_lead2')), findsNothing);
    expect(find.byKey(const Key('queue_send_blocked_banner')), findsNothing);
  });

  // The escape hatch. The platform cannot post this letter; the seller can
  // send it in ten seconds from the phone already in their pocket, and that
  // needs no WhatsApp integration, no approved template and no per-message
  // fee to Meta.
  testWidgets('a blocked card with a phone offers the seller\'s own WhatsApp',
      (tester) async {
    await _pump(tester, queue: <QueueLead>[
      QueueLead(
        id: 'lead1',
        recipient: 'Estado de Mato Grosso',
        recipientAddress: '',
        draft: 'Olá!',
        companyName: 'Estado de Mato Grosso',
        sendBlockedReason: 'NO_ADDRESS',
        whatsappPhone: '+55 65 3613-5300',
      ),
    ]);

    expect(find.byKey(const Key('queue_wa_send_lead1')), findsOneWidget);
  });

  // A button that opens a chat with nobody is worse than no button, and a
  // card that WILL send must not teach the seller to route around the send.
  testWidgets('no phone, or nothing blocking: no WhatsApp button',
      (tester) async {
    await _pump(tester, queue: <QueueLead>[
      QueueLead(
        id: 'lead1',
        recipient: 'Estado de Mato Grosso',
        recipientAddress: '',
        draft: 'Olá!',
        sendBlockedReason: 'NO_ADDRESS',
      ),
      QueueLead(
        id: 'lead2',
        recipient: 'Sem número que sirva',
        recipientAddress: '',
        draft: 'Olá!',
        sendBlockedReason: 'NO_ADDRESS',
        whatsappPhone: '99811',
      ),
      const QueueLead(
        id: 'lead3',
        recipient: 'Clínica Bem Viver',
        recipientAddress: 'contato@bemviver.com.br',
        draft: 'Olá!',
        whatsappPhone: '+55 65 3613-5300',
      ),
    ]);

    expect(find.byKey(const Key('queue_wa_send_lead1')), findsNothing,
        reason: 'no number to dial');
    expect(find.byKey(const Key('queue_wa_send_lead2')), findsNothing,
        reason: 'the number is not dialable');
    expect(find.byKey(const Key('queue_wa_send_lead3')), findsNothing,
        reason: 'this card sends normally');
  });
}
