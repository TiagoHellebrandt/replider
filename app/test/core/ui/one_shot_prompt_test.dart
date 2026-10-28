import 'dart:async';

import 'package:app/core/ui/one_shot_prompt.dart';
import 'package:flutter_test/flutter_test.dart';

// A first login used to land on three stacked dialogs — the product tour,
// the "what's new" announcement and the NPS survey all queue their
// post-frame callback from the SAME dashboard build, so their comments
// about taking "priority" over each other described an ordering that never
// happened. The slot makes the first one to run the only one on screen.

void main() {
  setUp(resetOneShotPromptForTest);

  test('the second prompt of a build is declined, not shown', () {
    final shown = <String>[];
    final declined = <String>[];
    final first = Completer<void>();

    showAsOneShotPrompt(() => first.future,
        onDeclined: () => declined.add('tour'));
    shown.add('tour');

    showAsOneShotPrompt(() async => shown.add('nps'),
        onDeclined: () => declined.add('nps'));

    expect(shown, ['tour'], reason: 'only the first prompt reached the screen');
    expect(declined, ['nps']);
  });

  // Declining must not consume the prompt: the launcher clears its session
  // flag and the dashboard's next build gets to try again.
  test('a declined prompt can show once the slot frees up', () async {
    final first = Completer<void>();
    showAsOneShotPrompt(() => first.future, onDeclined: () {});

    var second = false;
    showAsOneShotPrompt(() async => second = true, onDeclined: () {});
    expect(second, isFalse);

    first.complete();
    await Future<void>.delayed(Duration.zero);

    showAsOneShotPrompt(() async => second = true, onDeclined: () {});
    expect(second, isTrue);
  });

  test('closing a prompt releases the slot', () async {
    final done = Completer<void>();
    showAsOneShotPrompt(() => done.future, onDeclined: () {});
    expect(oneShotPromptIsOpen, isTrue);

    done.complete();
    await Future<void>.delayed(Duration.zero);
    expect(oneShotPromptIsOpen, isFalse);
  });

  // A prompt that throws must not wedge the slot shut for the session. The
  // error still surfaces — releasing the slot is not the same as hiding the
  // failure.
  test('a failed prompt still releases the slot', () async {
    Object? caught;
    await runZonedGuarded(() async {
      showAsOneShotPrompt(() => Future<void>.error(StateError('boom')),
          onDeclined: () {});
      await Future<void>.delayed(Duration.zero);
    }, (e, _) => caught = e);

    expect(oneShotPromptIsOpen, isFalse);
    expect(caught, isStateError, reason: 'the failure is not swallowed');
  });
}
