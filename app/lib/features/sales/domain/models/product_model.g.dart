// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'product_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

ProductModel _$ProductModelFromJson(Map<String, dynamic> json) => ProductModel(
      id: json['id'] as String,
      accountId: json['account_id'] as String,
      name: json['name'] as String,
      description: json['description'] as String? ?? '',
      priceCents: (json['price_cents'] as num?)?.toInt() ?? 0,
      currency: json['currency'] as String? ?? 'BRL',
      attributes: json['attributes'] as Map<String, dynamic>?,
      consumptionCycleDays:
          (json['consumption_cycle_days'] as num?)?.toInt() ?? 0,
      shelfLifeDays: (json['shelf_life_days'] as num?)?.toInt() ?? 0,
      active: json['active'] as bool? ?? true,
      sourceId: json['source_id'] as String?,
      sourcePage: (json['source_page'] as num?)?.toInt() ?? 0,
      createdAt: json['created_at'] as String?,
      updatedAt: json['updated_at'] as String?,
    );

Map<String, dynamic> _$ProductModelToJson(ProductModel instance) =>
    <String, dynamic>{
      'id': instance.id,
      'account_id': instance.accountId,
      'name': instance.name,
      'description': instance.description,
      'price_cents': instance.priceCents,
      'currency': instance.currency,
      'attributes': instance.attributes,
      'consumption_cycle_days': instance.consumptionCycleDays,
      'shelf_life_days': instance.shelfLifeDays,
      'active': instance.active,
      'source_id': instance.sourceId,
      'source_page': instance.sourcePage,
      'created_at': instance.createdAt,
      'updated_at': instance.updatedAt,
    };
