import 'dart:async';
import 'dart:typed_data';
import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:app/core/utils/text_search.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/features/dashboard/presentation/controllers/ai_debounce_controller.dart';
import 'package:app/core/network/websocket_service.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/billing/presentation/controllers/billing_controller.dart';
import 'package:app/features/dashboard/data/datasources/dashboard_remote_datasource.dart';
import 'package:app/features/dashboard/presentation/controllers/inbox_assignment_filter_controller.dart';
import 'package:app/features/dashboard/data/models/conversation_api_model.dart';
import 'package:app/features/dashboard/domain/models/conversation_model.dart';
import 'package:app/features/dashboard/domain/models/lead_profile_model.dart';
import 'package:app/features/dashboard/domain/models/email_risk.dart';
import 'package:app/features/dashboard/domain/models/message_attachment.dart';
import 'package:app/features/dashboard/domain/models/message_model.dart';
import 'package:app/features/sales/domain/models/lead_model.dart';
import 'package:app/i18n/strings.g.dart';

part 'dashboard_controller.g.dart';

// ── Datasource Provider ───────────────────────────────────────────────────

@riverpod
DashboardRemoteDataSource dashboardRemoteDataSource(Ref ref) {
  final dio = ref.watch(dioProvider);
  return DashboardRemoteDataSource(dio);
}

// ── API → Domain Mappers ──────────────────────────────────────────────────

ConversationSource _parseSource(String? channelType) {
  final lower = (channelType ?? '').toLowerCase();
  // Email is checked first because it is the SDR product's main channel and
  // the fallback below is whatsapp: an unlisted type does not merely lose its
  // badge, it inherits WhatsApp's 24h-window rules, which do not exist in
  // email. That is what an email thread used to look like here.
  if (lower.contains('email')) return ConversationSource.email;
  if (lower.contains('telegram')) return ConversationSource.telegram;
  if (lower.contains('instagram')) return ConversationSource.instagram;
  if (lower.contains('sms')) return ConversationSource.sms;
  // Internal CS channel (canal-interno) — platform↔customer threads.
  if (lower.contains('internal')) return ConversationSource.internal;
  return ConversationSource.whatsapp;
}

ConversationStatus _parseStatus(String status) {
  return switch (status) {
    'pending_human' => ConversationStatus.pendingHuman,
    'closed' => ConversationStatus.closed,
    _ => ConversationStatus.open,
  };
}

/// Resolves the inbox chip's text. A lead with no stage (created before
/// the board) still shows something the attendant can read.
String _leadStageLabel(String? stageName, String? status) {
  final stage = stageName?.trim() ?? '';
  if (stage.isNotEmpty) return stage;
  final raw = status?.trim() ?? '';
  if (raw.isEmpty) return '';
  return LeadPipelineStatus.fromWire(raw).label;
}

ConversationModel _mapToConversation(ConversationApiModel api) {
  return ConversationModel(
    id: api.id,
    channelId: api.channelId,
    contactExternalId: api.contactId,
    contactName: api.contactName ?? api.contactId,
    contactUsername: api.contactUsername,
    contactAvatarUrl: api.contactAvatarUrl,
    contactBio: api.contactBio,
    lastMessage: api.lastMessage ?? t.dashboard.no_messages,
    lastMessageType: api.lastMessageType ?? '',
    leadId: api.leadId ?? '',
    // Stage name first, canonical status as the fallback — see
    // ConversationModel.leadStageLabel.
    leadStageLabel: _leadStageLabel(api.leadStageName, api.leadStatus),
    inboxCategory: api.inboxCategory ?? '',
    // The LAST MESSAGE's time, not updated_at: changing a tag or priority
    // touches updated_at and made the tile claim "agora" — and it could
    // contradict the list's own ordering, which sorts by the customer's
    // last message. toLocal for the same reason as _messageFromJson.
    lastMessageAt: (DateTime.tryParse(api.lastCustomerMessageAt ?? '') ??
            DateTime.tryParse(api.updatedAt) ??
            DateTime.now())
        .toLocal(),
    source: _parseSource(api.channelType),
    // Counted server-side from messages.read_at. The old derivation from
    // status == pending_human lit the badge for "waiting on a human", which
    // stays true after the attendant has read everything.
    unreadCount: api.unreadCount ?? 0,
    status: _parseStatus(api.status),
    assignedTo: api.assignedTo,
    assignedToName: api.assignedToName,
    assignedAt:
        api.assignedAt == null ? null : DateTime.tryParse(api.assignedAt!),
    isBotPaused: api.isBotPaused,
    priority: api.priority ?? 'normal',
    tags: (api.tags ?? [])
        .map((t) => ConversationTag(id: t.id, name: t.name, color: t.color))
        .toList(),
    sentiment: api.sentiment,
    sentimentScore: api.sentimentScore,
  );
}

