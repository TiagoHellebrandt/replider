import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/features/sales/presentation/controllers/product_import_controller.dart';
import 'package:app/i18n/strings.g.dart';

// Importing nineteen brochures, the third exhausted the account's AI quota.
// The batch then spent minutes uploading the other sixteen to reproduce the
// same refusal — one doomed multimodal call at a time — and the person watched
// a spinner to be told "tente de novo".
//
// Two rules come out of that, and these pin both: a refusal about the SERVICE
// stops the batch, a refusal about THIS FILE does not.

/// Answers each request with a status from [statuses], in call order.
class _ScriptedAdapter implements HttpClientAdapter {
  _ScriptedAdapter(this.statuses);
  final List<int> statuses;
  int calls = 0;

  @override
  Future<ResponseBody> fetch(RequestOptions options,
      Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async {
    final i = calls++;
    final status = i < statuses.length ? statuses[i] : 200;
    if (status == 200) {
      return ResponseBody.fromString(
        '{"rows":[],"total":0,"importable":0,"duplicates":0}',
        200,
        headers: {
          Headers.contentTypeHeader: [Headers.jsonContentType]
        },
      );
    }
    return ResponseBody.fromString(
      '{"code":"PRODUCT_PDF_QUOTA_EXCEEDED","message":"a cota acabou (o provedor pede 57s)"}',
      status,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType]
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

List<({Uint8List bytes, String filename})> _files(int n) => [
      for (var i = 0; i < n; i++)
        (bytes: Uint8List.fromList('%PDF-1.4'.codeUnits), filename: 'f$i.pdf'),
    ];

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  ProductImportActions actionsWith(_ScriptedAdapter adapter) {
    final dio = Dio(BaseOptions(baseUrl: 'https://api.test/v1'));
    dio.httpClientAdapter = adapter;
    final container =
        ProviderContainer(overrides: [dioProvider.overrideWithValue(dio)]);
    addTearDown(container.dispose);
    return container.read(productImportActionsProvider);
  }

  test('an exhausted quota stops the batch instead of grinding the rest',
      (() async {
    // 9 files; the 429 lands in the first window of three.
    final adapter = _ScriptedAdapter([200, 200, 429]);
    final batch = await actionsWith(adapter).previewMany(_files(9));

    // Two read, seven reported as failed — and only the first window was ever
    // sent. Whatever the exact window size, the point is that the remaining
    // files did NOT each cost a request.
    expect(batch.failed.length, 7);
    expect(adapter.calls, lessThan(9),
        reason: 'the doomed files were uploaded anyway');
    // The reason travels to every file it applies to, in the server's words —
    // the wait is the difference between "try again" and "try again in 57s".
    expect(batch.reasons.first, contains('57s'));
    for (final f in batch.failed) {
      expect(f.reason, contains('57s'));
    }
  }));

  test('a timeout on ONE document does not condemn the others', (() async {
    // 504: the model did not finish reading THIS file. The next brochure is
    // usually smaller and deserves its turn.
    final adapter = _ScriptedAdapter([504, 200, 200, 200]);
    final batch = await actionsWith(adapter).previewMany(_files(4));

    expect(batch.failed.length, 1);
    expect(adapter.calls, 4, reason: 'the batch stopped on a per-file failure');
  }));

  test('progress is reported and reaches the total', (() async {
    final seen = <(int, int)>[];
    final batch = await actionsWith(_ScriptedAdapter(const []))
        .previewMany(_files(7), onProgress: (d, t) => seen.add((d, t)));

    expect(batch.failed, isEmpty);
    expect(seen.last, (7, 7));
    // Monotonic: a progress line that goes backwards reads as a bug.
    for (var i = 1; i < seen.length; i++) {
      expect(seen[i].$1, greaterThanOrEqualTo(seen[i - 1].$1));
    }
  }));
}
