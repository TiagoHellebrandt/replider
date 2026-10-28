import 'package:dio/dio.dart';
import 'package:retrofit/retrofit.dart';

part 'lgpd_remote_datasource.g.dart';

/// Thin Retrofit client for the admin-only LGPD endpoints.
///
/// Security: all three endpoints are admin-scoped on the server. The
/// Flutter side doesn't attempt to enforce role here — the Dio auth
/// interceptor attaches the JWT and the server returns 403 for non-admins.
@RestApi()
abstract class LgpdRemoteDataSource {
  factory LgpdRemoteDataSource(Dio dio) = _LgpdRemoteDataSource;

  /// POST /lgpd/export {external_id} → full JSON bundle.
  ///
  /// The identifier is a phone number and travels in the BODY. As a query
  /// param it was landing in the nginx access log, the edge provider's log and
  /// the browser history — the LGPD screen itself scattering the personal data
  /// of the person exercising a privacy right.
  @POST('/lgpd/export')
  Future<HttpResponse<dynamic>> exportContact(
    @Body() Map<String, dynamic> body,
  );

  /// POST /lgpd/anonymize {external_id} → 204. Body, for the same reason.
  @POST('/lgpd/anonymize')
  Future<HttpResponse<dynamic>> anonymizeContact(
    @Body() Map<String, dynamic> body,
  );

  /// GET /audit?limit=&offset=&action=&entity_type=&actor_id=&from=&to=
  /// → {total, entries[]}
  @GET('/audit')
  Future<HttpResponse<dynamic>> listAudit(
    @Query('limit') int limit,
    @Query('offset') int offset, {
    @Query('action') String? action,
    @Query('entity_type') String? entityType,
    @Query('actor_id') String? actorId,
    @Query('from') String? from,
    @Query('to') String? to,
  });
}