SenderType _parseSender(String direction, String sender) {
  if (direction == 'inbound') return SenderType.customer;
  if (sender == 'bot') return SenderType.bot;
  return SenderType.agent;
}

// ── Providers ─────────────────────────────────────────────────────────────

@riverpod

/// Active ORDER BY for the conversation list. Values mirror the API:
///   "" / "last_message" — most recent customer activity first (default).
///   "first_in_conv"     — oldest first-inbound-at first (prevents
///                         forgotten customers bubbling to the bottom
///                         when new traffic arrives).
///   "oldest"            — conversation created_at ASC.
@Riverpod(keepAlive: true)
class ConversationSort extends _$ConversationSort {
  @override
  String build() => '';

  void update(String sort) => state = sort;
}

/// Which channel the inbox is showing. Null means all of them.
///
/// One inbox with a filter, not two tabs: the salesperson thinks about the
/// clinic, not about the pipe the last message came down. Splitting email and
/// WhatsApp into tabs would cut one customer's story in half.
@riverpod
class ConversationChannelFilter extends _$ConversationChannelFilter {
  @override
  String? build() => null;

  void update(String? channel) => state = channel;
}

/// Inbox triage bucket (§6.10 camada 3).
///
/// Null means every category. Sent to the server rather than applied here
/// for the same reason as the channel filter: a page filtered client-side
/// would report "nenhum lead quente" for an inbox that has several, just
/// past the page limit.
@riverpod
class ConversationCategoryFilter extends _$ConversationCategoryFilter {
  @override
  String? build() => null;

  void update(String? category) => state = category;
}

@riverpod
class ConversationStatusFilter extends _$ConversationStatusFilter {
  @override
  String? build() => null;

  void update(String? status) => state = status;
}

/// Tracks which conversations currently have the AI bot "typing" (processing).
@riverpod
class TypingState extends _$TypingState {
  @override
  Map<String, bool> build() => {};

  void setTyping(String conversationId, {required bool isTyping}) {
    state = {...state, conversationId: isTyping};
  }

  bool isTyping(String conversationId) => state[conversationId] ?? false;
}

/// Holds the latest usage alert percentage received via WebSocket.
/// Null means no alert. Consumed by the UI to show a snackbar or banner.
@riverpod
class UsageAlert extends _$UsageAlert {
  @override
  double? build() => null;

  void setAlert(double percentUsed) => state = percentUsed;
  void clear() => state = null;
}

/// Watches WebSocket events and triggers conversation/message refreshes in real-time.
@Riverpod(keepAlive: true)
class RealtimeListener extends _$RealtimeListener {
  StreamSubscription<WSEvent>? _sub;

  @override
  void build() {
    final ws = ref.watch(webSocketServiceProvider);
    _sub?.cancel();
    _sub = ws.events.listen(_onEvent);
    ref.onDispose(() => _sub?.cancel());
  }

