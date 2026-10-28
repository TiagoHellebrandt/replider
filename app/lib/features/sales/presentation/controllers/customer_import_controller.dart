import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/features/sales/domain/models/customer_import.dart';

/// Bringing in the customer list she already has.
///
/// Two calls, and the split is the safeguard: [preview] reads the file and
/// reports what it found, [confirm] creates from the rows a person looked at.
/// There is deliberately no upload-and-create — this file decides who the
/// account will be writing to.
class CustomerImportActions {
  const CustomerImportActions(this._ref);

  final Ref _ref;

  /// Uploads the file and returns what the server made of it. Creates nothing.
  Future<CustomerImportPreview> preview({
    required Uint8List bytes,
    required String filename,
  }) async {
    final dio = _ref.read(dioProvider);
    final form = FormData.fromMap(<String, dynamic>{
      'file': MultipartFile.fromBytes(bytes, filename: filename),
    });
    final res = await dio.post<Map<String, dynamic>>(
      '/contacts/import/preview',
      data: form,
    );
    return CustomerImportPreview.fromJson(
        res.data ?? const <String, dynamic>{});
  }

  /// Creates a contact per confirmed row.
  ///
  /// Sends back the ROWS the preview showed rather than the file: she approved
  /// what she saw, and re-uploading would let a file that changed underneath be
  /// imported instead. The server re-derives each one anyway.
  Future<CustomerImportResult> confirm(List<CustomerImportRow> rows) async {
    final dio = _ref.read(dioProvider);
    final res = await dio.post<Map<String, dynamic>>(
      '/contacts/import',
      data: <String, dynamic>{
        'rows': rows.map((r) => r.toJson()).toList(),
      },
    );
    return CustomerImportResult.fromJson(res.data ?? const <String, dynamic>{});
  }
}

final customerImportActionsProvider =
    Provider<CustomerImportActions>(CustomerImportActions.new);
