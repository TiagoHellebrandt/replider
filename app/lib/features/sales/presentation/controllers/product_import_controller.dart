import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/features/sales/domain/models/product_import.dart';

/// Bringing in the price list she already has.
///
/// Two calls, and the split is the safeguard: [preview] reads the file and
/// reports what it found, [confirm] creates from the rows a person looked at.
/// There is deliberately no upload-and-create — the row this parser most needs a
/// human to see is a price it refuses to guess.
class ProductImportActions {
  const ProductImportActions(this._ref);

  final Ref _ref;

  /// Uploads one spreadsheet and returns what the server made of it. Creates
  /// nothing. Private: screens read files through [previewMany], so there is a
  /// single entry point whether the person picked one file or five.
  Future<ProductImportPreview> _previewCsv({
    required Uint8List bytes,
    required String filename,
  }) async {
    final dio = _ref.read(dioProvider);
    final form = FormData.fromMap(<String, dynamic>{
      'file': MultipartFile.fromBytes(bytes, filename: filename),
    });
    final res = await dio.post<Map<String, dynamic>>(
      '/products/import/preview',
      data: form,
    );
    return ProductImportPreview.fromJson(res.data ?? const <String, dynamic>{});
  }

  /// Sends a manufacturer brochure PDF through the server's AI extraction and
  /// returns the SAME preview shape the CSV path returns. Creates nothing —
  /// confirmation is [confirm], exactly like the spreadsheet.
  Future<ProductImportPreview> _previewPdf({
    required Uint8List bytes,
    required String filename,
  }) async {
    final dio = _ref.read(dioProvider);
    final form = FormData.fromMap(<String, dynamic>{
      'file': MultipartFile.fromBytes(bytes, filename: filename),
    });
    final res = await dio.post<Map<String, dynamic>>(
      '/products/import/pdf',
      data: form,
      // Extraction goes through a multimodal model reading every page —
      // give it more room than a plain upload before Dio gives up.
      options: Options(receiveTimeout: const Duration(minutes: 3)),
    );
    return ProductImportPreview.fromJson(res.data ?? const <String, dynamic>{});
  }

  /// How many files are read at once.
  ///
  /// Was one. Nineteen brochures at ~10s each is three minutes of watching a
  /// spinner, and almost all of that is waiting on the provider rather than on
  /// us — so a few in flight is nearly free wall-clock. Small on purpose: each
  /// PDF is a multimodal extraction billed against the account's document
  /// allowance, and a wide fan-out would burst an AI rate limit that is
  /// per-minute, turning a slow import into a failed one.
  static const _readConcurrency = 3;

  /// Reads SEVERAL files and merges what each one found.
  ///
  /// A file that fails does NOT discard the ones that worked — its name comes
  /// back in [ProductImportBatch.failed] so the person retries just that one
  /// instead of the whole batch.
  Future<ProductImportBatch> previewMany(
    List<({Uint8List bytes, String filename})> files, {
    void Function(int done, int total)? onProgress,
  }) async {
    final ok = <({String filename, ProductImportPreview preview})>[];
    final failed = <ProductImportFailure>[];
    var done = 0;

    // Windows rather than a free-running pool: after each one we can ask
    // whether it is still worth starting the next. A service-level refusal
    // (antivirus down, AI quota spent, session gone) answers the same for
    // every remaining file — marching through sixteen more identical failures
    // wastes minutes and buries the one sentence that matters.
    for (var start = 0; start < files.length; start += _readConcurrency) {
      final window = files.skip(start).take(_readConcurrency).toList();
      final results = await Future.wait(window.map((f) async {
        try {
          final isPdf = f.filename.toLowerCase().endsWith('.pdf');
          final out = isPdf
              ? await _previewPdf(bytes: f.bytes, filename: f.filename)
              : await _previewCsv(bytes: f.bytes, filename: f.filename);
          return (filename: f.filename, preview: out, error: null as Object?);
        } catch (e) {
          return (filename: f.filename, preview: null, error: e);
        }
      }));

      Object? stopper;
      for (final r in results) {
        done++;
        final preview = r.preview;
        if (preview != null) {
          ok.add((filename: r.filename, preview: preview));
          continue;
        }
        final e = r.error!;
        failed.add(ProductImportFailure(
            filename: r.filename, reason: parseApiError(e)));
        stopper ??= _isServiceLevel(e) ? e : null;
      }
      onProgress?.call(done, files.length);

      if (stopper != null) {
        final reason = parseApiError(stopper);
        for (final rest in files.skip(start + window.length)) {
          failed.add(
              ProductImportFailure(filename: rest.filename, reason: reason));
        }
        onProgress?.call(files.length, files.length);
        break;
      }
    }
    return ProductImportBatch.merge(ok, failed);
  }

  /// True when the refusal is about the SERVICE rather than this file:
  /// retrying the next document would only reproduce it.
  static bool _isServiceLevel(Object e) {
    if (e is! DioException) return false;
    final s = e.response?.statusCode;
    if (s == null) return false;
    // 504 is the one 5xx that is NOT about the service: it means the model did
    // not finish reading THIS document in time — a property of this file's
    // size. The next brochure is usually smaller and deserves its turn.
    if (s == 504) return false;
    // 409 too: an account-level refusal (no BYOK key, plan conflict) answers
    // the same for every file, so uploading the other eighteen only spends
    // the person's minutes to reprint one sentence.
    //
    // 429 is the one that cost real minutes: an exhausted AI quota answers the
    // same for every remaining file, and while it was flattened into a generic
    // 400 the batch ground through sixteen more doomed uploads to arrive at
    // "tente de novo".
    return s >= 500 || s == 401 || s == 403 || s == 409 || s == 429;
  }

  /// Creates a product per confirmed row.
  ///
  /// Sends back the ROWS the preview showed rather than the file: she approved
  /// what she saw, and re-uploading would let a file that changed underneath be
  /// imported instead. The server re-checks each one anyway.
  Future<ProductImportResult> confirm(List<ProductImportRow> rows) async {
    final dio = _ref.read(dioProvider);
    final res = await dio.post<Map<String, dynamic>>(
      '/products/import',
      data: <String, dynamic>{
        'rows': rows.map((r) => r.toJson()).toList(),
      },
    );
    return ProductImportResult.fromJson(res.data ?? const <String, dynamic>{});
  }
}

final productImportActionsProvider =
    Provider<ProductImportActions>(ProductImportActions.new);
