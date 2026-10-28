import 'package:app/features/sales/presentation/screens/funnel_branch_screen.dart';
import 'package:flutter_test/flutter_test.dart';

// One customer list, three views of it. The parsing is what the redirects from
// the old addresses depend on: `/sales/portfolio` and `/sales/my-customers` now
// land on `?view=`, so a value this cannot read is a link that goes nowhere
// useful.

void main() {
  test('the old addresses map onto their views', () {
    expect(CustomersView.fromQuery('portfolio'), CustomersView.portfolio);
    expect(CustomersView.fromQuery('inbox'), CustomersView.inbox);
    expect(CustomersView.fromQuery('pipeline'), CustomersView.pipeline);
  });

  // A bad or missing value lands on the pipeline rather than erroring: somebody
  // arriving from a stale link should see the board, not a broken screen.
  test('anything unreadable falls back to the pipeline', () {
    for (final raw in <String?>[null, '', 'portfolo', 'PORTFOLIO', 'kanban']) {
      expect(CustomersView.fromQuery(raw), CustomersView.pipeline,
          reason: 'view=$raw must not break the destination');
    }
  });

  test('there are exactly three views', () {
    // A fourth would be a fourth answer to "which screen is this customer on?",
    // which is the question movement 6 exists to close.
    expect(CustomersView.values, hasLength(3));
  });
}
