import 'package:dio/dio.dart';
import 'package:retrofit/retrofit.dart';
import 'package:app/features/settings/domain/models/two_factor_models.dart';

part 'two_factor_remote_datasource.g.dart';

/// Mirrors the /v1/me/2fa surface. All endpoints require an
/// authenticated session; super-admin enrolment uses the same flow.
@RestApi()
abstract class TwoFactorRemoteDataSource {
  factory TwoFactorRemoteDataSource(Dio dio) = _TwoFactorRemoteDataSource;

  @GET('/me/2fa/status')
  Future<TwoFactorStatus> status();

  @POST('/me/2fa/setup')
  Future<TwoFactorSetupResult> setup();

  @POST('/me/2fa/verify')
  Future<TwoFactorVerifyResult> verify(@Body() Map<String, dynamic> body);

  @POST('/me/2fa/disable')
  Future<void> disable(@Body() Map<String, dynamic> body);
}
