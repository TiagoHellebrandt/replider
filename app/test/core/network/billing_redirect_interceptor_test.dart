import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:app/core/network/billing_redirect_interceptor.dart';

void main() {
  late GlobalKey<NavigatorState> navKey;
  late GoRouter router;
  // Mutable clock the interceptor reads, so we control the cooldown window.
  late DateTime now;

  GoRouter buildRouter(String initial) {
    navKey = GlobalKey<NavigatorState>();
    return GoRouter(
      navigatorKey: navKey,
      initialLocation: initial,
      routes: [
        GoRoute(path: '/home', builder: (_, __) => const Text('home')),
        GoRoute(path: '/billing', builder: (_, __) => const Text('billing')),
        GoRoute(path: '/settings', builder: (_, __) => const Text('settings')),
      ],
    );
  }

  Future<void> pumpApp(WidgetTester tester) async {
    await tester.pumpWidget(MaterialApp.router(routerConfig: router));
    await tester.pumpAndSettle();
  }

  DioException error402(String code) => DioException(
        requestOptions: RequestOptions(path: '/agents'),
        response: Response(
          requestOptions: RequestOptions(path: '/agents'),
          statusCode: 402,
          data: {'code': code},
        ),
      );

  void fire(BillingRedirectInterceptor i, DioException err) {
    i.onError(err, _Sink());
  }

  setUp(() {
    now = DateTime(2026, 6, 5, 12, 0, 0);
  });

  testWidgets('first 402 navigates to /billing', (tester) async {
    router = buildRouter('/home');
    await pumpApp(tester);
    final i = BillingRedirectInterceptor(navKey, now: () => now);

    fire(i, error402('TRIAL_EXPIRED'));
    await tester.pump();
    await tester.pumpAndSettle();

    expect(find.text('billing'), findsOneWidget);
  });

  testWidgets('burst of 402s within cooldown navigates only once',
      (tester) async {
    router = buildRouter('/home');
    await pumpApp(tester);
    final i = BillingRedirectInterceptor(navKey, now: () => now);

    fire(i, error402('SUBSCRIPTION_INACTIVE'));
    await tester.pump();
    await tester.pumpAndSettle();
    expect(find.text('billing'), findsOneWidget);

    // User navigates away, then a second 402 fires inside the 5s window.
    router.go('/home');
    await tester.pumpAndSettle();
    now = now.add(const Duration(seconds: 2));
    fire(i, error402('SUBSCRIPTION_INACTIVE'));
    await tester.pump();
    await tester.pumpAndSettle();

    // Cooldown still active → stays on /home.
    expect(find.text('home'), findsOneWidget);
  });

  testWidgets('second 402 after cooldown re-navigates to /billing',
      (tester) async {
    router = buildRouter('/home');
    await pumpApp(tester);
    final i = BillingRedirectInterceptor(navKey, now: () => now);

    fire(i, error402('TRIAL_EXPIRED'));
    await tester.pump();
    await tester.pumpAndSettle();
    expect(find.text('billing'), findsOneWidget);

    router.go('/home');
    await tester.pumpAndSettle();

    // Past the 5s cooldown a fresh 402 must redirect again — this is the
    // regression the old single-shot `_hasRedirectedOnce` flag failed.
    now = now.add(const Duration(seconds: 6));
    fire(i, error402('TRIAL_EXPIRED'));
    await tester.pump();
    await tester.pumpAndSettle();

    expect(find.text('billing'), findsOneWidget);
  });

  testWidgets('does not redirect while already on /settings', (tester) async {
    router = buildRouter('/settings');
    await pumpApp(tester);
    final i = BillingRedirectInterceptor(navKey, now: () => now);

    fire(i, error402('TRIAL_EXPIRED'));
    await tester.pump();
    await tester.pumpAndSettle();

    expect(find.text('settings'), findsOneWidget);
  });

  testWidgets('ignores non-billing 402 codes', (tester) async {
    router = buildRouter('/home');
    await pumpApp(tester);
    final i = BillingRedirectInterceptor(navKey, now: () => now);

    fire(i, error402('SOME_OTHER_CODE'));
    await tester.pump();
    await tester.pumpAndSettle();

    expect(find.text('home'), findsOneWidget);
  });
}

/// Swallows `handler.next` so the chain terminates in the test.
class _Sink extends ErrorInterceptorHandler {
  @override
  void next(DioException err) {}
}
