import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/features/sales/domain/models/funnel_model.dart';
import 'package:app/features/sales/presentation/controllers/products_controller.dart';
import 'package:app/i18n/strings.g.dart';

part 'funnels_controller.g.dart';

/// Account-scoped list of sales funnels (US-006/US-009), kept alive so the list
/// survives navigation between the list screen and the config screen. Reuses the
/// shared [salesRemoteDataSourceProvider] from [ProductsList].
@Riverpod(keepAlive: true)
class FunnelsList extends _$FunnelsList {
  @override
  Future<List<FunnelModel>> build() => _fetch();

  Future<List<FunnelModel>> _fetch() async {
    final ds = ref.read(salesRemoteDataSourceProvider);
    final res = await ds.listFunnels();
    final raw = res.data as List<dynamic>? ?? const [];
    return raw
        .map((e) => FunnelModel.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<void> refresh() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(_fetch);
  }

  /// Creates a funnel from [funnel]'s editable fields. Returns null on success,
  /// otherwise a localised error.
  Future<String?> create(FunnelModel funnel) async {
    final ds = ref.read(salesRemoteDataSourceProvider);
    try {
      await ds.createFunnel(funnel.toRequestBody());
      await refresh();
      return null;
    } on DioException catch (e) {
      return parseApiError(e);
    } catch (_) {
      return t.errors.UNKNOWN;
    }
  }

  Future<String?> updateFunnel(FunnelModel funnel) async {
    final ds = ref.read(salesRemoteDataSourceProvider);
    try {
      await ds.updateFunnel(funnel.id, funnel.toRequestBody());
      await refresh();
      return null;
    } on DioException catch (e) {
      return parseApiError(e);
    } catch (_) {
      return t.errors.UNKNOWN;
    }
  }

  Future<String?> delete(String id) async {
    final ds = ref.read(salesRemoteDataSourceProvider);
    try {
      await ds.deleteFunnel(id);
      await refresh();
      return null;
    } on DioException catch (e) {
      return parseApiError(e);
    } catch (_) {
      return t.errors.UNKNOWN;
    }
  }

  /// Fires a sample event at the funnel's configured webhook (US-010/US-017).
  /// On success returns the API response's HTTP status + body; on failure the
  /// [error] field carries a localised message.
  Future<FunnelWebhookTestResult> testWebhook(String id) async {
    final ds = ref.read(salesRemoteDataSourceProvider);
    try {
      final res = await ds.testFunnelWebhook(id);
      return FunnelWebhookTestResult(
        status: res.response.statusCode,
        body: res.data == null ? '' : res.data.toString(),
      );
    } on DioException catch (e) {
      return FunnelWebhookTestResult(error: parseApiError(e));
    } catch (_) {
      return FunnelWebhookTestResult(error: t.errors.UNKNOWN);
    }
  }
}

/// Single funnel by id (US-024). Used by the lead approach card to resolve the
/// funnel's channel so it can list that channel's approved WhatsApp templates
/// for the human-in-the-loop send. Cheap GET; autoDispose.
@riverpod
Future<FunnelModel> funnel(Ref ref, String id) async {
  final ds = ref.read(salesRemoteDataSourceProvider);
  final res = await ds.getFunnel(id);
  return FunnelModel.fromJson(res.data as Map<String, dynamic>);
}

/// Outcome of a funnel webhook test fire ([FunnelsList.testWebhook]). Either
/// [error] is set (request failed) or [status]/[body] describe the response.
class FunnelWebhookTestResult {
  final String? error;
  final int? status;
  final String? body;

  const FunnelWebhookTestResult({this.error, this.status, this.body});

  bool get ok => error == null;
}
