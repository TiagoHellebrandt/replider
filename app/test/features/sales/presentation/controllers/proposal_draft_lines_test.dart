import 'package:app/features/sales/presentation/controllers/proposal_controller.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

// "Cotação em um minuto": the priced-offer form opens with the matcher's picks
// already in it. The whole value is in the suggestion being a STARTING POINT —
// so the tests here are about what happens after the seller disagrees with it.

ProviderContainer _container() {
  final c = ProviderContainer();
  addTearDown(c.dispose);
  // The notifier is autoDispose; listening keeps it alive across reads.
  c.listen(proposalDraftLinesProvider, (_, __) {});
  return c;
}

void main() {
  test('the matcher\'s picks arrive as one of each', () {
    final c = _container();

    c.read(proposalDraftLinesProvider.notifier).suggest(['p1', 'p2']);

    expect(c.read(proposalDraftLinesProvider), {'p1': 1, 'p2': 1});
  });

  // The one that matters. A suggestion that comes back after it was deleted
  // is not a suggestion, and this screen rebuilds often enough for a naive
  // "seed when empty" to resurrect a removed line within seconds.
  test('a removed line stays removed', () {
    final c = _container();
    final notifier = c.read(proposalDraftLinesProvider.notifier);

    notifier.suggest(['p1']);
    notifier.remove('p1');
    expect(c.read(proposalDraftLinesProvider), isEmpty);

    notifier.suggest(['p1']);
    expect(c.read(proposalDraftLinesProvider), isEmpty,
        reason: 'the suggestion argued back');
  });

  test('an edited quantity survives a second suggestion', () {
    final c = _container();
    final notifier = c.read(proposalDraftLinesProvider.notifier);

    notifier.suggest(['p1']);
    notifier.add('p1');
    notifier.add('p1');
    notifier.suggest(['p1', 'p2']);

    expect(c.read(proposalDraftLinesProvider), {'p1': 3});
  });

  // A lead the matcher said nothing about must not burn the one suggestion:
  // the lead detail resolves a frame or two after the screen mounts, and an
  // empty first read would otherwise lock the form blank for good.
  test('an empty pick list does not consume the suggestion', () {
    final c = _container();
    final notifier = c.read(proposalDraftLinesProvider.notifier);

    notifier.suggest(const <String>[]);
    notifier.suggest(['p1']);

    expect(c.read(proposalDraftLinesProvider), {'p1': 1});
  });

  test('blank ids are dropped rather than priced', () {
    final c = _container();

    c.read(proposalDraftLinesProvider.notifier).suggest(['', '  ', 'p1']);

    expect(c.read(proposalDraftLinesProvider), {'p1': 1});
  });

  test('lines convert to what the API takes', () {
    final c = _container();
    final notifier = c.read(proposalDraftLinesProvider.notifier);

    notifier.suggest(['p1']);
    notifier.add('p1');

    final lines = notifier.lines;
    expect(lines, hasLength(1));
    expect(lines.single.toJson(), {'product_id': 'p1', 'quantity': 2});
  });
}
