import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/features/feedback/presentation/controllers/cancellation_survey_controller.dart';

/// Minimal adapter that records the request and returns a canned status, so
/// the controller's `/me/cancellation-survey` POST can be exercised without a
/// network.
class _FakeAdapter implements HttpClientAdapter {
  _FakeAdapter(this.statusCode);
  final int statusCode;
  RequestOptions? lastRequest;
  String? lastBody;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<dynamic>? cancelFuture,
  ) async {
    lastRequest = options;
    lastBody = options.data?.toString();
    return ResponseBody.fromString('', statusCode);
  }

  @override
  void close({bool force = false}) {}
}

Dio _dioWith(_FakeAdapter adapter) {
  final dio = Dio(BaseOptions(baseUrl: 'http://test/v1'));
  dio.httpClientAdapter = adapter;
  return dio;
}

void main() {
  test('reason keys match the API whitelist exactly', () {
    expect(cancellationReasonKeys, <String>[
      'too_expensive',
      'missing_features',
      'not_using',
      'switched_competitor',
      'technical_issues',
      'temporary_pause',
      'other',
    ]);
  });

  test('submit posts reason+comment and returns null on success', () async {
    final adapter = _FakeAdapter(204);
    final container = ProviderContainer(
      overrides: [dioProvider.overrideWithValue(_dioWith(adapter))],
    );
    addTearDown(container.dispose);

    final err = await container
        .read(cancellationSurveyControllerProvider.notifier)
        .submit('too_expensive', 'too pricey');

    expect(err, isNull);
    expect(adapter.lastRequest?.path, '/me/cancellation-survey');
    expect(adapter.lastBody, contains('too_expensive'));
    expect(adapter.lastBody, contains('too pricey'));
  });

  test('submit returns an error string on failure', () async {
    final adapter = _FakeAdapter(500);
    final container = ProviderContainer(
      overrides: [dioProvider.overrideWithValue(_dioWith(adapter))],
    );
    addTearDown(container.dispose);

    final err = await container
        .read(cancellationSurveyControllerProvider.notifier)
        .submit('other', '');

    expect(err, isNotNull);
  });
}
