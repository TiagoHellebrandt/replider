import 'dart:typed_data';

import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/dashboard/presentation/controllers/ai_debounce_controller.dart';
import 'package:app/features/dashboard/presentation/widgets/ai_debounce_banner.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

// "A IA responde em ~40s" (K1).
//
// The banner exists because a wait the attendant cannot see is a bot that looks
// dead. So the tests pin that it appears only when a reply is pending, says how
// long, explains a collapsed burst, and offers a way to stop waiting.

/// A Dio whose adapter answers 202 without touching a socket, so the
/// answer-now path can be driven to completion without leaving a pending
/// request-timeout Timer behind.
class _AcceptedAdapter implements HttpClientAdapter {
  int calls = 0;

  @override
  Future<ResponseBody> fetch(RequestOptions options,
      Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async {
    calls++;
    return ResponseBody.fromString('', 202);
  }

  @override
  void close({bool force = false}) {}
}

Future<ProviderContainer> _pump(
  WidgetTester tester, {
  AiDebounceState? state,
  String conversationId = 'c1',
  HttpClientAdapter? adapter,
}) async {
  final dio = Dio();
  if (adapter != null) dio.httpClientAdapter = adapter;
  final container = ProviderContainer(overrides: [
    dioProvider.overrideWithValue(dio),
  ]);
  addTearDown(container.dispose);

  if (state != null) {
    container
        .read(aiDebounceStatesProvider.notifier)
        .set(conversationId, state);
  }

  await tester.pumpWidget(UncontrolledProviderScope(
    container: container,
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      home: Scaffold(
        body: AiDebounceBanner(conversationId: conversationId),
      ),
    ),
  ));
  await tester.pump();
  return container;
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  testWidgets('nothing pending renders nothing at all', (tester) async {
    await _pump(tester);

    // A banner that is always present is a banner nobody reads.
    expect(find.byKey(const Key('ai_debounce_banner')), findsNothing);
  });

  testWidgets('a pending reply says how long the AI will wait', (tester) async {
    await _pump(tester,
        state: AiDebounceState(
          respondsAt: DateTime.now().add(const Duration(seconds: 40)),
          messageCount: 1,
        ));

    expect(find.byKey(const Key('ai_debounce_banner')), findsOneWidget);
    final text =
        tester.widget<Text>(find.byKey(const Key('ai_debounce_text'))).data!;
    // The exact second drifts with the clock; what matters is that it counts
    // down rather than showing a bare "aguardando".
    expect(text, contains('IA responde'));
  });

  testWidgets('a single message needs no explanation', (tester) async {
    await _pump(tester,
        state: AiDebounceState(
          respondsAt: DateTime.now().add(const Duration(seconds: 30)),
          messageCount: 1,
        ));

    expect(find.byKey(const Key('ai_debounce_count')), findsNothing);
  });

  testWidgets('a collapsed burst explains itself', (tester) async {
    await _pump(tester,
        state: AiDebounceState(
          respondsAt: DateTime.now().add(const Duration(seconds: 30)),
          messageCount: 3,
        ));

    // Three messages is WHY the AI has not answered the first one.
    expect(find.text(t.ai_debounce.collapsed(count: 3)), findsOneWidget);
  });

  testWidgets('a wait that is up says it is answering, not "in 0s"',
      (tester) async {
    await _pump(tester,
        state: AiDebounceState(
          respondsAt: DateTime.now().subtract(const Duration(seconds: 5)),
          messageCount: 1,
        ));

    expect(
      tester.widget<Text>(find.byKey(const Key('ai_debounce_text'))).data,
      t.ai_debounce.answering,
    );
    // And there is nothing left to hurry.
    expect(find.byKey(const Key('ai_debounce_now')), findsNothing);
  });

  testWidgets('the attendant can stop waiting', (tester) async {
    final adapter = _AcceptedAdapter();
    final container = await _pump(tester,
        adapter: adapter,
        state: AiDebounceState(
          respondsAt: DateTime.now().add(const Duration(seconds: 40)),
          messageCount: 2,
        ));

    expect(find.byKey(const Key('ai_debounce_now')), findsOneWidget);
    await tester.tap(find.byKey(const Key('ai_debounce_now')));
    await tester.pumpAndSettle();

    // The server said "scheduled" (202), so the banner goes: the reply itself
    // arrives through the normal new-message path, and leaving a countdown on
    // screen would have it tick down against a wait that no longer exists.
    expect(adapter.calls, 1);
    expect(container.read(aiDebounceStatesProvider), isEmpty);
  });

  test('clearing removes the entry rather than zeroing it', () {
    final container = ProviderContainer();
    addTearDown(container.dispose);
    final notifier = container.read(aiDebounceStatesProvider.notifier);

    notifier.set(
        'c1',
        AiDebounceState(
          respondsAt: DateTime.now().add(const Duration(seconds: 10)),
          messageCount: 1,
        ));
    expect(container.read(aiDebounceStatesProvider), hasLength(1));

    notifier.clear('c1');
    // A lingering zeroed entry would keep a banner on screen after the reply
    // landed, which teaches people to ignore the banner.
    expect(container.read(aiDebounceStatesProvider), isEmpty);

    // Clearing something that was never there must not add an entry.
    notifier.clear('c-nope');
    expect(container.read(aiDebounceStatesProvider), isEmpty);
  });

  test('secondsLeft never goes negative', () {
    final past = AiDebounceState(
      respondsAt: DateTime.now().subtract(const Duration(minutes: 5)),
      messageCount: 1,
    );

    // A negative countdown would render "em -300s".
    expect(past.secondsLeft(DateTime.now()), 0);
  });

  test('one conversation waiting does not put a banner on another', () {
    final container = ProviderContainer();
    addTearDown(container.dispose);
    final notifier = container.read(aiDebounceStatesProvider.notifier);

    notifier.set(
        'c1',
        AiDebounceState(
          respondsAt: DateTime.now().add(const Duration(seconds: 10)),
          messageCount: 1,
        ));

    expect(notifier.forConversation('c1'), isNotNull);
    expect(notifier.forConversation('c2'), isNull);
  });
}