  void _onEvent(WSEvent event) {
    switch (event.event) {
      case 'new_message':
        ref.invalidate(conversationsProvider);
        if (event.conversationId != null) {
          ref.invalidate(messagesProvider(event.conversationId!));
        }
        // Clear typing indicator when a message arrives
        if (event.conversationId != null) {
          ref.read(typingStateProvider.notifier).setTyping(
                event.conversationId!,
                isTyping: false,
              );
        }
      case 'typing':
        if (event.conversationId != null) {
          ref.read(typingStateProvider.notifier).setTyping(
                event.conversationId!,
                isTyping: true,
              );
        }
      case 'typing_stopped':
        if (event.conversationId != null) {
          ref.read(typingStateProvider.notifier).setTyping(
                event.conversationId!,
                isTyping: false,
              );
        }
      case 'usage_alert':
        // Update the usage alert state so the UI can show a banner/snackbar
        final percentUsed =
            (event.raw['percent_used'] as num?)?.toDouble() ?? 0;
        ref.read(usageAlertProvider.notifier).setAlert(percentUsed);
        // Refresh billing usage data
        ref.invalidate(accountUsageProvider);
      case 'conversation_assignment_changed':
        // Human takeover state changed (assumed/released). Invalidate
        // the list so every connected attendant sees the badge update
        // and the reply input enables/disables in real time.
        ref.invalidate(conversationsProvider);
      case 'conversation_updated':
        // Assign, resolve and inbound routing all emit this — and it went
        // unhandled while the Conversas tab BADGE reacted to it, so the
        // badge and the list under it contradicted each other on screen.
        ref.invalidate(conversationsProvider);
        if (event.conversationId != null) {
          ref.invalidate(messagesProvider(event.conversationId!));
        }
      case 'message_send_failed':
        // An outbound bubble that was rendered optimistically (bot or
        // human reply) failed at the channel send step. Re-fetch the
        // transcript — the row now carries failed_at/failed_reason, so
        // the bubble flips to the "not delivered" state.
        if (event.conversationId != null) {
          ref.invalidate(messagesProvider(event.conversationId!));
        }
      case 'ai_debounce':
        // K1: the AI is waiting for the customer to finish typing. Without
        // this the attendant sees an inbound message and a bot that appears
        // to have died.
        //
        // No `responds_at` means the wait is over — the entry is REMOVED
        // rather than zeroed, because a banner that lingers after the reply
        // landed teaches people to ignore the banner.
        final conversationId = event.conversationId;
        if (conversationId == null) break;
        final respondsAt =
            DateTime.tryParse(event.raw['responds_at'] as String? ?? '');
        if (respondsAt == null) {
          ref.read(aiDebounceStatesProvider.notifier).clear(conversationId);
        } else {
          ref.read(aiDebounceStatesProvider.notifier).set(
                conversationId,
                AiDebounceState(
                  respondsAt: respondsAt.toLocal(),
                  messageCount:
                      (event.raw['message_count'] as num?)?.toInt() ?? 1,
                ),
              );
        }
    }
  }
}

/// The navigation badge: open threads holding a customer message nobody
/// opened (`GET /conversations/unread-count`). keepAlive because the SHELL
/// wears it; refreshed by the same WebSocket events that refresh the inbox,
/// debounced so a burst costs one request. Also bumped after POST /read so
/// the badge drops the moment the thread is opened.
@Riverpod(keepAlive: true)
class UnreadCount extends _$UnreadCount {
  Timer? _debounce;

  @override
  Future<int> build() async {
    final sub = ref.listen(wsEventsProvider, (_, next) {
      final event = next.asData?.value;
      if (event == null) return;
      if (event.event == 'new_message' ||
          event.event == 'conversation_updated') {
        refresh();
      }
    });
    ref.onDispose(() {
      _debounce?.cancel();
      sub.close();
    });
    return _fetch();
  }

  /// Debounced re-read. Sets the state directly rather than invalidating:
  /// a keepAlive provider nobody is reading at that instant would only
  /// rebuild on the NEXT read, and the badge would lag one navigation.
  void refresh() {
    _debounce?.cancel();
    _debounce = Timer(const Duration(seconds: 1), () async {
      state = AsyncData(await _fetch());
    });
  }

  Future<int> _fetch() async {
    try {
      final resp =
          await ref.read(dashboardRemoteDataSourceProvider).unreadCount();
      final data = resp.data;
      if (data is Map<String, dynamic>) {
        return (data['count'] as num?)?.toInt() ?? 0;
      }
      return 0;
    } catch (_) {
      // The badge is a courtesy: a failed count renders as no badge.
      return 0;
    }
  }
}

/// How many rows the inbox asks for. "Carregar mais" grows it by a page;
/// the list is re-read at the larger size so it stays ONE server-sorted
/// list rather than pages stitched client-side (a new message would
/// otherwise repeat or skip a row across the seam).
@riverpod
class ConversationListLimit extends _$ConversationListLimit {
  static const pageSize = 50;
  static const maxLimit = 200; // the API's hard cap

  @override
  int build() => pageSize;

  void loadMore() => state = (state + pageSize).clamp(pageSize, maxLimit);
}

