import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/features/feedback/presentation/controllers/nps_prompt_controller.dart';

/// Fake adapter: answers GET /me/nps-prompt with a canned `eligible`, and
/// records the POSTs to /me/nps and /me/nps/dismiss with a canned status.
class _FakeAdapter implements HttpClientAdapter {
  _FakeAdapter({this.eligible = true, this.postStatus = 204});
  final bool eligible;
  final int postStatus;
  final List<String> paths = [];
  String? lastPostBody;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<dynamic>? cancelFuture,
  ) async {
    paths.add(options.path);
    if (options.method == 'GET') {
      return ResponseBody.fromString(
        '{"eligible": $eligible}',
        200,
        headers: {
          Headers.contentTypeHeader: [Headers.jsonContentType],
        },
      );
    }
    lastPostBody = options.data?.toString();
    return ResponseBody.fromString('', postStatus);
  }

  @override
  void close({bool force = false}) {}
}

ProviderContainer _container(_FakeAdapter adapter) {
  final dio = Dio(BaseOptions(baseUrl: 'http://test/v1'))
    ..httpClientAdapter = adapter;
  final c = ProviderContainer(
    overrides: [dioProvider.overrideWithValue(dio)],
  );
  return c;
}

void main() {
  test('build returns false (show) when the server says eligible', () async {
    final container = _container(_FakeAdapter(eligible: true));
    addTearDown(container.dispose);

    final handled = await container.read(npsPromptControllerProvider.future);
    expect(handled, isFalse);
  });

  test('build returns true (hide) when the server says not eligible', () async {
    final container = _container(_FakeAdapter(eligible: false));
    addTearDown(container.dispose);

    final handled = await container.read(npsPromptControllerProvider.future);
    expect(handled, isTrue);
  });

  test('build fails closed (hide) on a network/parse error', () async {
    // No dio override → real Dio hits a dead host → throws → fail closed.
    final container = ProviderContainer();
    addTearDown(container.dispose);

    expect(await container.read(npsPromptControllerProvider.future), isTrue);
  });

  test('dismiss posts /me/nps/dismiss and hides the prompt', () async {
    final adapter = _FakeAdapter(eligible: true);
    final container = _container(adapter);
    addTearDown(container.dispose);

    await container.read(npsPromptControllerProvider.future);
    await container.read(npsPromptControllerProvider.notifier).dismiss();

    expect(container.read(npsPromptControllerProvider).value, isTrue);
    expect(adapter.paths, contains('/me/nps/dismiss'));
  });

  test('submit posts score+comment, returns null and hides the prompt',
      () async {
    final adapter = _FakeAdapter(eligible: true, postStatus: 204);
    final container = _container(adapter);
    addTearDown(container.dispose);

    await container.read(npsPromptControllerProvider.future);
    final err = await container
        .read(npsPromptControllerProvider.notifier)
        .submit(9, 'loved it');

    expect(err, isNull);
    expect(container.read(npsPromptControllerProvider).value, isTrue);
    expect(adapter.paths, contains('/me/nps'));
    expect(adapter.lastPostBody, contains('9'));
    expect(adapter.lastPostBody, contains('loved it'));
  });

  test('submit returns an error string and does NOT hide on failure', () async {
    final adapter = _FakeAdapter(eligible: true, postStatus: 500);
    final container = _container(adapter);
    addTearDown(container.dispose);

    await container.read(npsPromptControllerProvider.future);
    final err = await container
        .read(npsPromptControllerProvider.notifier)
        .submit(2, '');

    expect(err, isNotNull);
    // A failed submit must leave the gate open so the user can retry.
    expect(container.read(npsPromptControllerProvider).value, isFalse);
  });
}
