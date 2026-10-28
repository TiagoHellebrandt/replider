import 'package:dio/dio.dart';
import 'package:retrofit/retrofit.dart';
import 'package:app/features/settings/domain/models/tag_model.dart';

part 'tag_remote_datasource.g.dart';

@RestApi()
abstract class TagRemoteDataSource {
  factory TagRemoteDataSource(Dio dio) = _TagRemoteDataSource;

  @GET('/tags')
  Future<List<TagModel>> list();

  @POST('/tags')
  Future<TagModel> create(@Body() Map<String, dynamic> body);

  @PUT('/tags/{id}')
  Future<TagModel> update(
    @Path('id') String id,
    @Body() Map<String, dynamic> body,
  );

  @DELETE('/tags/{id}')
  Future<void> delete(@Path('id') String id);

  @GET('/conversations/{id}/tags')
  Future<List<TagModel>> listByConversation(@Path('id') String conversationId);

  @POST('/conversations/{id}/tags')
  Future<void> addToConversation(
    @Path('id') String conversationId,
    @Body() Map<String, dynamic> body,
  );

  @DELETE('/conversations/{id}/tags/{tagId}')
  Future<void> removeFromConversation(
    @Path('id') String conversationId,
    @Path('tagId') String tagId,
  );
}