@riverpod
Future<List<ConversationModel>> conversations(Ref ref) async {
  // Activate real-time listener (keepAlive provider — activates once)
  ref.watch(realtimeListenerProvider);

  final datasource = ref.watch(dashboardRemoteDataSourceProvider);
  final statusFilter = ref.watch(conversationStatusFilterProvider);
  final sort = ref.watch(conversationSortProvider);
  // Channel goes to the server (SQL, before pagination) for the same reason
  // the assignment scope does: filtering the page here would drop every
  // email thread that ranked behind a page of WhatsApp.
  final channel = ref.watch(conversationChannelFilterProvider);
  final category = ref.watch(conversationCategoryFilterProvider);
  // Search goes to the server (ILIKE in SQL, before pagination): filtering
  // the page here answered "nada encontrado" for a name on page two. The
  // accent fold that used to run client-side is gone with it — an ILIKE
  // does not fold, and folding only the page would hide the server's own
  // matches.
  final searchQuery = ref.watch(conversationSearchProvider).trim();
  final limit = ref.watch(conversationListLimitProvider);
  // "Mine / Unassigned / All" scope (US-017/US-020): filtered
  // server-side (in SQL, before pagination) so counters/badges follow
  // suit. 'none' is the backend sentinel for the unassigned bucket.
  final scope = await ref.watch(inboxAssignmentFilterProvider.future);
  final currentUserId = ref.watch(authControllerProvider).asData?.value?.id;
  final assigned = switch (scope) {
    InboxAssignmentScope.mine =>
      (currentUserId?.isNotEmpty ?? false) ? currentUserId : null,
    InboxAssignmentScope.unassigned => 'none',
    InboxAssignmentScope.all => null,
  };

  try {
    final apiConversations = await datasource.getConversations(
      statusFilter,
      sort: sort.isEmpty ? null : sort,
      assigned: assigned,
      channel: channel,
      category: category,
      q: searchQuery.isEmpty ? null : searchQuery,
      limit: limit,
    );
    return apiConversations.map(_mapToConversation).toList();
  } on Exception {
    // Rethrow untouched: wrapping in a plain Exception threw away the
    // DioException's error CODE, so parseApiError translated nothing.
    rethrow;
  }
}

@riverpod
class SelectedConversation extends _$SelectedConversation {
  @override
  String? build() => null;

  void select(String? id) => state = id;
}

/// Which email the reader has open (message id), or null for the inbox
/// list. Lifted out of EmailInboxView so the detail panel can swap its
/// chrome while a mail is being read — the contact header and notes bar
/// hide, since the reader's own header already names the sender.
@riverpod
class EmailReaderOpen extends _$EmailReaderOpen {
  @override
  String? build() => null;

  void open(String messageId) => state = messageId;

  void close() => state = null;
}

/// One-shot composer prefill (US-015 co-pilot templates): a caller sets
/// the text BEFORE selecting a conversation + navigating to /home; the
/// conversation detail panel consumes it into the message input and
/// clears it. Never auto-sends — the seller always reviews first.
@riverpod
class ComposerPrefill extends _$ComposerPrefill {
  @override
  String? build() => null;

  void set(String? text) => state = text;
}

/// One mapper for every message list. It existed twice — and the transcript
/// copy silently lacked `attachments:` and `risk:`, so the paperclip and the
/// anti-scam banner were dead data everywhere but the Arquivos tab.
MessageModel _messageFromJson(
    String conversationId, Map<String, dynamic> json) {
  final direction = json['direction'] as String? ?? 'inbound';
  final sender = json['sender'] as String? ?? '';
  final senderName = json['sender_name'] as String? ?? '';
  return MessageModel(
    id: json['id'] as String? ?? '',
    conversationId: conversationId,
    content: json['content'] as String? ?? '',
    senderType: _parseSender(direction, sender),
    senderLabel: senderName.isNotEmpty ? senderName : sender,
    // toLocal: the API speaks UTC and every render site trusted the value
    // as-is, which put every timestamp three hours in the future for a
    // Brazilian operator and flipped "Hoje" at 21:00.
    createdAt: (DateTime.tryParse(json['created_at'] as String? ?? '') ??
            DateTime.now())
        .toLocal(),
    messageType: parseMessageType(json['type'] as String?),
    mediaUrl: json['media_url'] as String?,
    metadata: json['metadata'] is Map<String, dynamic>
        ? json['metadata'] as Map<String, dynamic>
        : null,
    transcript: json['transcript'] as String?,
    failedAt: DateTime.tryParse(json['failed_at'] as String? ?? '')?.toLocal(),
    failedReason: json['failed_reason'] as String?,
    attachments: (json['attachments'] as List<dynamic>? ?? const <dynamic>[])
        .whereType<Map<String, dynamic>>()
        .map(MessageAttachment.fromJson)
        .toList(),
    // Present only when something was found. Null is the normal case and
    // means "nothing to say", never "unchecked".
    risk: EmailRisk.fromJson(json['risk'] as Map<String, dynamic>?),
  );
}

