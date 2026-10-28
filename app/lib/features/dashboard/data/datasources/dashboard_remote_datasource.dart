import 'package:dio/dio.dart';
import 'package:retrofit/retrofit.dart';
import 'package:app/features/dashboard/data/models/conversation_api_model.dart';

part 'dashboard_remote_datasource.g.dart';

@RestApi()
abstract class DashboardRemoteDataSource {
  factory DashboardRemoteDataSource(Dio dio) = _DashboardRemoteDataSource;

  @GET('/conversations')
  Future<List<ConversationApiModel>> getConversations(
    @Query('status') String? status, {
    @Query('sort') String? sort,
    @Query('assigned') String? assigned,
    @Query('channel') String? channel,

    /// Inbox triage bucket (§6.10 camada 3). Filtered in SQL, before
    /// pagination — a page filtered client-side would answer "nenhum lead
    /// quente" for an inbox that has several, just further down.
    @Query('category') String? category,

    /// Free-text search (contact name / username / sender / last message)
    /// and priority — both SQL filters, before pagination. A page filtered
    /// client-side answered "nada encontrado" for a name on page two.
    @Query('q') String? q,
    @Query('priority') String? priority,

    /// Page size / start. The API caps limit at 200 (default 50).
    @Query('limit') int? limit,
    @Query('offset') int? offset,
  });

  /// [before] pages OLDER messages: an RFC3339 `created_at` cursor, or the
  /// sentinel `latest` for the newest page. Without it the API returns the
  /// OLDEST `limit` rows, which for a long thread hides the newest ones.
  @GET('/conversations/{id}/messages')
  Future<HttpResponse<dynamic>> getMessages(
    @Path('id') String conversationId, {
    @Query('limit') int? limit,
    @Query('before') String? before,
  });

  /// Backs the "Arquivos" tab on the contact detail sheet. Returns
  /// every message in the conversation that carries a media_url
  /// (images, videos, audios, documents), newest first.
  @GET('/conversations/{id}/media')
  Future<HttpResponse<dynamic>> getConversationMedia(
    @Path('id') String conversationId,
  );

  /// Lifetime aggregates for the contact behind a conversation: total
  /// conversations, total messages, first/last interaction.
  @GET('/conversations/{id}/contact-stats')
  Future<HttpResponse<dynamic>> getContactStats(
    @Path('id') String conversationId,
  );

  /// Sales-funnel memory for the contact: the funnel's metadata schema
  /// merged with the lead's collected values, status, score and buy-intent
  /// flag. Returns `in_funnel: false` (or 404) when the conversation is not
  /// bound to any funnel. Backs the "Perfil de vendas" section.
  @GET('/conversations/{id}/sales-profile')
  Future<HttpResponse<dynamic>> getSalesProfile(
    @Path('id') String conversationId,
  );

  /// Manual single-field edit of a lead's collected metadata by an
  /// attendant (US-014). Body: `{field_key, value}`. The API records the
  /// value with `source:'human'`, so the origin badge flips to "Humano".
  @PATCH('/leads/{id}/metadata')
  Future<HttpResponse<dynamic>> updateLeadMetadata(
    @Path('id') String leadId,
    @Body() Map<String, dynamic> body,
  );

  @POST('/conversations/{id}/reply')
  Future<void> replyToConversation(
    @Path('id') String conversationId,
    @Body() Map<String, dynamic> body,
  );

  @PATCH('/conversations/{id}/resume-bot')
  Future<void> resumeBot(@Path('id') String conversationId);

  @POST('/conversations/{id}/assign')
  Future<HttpResponse<dynamic>> assumeConversation(
    @Path('id') String conversationId,
  );

  @POST('/conversations/{id}/release')
  Future<HttpResponse<dynamic>> releaseConversation(
    @Path('id') String conversationId,
  );

  @POST('/conversations/{id}/send-message')
  Future<HttpResponse<dynamic>> sendHumanMessage(
    @Path('id') String conversationId,
    @Body() Map<String, dynamic> body,
  );

  @POST('/media/upload')
  Future<HttpResponse<dynamic>> requestUploadUrl(
    @Body() Map<String, String> body,
  );

  @PATCH('/conversations/{id}/assign')
  Future<void> assignConversation(
    @Path('id') String conversationId,
    @Body() Map<String, dynamic> body,
  );

  @PATCH('/conversations/{id}/priority')
  Future<void> setPriority(
    @Path('id') String conversationId,
    @Body() Map<String, dynamic> body,
  );

  @PATCH('/conversations/{id}/resolve')
  Future<void> resolveConversation(@Path('id') String conversationId);

  /// Resolve's inverse. Idempotent on an open thread; 409
  /// `CONV_ALREADY_OPEN` when another open thread exists for the sender.
  @PATCH('/conversations/{id}/reopen')
  Future<void> reopenConversation(@Path('id') String conversationId);

  /// The navigation badge: open threads holding a customer message nobody
  /// opened (`count`) and the total of such messages (`messages`).
  @GET('/conversations/unread-count')
  Future<HttpResponse<dynamic>> unreadCount();

  @POST('/conversations/{id}/read')
  Future<void> markAsRead(@Path('id') String conversationId);

  /// Batch action over a list of conversation IDs. Action is one of
  /// `resolve`, `assign`, `set_priority`. Backend returns per-row
  /// success/error so partial failures don't block the rest of the
  /// batch.
  @POST('/conversations/bulk')
  Future<HttpResponse<dynamic>> bulkAction(
    @Body() Map<String, dynamic> body,
  );

  /// Returns up to 3 short reply candidates produced by the LLM
  /// based on the last ~12 messages of the conversation. Sync,
  /// ~1s on Gemini Flash. The composer shows them as chips above
  /// the input; tapping fills the field instead of auto-sending so
  /// the operator can edit before shipping.
  @POST('/conversations/{id}/suggest-replies')
  Future<HttpResponse<dynamic>> suggestReplies(
    @Path('id') String conversationId,
  );

  /// The BRIEF: agreed / pending / next step, cached server-side and only
  /// regenerated when the conversation moved. 204 = thread too short.
  @GET('/conversations/{id}/summary')
  Future<HttpResponse<dynamic>> conversationBrief(
    @Path('id') String conversationId,
  );
}
