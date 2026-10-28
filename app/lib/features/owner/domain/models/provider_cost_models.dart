import 'package:json_annotation/json_annotation.dart';

part 'provider_cost_models.g.dart';

/// One row from `GET /v1/owner/provider-costs` (US-005, vendedores v3).
/// Costs are integer cents of USD per 1M tokens; a price change is a
/// new row with a later [effectiveFrom] — history is preserved so past
/// profit cycles recompute with the price that was in force then.
@JsonSerializable(createToJson: false)
class OwnerProviderCost {
  final String id;
  final String provider;
  final String model;
  @JsonKey(name: 'input_cost_per_1m_cents_usd')
  final int inputCostPer1mCentsUsd;
  @JsonKey(name: 'output_cost_per_1m_cents_usd')
  final int outputCostPer1mCentsUsd;
  @JsonKey(name: 'effective_from')
  final DateTime effectiveFrom;
  @JsonKey(name: 'created_at')
  final DateTime createdAt;

  OwnerProviderCost({
    required this.id,
    required this.provider,
    required this.model,
    required this.inputCostPer1mCentsUsd,
    required this.outputCostPer1mCentsUsd,
    required this.effectiveFrom,
    required this.createdAt,
  });

  factory OwnerProviderCost.fromJson(Map<String, dynamic> json) =>
      _$OwnerProviderCostFromJson(json);
}

/// `GET /v1/owner/provider-costs` bundles the table with the manual
/// USD->BRL rate (BRL cents per 1 USD) so the screen renders in one GET.
class OwnerProviderCostsResponse {
  final List<OwnerProviderCost> items;
  final int usdBrlRateCents;

  OwnerProviderCostsResponse({
    required this.items,
    required this.usdBrlRateCents,
  });

  factory OwnerProviderCostsResponse.fromJson(Map<String, dynamic> json) {
    final items = (json['items'] as List?)
            ?.map((e) => OwnerProviderCost.fromJson(e as Map<String, dynamic>))
            .toList() ??
        const <OwnerProviderCost>[];
    return OwnerProviderCostsResponse(
      items: items,
      usdBrlRateCents: (json['usd_brl_rate_cents'] as num?)?.toInt() ?? 550,
    );
  }
}