/// Real messages from the API. Invalidated by RealtimeListener on new_message events.
@riverpod
Future<List<MessageModel>> messages(Ref ref, String conversationId) async {
  final datasource = ref.watch(dashboardRemoteDataSourceProvider);
  // No wrapping: swallowing the DioException into a plain Exception threw
  // away the error CODE, so parseApiError could never translate it.
  // The NEWEST page. Without `before` the API returns the oldest rows, so a
  // thread past the page size opened on its beginning and the latest
  // reply — the one the attendant came for — was not on screen.
  final response = await datasource.getMessages(
    conversationId,
    limit: messagePageSize,
    before: 'latest',
  );
  final rawList = response.data as List<dynamic>? ?? [];
  return rawList
      .whereType<Map<String, dynamic>>()
      .map((json) => _messageFromJson(conversationId, json))
      .toList();
}

/// Transcript page size. The API caps a page at 200.
const messagePageSize = 100;

/// Older pages of a transcript, loaded on demand ("carregar anteriores").
///
/// Separate from [messagesProvider] on purpose: that one is invalidated on
/// every WebSocket event, and folding the history into it would throw the
/// attendant back to the newest page mid-read whenever a message arrived.
/// [MessageHistoryState.messages] is chronological and strictly older than
/// the newest page; the detail panel concatenates the two.
class MessageHistoryState {
  const MessageHistoryState({
    this.messages = const [],
    this.exhausted = false,
    this.loading = false,
  });

  final List<MessageModel> messages;

  /// The server returned a short page: there is nothing older.
  final bool exhausted;
  final bool loading;

  MessageHistoryState copyWith({
    List<MessageModel>? messages,
    bool? exhausted,
    bool? loading,
  }) =>
      MessageHistoryState(
        messages: messages ?? this.messages,
        exhausted: exhausted ?? this.exhausted,
        loading: loading ?? this.loading,
      );
}

@riverpod
class MessageHistory extends _$MessageHistory {
  @override
  MessageHistoryState build(String conversationId) =>
      const MessageHistoryState();

  /// Fetches the page before [oldestShown] (the oldest message currently on
  /// screen — from the history when there is one, else from the newest
  /// page) and prepends it.
  Future<void> loadOlder(DateTime oldestShown) async {
    if (state.loading || state.exhausted) return;
    state = state.copyWith(loading: true);
    try {
      final datasource = ref.read(dashboardRemoteDataSourceProvider);
      final response = await datasource.getMessages(
        conversationId,
        limit: messagePageSize,
        before: oldestShown.toUtc().toIso8601String(),
      );
      final rawList = response.data as List<dynamic>? ?? [];
      final page = rawList
          .whereType<Map<String, dynamic>>()
          .map((json) => _messageFromJson(conversationId, json))
          .toList();
      final known = {for (final m in state.messages) m.id};
      state = MessageHistoryState(
        messages: [
          ...page.where((m) => !known.contains(m.id)),
          ...state.messages,
        ],
        exhausted: page.length < messagePageSize,
      );
    } catch (_) {
      // Leave what was loaded; the button stays and the user can retry.
      state = state.copyWith(loading: false);
    }
  }
}

/// [history] first, then the newest page, without repeating a message
/// that moved from one to the other while both were being read.
List<MessageModel> mergeMessageHistory(
  List<MessageModel> history,
  List<MessageModel> latest,
) {
  if (history.isEmpty) return latest;
  final inLatest = {for (final m in latest) m.id};
  return [
    ...history.where((m) => !inLatest.contains(m.id)),
    ...latest,
  ];
}

/// Media-only message list for the "Arquivos" tab on the contact
/// detail sheet. Backed by GET /conversations/:id/media which already
/// filters out text messages and resolves GCS object paths to signed
/// URLs server-side, so the UI can render the URLs directly.
@riverpod
Future<List<MessageModel>> conversationMedia(
    Ref ref, String conversationId) async {
  final datasource = ref.watch(dashboardRemoteDataSourceProvider);
  final response = await datasource.getConversationMedia(conversationId);
  final rawList = response.data as List<dynamic>? ?? [];
  return rawList
      .whereType<Map<String, dynamic>>()
      .map((json) => _messageFromJson(conversationId, json))
      .toList();
}

/// Lifetime stats for the contact behind a conversation. Surfaces
/// "cliente desde X / Y conversas / Z mensagens" on the detail sheet.
class ContactStatsModel {
  final int totalConversations;
  final int totalMessages;
  final DateTime? firstContactAt;
  final DateTime? lastMessageAt;

  const ContactStatsModel({
    required this.totalConversations,
    required this.totalMessages,
    this.firstContactAt,
    this.lastMessageAt,
  });

