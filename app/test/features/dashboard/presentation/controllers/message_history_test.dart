import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:retrofit/retrofit.dart';

import 'package:app/features/dashboard/data/datasources/dashboard_remote_datasource.dart';
import 'package:app/features/dashboard/domain/models/message_model.dart';
import 'package:app/features/dashboard/presentation/controllers/dashboard_controller.dart';

/// Hand-rolled: a codegen-less mockito Mock cannot stub a method whose
/// return type is a non-nullable Future (the call inside `when` throws
/// before the stub is recorded).
class _FakeDataSource implements DashboardRemoteDataSource {
  _FakeDataSource(this.onGetMessages);

  final Future<HttpResponse<dynamic>> Function(
      String id, int? limit, String? before) onGetMessages;
  final calls = <String?>[];

  @override
  Future<HttpResponse<dynamic>> getMessages(String conversationId,
      {int? limit, String? before}) {
    calls.add(before);
    return onGetMessages(conversationId, limit, before);
  }

  @override
  dynamic noSuchMethod(Invocation invocation) =>
      throw UnimplementedError('${invocation.memberName}');
}

MessageModel _msg(String id, DateTime at) => MessageModel(
      id: id,
      conversationId: 'c1',
      content: id,
      senderType: SenderType.customer,
      senderLabel: 'x',
      createdAt: at,
      messageType: MessageType.text,
    );

Map<String, dynamic> _row(int i) => {
      'id': 'm$i',
      'conversation_id': 'c1',
      'content': 'm$i',
      'sender': '5511',
      'direction': 'inbound',
      'type': 'text',
      'created_at': DateTime.utc(2026, 9, 1, 10, i).toIso8601String(),
    };

HttpResponse<dynamic> _page(List<Map<String, dynamic>> rows) => HttpResponse(
      rows,
      Response<dynamic>(requestOptions: RequestOptions(path: '/x'), data: rows),
    );

void main() {
  group('mergeMessageHistory', () {
    test('history first, newest page last, no repeats', () {
      final t0 = DateTime(2026, 9, 1, 10);
      final history = [
        _msg('a', t0),
        _msg('b', t0.add(const Duration(minutes: 1)))
      ];
      final latest = [
        _msg('b', t0.add(const Duration(minutes: 1))),
        _msg('c', t0.add(const Duration(minutes: 2)))
      ];

      final merged = mergeMessageHistory(history, latest);

      expect(merged.map((m) => m.id), ['a', 'b', 'c']);
    });

    test('no history returns the newest page untouched', () {
      final latest = [_msg('c', DateTime(2026))];
      expect(identical(mergeMessageHistory(const [], latest), latest), isTrue);
    });
  });

  group('ConversationListLimit', () {
    test('starts at one page and grows by a page up to the API cap', () {
      final container = ProviderContainer();
      addTearDown(container.dispose);
      expect(container.read(conversationListLimitProvider), 50);

      container.read(conversationListLimitProvider.notifier).loadMore();
      expect(container.read(conversationListLimitProvider), 100);

      for (var i = 0; i < 5; i++) {
        container.read(conversationListLimitProvider.notifier).loadMore();
      }
      expect(container.read(conversationListLimitProvider), 200,
          reason: 'the API caps limit at 200');
    });
  });

  group('MessageHistory', () {
    test(
        'loadOlder prepends the page before the cursor and stops on a short page',
        () async {
      final ds =
          _FakeDataSource((_, __, ___) async => _page([_row(1), _row(2)]));
      final container = ProviderContainer(overrides: [
        dashboardRemoteDataSourceProvider.overrideWithValue(ds),
      ]);
      addTearDown(container.dispose);
      // Keep the autoDispose family alive across the awaits.
      final sub = container.listen(messageHistoryProvider('c1'), (_, __) {});
      addTearDown(sub.close);

      final oldestShown = DateTime.utc(2026, 9, 1, 10, 3);
      await container
          .read(messageHistoryProvider('c1').notifier)
          .loadOlder(oldestShown);

      final state = container.read(messageHistoryProvider('c1'));
      expect(state.messages.map((m) => m.id), ['m1', 'm2']);
      expect(state.exhausted, isTrue, reason: '2 < page size: nothing older');
      expect(state.loading, isFalse);
      expect(ds.calls, [oldestShown.toIso8601String()],
          reason: 'the cursor is the oldest message on screen, in UTC');

      // Exhausted: a second call does not hit the network.
      await container
          .read(messageHistoryProvider('c1').notifier)
          .loadOlder(oldestShown);
      expect(ds.calls, hasLength(1));
    });

    test('a failed page keeps what was loaded and clears loading', () async {
      final ds = _FakeDataSource((_, __, ___) async =>
          throw DioException(requestOptions: RequestOptions(path: '/x')));
      final container = ProviderContainer(overrides: [
        dashboardRemoteDataSourceProvider.overrideWithValue(ds),
      ]);
      addTearDown(container.dispose);
      final sub = container.listen(messageHistoryProvider('c1'), (_, __) {});
      addTearDown(sub.close);

      await container
          .read(messageHistoryProvider('c1').notifier)
          .loadOlder(DateTime.utc(2026));

      final state = container.read(messageHistoryProvider('c1'));
      expect(state.messages, isEmpty);
      expect(state.loading, isFalse);
      expect(state.exhausted, isFalse, reason: 'the button stays for a retry');
    });
  });
}
