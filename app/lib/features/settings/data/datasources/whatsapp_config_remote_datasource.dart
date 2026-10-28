import 'package:dio/dio.dart';
import 'package:retrofit/retrofit.dart';

part 'whatsapp_config_remote_datasource.g.dart';

@RestApi()
abstract class WhatsAppConfigRemoteDataSource {
  factory WhatsAppConfigRemoteDataSource(Dio dio) =
      _WhatsAppConfigRemoteDataSource;

  /// Uploads an image / video / document to Meta's Resumable Upload API
  /// and returns `{handle, mime_type}`. The handle goes into a template
  /// HEADER's `example.header_handle` field at create time.
  @MultiPart()
  @POST('/channels/{id}/whatsapp/templates/upload-media')
  Future<HttpResponse<dynamic>> uploadTemplateMedia(
    @Path('id') String id,
    @Part(name: 'file') MultipartFile file,
  );

  @GET('/channels/{id}/templates')
  Future<HttpResponse<dynamic>> listTemplates(@Path('id') String id);

  @POST('/channels/{id}/templates')
  Future<HttpResponse<dynamic>> createTemplate(
    @Path('id') String id,
    @Body() Map<String, dynamic> body,
  );

  @DELETE('/channels/{id}/templates/{name}')
  Future<HttpResponse<dynamic>> deleteTemplate(
    @Path('id') String id,
    @Path('name') String name,
  );
}
