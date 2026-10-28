/// The messaging platform a conversation originated from.
enum ConversationSource { whatsapp, email, instagram, telegram, sms, internal }

/// Current lifecycle state of a conversation.
///
/// - [open] — Active, bot or human may respond.
/// - [pendingHuman] — Bot paused, waiting for a human agent.
/// - [closed] — Resolved, no further messages expected.
enum ConversationStatus { open, pendingHuman, closed }

/// A label attached to a conversation for organization/filtering.
class ConversationTag {
  final String id;
  final String name;
  final String color;

  const ConversationTag(
      {required this.id, required this.name, required this.color});
}

/// A conversation between a customer and the business, displayed in the
/// dashboard conversation list.
///
/// Enriched with contact info, last message preview, unread count, tags,
/// and assignment status. Fetched via `GET /v1/conversations`.
class ConversationModel {
  final String id;

  /// Channel that owns this conversation. Needed by composer surfaces
  /// that talk to the channel directly (template send, profile fetch,
  /// per-channel diagnostics) without re-querying the backend.
  final String channelId;

  /// Channel-side identifier of the contact: WhatsApp phone in E.164
  /// without the leading "+" (e.g. 5511999999999), Telegram chat_id,
  /// Instagram page-scoped id. Required when calling endpoints that
  /// route by destination (template send, outbound media).
  final String contactExternalId;
  final String contactName;
  final String? contactUsername;
  final String? contactAvatarUrl;
  final String? contactBio;
  final String lastMessage;
  final String lastMessageType;

  /// Lead context (US-M024). Empty when this contact is not a lead: an
  /// attendant answering a stranger should not see a pipeline chip that
  /// means nothing.
  final String leadId;

  /// The customer's own column name, falling back to the canonical
  /// status for leads created before the board existed — a chip reading
  /// the status is still more useful than no chip.
  final String leadStageLabel;

  /// Inbox triage bucket (§6.10 camada 3): `hot_lead`, `customer`,
  /// `supplier`, `financial`, `newsletter`, `quarantine` or `other`.
  ///
  /// Empty when the conversation predates triage or arrived on a channel
  /// that is not a shared mailbox — the tile then renders exactly as before.
  final String inboxCategory;

  final DateTime lastMessageAt;
  final ConversationSource source;
  final int unreadCount;
  final ConversationStatus status;
  final String? assignedTo;
  final String? assignedToName;
  final DateTime? assignedAt;

  /// True when a human attendant has taken over — the bot will not
  /// auto-reply until the conversation is released. Mirrors the API
  /// `is_bot_paused` flag. The reply input is enabled only when true.
  final bool isBotPaused;
  final String priority;
  final List<ConversationTag> tags;

  /// LLM-classified sentiment of the inbound stream — one of
  /// positive / neutral / negative / angry. Null when the analyzer
  /// hasn't run (media-only thread, very short messages, or it's
  /// disabled).
  final String? sentiment;
  final double? sentimentScore;

  const ConversationModel({
    required this.id,
    required this.channelId,
    required this.contactExternalId,
    required this.contactName,
    this.contactUsername,
    this.contactAvatarUrl,
    this.contactBio,
    required this.lastMessage,
    this.lastMessageType = '',
    this.leadId = '',
    this.leadStageLabel = '',
    this.inboxCategory = '',
    required this.lastMessageAt,
    required this.source,
    this.unreadCount = 0,
    this.status = ConversationStatus.open,
    this.assignedTo,
    this.assignedToName,
    this.assignedAt,
    this.isBotPaused = false,
    this.priority = 'normal',
    this.tags = const [],
    this.sentiment,
    this.sentimentScore,
  });

  /// Copy with takeover fields updated. Keeps the rest immutable and
  /// keeps WS event handlers from having to rebuild the full object.
  ConversationModel copyWithAssignment({
    String? assignedTo,
    String? assignedToName,
    DateTime? assignedAt,
    bool? isBotPaused,
    ConversationStatus? status,
  }) {
    return ConversationModel(
      id: id,
      channelId: channelId,
      contactExternalId: contactExternalId,
      contactName: contactName,
      contactUsername: contactUsername,
      contactAvatarUrl: contactAvatarUrl,
      contactBio: contactBio,
      lastMessage: lastMessage,
      lastMessageType: lastMessageType,
      lastMessageAt: lastMessageAt,
      source: source,
      unreadCount: unreadCount,
      status: status ?? this.status,
      assignedTo: assignedTo,
      assignedToName: assignedToName,
      assignedAt: assignedAt,
      isBotPaused: isBotPaused ?? this.isBotPaused,
      priority: priority,
      tags: tags,
      sentiment: sentiment,
      sentimentScore: sentimentScore,
    );
  }
}