  factory ContactStatsModel.fromJson(Map<String, dynamic> json) =>
      ContactStatsModel(
        totalConversations: (json['total_conversations'] as num?)?.toInt() ?? 0,
        totalMessages: (json['total_messages'] as num?)?.toInt() ?? 0,
        firstContactAt: _parseDate(json['first_contact_at']),
        lastMessageAt: _parseDate(json['last_message_at']),
      );

  static DateTime? _parseDate(dynamic raw) {
    if (raw is! String || raw.isEmpty) return null;
    return DateTime.tryParse(raw);
  }
}

@riverpod
Future<ContactStatsModel> contactStats(Ref ref, String conversationId) async {
  final datasource = ref.watch(dashboardRemoteDataSourceProvider);
  final response = await datasource.getContactStats(conversationId);
  final raw = response.data as Map<String, dynamic>? ?? const {};
  return ContactStatsModel.fromJson(raw);
}

/// Sales-funnel memory for the contact behind a conversation. A 404 means
/// the conversation is not bound to any funnel (or no lead yet) — surfaced
/// as the empty state, not an error.
@riverpod
Future<LeadProfileModel> salesProfile(Ref ref, String conversationId) async {
  final datasource = ref.watch(dashboardRemoteDataSourceProvider);
  try {
    final response = await datasource.getSalesProfile(conversationId);
    final raw = response.data as Map<String, dynamic>? ?? const {};
    return LeadProfileModel.fromJson(raw);
  } on DioException catch (e) {
    if (e.response?.statusCode == 404) {
      return const LeadProfileModel.empty();
    }
    rethrow;
  }
}

@riverpod
class ConversationSearch extends _$ConversationSearch {
  @override
  String build() => '';

  void update(String query) => state = query;
  void clear() => state = '';
}

@riverpod
class MessageSearch extends _$MessageSearch {
  @override
  String build() => '';

  void update(String query) => state = query;
  void clear() => state = '';
}

/// Indices (into the messages list) of messages that match the current
/// search query, in chronological order (oldest first = index 0).
/// Drives the search nav bar: count display + prev/next scrolling.
final matchedMessageIndicesProvider =
    Provider.autoDispose.family<List<int>, String>((ref, conversationId) {
  final query = foldForSearch(ref.watch(messageSearchProvider).trim());
  if (query.isEmpty) return const [];
  final messages =
      ref.watch(messagesProvider(conversationId)).asData?.value ?? const [];
  return [
    for (var i = 0; i < messages.length; i++)
      if (foldForSearch(messages[i].content).contains(query)) i,
  ];
});

// ── Actions ───────────────────────────────────────────────────────────────

@riverpod
class ConversationActions extends _$ConversationActions {
  @override
  FutureOr<void> build() {}

