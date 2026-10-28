/// Models for the internal CS support thread (canal-interno US-005): the
/// logged-in customer user's private chat with the Replider platform
/// (AI first, seller takeover). Wire shape is the API's `domain.Message`
/// JSON — mapped by hand like the dashboard's `MessageModel`, but with
/// support-thread semantics: the customer's OWN messages travel as
/// `inbound` (the platform is the receiving side), platform replies
/// (bot or seller) as `outbound`.
class SupportMessage {
  final String id;
  final String text;

  /// True when the logged-in user wrote it (direction == 'inbound').
  final bool fromMe;
  final String? senderName;
  final DateTime? createdAt;

  const SupportMessage({
    required this.id,
    required this.text,
    required this.fromMe,
    this.senderName,
    this.createdAt,
  });

  factory SupportMessage.fromJson(Map<String, dynamic> json) {
    return SupportMessage(
      id: json['id'] as String? ?? '',
      text: json['content'] as String? ?? '',
      fromMe: (json['direction'] as String?) == 'inbound',
      senderName: json['sender_name'] as String?,
      createdAt: DateTime.tryParse(json['created_at'] as String? ?? ''),
    );
  }
}

/// One page of the user's support thread (GET /me/support/messages).
/// [conversationId] is null before the user's first message.
class SupportThread {
  final String? conversationId;
  final List<SupportMessage> messages;

  const SupportThread({this.conversationId, this.messages = const []});

  factory SupportThread.fromJson(Map<String, dynamic> json) {
    final messages = (json['messages'] as List<dynamic>? ?? const [])
        .whereType<Map<String, dynamic>>()
        .map(SupportMessage.fromJson)
        .toList();
    return SupportThread(
      conversationId: json['conversation_id'] as String?,
      messages: messages,
    );
  }
}

/// POST /me/support/messages response: the persisted message plus the
/// thread id (needed on the very first send, when the GET had none).
class SupportSendResult {
  final String? conversationId;
  final SupportMessage message;

  const SupportSendResult({this.conversationId, required this.message});
}
