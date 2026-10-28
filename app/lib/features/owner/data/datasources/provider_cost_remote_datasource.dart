import 'package:dio/dio.dart';
import 'package:retrofit/retrofit.dart';
import 'package:app/features/owner/domain/models/provider_cost_models.dart';

part 'provider_cost_remote_datasource.g.dart';

@RestApi()
abstract class ProviderCostRemoteDataSource {
  factory ProviderCostRemoteDataSource(Dio dio) = _ProviderCostRemoteDataSource;

  @GET('/owner/provider-costs')
  Future<OwnerProviderCostsResponse> list();

  @POST('/owner/provider-costs')
  Future<OwnerProviderCost> create(@Body() Map<String, dynamic> body);

  @PUT('/owner/provider-costs/{id}')
  Future<void> update(@Path('id') String id, @Body() Map<String, dynamic> body);

  @DELETE('/owner/provider-costs/{id}')
  Future<void> delete(@Path('id') String id);

  @PUT('/owner/exchange-rate')
  Future<void> setExchangeRate(@Body() Map<String, dynamic> body);
}
