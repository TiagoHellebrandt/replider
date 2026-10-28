import 'package:dio/dio.dart';
import 'package:retrofit/retrofit.dart';
import 'package:app/features/owner/domain/models/owner_account_models.dart';

part 'owner_remote_datasource.g.dart';

/// Platform-admin API. All endpoints sit behind RequireSuperAdmin —
/// non-allowlisted callers get 403 FORBIDDEN, which the controller
/// uses to hide the menu entry.
@RestApi()
abstract class OwnerRemoteDataSource {
  factory OwnerRemoteDataSource(Dio dio) = _OwnerRemoteDataSource;

  @GET('/owner/accounts')
  Future<OwnerAccountsListResponse> list({
    @Query('search') String search = '',
    @Query('status') String status = '',
    @Query('limit') int limit = 50,
    @Query('offset') int offset = 0,
  });

  @GET('/owner/accounts/{id}')
  Future<OwnerAccountDetail> getOne(@Path('id') String id);

  @POST('/owner/accounts')
  Future<OwnerAccountSummary> createContract(@Body() Map<String, dynamic> body);

  @PATCH('/owner/accounts/{id}/overrides')
  Future<void> setOverrides(
    @Path('id') String id,
    @Body() Map<String, dynamic> body,
  );

  @POST('/owner/accounts/{id}/suspend')
  Future<void> suspend(
    @Path('id') String id,
    @Body() Map<String, dynamic> body,
  );

  @POST('/owner/accounts/{id}/reactivate')
  Future<void> reactivate(@Path('id') String id);

  @PATCH('/owner/accounts/{id}/subscription-status')
  Future<void> setSubscriptionStatus(
    @Path('id') String id,
    @Body() Map<String, dynamic> body,
  );

  /// Apply a contract plan (plan + caps + trial-bypass + active status)
  /// to an ALREADY-registered account. Use createContract for a brand-new
  /// account/email; this one for an existing one.
  ///
  /// Returns void on purpose: the caller only re-fetches the detail after
  /// success, so parsing the response body here would be pure downside —
  /// a hiccup deserializing the account (the server has already applied
  /// the change) would surface as a false "unexpected error" alert.
  @PATCH('/owner/accounts/{id}/apply-contract')
  Future<void> applyContract(
    @Path('id') String id,
    @Body() Map<String, dynamic> body,
  );

  /// Link/reallocate ({seller_email, lead_source}) or unlink
  /// ({seller_email: ''}) the seller of a customer account (US-002).
  /// Returns void for the same reason as [applyContract] — the caller
  /// re-fetches the detail after success.
  @PUT('/owner/accounts/{id}/seller')
  Future<void> setSeller(
    @Path('id') String id,
    @Body() Map<String, dynamic> body,
  );

  @GET('/owner/accounts/top')
  Future<TopAccountsResponse> top({@Query('limit') int limit = 10});

  @GET('/owner/system-health')
  Future<OwnerSystemHealth> systemHealth();

  @POST('/owner/accounts/{id}/grant-credits')
  Future<void> grantCredits(
    @Path('id') String id,
    @Body() Map<String, dynamic> body,
  );

  @POST('/owner/accounts/{id}/extend-trial')
  Future<void> extendTrial(
    @Path('id') String id,
    @Body() Map<String, dynamic> body,
  );

  @POST('/owner/accounts/{id}/impersonate')
  Future<ImpersonateResponse> impersonate(
    @Path('id') String id,
    @Body() Map<String, dynamic> body,
  );

  @GET('/owner/accounts/{id}/conversations')
  Future<OwnerConversationsResponse> conversations(
    @Path('id') String id, {
    @Query('limit') int limit = 50,
    @Query('offset') int offset = 0,
  });

  @GET('/owner/accounts/{id}/usage-trend')
  Future<OwnerUsageTrendResponse> usageTrend(@Path('id') String id);

  @GET('/owner/accounts/{id}/invoices')
  Future<OwnerInvoicesResponse> invoices(@Path('id') String id);
}

class OwnerInvoicesResponse {
  final List<OwnerInvoice> items;

  OwnerInvoicesResponse({required this.items});

  factory OwnerInvoicesResponse.fromJson(Map<String, dynamic> json) {
    final items = (json['items'] as List?)
            ?.map((e) => OwnerInvoice.fromJson(e as Map<String, dynamic>))
            .toList() ??
        const [];
    return OwnerInvoicesResponse(items: items);
  }
}

class OwnerUsageTrendResponse {
  final List<OwnerUsageTrendDay> items;

  OwnerUsageTrendResponse({required this.items});

  factory OwnerUsageTrendResponse.fromJson(Map<String, dynamic> json) {
    final items = (json['items'] as List?)
            ?.map((e) => OwnerUsageTrendDay.fromJson(e as Map<String, dynamic>))
            .toList() ??
        const [];
    return OwnerUsageTrendResponse(items: items);
  }
}

/// Wrapper around the GET conversations envelope ({items: [...]}).
class OwnerConversationsResponse {
  final List<OwnerConversationRow> items;

  OwnerConversationsResponse({required this.items});

  factory OwnerConversationsResponse.fromJson(Map<String, dynamic> json) {
    final items = (json['items'] as List?)
            ?.map(
                (e) => OwnerConversationRow.fromJson(e as Map<String, dynamic>))
            .toList() ??
        const [];
    return OwnerConversationsResponse(items: items);
  }
}

/// Wrapper around the GET /owner/accounts envelope ({items, total}).
class OwnerAccountsListResponse {
  final List<OwnerAccountListing> items;
  final int total;

  OwnerAccountsListResponse({required this.items, required this.total});

  factory OwnerAccountsListResponse.fromJson(Map<String, dynamic> json) {
    final items = (json['items'] as List?)
            ?.map(
                (e) => OwnerAccountListing.fromJson(e as Map<String, dynamic>))
            .toList() ??
        const [];
    return OwnerAccountsListResponse(
      items: items,
      total: (json['total'] as num?)?.toInt() ?? 0,
    );
  }
}
