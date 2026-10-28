import 'package:app/features/dashboard/domain/models/email_risk.dart';
import 'package:app/features/dashboard/domain/models/message_attachment.dart';

enum SenderType { customer, agent, bot }

/// Message types that carry media or special payloads.
enum MessageType {
  text,
  image,
  audio,
  voice,
  video,
  document,
  sticker,
  location
}

MessageType parseMessageType(String? raw) => switch (raw) {
      'image' => MessageType.image,
      'audio' => MessageType.audio,
      'voice' => MessageType.voice,
      'video' => MessageType.video,
      'document' => MessageType.document,
      'sticker' => MessageType.sticker,
      'location' => MessageType.location,
      _ => MessageType.text,
    };

/// A single message within a conversation.
/// Can be inbound (customer), outbound (bot/agent), or system.
class MessageModel {
  final String id;
  final String conversationId;
  final String content;
  final SenderType senderType;
  final String senderLabel; // display name: "bot", agent name, or customer id
  final DateTime createdAt;
  final MessageType messageType;
  final String? mediaUrl; // CDN URL for media, or "geo:lat,lon" for location
  // metadata is the verbatim JSON the API attached to the row.
  final Map<String, dynamic>? metadata;
  // transcript is the cached server-side transcription of an audio/voice
  // message (the `transcript` column). Nil when the API hasn't cached
  // one — for inbound audio the inline transcription usually lands in
  // `content` instead; see [audioTranscript].
  final String? transcript;
  // failedAt/failedReason: the outbound send failed at the channel
  // adapter (or a later delivery-status webhook). The bubble renders a
  // "not delivered" indicator so the attendant knows the customer never
  // got this message.
  final DateTime? failedAt;
  final String? failedReason;

  /// Files that arrived with this message. Empty for every channel but
  /// email today. Fetched separately from the transcript because the scan
  /// verdict changes after the message is written — a rescan settles a file
  /// that was 'failed' when the bubble was first rendered.
  final List<MessageAttachment> attachments;

  /// What the impersonation checks found, or null when they found nothing.
  /// Null is the normal case and means "nothing to say" — never "unchecked".
  final EmailRisk? risk;

  const MessageModel({
    required this.id,
    required this.conversationId,
    required this.content,
    required this.senderType,
    this.senderLabel = '',
    required this.createdAt,
    this.messageType = MessageType.text,
    this.mediaUrl,
    this.metadata,
    this.transcript,
    this.failedAt,
    this.failedReason,
    this.attachments = const [],
    this.risk,
  });

  /// True when this outbound message never reached the customer.
  bool get sendFailed => failedAt != null;

  /// The transcript to render under an audio/voice bubble, or null when
  /// there isn't one yet. Inbound audio is transcribed at webhook time
  /// and the text lands in `content` (the AI pipeline needs it), so the
  /// content is either the real transcript or a "[...]" placeholder
  /// such as "[audio not transcribed]". The cached `transcript` column
  /// (set by the on-demand transcribe endpoint) takes precedence.
  String? get audioTranscript {
    if (transcript != null && transcript!.trim().isNotEmpty) {
      return transcript!.trim();
    }
    if (messageType != MessageType.audio && messageType != MessageType.voice) {
      return null;
    }
    final c = content.trim();
    if (c.isEmpty) return null;
    if (c.startsWith('[') && c.endsWith(']')) return null; // placeholder
    return c;
  }
}