  Future<void> reply(String conversationId, String message) async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final datasource = ref.read(dashboardRemoteDataSourceProvider);
      await datasource.replyToConversation(
        conversationId,
        {'message': message},
      );
      ref.invalidate(messagesProvider(conversationId));
      ref.invalidate(conversationsProvider);
    });
  }

  /// Uploads a media file to GCS and sends it as a reply.
  /// [fileBytes] — raw file content; [filename] — original filename;
  /// [contentType] — MIME type (e.g. "image/jpeg"); [messageType] — domain type
  /// ("image", "audio", "voice", "video", "document").
  Future<void> replyWithMedia(
    String conversationId,
    Uint8List fileBytes,
    String filename,
    String contentType,
    String messageType, {
    String caption = '',
  }) async {
    state = const AsyncValue.loading();
    try {
      final datasource = ref.read(dashboardRemoteDataSourceProvider);

      // 1. Request a signed upload URL from the API
      final uploadResp = await datasource.requestUploadUrl({
        'filename': filename,
        'content_type': contentType,
      });
      final uploadData = uploadResp.data as Map<String, dynamic>;
      final signedUrl = uploadData['signed_url'] as String;
      // object_path is stored in the DB; the API resolves it to a signed read URL on-demand.
      final objectPath = uploadData['object_path'] as String;

      // 2. PUT the file directly to GCS (no auth header — signed URL handles auth).
      // Pass raw bytes instead of a Stream: Dio's BrowserHttpClientAdapter
      // (Flutter web) cannot serialize a Stream body, and the PUT silently
      // failed with no user feedback.
      final uploadDio = Dio();
      await uploadDio.put<void>(
        signedUrl,
        data: fileBytes,
        options: Options(
          headers: {
            Headers.contentTypeHeader: contentType,
            Headers.contentLengthHeader: fileBytes.length,
          },
          followRedirects: false,
          validateStatus: (status) => status != null && status < 400,
        ),
      );

      // 3. Send the reply — pass object_path as media_url; API signs it before forwarding
      final body = <String, dynamic>{
        'message': caption,
        'message_type': messageType,
        'media_url': objectPath,
      };
      await datasource.replyToConversation(conversationId, body);

      ref.invalidate(messagesProvider(conversationId));
      ref.invalidate(conversationsProvider);
      state = const AsyncValue.data(null);
    } catch (e, st) {
      // Surface the error to the caller — the previous AsyncValue.guard
      // swallowed it, so the composer's try/catch never fired and the
      // user saw nothing when the upload failed.
      state = AsyncValue.error(e, st);
      rethrow;
    }
  }

  /// Saves an attendant's manual edit of a lead metadata field (US-014).
  /// The API records [value] with `source:'human'`; on success the
  /// conversation's sales profile is invalidated so the displayed value
  /// and origin badge ("Humano") refresh.
  Future<void> updateLeadMetadata(
    String conversationId,
    String leadId,
    String fieldKey,
    String value,
  ) async {
    state = const AsyncValue.loading();
    try {
      final datasource = ref.read(dashboardRemoteDataSourceProvider);
      await datasource.updateLeadMetadata(
        leadId,
        {'field_key': fieldKey, 'value': value},
      );
      ref.invalidate(salesProfileProvider(conversationId));
      state = const AsyncValue.data(null);
    } catch (e, st) {
      state = AsyncValue.error(e, st);
      rethrow;
    }
  }

  Future<void> resumeBot(String conversationId) async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final datasource = ref.read(dashboardRemoteDataSourceProvider);
      await datasource.resumeBot(conversationId);
      ref.invalidate(conversationsProvider);
    });
  }

  // ── Human Takeover ─────────────────────────────────────────────────
  // These three actions map to the dedicated takeover endpoints so the
  // flow is explicit — "assume" pauses the bot; "release" un-pauses it;
  // "sendHumanMessage" only works while under takeover. They live next
  // to the legacy reply/resumeBot methods so the UI can migrate in
  // small steps.

  Future<void> assumeConversation(String conversationId) async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final datasource = ref.read(dashboardRemoteDataSourceProvider);
      await datasource.assumeConversation(conversationId);
      ref.invalidate(conversationsProvider);
    });
  }

  Future<void> releaseConversation(String conversationId) async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final datasource = ref.read(dashboardRemoteDataSourceProvider);
      await datasource.releaseConversation(conversationId);
      ref.invalidate(conversationsProvider);
    });
  }

  Future<void> sendHumanMessage(String conversationId, String content,
      {String? subject}) async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final datasource = ref.read(dashboardRemoteDataSourceProvider);
      await datasource.sendHumanMessage(conversationId, {
        'content': content,
        'type': 'text',
        // Email only: the server stamps it into the message metadata and the
        // adapter uses it as the outbound Subject header.
        if (subject != null && subject.trim().isNotEmpty)
          'subject': subject.trim(),
      });
      ref.invalidate(messagesProvider(conversationId));
      ref.invalidate(conversationsProvider);
    });
  }

  Future<void> assignConversation(String conversationId, String? userId) async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final datasource = ref.read(dashboardRemoteDataSourceProvider);
      // The handler binds `assigned_to` — sending `user_id` arrived as nil,
      // which the server defines as UNASSIGN. Picking an attendant was
      // doing the opposite of the click.
      await datasource.assignConversation(
        conversationId,
        {'assigned_to': userId},
      );
      ref.invalidate(conversationsProvider);
    });
  }

  Future<void> setPriority(String conversationId, String priority) async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final datasource = ref.read(dashboardRemoteDataSourceProvider);
      await datasource.setPriority(
        conversationId,
        {'priority': priority},
      );
      ref.invalidate(conversationsProvider);
    });
  }

  Future<void> resolveConversation(String conversationId) async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final datasource = ref.read(dashboardRemoteDataSourceProvider);
      await datasource.resolveConversation(conversationId);
      ref.invalidate(conversationsProvider);
    });
  }

  /// Resolve's inverse — the thread comes back to the open list.
  Future<void> reopenConversation(String conversationId) async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final datasource = ref.read(dashboardRemoteDataSourceProvider);
      await datasource.reopenConversation(conversationId);
      ref.invalidate(conversationsProvider);
    });
  }

  Future<void> markAsRead(String conversationId) async {
    state = await AsyncValue.guard(() async {
      final datasource = ref.read(dashboardRemoteDataSourceProvider);
      await datasource.markAsRead(conversationId);
      // The badge and the row both read messages.read_at — drop them now
      // rather than on the next WebSocket event.
      ref.read(unreadCountProvider.notifier).refresh();
      ref.invalidate(conversationsProvider);
    });
  }

  /// Fires the batch endpoint. Returns the count of successful rows so
  /// the caller can show "X de Y aplicados" feedback. Invalidates the
  /// list once, regardless of partial failures, so the UI catches up
  /// in one repaint instead of N flickers.
  Future<int> bulkAction({
    required String action,
    required List<String> conversationIds,
    Map<String, String>? params,
  }) async {
    state = const AsyncValue.loading();
    int success = 0;
    state = await AsyncValue.guard(() async {
      final datasource = ref.read(dashboardRemoteDataSourceProvider);
      final res = await datasource.bulkAction({
        'action': action,
        'conversation_ids': conversationIds,
        if (params != null) 'params': params,
      });
      final body = res.data;
      if (body is Map && body['results'] is List) {
        for (final r in body['results'] as List) {
          if (r is Map && r['success'] == true) success++;
        }
      }
      ref.invalidate(conversationsProvider);
    });
    return success;
  }
}

