import 'package:dio/dio.dart';
import 'package:app/features/auth/domain/models/auth_dto.dart';
import 'package:retrofit/retrofit.dart';

part 'auth_remote_datasource.g.dart';

@RestApi()
abstract class AuthRemoteDataSource {
  factory AuthRemoteDataSource(Dio dio) = _AuthRemoteDataSource;

  @POST('/auth/login')
  Future<AuthenticateUserResponse> login(@Body() AuthenticateUserDTO request);

  @POST('/auth/2fa/verify-login')
  Future<VerifyMfaLoginResponse> verifyMfaLogin(
      @Body() VerifyMfaLoginDTO request);

  @POST('/accounts')
  Future<HandleCreateAccountResponse> register(
      @Body() CreateAccountDTO request);

  @POST('/auth/forgot-password')
  Future<void> forgotPassword(@Body() ForgotPasswordDTO request);

  // Consumes the reset token from the emailed link + sets the new password.
  @POST('/auth/reset-password')
  Future<void> resetPassword(@Body() ResetPasswordDTO request);

  // Re-issues a verification token + mails a new link. Server always
  // returns 200 (enumeration-safe) so the UI never needs to branch on
  // "email not found". Rate-limited at 3 per minute per IP.
  @POST('/auth/resend-verification')
  Future<void> resendVerification(@Body() ForgotPasswordDTO request);

  // Consumes the verification token from the emailed link. The email
  // links to the Flutter /verify-email screen, which calls this to flip
  // is_email_verified server-side. Throws DioException on an invalid or
  // expired token.
  @GET('/auth/verify-email')
  Future<void> verifyEmail(@Query('token') String token);

  @POST('/auth/google')
  Future<SocialAuthResponse> loginWithGoogle(@Body() SocialAuthDTO request);

  @POST('/auth/meta')
  Future<SocialAuthResponse> loginWithMeta(@Body() SocialAuthDTO request);

  @GET('/me')
  Future<GetMeResponse> getCurrentUser(@Header('Authorization') String token);
}
