// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'plan_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

PlanModel _$PlanModelFromJson(Map<String, dynamic> json) => PlanModel(
      id: json['id'] as String,
      name: json['name'] as String,
      price: (json['price'] as num).toInt(),
      currency: json['currency'] as String,
      aiProvider: json['ai_provider'] as String,
      aiModel: json['ai_model'] as String,
      inputMultiplier: (json['input_multiplier'] as num).toDouble(),
      outputMultiplier: (json['output_multiplier'] as num).toDouble(),
      maxAgents: (json['max_agents'] as num).toInt(),
      maxChannels: (json['max_channels'] as num).toInt(),
      maxSeats: (json['max_seats'] as num?)?.toInt() ?? 0,
      maxConversationsPerMonth:
          (json['max_conversations_per_month'] as num).toInt(),
      maxAiTokensPerMonth: (json['max_ai_tokens_per_month'] as num).toInt(),
      maxStorageMb: (json['max_storage_mb'] as num).toInt(),
      maxBroadcastMessagesPerMonth:
          (json['max_broadcast_messages_per_month'] as num?)?.toInt() ?? 0,
      maxRecipientsPerCampaign:
          (json['max_recipients_per_campaign'] as num?)?.toInt() ?? 0,
      overageConversationCents:
          (json['overage_conversation_cents'] as num?)?.toInt() ?? 0,
      overageCapMultiplier:
          (json['overage_cap_multiplier'] as num?)?.toDouble() ?? 0.0,
      stripePriceId: json['stripe_price_id'] as String?,
      stripePriceIdAnnual: json['stripe_price_id_annual'] as String?,
      createdAt: json['created_at'] as String,
      updatedAt: json['updated_at'] as String,
    );

Map<String, dynamic> _$PlanModelToJson(PlanModel instance) => <String, dynamic>{
      'id': instance.id,
      'name': instance.name,
      'price': instance.price,
      'currency': instance.currency,
      'ai_provider': instance.aiProvider,
      'ai_model': instance.aiModel,
      'input_multiplier': instance.inputMultiplier,
      'output_multiplier': instance.outputMultiplier,
      'max_agents': instance.maxAgents,
      'max_channels': instance.maxChannels,
      'max_seats': instance.maxSeats,
      'max_conversations_per_month': instance.maxConversationsPerMonth,
      'max_ai_tokens_per_month': instance.maxAiTokensPerMonth,
      'max_storage_mb': instance.maxStorageMb,
      'max_broadcast_messages_per_month': instance.maxBroadcastMessagesPerMonth,
      'max_recipients_per_campaign': instance.maxRecipientsPerCampaign,
      'overage_conversation_cents': instance.overageConversationCents,
      'overage_cap_multiplier': instance.overageCapMultiplier,
      'stripe_price_id': instance.stripePriceId,
      'stripe_price_id_annual': instance.stripePriceIdAnnual,
      'created_at': instance.createdAt,
      'updated_at': instance.updatedAt,
    };

ContactSalesChannels _$ContactSalesChannelsFromJson(
        Map<String, dynamic> json) =>
    ContactSalesChannels(
      email: json['email'] as String,
      whatsapp: json['whatsapp'] as String,
      whatsappLink: json['whatsapp_link'] as String,
      whatsappText: json['whatsapp_text'] as String,
      bookingUrl: json['booking_url'] as String? ?? '',
    );

Map<String, dynamic> _$ContactSalesChannelsToJson(
        ContactSalesChannels instance) =>
    <String, dynamic>{
      'email': instance.email,
      'whatsapp': instance.whatsapp,
      'whatsapp_link': instance.whatsappLink,
      'whatsapp_text': instance.whatsappText,
      'booking_url': instance.bookingUrl,
    };
