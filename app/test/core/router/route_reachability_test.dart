import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

// Every route in the router must be reachable by clicking.
//
// Three screens shipped in one day with no entry point: built, tested, routed —
// and invisible, because nothing anywhere called `context.go` with their path.
// No existing gate could catch it: `flutter analyze` has no concept of a
// reachable screen, and a widget test mounts the screen directly, so it passes
// precisely because it never had to navigate there.
//
// This reads the source rather than the widget tree because reachability is a
// property of the whole app, not of any one screen.

/// Routes with no in-app link, each for a reason worth writing down.
///
/// Adding an entry here is a decision that a screen is reached some other way.
/// Adding one because the test went red is how the drawer of dead screens
/// refills.
const _justifiedWithoutLink = <String, String>{
  // Shell branches: the bottom/rail navigation switches by index, never by path.
  '/start': 'shell branch (index 0)',
  '/funnel': 'shell branch (index 1)',
  '/home': 'shell branch (index 2)',
  '/prospect': 'shell branch (index 3)',
  // Left the shell and the account menu on 2026-09-01 (the MCP connector does
  // the same job). Answers by URL only until 2026-10-01, then delete the route
  // and copilot_screen.dart together with this line.
  '/copilot': 'URL-only for 30 days (bookmarks), removed 2026-10-01',
  // Merged into Clientes › Carteira on 2026-09-01; a redirect for saved links.
  '/contacts': 'redirects to /funnel?view=portfolio until 2026-10-01',
  // Login is where the router sends people; nothing links to it on purpose.
  '/': 'router redirect target',
  // Arrived at from an email, never from a screen.
  '/reset-password': 'email link',
  // Legacy alias kept for old bookmarks/links: redirects into the shell.
  // In-app CTAs now go straight to /prospect?tab=signals (the Radar; the
  // hunt tab died in phase 3).
  '/sales/prospecting': 'redirect alias (bookmarks)',
  '/verify-email': 'email link',
  '/forgot-password': 'linked from the login form',
  '/register': 'linked from the login form',
  // Pure redirect, no screen of its own.
  '/sales/approvals': 'redirects to /prospect?tab=queue',
  '/sales': 'redirects to /funnel; the old hub screen is gone',
  // Movement 6: three views of one customer list. These addresses stay as
  // redirects because somebody may have them saved, but nothing links to them
  // any more — the views are reached by the toggle in the destination.
  '/sales/portfolio': 'redirects to /funnel?view=portfolio',
  '/sales/my-customers': 'redirects to /funnel?view=inbox',
  // Reached only when the account is new, via the auth redirect.
  '/onboarding': 'router redirect for user.isNew',
};

String _repoFile(String relative) {
  // Tests run with the package root as cwd.
  return File(relative).readAsStringSync();
}

Iterable<String> _dartSources() sync* {
  final dir = Directory('lib');
  for (final e in dir.listSync(recursive: true)) {
    if (e is File && e.path.endsWith('.dart') && !e.path.endsWith('.g.dart')) {
      yield e.path;
    }
  }
}

/// Paths declared in the router.
Set<String> _declaredRoutes() {
  final src = _repoFile('lib/core/router/app_router.dart');
  return RegExp(r"path: '([^']+)'")
      .allMatches(src)
      .map((m) => m.group(1)!)
      .where((p) => p.startsWith('/'))
      .toSet();
}

/// Every literal handed to context.go/push/replace anywhere in the app.
///
/// Matched across newlines because these calls are routinely wrapped, and the
/// first version of this check missed a whole file by anchoring to one line.
///
/// The leading `\b` rather than `\.` matters: `account_menu.dart` takes the
/// navigator as a callback and calls a bare `go('/x')`, so a dot-anchored
/// pattern was blind to that entire file — which is how `/case-report` sat in
/// the justified list as an unlinked deep link while the account menu had been
/// linking it all along.
Set<String> _navigatedPaths() {
  final out = <String>{};
  final call = RegExp(r"\b(?:go|push|replace)\(\s*'(/[^']*)", dotAll: true);
  // Paths that travel as DATA before somebody go()es them: the home screen's
  // tiles carry a `route:` field and pending actions a `deepLink:`. Two dead
  // tiles shipped because only the literal-call regex above was checked —
  // '/sales/analytics' sat in a `route:` field pointing at a cut route.
  final field = RegExp(r"\b(?:route|deepLink):\s*'(/[^']*)");
  for (final path in _dartSources()) {
    if (path.endsWith('app_router.dart')) continue;
    final src = File(path).readAsStringSync();
    for (final m in call.allMatches(src)) {
      out.add(m.group(1)!);
    }
    for (final m in field.allMatches(src)) {
      out.add(m.group(1)!);
    }
  }
  return out;
}

