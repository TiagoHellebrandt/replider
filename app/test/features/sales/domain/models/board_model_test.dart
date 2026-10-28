import 'package:flutter_test/flutter_test.dart';
import 'package:app/features/sales/domain/models/board_model.dart';

BoardStage _stage(String id) => BoardStage(
      id: id,
      name: id,
      position: 0,
      mapsToStatus: 'prospected',
      isTerminal: false,
    );

BoardCard _card(
  String id, {
  String stage = 's1',
  double? pos,
  DateTime? updated,
}) =>
    BoardCard(
      leadId: id,
      stageId: stage,
      status: 'prospected',
      score: 0,
      boardPos: pos,
      updatedAt: updated,
    );

Board _board(List<BoardCard> cards) =>
    Board(stages: [_stage('s1'), _stage('s2')], cards: cards, truncated: false);

void main() {
  final old = DateTime(2026, 1, 1);
  final recent = DateTime(2026, 8, 1);

  group('compareForColumn', () {
    test('placed cards come first, in position order', () {
      final cards = [
        _card('a', pos: 2048),
        _card('b', pos: 1024),
        _card('c'),
      ]..sort(Board.compareForColumn);
      expect(cards.map((c) => c.leadId).toList(), ['b', 'a', 'c']);
    });

    test('never-placed cards keep most-stalled-first after the placed', () {
      final cards = [
        _card('fresh', updated: recent),
        _card('placed', pos: 5000),
        _card('stale', updated: old),
      ]..sort(Board.compareForColumn);
      expect(cards.map((c) => c.leadId).toList(), ['placed', 'stale', 'fresh']);
    });

    test('a same idle time breaks the tie by id, so the sort is stable', () {
      final t = DateTime(2026, 5, 5);
      final cards = [
        _card('zebra', updated: t),
        _card('alpha', updated: t),
      ]..sort(Board.compareForColumn);
      expect(cards.map((c) => c.leadId).toList(), ['alpha', 'zebra']);
    });
  });

  group('withCardPlaced', () {
    test('a midpoint between two placed cards', () {
      final b = _board([
        _card('a', pos: 1024),
        _card('b', pos: 2048),
        _card('x', pos: 9000),
      ]).withCardPlaced('x', 's1', beforeId: 'a', afterId: 'b');
      final x = b.cards.firstWhere((c) => c.leadId == 'x');
      expect(x.boardPos, 1536);
    });

    // The reported bug: first drag in an automatic column. The named
    // neighbors have no position, so the mirror must SEED the whole column
    // first (like the server) — otherwise the dropped card became the only
    // placed one and jumped to the top wherever the slot was.
    test('dropping below an unplaced neighbor lands BELOW it, not at the top',
        () {
      final b = _board([
        _card('a', updated: old), // most stalled → top of automatic order
        _card('b', updated: recent),
        _card('c', updated: DateTime(2026, 4, 1)),
      ]).withCardPlaced('b', 's1', beforeId: 'a', afterId: null);

      final ordered = (b.cards.toList()..sort(Board.compareForColumn))
          .map((c) => c.leadId)
          .toList();
      // b sits right after a, never before it.
      expect(ordered.indexOf('b'), greaterThan(ordered.indexOf('a')));
      expect(b.cards.firstWhere((c) => c.leadId == 'b').boardPos, isNotNull);
    });

    test('dropping between two unplaced neighbors seeds and lands between them',
        () {
      final b = _board([
        _card('a', updated: old), // automatic order: a, c, then b (freshest)
        _card('b', updated: recent),
        _card('c', updated: DateTime(2026, 4, 1)),
      ]).withCardPlaced('b', 's1', beforeId: 'a', afterId: 'c');

      final ordered = (b.cards.toList()..sort(Board.compareForColumn))
          .map((c) => c.leadId)
          .toList();
      expect(ordered, ['a', 'b', 'c']);
    });
  });

  test('hasManualOrder is true once any card in the column is placed', () {
    expect(_board([_card('a'), _card('b')]).hasManualOrder('s1'), isFalse);
    expect(
      _board([_card('a', pos: 10), _card('b')]).hasManualOrder('s1'),
      isTrue,
    );
  });

  test('withOrderReset clears the column back to automatic', () {
    final b =
        _board([_card('a', pos: 10), _card('b', pos: 20)]).withOrderReset('s1');
    expect(b.cards.every((c) => c.boardPos == null), isTrue);
  });
}
