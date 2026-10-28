// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'provider_cost_models.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

OwnerProviderCost _$OwnerProviderCostFromJson(Map<String, dynamic> json) =>
    OwnerProviderCost(
      id: json['id'] as String,
      provider: json['provider'] as String,
      model: json['model'] as String,
      inputCostPer1mCentsUsd:
          (json['input_cost_per_1m_cents_usd'] as num).toInt(),
      outputCostPer1mCentsUsd:
          (json['output_cost_per_1m_cents_usd'] as num).toInt(),
      effectiveFrom: DateTime.parse(json['effective_from'] as String),
      createdAt: DateTime.parse(json['created_at'] as String),
    );
