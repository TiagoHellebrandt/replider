// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'effective_plan_limits_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

EffectivePlanLimitsModel _$EffectivePlanLimitsModelFromJson(
        Map<String, dynamic> json) =>
    EffectivePlanLimitsModel(
      planId: json['plan_id'] as String?,
      planName: json['plan_name'] as String,
      isContractPlan: json['is_contract_plan'] as bool,
      maxAgents: (json['max_agents'] as num).toInt(),
      maxChannels: (json['max_channels'] as num).toInt(),
      maxSeats: (json['max_seats'] as num).toInt(),
      maxConversationsPerMonth:
          (json['max_conversations_per_month'] as num).toInt(),
      maxAiTokensPerMonth: (json['max_ai_tokens_per_month'] as num).toInt(),
      maxStorageMb: (json['max_storage_mb'] as num).toInt(),
      maxBroadcastMessagesPerMonth:
          (json['max_broadcast_messages_per_month'] as num).toInt(),
      maxRecipientsPerCampaign:
          (json['max_recipients_per_campaign'] as num).toInt(),
      overageConversationCents:
          (json['overage_conversation_cents'] as num).toInt(),
      overageCapMultiplier: (json['overage_cap_multiplier'] as num).toDouble(),
      maxLeadsPerMonth: (json['max_leads_per_month'] as num?)?.toInt() ?? 0,
      maxEnrichmentsPerMonth:
          (json['max_enrichments_per_month'] as num?)?.toInt() ?? 0,
      maxAiDraftsPerMonth:
          (json['max_ai_drafts_per_month'] as num?)?.toInt() ?? 0,
      maxDocumentExtractionsPerMonth:
          (json['max_document_extractions_per_month'] as num?)?.toInt() ?? 0,
      maxEmailMailboxes: (json['max_email_mailboxes'] as num?)?.toInt() ?? 0,
      maxFunnels: (json['max_funnels'] as num?)?.toInt() ?? 0,
    );

Map<String, dynamic> _$EffectivePlanLimitsModelToJson(
        EffectivePlanLimitsModel instance) =>
    <String, dynamic>{
      'plan_id': instance.planId,
      'plan_name': instance.planName,
      'is_contract_plan': instance.isContractPlan,
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
      'max_leads_per_month': instance.maxLeadsPerMonth,
      'max_enrichments_per_month': instance.maxEnrichmentsPerMonth,
      'max_ai_drafts_per_month': instance.maxAiDraftsPerMonth,
      'max_document_extractions_per_month':
          instance.maxDocumentExtractionsPerMonth,
      'max_email_mailboxes': instance.maxEmailMailboxes,
      'max_funnels': instance.maxFunnels,
    };
