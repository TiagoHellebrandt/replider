import 'package:flutter/widgets.dart';
import 'package:go_router/go_router.dart';

/// The one back-button rule for top-level screens that live OUTSIDE the shell.
///
/// Each of them used to invent its own fallback — three of them fell back to
/// `/home` (the v1 inbox), which a vendedor cannot even open. Pop when there
/// is somewhere to pop to; otherwise go to the landing screen, never to a
/// role-gated branch.
void appBack(BuildContext context, {String fallback = '/start'}) {
  if (context.canPop()) {
    context.pop();
  } else {
    context.go(fallback);
  }
}
