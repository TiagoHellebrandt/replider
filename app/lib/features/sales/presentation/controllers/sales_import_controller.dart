import 'dart:typed_data';

import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/features/sales/data/datasources/sales_remote_datasource.dart';
import 'package:app/features/sales/domain/models/sales_import.dart';

part 'sales_import_controller.g.dart';

@riverpod
SalesImportDataSource salesImportDataSource(Ref ref) =>
    SalesImportDataSource(ref.watch(dioProvider));

@riverpod
SalesImportActions salesImportActions(Ref ref) =>
    SalesImportActions(ref.watch(salesImportDataSourceProvider));

/// Bringing in the sales history the account already has.
///
/// Two calls, and the split is the safeguard: [preview] reads the file and
/// reports what it found, [confirm] records from the rows a person looked at.
/// There is deliberately no upload-and-create — this file becomes the
/// portfolio, the reorder dates and the cross-sell suggestions, and the rows
/// it most needs a person to see are the ones the server refuses to guess at:
/// a customer the base does not have, a date it could not read.
class SalesImportActions {
  const SalesImportActions(this._dataSource);

  final SalesImportDataSource _dataSource;

  /// Uploads the file and returns what the server made of it. Records nothing.
  Future<SalesImportPreview> preview({
    required Uint8List bytes,
    required String filename,
  }) async {
    final json =
        await _dataSource.previewSalesImport(bytes: bytes, filename: filename);
    return SalesImportPreview.fromJson(json);
  }

  /// Records a sale per confirmed row.
  Future<SalesImportResult> confirm(List<SalesImportRow> rows) async {
    final json =
        await _dataSource.importSales(rows.map((r) => r.toJson()).toList());
    return SalesImportResult.fromJson(json);
  }
}