/// The conversation BRIEF: agreed / pending / next step. Fetched from the
/// cached server-side summary (GET — the server only pays a new generation
/// when the thread moved since the last one). Null data = thread too short
/// to be worth summarising (204).
class ConversationBriefData {
  const ConversationBriefData({
    required this.text,
    required this.agreed,
    required this.pending,
    required this.nextStep,
    this.generatedAt,
  });

  final String text;
  final String agreed;
  final String pending;
  final String nextStep;
  final DateTime? generatedAt;

  bool get isStructured =>
      agreed.isNotEmpty || pending.isNotEmpty || nextStep.isNotEmpty;

  factory ConversationBriefData.fromJson(Map<String, dynamic> json) =>
      ConversationBriefData(
        text: json['text'] as String? ?? '',
        agreed: json['agreed'] as String? ?? '',
        pending: json['pending'] as String? ?? '',
        nextStep: json['next_step'] as String? ?? '',
        generatedAt: DateTime.tryParse(json['generated_at'] as String? ?? ''),
      );
}

@riverpod
class ConversationBrief extends _$ConversationBrief {
  @override
  Future<ConversationBriefData?> build(String conversationId) async {
    final ds = ref.read(dashboardRemoteDataSourceProvider);
    final res = await ds.conversationBrief(conversationId);
    final body = res.data;
    if (body is Map<String, dynamic>) {
      return ConversationBriefData.fromJson(body);
    }
    // 204: too short to summarise — an honest nothing, not an error.
    return null;
  }

  /// Re-reads the brief. The server regenerates only when the thread
  /// moved, so this is cheap when nothing changed.
  void refresh() => ref.invalidateSelf();
}

/// LLM-backed reply suggestions for a single conversation. autoDispose
/// so we don't pre-fetch suggestions for conversations the user isn't
/// looking at — wasted ¢. Family keyed by conversation_id keeps the
/// cache clean across navigations.
@riverpod
class ReplySuggestions extends _$ReplySuggestions {
  @override
  AsyncValue<List<String>> build(String conversationId) =>
      const AsyncValue.data([]);

  Future<void> refresh() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final ds = ref.read(dashboardRemoteDataSourceProvider);
      final res = await ds.suggestReplies(conversationId);
      final body = res.data;
      if (body is Map && body['suggestions'] is List) {
        return [
          for (final s in body['suggestions'] as List)
            if (s is String && s.isNotEmpty) s,
        ];
      }
      return <String>[];
    });
  }

  /// Clears the chips after the operator picked one or sent a manual
  /// reply — keeps the surface from going stale.
  void clear() {
    state = const AsyncValue.data([]);
  }
}

/// Multiselect state for the conversation list. Keeps the set of
/// selected ids + a flag for "selection mode" so the list switches
/// rendering (checkbox affordance, action toolbar) only when needed.
@riverpod
class ConversationSelection extends _$ConversationSelection {
  @override
  Set<String> build() => <String>{};

  void toggle(String id) {
    final next = {...state};
    if (next.contains(id)) {
      next.remove(id);
    } else {
      next.add(id);
    }
    state = next;
  }

  void clear() {
    if (state.isNotEmpty) state = <String>{};
  }

  void selectAll(Iterable<String> ids) {
    state = {...ids};
  }
}
