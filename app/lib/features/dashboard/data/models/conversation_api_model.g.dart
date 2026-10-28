// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'conversation_api_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

ConversationApiModel _$ConversationApiModelFromJson(
        Map<String, dynamic> json) =>
    ConversationApiModel(
      id: json['id'] as String,
      accountId: json['account_id'] as String,
      agentId: json['agent_id'] as String?,
      contactId: json['contact_id'] as String,
      contactName: json['contact_name'] as String?,
      contactUsername: json['contact_username'] as String?,
      contactAvatarUrl: json['contact_avatar_url'] as String?,
      contactBio: json['contact_bio'] as String?,
      channelId: json['channel_id'] as String,
      channelType: json['channel_type'] as String?,
      status: json['status'] as String,
      isBotPaused: json['is_bot_paused'] as bool,
      waitingSince: json['waiting_since'] as String?,
      lastCustomerMessageAt: json['last_customer_message_at'] as String?,
      lastMessage: json['last_message'] as String?,
      lastMessageType: json['last_message_type'] as String?,
      leadId: json['lead_id'] as String?,
      leadStageName: json['lead_stage_name'] as String?,
      leadStatus: json['lead_status'] as String?,
      inboxCategory: json['inbox_category'] as String?,
      assignedTo: json['assigned_to'] as String?,
      assignedToName: json['assigned_to_name'] as String?,
      assignedAt: json['assigned_at'] as String?,
      priority: json['priority'] as String?,
      tags: (json['tags'] as List<dynamic>?)
          ?.map((e) => ConversationApiTag.fromJson(e as Map<String, dynamic>))
          .toList(),
      sentiment: json['sentiment'] as String?,
      sentimentScore: (json['sentiment_score'] as num?)?.toDouble(),
      unreadCount: (json['unread_count'] as num?)?.toInt(),
      createdAt: json['created_at'] as String,
      updatedAt: json['updated_at'] as String,
    );

Map<String, dynamic> _$ConversationApiModelToJson(
        ConversationApiModel instance) =>
    <String, dynamic>{
      'id': instance.id,
      'account_id': instance.accountId,
      'agent_id': instance.agentId,
      'contact_id': instance.contactId,
      'contact_name': instance.contactName,
      'contact_username': instance.contactUsername,
      'contact_avatar_url': instance.contactAvatarUrl,
      'contact_bio': instance.contactBio,
      'channel_id': instance.channelId,
      'channel_type': instance.channelType,
      'status': instance.status,
      'is_bot_paused': instance.isBotPaused,
      'waiting_since': instance.waitingSince,
      'last_customer_message_at': instance.lastCustomerMessageAt,
      'last_message': instance.lastMessage,
      'last_message_type': instance.lastMessageType,
      'lead_id': instance.leadId,
      'lead_stage_name': instance.leadStageName,
      'lead_status': instance.leadStatus,
      'inbox_category': instance.inboxCategory,
      'assigned_to': instance.assignedTo,
      'assigned_to_name': instance.assignedToName,
      'assigned_at': instance.assignedAt,
      'priority': instance.priority,
      'tags': instance.tags,
      'sentiment': instance.sentiment,
      'sentiment_score': instance.sentimentScore,
      'unread_count': instance.unreadCount,
      'created_at': instance.createdAt,
      'updated_at': instance.updatedAt,
    };

ConversationApiTag _$ConversationApiTagFromJson(Map<String, dynamic> json) =>
    ConversationApiTag(
      id: json['id'] as String,
      name: json['name'] as String,
      color: json['color'] as String,
    );

Map<String, dynamic> _$ConversationApiTagToJson(ConversationApiTag instance) =>
    <String, dynamic>{
      'id': instance.id,
      'name': instance.name,
      'color': instance.color,
    };
