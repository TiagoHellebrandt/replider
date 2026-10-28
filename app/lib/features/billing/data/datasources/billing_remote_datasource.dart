import 'package:dio/dio.dart';
import 'package:retrofit/retrofit.dart';
import 'package:app/features/billing/domain/models/addon_pack_model.dart';
import 'package:app/features/billing/domain/models/effective_plan_limits_model.dart';
import 'package:app/features/billing/domain/models/plan_model.dart';
import 'package:app/features/billing/domain/models/usage_model.dart';
import 'package:app/features/billing/domain/models/checkout_response.dart';
import 'package:app/features/billing/domain/models/transparent_checkout_response.dart';

part 'billing_remote_datasource.g.dart';

@RestApi()
abstract class BillingRemoteDataSource {
  factory BillingRemoteDataSource(Dio dio) = _BillingRemoteDataSource;

  @GET('/plans')
  Future<List<PlanModel>> getPlans();

  @GET('/me/plan-limits')
  Future<EffectivePlanLimitsModel> getPlanLimits();

  @GET('/accounts/usage')
  Future<UsageModel> getUsage();

  @POST('/payments/checkout')
  Future<CheckoutResponse> createCheckout(@Body() Map<String, dynamic> body);

  // Country-routed transparent checkout: BRL→AbacatePay (PIX inline /
  // card hosted), non-BRL→Stripe. Returns a discriminated payload.
  @POST('/payments/transparent-checkout')
  Future<TransparentCheckoutResponse> createTransparentCheckout(
      @Body() Map<String, dynamic> body);

  @GET('/payments/transparent-checkout/{id}/status')
  Future<ChargeStatusResponse> getChargeStatus(@Path('id') String id);

  @GET('/payments/addon-packs')
  Future<List<AddonPackModel>> getAddonPacks(
      @Query('currency') String currency);

  @POST('/payments/addon-checkout')
  Future<CheckoutResponse> purchaseAddon(@Body() Map<String, dynamic> body);

  // US-175: canonical self-serve subscription-management endpoint.
  @POST('/billing/portal-session')
  Future<CheckoutResponse> openBillingPortal(@Body() Map<String, dynamic> body);
}
