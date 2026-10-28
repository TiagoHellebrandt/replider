import 'package:app/core/auth/session_expiry.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

// What `?from=` is allowed to carry.
//
// Being bounced to login must not cost someone their place, but the value
// survives a round trip through the address bar, so it is untrusted input.
// It also must not point at a screen that does not exist: a mistyped URL used
// to land the person on the dashboard after signing in, and briefly landed
// them on a 404 instead.

void main() {
  group('sanitizeReturnPath', () {
    test('keeps an in-app destination', () {
      expect(sanitizeReturnPath('/funnel'), '/funnel');
    });

    test('refuses anything that leaves the app', () {
      expect(sanitizeReturnPath('//evil.com'), isNull);
      expect(sanitizeReturnPath('https://evil.com'), isNull);
      expect(sanitizeReturnPath(r'/\evil.com'), isNull);
      expect(sanitizeReturnPath('/'), isNull);
    });
  });

  // Pins a go_router behaviour that constrains the design, documented here
  // because it is surprising and cost a wrong assumption:
  //
  // `state.matchedLocation` equals the requested path EVEN WHEN no route
  // claims it. So `redirect` cannot tell "you asked for a real screen while
  // logged out" apart from "you typed nonsense", and `?from=` may therefore
  // carry a path that does not exist.
  //
  // The accepted consequence: someone who mistypes a URL while logged out
  // signs in and lands on the not-found screen — which names the address and
  // offers a way to the start screen — instead of being silently deposited on
  // the dashboard. One extra click, and it tells them the link was wrong.
  //
  // If a future go_router makes matchedLocation distinguishable, this test
  // fails and the redirect can be tightened to drop unknown destinations.
  testWidgets('matchedLocation cannot distinguish an unmatched path',
      (tester) async {
    late GoRouterState seen;

    final router = GoRouter(
      initialLocation: '/',
      redirect: (context, state) {
        seen = state;
        return null;
      },
      routes: [
        GoRoute(path: '/', builder: (_, __) => const Text('home')),
        GoRoute(path: '/real', builder: (_, __) => const Text('real')),
      ],
      errorBuilder: (_, __) => const Text('not-found'),
    );
    addTearDown(router.dispose);

    await tester.pumpWidget(MaterialApp.router(routerConfig: router));
    await tester.pumpAndSettle();

    router.go('/nope');
    await tester.pumpAndSettle();

    expect(find.text('not-found'), findsOneWidget);
    expect(seen.uri.path, '/nope');
    // The constraint itself: redirect sees a "matched" location for a route
    // that does not exist, so it has nothing to filter on.
    expect(seen.matchedLocation, '/nope');
  });

  testWidgets('a matched path reports itself as matched', (tester) async {
    late GoRouterState seen;

    final router = GoRouter(
      initialLocation: '/',
      redirect: (context, state) {
        seen = state;
        return null;
      },
      routes: [
        GoRoute(path: '/', builder: (_, __) => const Text('home')),
        GoRoute(path: '/real', builder: (_, __) => const Text('real')),
      ],
      errorBuilder: (_, __) => const Text('not-found'),
    );
    addTearDown(router.dispose);

    await tester.pumpWidget(MaterialApp.router(routerConfig: router));
    await tester.pumpAndSettle();

    router.go('/real');
    await tester.pumpAndSettle();

    expect(find.text('real'), findsOneWidget);
    expect(seen.matchedLocation, '/real');
  });
}
