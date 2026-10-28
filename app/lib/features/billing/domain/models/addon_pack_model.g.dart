// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'addon_pack_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

AddonPackModel _$AddonPackModelFromJson(Map<String, dynamic> json) =>
    AddonPackModel(
      id: json['id'] as String,
      name: json['name'] as String,
      tokens: (json['tokens'] as num).toInt(),
      price: (json['price'] as num).toInt(),
      currency: json['currency'] as String? ?? 'brl',
    );

Map<String, dynamic> _$AddonPackModelToJson(AddonPackModel instance) =>
    <String, dynamic>{
      'id': instance.id,
      'name': instance.name,
      'tokens': instance.tokens,
      'price': instance.price,
      'currency': instance.currency,
    };
