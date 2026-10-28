// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'transparent_checkout_response.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

TransparentCheckoutResponse _$TransparentCheckoutResponseFromJson(
        Map<String, dynamic> json) =>
    TransparentCheckoutResponse(
      provider: json['provider'] as String,
      method: json['method'] as String,
      url: json['url'] as String?,
      pix: json['pix'] == null
          ? null
          : PixChargeModel.fromJson(json['pix'] as Map<String, dynamic>),
    );

Map<String, dynamic> _$TransparentCheckoutResponseToJson(
        TransparentCheckoutResponse instance) =>
    <String, dynamic>{
      'provider': instance.provider,
      'method': instance.method,
      'url': instance.url,
      'pix': instance.pix,
    };

PixChargeModel _$PixChargeModelFromJson(Map<String, dynamic> json) =>
    PixChargeModel(
      id: json['id'] as String,
      brCode: json['br_code'] as String,
      brCodeBase64: json['br_code_base64'] as String,
      amount: (json['amount'] as num).toInt(),
      status: json['status'] as String,
      expiresAt: json['expires_at'] as String?,
    );

Map<String, dynamic> _$PixChargeModelToJson(PixChargeModel instance) =>
    <String, dynamic>{
      'id': instance.id,
      'br_code': instance.brCode,
      'br_code_base64': instance.brCodeBase64,
      'amount': instance.amount,
      'status': instance.status,
      'expires_at': instance.expiresAt,
    };

ChargeStatusResponse _$ChargeStatusResponseFromJson(
        Map<String, dynamic> json) =>
    ChargeStatusResponse(
      status: json['status'] as String,
      paid: json['paid'] as bool,
    );

Map<String, dynamic> _$ChargeStatusResponseToJson(
        ChargeStatusResponse instance) =>
    <String, dynamic>{
      'status': instance.status,
      'paid': instance.paid,
    };
