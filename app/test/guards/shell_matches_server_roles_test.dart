import 'dart:io';

import 'package:app/core/auth/account_role.dart';
import 'package:flutter_test/flutter_test.dart';

// The shell hides what the server will refuse — `AppShell.destinationsFor`
// carries a `requires: AccountRole.x` per destination, written by hand to
// mirror a `RequireRole(...)` group in the Go router. Nothing checked that the
// two agreed: Clientes required `vendedor` in the app while `GET /funnels` sat
// in `RequireRole("admin")` on the server, and every vendedora opened her own
// destination onto a 403 with a Retry that could never succeed.
//
// This reads BOTH sources — the Dart shell and the Go route files — and asks,
// for every role the app would show a destination to, whether the server's
// gate on that destination's first request lets the same role through. It
// uses the app's own satisfaction table (`AccountRole.satisfies`, a copy of
// `domain/rbac.go`), so a drift there shows up here too.

/// Shell destination → the FIRST request its screen makes, as `METHOD /path`
/// relative to `/v1`. This is the request whose `RequireRole` decides whether
/// the person sees a screen or an error.
///
/// `null` means the route is gated inside the use case, not by a
/// `RequireRole` group, so there is nothing to parse; the entry still has to
/// exist so a new destination cannot ship without deciding what it hits.
const _firstRequest = <String, String?>{
  // StartScreen → GET /me/command-center: mounted on the bare `authMw`
  // group; `command_center.go` answers ErrForbidden to an attendant itself.
  '/start': null,
  // FunnelBranchScreen needs a funnel id before anything else.
  '/funnel': 'GET /funnels',
  // DashboardScreen: the inbox list.
  '/home': 'GET /conversations',
  // ProspectBranchScreen opens on the approval queue.
  '/prospect': 'GET /leads/approval-queue',
};

const _goRouterDir = '../api/internal/adapters/http/router';

/// `branch index → required role`, read from `AppShell.destinationsFor`.
Map<int, AccountRole> _shellRequirements() {
  final src = File('lib/core/router/app_shell.dart').readAsStringSync();
  final out = <int, AccountRole>{};
  final re = RegExp(
      r'_Destination\(\s*branch:\s*(\d+),.*?requires:\s*AccountRole\.(\w+)',
      dotAll: true);
  for (final m in re.allMatches(src)) {
    final role = AccountRole.values.where((r) => r.name == m.group(2));
    expect(role, isNotEmpty,
        reason: 'app_shell.dart requires an unknown role ${m.group(2)}');
    out[int.parse(m.group(1)!)] = role.single;
  }
  return out;
}

/// `branch index → path`, read from the router's `StatefulShellRoute` in
/// declaration order — the same order the shell's `branch:` numbers follow.
List<String> _shellBranchPaths() {
  final src = File('lib/core/router/app_router.dart').readAsStringSync();
  final start = src.indexOf('StatefulShellRoute.indexedStack(');
  expect(start, greaterThanOrEqualTo(0), reason: 'no StatefulShellRoute');
  // The shell's `branches: [...]` ends where the top-level routes resume.
  final end = src.indexOf('\n      ),\n', start);
  final body = src.substring(start, end);
  return RegExp(r"StatefulShellBranch\(.*?path: '([^']+)'", dotAll: true)
      .allMatches(body)
      .map((m) => m.group(1)!)
      .toList();
}

/// One middleware argument list → the role wire names of its `RequireRole`,
/// written inline (`middleware.RequireRole("admin")`) or through a local
/// alias (`leadActorRole := middleware.RequireRole(...)`, then
/// `v1.Group("/leads", authPaidMw, leadActorRole)`), which is how
/// lead_routes.go shares one role set across its groups.
List<String>? _requireRole(String args, Map<String, List<String>> aliases) {
  final m = RegExp(r'RequireRole\(([^)]*)\)').firstMatch(args);
  if (m != null) {
    return RegExp(r'"(\w+)"')
        .allMatches(m.group(1)!)
        .map((r) => r.group(1)!)
        .toList();
  }
  for (final name in RegExp(r'\b(\w+)\b').allMatches(args)) {
    final roles = aliases[name.group(1)];
    if (roles != null) return roles;
  }
  return null;
}

class _Group {
  _Group(this.parent, this.prefix, this.roles);
  final String parent;
  final String prefix;
  List<String>? roles;
}

