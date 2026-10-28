import 'package:flutter/widgets.dart';

/// A single slot for the product's one-shot prompts: the product tour, the
/// "what's new" announcement, the NPS survey, the quota wall and the
/// cancellation survey.
///
/// Each launcher used to dedupe only against ITSELF (`_xShownThisSession`),
/// which is enough to stop one prompt showing twice but does nothing to stop
/// two different prompts showing at once. Every launcher runs in the SAME
/// dashboard build and queues its own post-frame callback, so their
/// "priority" comments described an ordering that never happened: the
/// callbacks all fired and each pushed its own route. A first login could
/// land on three stacked dialogs, dismissed one at a time to reveal the next
/// — plus whatever the user had deliberately opened underneath.
///
/// So the slot is claimed at SHOW time, not at decide time. The first
/// callback to run gets the screen; the others back off and try again on a
/// later build (the dashboard rebuilds constantly), which turns a pile into
/// a queue.
bool _promptOnScreen = false;

/// Claims the slot. Returns false when another prompt already holds it — the
/// caller must then NOT show, and should leave its own session flag clear so
/// it gets another chance.
bool claimOneShotPrompt() {
  if (_promptOnScreen) return false;
  _promptOnScreen = true;
  return true;
}

/// Releases the slot. Safe to call more than once.
void releaseOneShotPrompt() {
  _promptOnScreen = false;
}

@visibleForTesting
bool get oneShotPromptIsOpen => _promptOnScreen;

@visibleForTesting
void resetOneShotPromptForTest() {
  _promptOnScreen = false;
}

/// Runs [show] only if the slot is free, releasing it when the prompt closes.
///
/// [onDeclined] runs when the slot was busy — launchers use it to clear their
/// session flag so the prompt is reconsidered on the next build instead of
/// being lost for the whole session.
void showAsOneShotPrompt(
  Future<void> Function() show, {
  required VoidCallback onDeclined,
}) {
  if (!claimOneShotPrompt()) {
    onDeclined();
    return;
  }
  show().whenComplete(releaseOneShotPrompt);
}
