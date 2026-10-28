import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/features/owner/data/datasources/provider_cost_remote_datasource.dart';
import 'package:app/features/owner/domain/models/provider_cost_models.dart';

part 'provider_cost_controller.g.dart';

@riverpod
ProviderCostRemoteDataSource providerCostDataSource(Ref ref) {
  return ProviderCostRemoteDataSource(ref.watch(dioProvider));
}

@riverpod
Future<OwnerProviderCostsResponse> ownerProviderCosts(Ref ref) {
  return ref.watch(providerCostDataSourceProvider).list();
}

@riverpod
class ProviderCostActions extends _$ProviderCostActions {
  @override
  void build() {}

  /// [effectiveFrom] is sent as YYYY-MM-DD — the API stores a DATE.
  Future<OwnerProviderCost> create({
    required String provider,
    required String model,
    required int inputCents,
    required int outputCents,
    required DateTime effectiveFrom,
  }) async {
    final out = await ref.read(providerCostDataSourceProvider).create({
      'provider': provider,
      'model': model,
      'input_cost_per_1m_cents_usd': inputCents,
      'output_cost_per_1m_cents_usd': outputCents,
      'effective_from': effectiveFrom.toIso8601String().substring(0, 10),
    });
    ref.invalidate(ownerProviderCostsProvider);
    return out;
  }

  Future<void> update({
    required String id,
    required int inputCents,
    required int outputCents,
    required DateTime effectiveFrom,
  }) async {
    await ref.read(providerCostDataSourceProvider).update(id, {
      'input_cost_per_1m_cents_usd': inputCents,
      'output_cost_per_1m_cents_usd': outputCents,
      'effective_from': effectiveFrom.toIso8601String().substring(0, 10),
    });
    ref.invalidate(ownerProviderCostsProvider);
  }

  Future<void> delete(String id) async {
    await ref.read(providerCostDataSourceProvider).delete(id);
    ref.invalidate(ownerProviderCostsProvider);
  }

  /// [rateCents] is BRL cents per 1 USD (550 = R$5.50).
  Future<void> setExchangeRate(int rateCents) async {
    await ref
        .read(providerCostDataSourceProvider)
        .setExchangeRate({'usd_brl_rate_cents': rateCents});
    ref.invalidate(ownerProviderCostsProvider);
  }
}