/// Every `METHOD /path` the Go router registers, with the role set that gates
/// it: the route's own `RequireRole` when it has one, else the nearest group's
/// up the chain (gin runs the group's middleware first, so a route-level
/// gate can only narrow the group's).
///
/// Parsed per function because the files reuse variable names (`protected`,
/// `g`) across setup functions.
Map<String, List<String>?> _goRoutes() {
  final out = <String, List<String>?>{};
  final files = Directory(_goRouterDir)
      .listSync()
      .whereType<File>()
      .where((f) => f.path.endsWith('.go') && !f.path.endsWith('_test.go'));
  final groupDecl = RegExp(
      r'^\s*(\w+)\s*:=\s*(\w+)\.Group\("([^"]*)"(.*)\)\s*$',
      multiLine: true);
  final groupUse = RegExp(r'^\s*(\w+)\.Use\((.*)\)\s*$', multiLine: true);
  final route = RegExp(
      r'^\s*(\w+)\.(GET|POST|PUT|PATCH|DELETE)\("([^"]*)"(.*)\)\s*$',
      multiLine: true);
  final alias = RegExp(r'^\s*(\w+)\s*:=\s*middleware\.RequireRole\(([^)]*)\)',
      multiLine: true);

  for (final file in files) {
    for (final fn in file.readAsStringSync().split('\nfunc ')) {
      final aliases = <String, List<String>>{
        for (final m in alias.allMatches(fn))
          m.group(1)!: _requireRole('RequireRole(${m.group(2)})', const {})!,
      };
      final groups = <String, _Group>{};
      for (final m in groupDecl.allMatches(fn)) {
        groups[m.group(1)!] = _Group(
            m.group(2)!, m.group(3)!, _requireRole(m.group(4)!, aliases));
      }
      for (final m in groupUse.allMatches(fn)) {
        final roles = _requireRole(m.group(2)!, aliases);
        if (roles != null) groups[m.group(1)!]?.roles = roles;
      }

      String prefix(String v) {
        final g = groups[v];
        return g == null ? '' : prefix(g.parent) + g.prefix;
      }

      List<String>? gate(String v) {
        final g = groups[v];
        if (g == null) return null;
        return g.roles ?? gate(g.parent);
      }

      for (final m in route.allMatches(fn)) {
        final key = '${m.group(2)} ${prefix(m.group(1)!)}${m.group(3)}';
        final own = _requireRole(m.group(4)!, aliases);
        // gin panics on a duplicate registration, so the first one is the
        // only one; keeping it mirrors what the server would do.
        out.putIfAbsent(key, () => own ?? gate(m.group(1)!));
      }
    }
  }
  return out;
}

AccountRole _wire(String wire) {
  final role = AccountRole.values.where((r) => r.wire == wire);
  expect(role, isNotEmpty, reason: 'Go router names an unknown role "$wire"');
  return role.single;
}

void main() {
  test('every shell destination declares the request it is gated by', () {
    final requirements = _shellRequirements();
    final paths = _shellBranchPaths();

    expect(requirements, isNotEmpty, reason: 'no destinations parsed');
    for (final branch in requirements.keys) {
      expect(branch, lessThan(paths.length),
          reason: 'destination branch $branch has no shell branch');
      final path = paths[branch];
      expect(_firstRequest.containsKey(path), isTrue,
          reason: 'Destination $path (branch $branch) has no entry in '
              '_firstRequest — say which request its screen makes first, or '
              'null with the reason if the use case gates it.');
    }
    final stale = _firstRequest.keys.where((p) => !paths.contains(p));
    expect(stale, isEmpty,
        reason: 'These _firstRequest entries name no shell branch: $stale');
  });

  test('the Go router parser still finds the routes it is asked about', () {
    // A refactor of the route files must not turn this guard vacuous.
    final routes = _goRoutes();
    for (final req in _firstRequest.values.whereType<String>()) {
      expect(routes.containsKey(req), isTrue,
          reason: '$req not found in $_goRouterDir — did the route move, or '
              'did its registration stop matching the parser?');
      expect(routes[req], isNotNull,
          reason: '$req has no RequireRole on its route or group chain');
    }
  });

  test("the app's required role satisfies the server's gate", () {
    final requirements = _shellRequirements();
    final paths = _shellBranchPaths();
    final routes = _goRoutes();

    final failures = <String>[];
    requirements.forEach((branch, requires) {
      final path = paths[branch];
      final req = _firstRequest[path];
      if (req == null) return;
      final server = routes[req]!.map(_wire).toSet();

      // The app shows the destination to every role that satisfies
      // `requires`; each of them must also pass the server's gate.
      final shown = AccountRole.values.where((r) => r.satisfies(requires));
      for (final role in shown) {
        if (server.any(role.satisfies)) continue;
        failures
            .add("$path (branch $branch): the app shows it to '${role.wire}' "
                "(requires: ${requires.wire}) but $req is "
                "RequireRole(${server.map((r) => '"${r.wire}"').join(', ')}) — "
                "a ${role.wire} gets 403.");
      }
    });

    expect(failures, isEmpty,
        reason: 'Shell destinations the server refuses for a role the app '
            'shows them to:\n  ${failures.join('\n  ')}\n\n'
            'Either raise `requires` in app_shell.dart or widen the Go '
            "group's RequireRole — the pattern is to hide what the server "
            'refuses, so nobody discovers a permission by receiving an error.');
  });
}
