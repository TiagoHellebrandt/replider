import 'package:dio/dio.dart';
import 'package:retrofit/retrofit.dart';
import 'package:app/features/owner/domain/models/coupon_models.dart';

part 'coupon_remote_datasource.g.dart';

@RestApi()
abstract class CouponRemoteDataSource {
  factory CouponRemoteDataSource(Dio dio) = _CouponRemoteDataSource;

  @GET('/owner/coupons')
  Future<OwnerCouponsResponse> list();

  @POST('/owner/coupons')
  Future<OwnerCoupon> create(@Body() Map<String, dynamic> body);

  @POST('/owner/coupons/{id}/deactivate')
  Future<void> deactivate(@Path('id') String id);

  /// Public lookup — no auth required. Returns `valid: false` when
  /// the code is unknown / inactive / expired / fully redeemed.
  @GET('/coupons/{code}')
  Future<CouponPublicLookup> publicLookup(@Path('code') String code);
}

class OwnerCouponsResponse {
  final List<OwnerCoupon> items;

  OwnerCouponsResponse({required this.items});

  factory OwnerCouponsResponse.fromJson(Map<String, dynamic> json) {
    final items = (json['items'] as List?)
            ?.map((e) => OwnerCoupon.fromJson(e as Map<String, dynamic>))
            .toList() ??
        const [];
    return OwnerCouponsResponse(items: items);
  }
}
