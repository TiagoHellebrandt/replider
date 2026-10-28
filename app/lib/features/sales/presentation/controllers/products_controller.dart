import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/features/sales/data/datasources/sales_remote_datasource.dart';
import 'package:app/features/sales/domain/models/product_model.dart';
import 'package:app/i18n/strings.g.dart';

part 'products_controller.g.dart';

@riverpod
SalesRemoteDataSource salesRemoteDataSource(Ref ref) {
  final dio = ref.watch(dioProvider);
  return SalesRemoteDataSource(dio);
}

/// Account-scoped catalog of products, kept alive so the list survives
/// navigation between the list screen and the create/edit form.
@Riverpod(keepAlive: true)
class ProductsList extends _$ProductsList {
  @override
  Future<List<ProductModel>> build() => _fetch();

  Future<List<ProductModel>> _fetch() async {
    final ds = ref.read(salesRemoteDataSourceProvider);
    final res = await ds.listProducts();
    final raw = res.data as List<dynamic>? ?? const [];
    return raw
        .map((e) => ProductModel.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<void> refresh() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(_fetch);
  }

  /// Creates a product. Returns null on success, otherwise a localised error.
  Future<String?> create({
    required String name,
    String description = '',
    int priceCents = 0,
    String currency = 'BRL',
    bool active = true,
    int consumptionCycleDays = 0,
  }) async {
    final ds = ref.read(salesRemoteDataSourceProvider);
    try {
      final res = await ds.createProduct({
        'name': name,
        'description': description,
        'price_cents': priceCents,
        'currency': currency,
        'active': active,
      });
      // The cycle rides its own route: it is what makes the reorder engine
      // see the product at all, and the product update never carried it.
      final created = res.data;
      final id = created is Map ? created['id'] as String? : null;
      if (consumptionCycleDays > 0 && id != null && id.isNotEmpty) {
        await _setCycle(id, consumptionCycleDays);
      }
      await refresh();
      return null;
    } on DioException catch (e) {
      return parseApiError(e);
    } catch (_) {
      return t.errors.UNKNOWN;
    }
  }

  Future<String?> updateProduct({
    required String id,
    required String name,
    String description = '',
    int priceCents = 0,
    String currency = 'BRL',
    bool active = true,
    int? consumptionCycleDays,
  }) async {
    final ds = ref.read(salesRemoteDataSourceProvider);
    try {
      await ds.updateProduct(id, {
        'name': name,
        'description': description,
        'price_cents': priceCents,
        'currency': currency,
        'active': active,
      });
      // Null means the form did not touch the cycle; zero clears it.
      if (consumptionCycleDays != null) {
        await _setCycle(id, consumptionCycleDays);
      }
      await refresh();
      return null;
    } on DioException catch (e) {
      return parseApiError(e);
    } catch (_) {
      return t.errors.UNKNOWN;
    }
  }

  /// `PUT /products/{id}/cycles` — the repurchase cycle, in days. Shelf life
  /// is preserved from the loaded product so this write never zeroes it.
  Future<void> _setCycle(String id, int consumptionCycleDays) async {
    final current = state.value
        ?.where((ProductModel p) => p.id == id)
        .map((p) => p.shelfLifeDays)
        .firstOrNull;
    await ref.read(dioProvider).put<void>('/products/$id/cycles', data: {
      'consumption_cycle_days': consumptionCycleDays,
      'shelf_life_days': current ?? 0,
    });
  }

  Future<String?> delete(String id) async {
    final ds = ref.read(salesRemoteDataSourceProvider);
    try {
      await ds.deleteProduct(id);
      await refresh();
      return null;
    } on DioException catch (e) {
      return parseApiError(e);
    } catch (_) {
      return t.errors.UNKNOWN;
    }
  }
}
