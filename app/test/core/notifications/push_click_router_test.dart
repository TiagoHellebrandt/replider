import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import 'package:app/core/notifications/push_click_router.dart';
import 'package:app/core/notifications/push_notification_service.dart';
import 'package:app/core/router/app_router.dart';

/// Captures the click handler the bridge registers so tests can fire
/// synthetic notification clicks.
class _FakePushService implements PushNotificationService {
  void Function(String target)? clickHandler;
  int disposeCalls = 0;

  @override
  void Function() onNotificationClick(void Function(String target) handler) {
    clickHandler = handler;
    return () => disposeCalls++;
  }

  @override
  bool get isSupported => true;

  @override
  PushPermission get currentPermission => PushPermission.unknown;

  @override
  Future<PushPermission> requestPermission() async => PushPermission.unknown;

  @override
  Future<PushSubscriptionPayload?> subscribe(String vapidPublicKey) async =>
      null;

  @override
  Future<PushSubscriptionPayload?> currentSubscription() async => null;

  @override
  Future<String?> unsubscribe() async => null;

  @override
  void Function() onSubscriptionChange(
          void Function(SubscriptionChange event) handler) =>
      () {};
}

void main() {
  GoRouter testRouter() => GoRouter(
        routes: [
          GoRoute(path: '/', builder: (_, __) => const Text('home-screen')),
          GoRoute(
            path: '/support',
            builder: (_, __) => const Text('support-screen'),
          ),
          // Mirrors the real /home route: the query string is what carries
          // the thread to open (push_dispatch.go's click_url).
          GoRoute(
            path: '/home',
            builder: (_, state) =>
                Text('home:${state.uri.queryParameters['conversation'] ?? ''}'),
          ),
        ],
      );

  Widget harness(_FakePushService fake, GoRouter router) {
    return ProviderScope(
      overrides: [
        pushNotificationServiceProvider.overrideWithValue(fake),
        appRouterProvider.overrideWith((ref) => router),
      ],
      child: Consumer(
        builder: (context, ref, _) {
          // Same wiring as RepliderApp: keep the bridge alive.
          ref.watch(pushClickRouterProvider);
          return MaterialApp.router(
            routerConfig: ref.watch(appRouterProvider),
          );
        },
      ),
    );
  }

  testWidgets('notification click deep-links into the target route',
      (tester) async {
    final fake = _FakePushService();
    final router = testRouter();
    addTearDown(router.dispose);

    await tester.pumpWidget(harness(fake, router));
    await tester.pumpAndSettle();

    expect(find.text('home-screen'), findsOneWidget);
    expect(fake.clickHandler, isNotNull,
        reason: 'the bridge must register on app start');

    fake.clickHandler!('/support');
    await tester.pumpAndSettle();

    expect(find.text('support-screen'), findsOneWidget);
  });

  testWidgets('normalizes a target missing the leading slash', (tester) async {
    final fake = _FakePushService();
    final router = testRouter();
    addTearDown(router.dispose);

    await tester.pumpWidget(harness(fake, router));
    await tester.pumpAndSettle();

    fake.clickHandler!('support');
    await tester.pumpAndSettle();

    expect(find.text('support-screen'), findsOneWidget);
  });

  testWidgets('empty target falls back to home', (tester) async {
    final fake = _FakePushService();
    final router = testRouter();
    addTearDown(router.dispose);

    await tester.pumpWidget(harness(fake, router));
    await tester.pumpAndSettle();

    fake.clickHandler!('/support');
    await tester.pumpAndSettle();
    fake.clickHandler!('');
    await tester.pumpAndSettle();

    expect(find.text('home-screen'), findsOneWidget);
  });

  // The inbound push's click_url: the query string must survive the hop,
  // it is what tells the inbox which thread to open.
  testWidgets('keeps the query string of a conversation deep link',
      (tester) async {
    final fake = _FakePushService();
    final router = testRouter();
    addTearDown(router.dispose);

    await tester.pumpWidget(harness(fake, router));
    await tester.pumpAndSettle();

    fake.clickHandler!('/home?conversation=01ABC');
    await tester.pumpAndSettle();

    expect(find.text('home:01ABC'), findsOneWidget);
  });
}
