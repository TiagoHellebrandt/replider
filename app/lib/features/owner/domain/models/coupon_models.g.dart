// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'coupon_models.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

OwnerCoupon _$OwnerCouponFromJson(Map<String, dynamic> json) => OwnerCoupon(
      id: json['id'] as String,
      code: json['code'] as String,
      stripeCouponId: json['stripe_coupon_id'] as String,
      stripePromoId: json['stripe_promo_id'] as String?,
      percentOff: (json['percent_off'] as num).toInt(),
      maxRedemptions: (json['max_redemptions'] as num?)?.toInt(),
      redeemedCount: (json['redeemed_count'] as num).toInt(),
      expiresAt: json['expires_at'] == null
          ? null
          : DateTime.parse(json['expires_at'] as String),
      isActive: json['is_active'] as bool,
      createdBy: json['created_by'] as String,
      createdAt: DateTime.parse(json['created_at'] as String),
    );

CouponPublicLookup _$CouponPublicLookupFromJson(Map<String, dynamic> json) =>
    CouponPublicLookup(
      code: json['code'] as String,
      percentOff: (json['percent_off'] as num).toInt(),
      valid: json['valid'] as bool,
    );
