import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/features/owner/data/datasources/coupon_remote_datasource.dart';
import 'package:app/features/owner/domain/models/coupon_models.dart';

part 'coupon_controller.g.dart';

@riverpod
CouponRemoteDataSource couponDataSource(Ref ref) {
  return CouponRemoteDataSource(ref.watch(dioProvider));
}

@riverpod
Future<OwnerCouponsResponse> ownerCoupons(Ref ref) {
  return ref.watch(couponDataSourceProvider).list();
}

/// Public lookup keyed by code. Used by the register / billing page
/// to render the "X% off applied" banner before the user is signed in.
@riverpod
Future<CouponPublicLookup?> couponPublicLookup(Ref ref, String? code) async {
  if (code == null || code.trim().isEmpty) return null;
  try {
    return await ref
        .watch(couponDataSourceProvider)
        .publicLookup(code.trim().toUpperCase());
  } on DioException {
    return null;
  }
}

@riverpod
class CouponActions extends _$CouponActions {
  @override
  void build() {}

  Future<OwnerCoupon> create({
    required String code,
    required int percentOff,
    int? maxRedemptions,
    DateTime? expiresAt,
  }) async {
    final body = <String, dynamic>{
      'code': code,
      'percent_off': percentOff,
    };
    if (maxRedemptions != null) body['max_redemptions'] = maxRedemptions;
    if (expiresAt != null) body['expires_at'] = expiresAt.toIso8601String();
    final out = await ref.read(couponDataSourceProvider).create(body);
    ref.invalidate(ownerCouponsProvider);
    return out;
  }
}
