import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:app/core/network/dio_client.dart';
import 'package:app/core/network/websocket_service.dart';
import 'package:app/features/dashboard/presentation/controllers/waiting_count_controller.dart';
import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

class _FixedAdapter implements HttpClientAdapter {
  _FixedAdapter(this.count);
  int count;
  int hits = 0;

  @override
  Future<ResponseBody> fetch(RequestOptions options,
      Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async {
    hits++;
    return ResponseBody.fromString(jsonEncode({'count': count}), 200, headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    });
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));

  test('reads the count and refreshes on inbox events, debounced', () async {
    final adapter = _FixedAdapter(3);
    final dio = Dio(BaseOptions(baseUrl: 'http://x'))
      ..httpClientAdapter = adapter;
    final ws = StreamController<WSEvent>.broadcast();

    final container = ProviderContainer(overrides: [
      dioProvider.overrideWithValue(dio),
      wsEventsProvider.overrideWith((ref) => ws.stream),
    ]);
    // Teardown order matters (LIFO): dispose the container FIRST so the
    // wsEvents subscription cancels — a broadcast close() waits for its
    // listeners and would deadlock the teardown otherwise.
    addTearDown(ws.close);
    addTearDown(container.dispose);
    // The shell keeps the provider watched for the whole session; without a
    // live listener Riverpod pauses its subscriptions and the WS refresh
    // would (correctly) not run. The test mirrors the shell.
    final keep = container.listen(waitingCountProvider, (_, __) {});
    addTearDown(keep.close);

    expect(await container.read(waitingCountProvider.future), 3);
    expect(adapter.hits, 1);

    // A burst of messages costs ONE refresh, after the debounce window.
    adapter.count = 5;
    ws.add(WSEvent(event: 'new_message', raw: {}));
    ws.add(WSEvent(event: 'new_message', raw: {}));
    ws.add(WSEvent(event: 'conversation_updated', raw: {}));
    await Future<void>.delayed(const Duration(milliseconds: 50));
    expect(adapter.hits, 1, reason: 'debounce holds the burst');
    await Future<void>.delayed(const Duration(seconds: 2, milliseconds: 200));
    expect(await container.read(waitingCountProvider.future), 5);
    expect(adapter.hits, 2);
  });

  test('a failed read is zero, never an error', () async {
    final dio = Dio(BaseOptions(baseUrl: 'http://x'))
      ..httpClientAdapter = _ThrowingAdapter();
    final container = ProviderContainer(overrides: [
      dioProvider.overrideWithValue(dio),
      wsEventsProvider.overrideWith((ref) => const Stream.empty()),
    ]);
    addTearDown(container.dispose);

    expect(await container.read(waitingCountProvider.future), 0);
  });
}

class _ThrowingAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(RequestOptions options,
          Stream<Uint8List>? requestStream, Future<void>? cancelFuture) =>
      throw DioException(requestOptions: options);

  @override
  void close({bool force = false}) {}
}
