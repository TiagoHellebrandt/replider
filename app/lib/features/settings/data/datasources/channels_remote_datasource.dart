import 'package:dio/dio.dart';
import 'package:retrofit/retrofit.dart';

part 'channels_remote_datasource.g.dart';

@RestApi()
abstract class ChannelsRemoteDataSource {
  factory ChannelsRemoteDataSource(Dio dio) = _ChannelsRemoteDataSource;

  @GET('/channels')
  Future<HttpResponse<dynamic>> getChannels();

  @POST('/channels')
  Future<HttpResponse<dynamic>> createChannel(
    @Body() Map<String, dynamic> body,
  );

  @PUT('/channels/{id}')
  Future<HttpResponse<dynamic>> updateChannel(
    @Path('id') String id,
    @Body() Map<String, dynamic> body,
  );

  @DELETE('/channels/{id}')
  Future<HttpResponse<dynamic>> deleteChannel(
    @Path('id') String id,
  );

  @PATCH('/channels/{id}/toggle')
  Future<HttpResponse<dynamic>> toggleChannel(
    @Path('id') String id,
  );

  @POST('/channels/{id}/test')
  Future<HttpResponse<dynamic>> testChannel(
    @Path('id') String id,
  );

  /// Completes the WhatsApp Embedded Signup handshake. Body carries
  /// the authorization `code` from the FB popup plus `phone_number_id`
  /// and `waba_id` from Meta's session_info event; backend exchanges
  /// the code for a long-lived token and creates the channel in one
  /// call.
  @POST('/channels/whatsapp/signup')
  Future<HttpResponse<dynamic>> completeWhatsAppSignup(
    @Body() Map<String, dynamic> body,
  );

  /// Fetches webhook subscription state so the WhatsApp settings
  /// screen can surface "✅ webhook ativo" vs "❌ inscrição falhou".
  @GET('/channels/{id}/webhook-status')
  Future<HttpResponse<dynamic>> getWebhookStatus(
    @Path('id') String id,
  );

  /// Re-subscribes the channel to its provider's webhook events
  /// (covers the rare case where Meta drops the app subscription or
  /// the initial signup failed to persist it).
  @POST('/channels/{id}/webhook-register')
  Future<HttpResponse<dynamic>> registerWebhook(
    @Path('id') String id,
  );

  /// Registers the WhatsApp phone number with Meta's Cloud API so
  /// platform_type flips from NOT_APPLICABLE to CLOUD_API. Required
  /// when the number was previously owned by another integration
  /// (Catholize, Chatwoot, etc) and Meta is still routing inbound
  /// webhooks there.
  @POST('/channels/{id}/whatsapp/register')
  Future<HttpResponse<dynamic>> registerWhatsAppPhone(
    @Path('id') String id,
    @Body() Map<String, dynamic> body,
  );

  /// Asks Meta to send a Cloud API verification code to the phone via
  /// SMS or voice. Used to reset the 2FA PIN when the customer doesn't
  /// remember the one set by a previous integration.
  @POST('/channels/{id}/whatsapp/request-code')
  Future<HttpResponse<dynamic>> requestWhatsAppCode(
    @Path('id') String id,
    @Body() Map<String, dynamic> body,
  );

  /// Verifies the code Meta just sent. If `new_pin` is included, the
  /// server immediately re-registers the phone with that PIN so the
  /// number flips to platform_type=CLOUD_API in one round-trip.
  @POST('/channels/{id}/whatsapp/verify-code')
  Future<HttpResponse<dynamic>> verifyWhatsAppCode(
    @Path('id') String id,
    @Body() Map<String, dynamic> body,
  );

  /// Deregisters the phone from its current Cloud API owner so /register
  /// can set a fresh PIN on our app. Succeeds only when the current
  /// token is for the app that owns the number — otherwise the caller
  /// has to ask the previous provider to release it.
  @POST('/channels/{id}/whatsapp/deregister')
  Future<HttpResponse<dynamic>> deregisterWhatsAppPhone(
    @Path('id') String id,
  );

  /// Sends a pre-approved WhatsApp template to an arbitrary number.
  /// Used by the "Enviar template de teste" button during Meta review
  /// demos so the reviewer sees a real outbound message without
  /// needing to create a Campaign.
  @POST('/channels/{id}/templates/send')
  Future<HttpResponse<dynamic>> sendTemplate(
    @Path('id') String id,
    @Body() Map<String, dynamic> body,
  );
}
