import 'package:dio/dio.dart';
import 'package:retrofit/retrofit.dart';
import 'package:app/features/settings/domain/models/canned_response_model.dart';

part 'canned_response_remote_datasource.g.dart';

@RestApi()
abstract class CannedResponseRemoteDataSource {
  factory CannedResponseRemoteDataSource(Dio dio) =
      _CannedResponseRemoteDataSource;

  @GET('/canned-responses')
  Future<List<CannedResponseModel>> list();

  @POST('/canned-responses')
  Future<CannedResponseModel> create(@Body() Map<String, dynamic> body);

  @PUT('/canned-responses/{id}')
  Future<CannedResponseModel> update(
    @Path('id') String id,
    @Body() Map<String, dynamic> body,
  );

  @DELETE('/canned-responses/{id}')
  Future<void> delete(@Path('id') String id);
}
