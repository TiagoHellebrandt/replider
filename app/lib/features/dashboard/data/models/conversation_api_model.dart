import 'package:json_annotation/json_annotation.dart';

part 'conversation_api_model.g.dart';

@JsonSerializable(fieldRename: FieldRename.snake)
class ConversationApiModel {
  final String id;
  final String accountId;
  final String? agentId;
  final String contactId;
  final String? contactName;
  final String? contactUsername;
  final String? contactAvatarUrl;
  final String? contactBio;
  final String channelId;
  final String? channelType;
  final String status;
  final bool isBotPaused;
  final String? waitingSince;
  final String? lastCustomerMessageAt;
  final String? lastMessage;
  final String? lastMessageType;

  /// Lead context for the inbox chip (US-M024). All three are absent when
  /// the contact is not a lead — a pure support conversation renders
  /// exactly as before.
  final String? leadId;
  final String? leadStageName;
  final String? leadStatus;

  /// Inbox triage bucket (§6.10 camada 3). Absent for a conversation
  /// classified before the feature existed, and for every channel that is
  /// not a shared mailbox.
  final String? inboxCategory;
  final String? assignedTo;
  final String? assignedToName;
  final String? assignedAt;
  final String? priority;
  final List<ConversationApiTag>? tags;
  final String? sentiment;
  final double? sentimentScore;

  /// Customer messages nobody opened yet (`messages.read_at IS NULL`).
  /// Counted server-side; POST /conversations/:id/read clears it.
  final int? unreadCount;
  final String createdAt;
  final String updatedAt;

  const ConversationApiModel({
    required this.id,
    required this.accountId,
    this.agentId,
    required this.contactId,
    this.contactName,
    this.contactUsername,
    this.contactAvatarUrl,
    this.contactBio,
    required this.channelId,
    this.channelType,
    required this.status,
    required this.isBotPaused,
    this.waitingSince,
    this.lastCustomerMessageAt,
    this.lastMessage,
    this.lastMessageType,
    this.leadId,
    this.leadStageName,
    this.leadStatus,
    this.inboxCategory,
    this.assignedTo,
    this.assignedToName,
    this.assignedAt,
    this.priority,
    this.tags,
    this.sentiment,
    this.sentimentScore,
    this.unreadCount,
    required this.createdAt,
    required this.updatedAt,
  });

  factory ConversationApiModel.fromJson(Map<String, dynamic> json) =>
      _$ConversationApiModelFromJson(json);
  Map<String, dynamic> toJson() => _$ConversationApiModelToJson(this);
}

@JsonSerializable(fieldRename: FieldRename.snake)
class ConversationApiTag {
  final String id;
  final String name;
  final String color;

  const ConversationApiTag({
    required this.id,
    required this.name,
    required this.color,
  });

  factory ConversationApiTag.fromJson(Map<String, dynamic> json) =>
      _$ConversationApiTagFromJson(json);
  Map<String, dynamic> toJson() => _$ConversationApiTagToJson(this);
}