/// Turns `/settings/agent/${a.id}/knowledge` and `/funnel?lead=x` into a shape
/// comparable with the router's `/settings/agent/:id/knowledge`.
String _normalize(String raw) {
  var s = raw.split('?').first;
  s = s.replaceAll(RegExp(r'\$\{[^}]*\}'), '*');
  s = s.replaceAll(RegExp(r'\$[A-Za-z_][A-Za-z0-9_.]*'), '*');
  // An interpolation containing its own quotes — `'/x${a ? '' : '?y=$z'}'` —
  // ends the literal early for any regex, leaving a dangling `${`. Everything
  // from there on is expression, not path. Dropping it recovers the real
  // prefix instead of writing the route off as an orphan, which is what the
  // first version of this check did to a route that was linked all along.
  final dangling = s.indexOf(r'${');
  if (dangling >= 0) s = s.substring(0, dangling);
  return s;
}

bool _matches(String route, Iterable<String> navigated) {
  final pattern = RegExp(
      '^${route.split('/').map((seg) => seg.startsWith(':') ? '[^/]+' : RegExp.escape(seg)).join('/')}\$');
  for (final n in navigated) {
    final candidate = _normalize(n).replaceAll('*', 'x');
    if (pattern.hasMatch(candidate)) return true;
  }
  return false;
}

/// True when [navigated] (already normalized) is answered by some declared
/// route, treating `:param` and interpolated `*` segments as wildcards.
bool _isDeclared(String navigated, Iterable<String> declared) {
  final segs = navigated.split('/');
  for (final route in declared) {
    final rsegs = route.split('/');
    if (rsegs.length != segs.length) continue;
    var ok = true;
    for (var i = 0; i < segs.length; i++) {
      if (rsegs[i].startsWith(':') || segs[i] == '*') continue;
      if (rsegs[i] != segs[i]) {
        ok = false;
        break;
      }
    }
    if (ok) return true;
  }
  return false;
}

void main() {
  test('every route has a link somewhere in the app', () {
    final declared = _declaredRoutes();
    final navigated = _navigatedPaths();

    final orphans = <String>[];
    for (final route in declared) {
      if (_justifiedWithoutLink.containsKey(route)) continue;
      if (!_matches(route, navigated)) orphans.add(route);
    }
    orphans.sort();

    expect(
      orphans,
      isEmpty,
      reason: 'These routes exist but nothing in the app navigates to them, so '
          'the only way in is typing the URL:\n  ${orphans.join('\n  ')}\n\n'
          'Either add a link, delete the screen, or — if it is genuinely '
          'reached another way — add it to _justifiedWithoutLink with the '
          'reason.',
    );
  });

  // The other direction, and the one that bit: a `/sales` hub screen was
  // deleted while eight back buttons still fell back to `context.go('/sales')`.
  // Nothing caught it — reachability only asks whether routes have links, and
  // a link to nowhere lands on the not-found screen at runtime, which no test
  // ever visits.
  test('every navigated path names a route that exists', () {
    final declared = _declaredRoutes();
    final dead = <String>[];

    for (final raw in _navigatedPaths()) {
      // A literal cut short by its own interpolated quotes is not a path we can
      // check; asserting on the truncated prefix would fail honest code.
      if (raw.contains(r'${')) continue;
      final normalized = _normalize(raw);
      if (normalized.isEmpty || normalized == '/') continue;
      if (!_isDeclared(normalized, declared)) dead.add(raw);
    }
    dead.sort();

    expect(
      dead,
      isEmpty,
      reason: 'These paths are navigated to but no route answers them, so the '
          'tap lands on the not-found screen:\n  ${dead.join('\n  ')}',
    );
  });

  test('the exception list does not outlive its routes', () {
    // A justification for a route that no longer exists is stale paperwork
    // that quietly widens the hole for the next route with the same path.
    final declared = _declaredRoutes();
    final stale =
        _justifiedWithoutLink.keys.where((r) => !declared.contains(r)).toList();

    expect(stale, isEmpty,
        reason: 'These are excused but no longer in the router: $stale');
  });
}
