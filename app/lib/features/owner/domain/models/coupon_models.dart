import 'package:json_annotation/json_annotation.dart';

part 'coupon_models.g.dart';

/// One row from `GET /v1/owner/coupons`. Owner-panel only — the
/// public lookup at `/v1/coupons/:code` returns the slimmer
/// [CouponPublicLookup] shape.
@JsonSerializable(fieldRename: FieldRename.snake, createToJson: false)
class OwnerCoupon {
  final String id;
  final String code;
  final String stripeCouponId;
  final String? stripePromoId;
  final int percentOff;
  final int? maxRedemptions;
  final int redeemedCount;
  final DateTime? expiresAt;
  final bool isActive;
  final String createdBy;
  final DateTime createdAt;

  OwnerCoupon({
    required this.id,
    required this.code,
    required this.stripeCouponId,
    this.stripePromoId,
    required this.percentOff,
    this.maxRedemptions,
    required this.redeemedCount,
    this.expiresAt,
    required this.isActive,
    required this.createdBy,
    required this.createdAt,
  });

  factory OwnerCoupon.fromJson(Map<String, dynamic> json) =>
      _$OwnerCouponFromJson(json);
}

/// Slim public-lookup shape from `GET /v1/coupons/:code`. The
/// register / billing page reads this to render the discount preview
/// banner.
@JsonSerializable(fieldRename: FieldRename.snake, createToJson: false)
class CouponPublicLookup {
  final String code;
  final int percentOff;
  final bool valid;

  CouponPublicLookup({
    required this.code,
    required this.percentOff,
    required this.valid,
  });

  factory CouponPublicLookup.fromJson(Map<String, dynamic> json) =>
      _$CouponPublicLookupFromJson(json);
}
