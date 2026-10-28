import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

// Every screen the router mounts outside the shell must offer a way out.
//
// `/settings/*` and most of `/sales/*` render with no navigation rail and no
// bottom bar — whatever their AppBar provides is the only exit. Twelve of them
// shipped with none: you reached the screen and the only way back was editing
// the URL. On the installed PWA, where there is no browser chrome at all, that
// is a dead end with no recovery.
//
// Reads the source because the property belongs to the screen file, not to any
// one widget test: a widget test mounts the screen alone and never asks how
// somebody leaves it.

/// Screens that legitimately have no back affordance of their own.
///
/// Adding an entry is a decision that the screen carries navigation some other
/// way. Adding one to silence the test is how the drawer of dead ends refills.
const _shellBranches = <String, String>{
  'funnel_branch_screen.dart': 'shell branch — the rail is always present',
  'prospect_branch_screen.dart': 'shell branch — the rail is always present',
  'dashboard_screen.dart': 'shell branch — the rail is always present',
  'start_screen.dart': 'shell branch — the rail is always present',
  'login_screen.dart': 'entry point; there is nowhere to go back to',
  'onboarding_screen.dart': 'linear flow with its own step navigation',
};

void main() {
  test('a screen with an AppBar offers a way out', () {
    final offenders = <String>[];

    for (final entity in Directory('lib/features').listSync(recursive: true)) {
      if (entity is! File) continue;
      final path = entity.path;
      if (!path.endsWith('.dart') || path.endsWith('.g.dart')) continue;
      if (!path.contains('/presentation/screens/')) continue;

      final name = path.split('/').last;
      if (_shellBranches.containsKey(name)) continue;

      final src = entity.readAsStringSync();
      if (!src.contains('AppBar(')) continue;

      // Any of these counts: an explicit leading, a pop, or Flutter's own
      // automatic back button (which only appears when the route was pushed).
      final hasExit = src.contains('leading:') ||
          src.contains('context.pop()') ||
          src.contains('canPop()') ||
          src.contains('Navigator.of(context).pop');

      if (!hasExit) offenders.add(name);
    }
    offenders.sort();

    expect(
      offenders,
      isEmpty,
      reason: 'These screens render an AppBar with no way back. Outside the '
          'shell there is no rail and no bottom bar, so the person is stuck:\n'
          '  ${offenders.join('\n  ')}\n\n'
          'Add a back button, or — if the screen really carries navigation '
          'another way — add it to _shellBranches with the reason.',
    );
  });

  test('the exception list does not outlive its screens', () {
    final existing = Directory('lib/features')
        .listSync(recursive: true)
        .whereType<File>()
        .map((f) => f.path.split('/').last)
        .toSet();

    final stale =
        _shellBranches.keys.where((n) => !existing.contains(n)).toList();

    expect(stale, isEmpty,
        reason: 'Excused screens that no longer exist: $stale');
  });
}
