import 'package:app/features/sales/presentation/controllers/approval_queue_controller.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:flutter_test/flutter_test.dart';

// The queue endpoint answers with a page plus the size of the backlog behind
// it. These pin the reading of that answer: what the screen may claim, and what
// it must refuse to claim when a field never arrived.

void main() {
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  group('QueueLead.fromJson', () {
    test('reads the funnel and when the draft started waiting', () {
      final lead = QueueLead.fromJson(<String, dynamic>{
        'id': 'lead1',
        'funnel_id': 'fn1',
        'contact_label': 'Clínica Bem Viver',
        'contact_address': '+5511900000001',
        'approach_draft': 'Olá',
        'updated_at': '2026-07-20T10:00:00Z',
      });

      expect(lead.funnelId, 'fn1');
      expect(
          lead.waitingSince, DateTime.parse('2026-07-20T10:00:00Z').toLocal());
    });

    // An unparseable date is absent, not epoch: 1970 would pin the card to the
    // top of a list ordered by staleness and claim a 56-year wait.
    test('a broken timestamp is no timestamp', () {
      for (final raw in <dynamic>['', '   ', 'ontem', 42, null]) {
        final lead = QueueLead.fromJson(<String, dynamic>{
          'id': 'lead1',
          'updated_at': raw,
        });
        expect(lead.waitingSince, isNull, reason: 'for $raw');
      }
    });
  });

  group('ApprovalQueuePage', () {
    // The page is capped, so the difference between what came back and what is
    // waiting is the one thing the screen cannot infer on its own.
    test('a page smaller than the backlog knows it is partial', () {
      expect(
        ApprovalQueuePage(items: const [], total: 340, byFunnel: const [])
            .isPartial,
        isTrue,
      );
      expect(
        ApprovalQueuePage(
          items: [
            const QueueLead(
              id: 'lead1',
              recipient: 'a',
              recipientAddress: 'a',
              draft: 'x',
            ),
          ],
          total: 1,
          byFunnel: const [],
        ).isPartial,
        isFalse,
      );
    });
  });

  group('queueSourceOf', () {
    QueueLead lead(String source) => QueueLead(
          id: 'l1',
          recipient: 'Hospital',
          recipientAddress: 'a',
          draft: 'x',
          source: source,
        );

    // The queue exists to stop the seller reading forty cold drafts to find
    // the two that pay. `cnes_park` — an establishment read out of a
    // government registry, which has never heard of us — fell through to the
    // default and rendered as "Da sua carteira": 40 strangers presented as
    // the seller's own book, on an account that had never sold anything.
    test('a hospital from the CNES registry is cold, not carteira', () {
      expect(queueSourceOf(lead('cnes_park')), QueueSource.frio);
      expect(queueSourceOf(lead('google_places')), QueueSource.frio);
    });

    test('a tender is its own group, and the seller book is the fallback', () {
      expect(queueSourceOf(lead('pncp_signal')), QueueSource.sinal);
      expect(queueSourceOf(lead('csv_import')), QueueSource.carteira);
      expect(queueSourceOf(lead('')), QueueSource.carteira);
    });
  });

  group('formatQueueWait', () {
    final now = DateTime.parse('2026-07-20T12:00:00Z');

    test('reports the coarsest unit that still says something', () {
      expect(
        formatQueueWait(now.subtract(const Duration(minutes: 20)), now: now),
        t.lead_detail.queue_waiting_minutes(count: '20'),
      );
      expect(
        formatQueueWait(now.subtract(const Duration(hours: 5)), now: now),
        t.lead_detail.queue_waiting_hours(count: '5'),
      );
      expect(
        formatQueueWait(now.subtract(const Duration(days: 3, hours: 4)),
            now: now),
        t.lead_detail.queue_waiting_days(count: '3'),
      );
    });

    // A clock skew must read as brand new, never as a negative age — and a
    // draft with no timestamp gets no age rather than a made-up one.
    test('never renders a negative or invented age', () {
      expect(
        formatQueueWait(now.add(const Duration(minutes: 5)), now: now),
        t.lead_detail.queue_waiting_minutes(count: '1'),
      );
      expect(formatQueueWait(null, now: now), isNull);
    });
  });

  group('ApprovalQueueFilter', () {
    // copyWith cannot express "back to every funnel" by passing null — that is
    // indistinguishable from "leave it alone", which is why clearing is its own
    // flag. Getting this wrong makes the All chip do nothing.
    test('clearing the funnel is distinct from leaving it alone', () {
      const selected = ApprovalQueueFilter(funnelId: 'fn1', flaggedOnly: true);

      expect(selected.copyWith(newestFirst: true).funnelId, 'fn1');
      expect(selected.copyWith(clearFunnel: true).funnelId, isNull);
      // Clearing one control leaves the others where the operator put them.
      expect(selected.copyWith(clearFunnel: true).flaggedOnly, isTrue);
    });
  });

  group('AnticipatedObjection', () {
    test('parses a list, a JSON string, and junk', () {
      final fromList = AnticipatedObjection.listFrom([
        {'label': 'Já temos fornecedor', 'counter': 'Peça a compra pequena.'},
        {'label': '', 'counter': 'no label, dropped'},
        'junk',
      ]);
      expect(fromList, hasLength(1));
      expect(fromList.first.label, 'Já temos fornecedor');
      expect(fromList.first.counter, 'Peça a compra pequena.');

      final fromString = AnticipatedObjection.listFrom(
          '[{"label":"Sem verba","counter":"Locação."}]');
      expect(fromString, hasLength(1));

      expect(AnticipatedObjection.listFrom('not json'), isEmpty);
      expect(AnticipatedObjection.listFrom(null), isEmpty);
    });

    test('QueueLead carries the objections from the API row', () {
      final lead = QueueLead.fromJson({
        'id': 'l1',
        'approach_draft': 'Olá',
        'anticipated_objections': [
          {'label': 'Só licitação', 'counter': 'Entramos no pregão.'},
        ],
      });
      expect(lead.objections, hasLength(1));
      expect(lead.objections.first.label, 'Só licitação');
    });
  });

  // The two facts that make one card worth opening before another, and the one
  // that says a card cannot be sent at all.
  //
  // Production, 2026-09-10: fifty cards, every one showing no value and no date,
  // untouched for forty days. A list with no first item is a list nobody starts.
  group('what makes a card rankable', () {
    test('reads the tender deadline and value out of the enrichment', () {
      final lead = QueueLead.fromJson(<String, dynamic>{
        'id': 'lead1',
        'enrichment': <String, dynamic>{
          'edital_prazo': '2026-09-24',
          'edital_valor_estimado': '250.000,00 BRL',
        },
      });

      expect(lead.tenderValue, '250.000,00 BRL');
      expect(lead.tenderDeadline, isNotNull);
      expect(lead.daysToDeadline(DateTime(2026, 9, 21)), 3);
      // Past the date the count goes negative rather than clamping: writing
      // about a closed notice is a different mistake from writing late.
      expect(lead.daysToDeadline(DateTime(2026, 9, 25)), -1);
    });

    test('a lead with no tender has no deadline, which is not "plenty of time"',
        () {
      final lead = QueueLead.fromJson(<String, dynamic>{'id': 'lead1'});

      expect(lead.tenderDeadline, isNull);
      expect(lead.daysToDeadline(DateTime(2026, 9, 21)), isNull);
      expect(lead.isSendBlocked, isFalse);
    });

    test('a card the account cannot post says so on itself', () {
      final lead = QueueLead.fromJson(<String, dynamic>{
        'id': 'lead1',
        'send_blocked_reason': 'NO_ADDRESS',
      });

      expect(lead.isSendBlocked, isTrue);
      expect(lead.sendBlockedReason, 'NO_ADDRESS');
    });

    // An older server does not send the field. Absent must mean "nothing is
    // blocking", or upgrading the app would red-flag every card in the queue.
    test('an absent reason blocks nothing', () {
      expect(
        QueueLead.fromJson(
                <String, dynamic>{'id': 'l', 'send_blocked_reason': '  '})
            .isSendBlocked,
        isFalse,
      );
    });
  });
}
