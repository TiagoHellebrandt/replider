// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'usage_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

UsageModel _$UsageModelFromJson(Map<String, dynamic> json) => UsageModel(
      aiTokensUsed: (json['ai_tokens_used'] as num).toInt(),
      aiInputTokensUsed: (json['ai_input_tokens_used'] as num?)?.toInt() ?? 0,
      aiOutputTokensUsed: (json['ai_output_tokens_used'] as num?)?.toInt() ?? 0,
      addonTokens: (json['addon_tokens'] as num?)?.toInt() ?? 0,
      effectiveLimit: (json['effective_limit'] as num?)?.toInt() ?? 0,
      conversationsCount: (json['conversations_count'] as num).toInt(),
      maxAiTokensPerMonth: (json['max_ai_tokens_per_month'] as num).toInt(),
      maxConversationsPerMonth:
          (json['max_conversations_per_month'] as num).toInt(),
      tokensUsagePercent:
          (json['tokens_usage_percent'] as num?)?.toDouble() ?? 0,
      conversationsUsagePercent:
          (json['conversations_usage_percent'] as num?)?.toDouble() ?? 0,
      planName: json['plan_name'] as String? ?? 'Trial',
      isTrialing: json['is_trialing'] as bool? ?? false,
      overageConversationsCount:
          (json['overage_conversations_count'] as num?)?.toInt() ?? 0,
      overageCentsCharged:
          (json['overage_cents_charged'] as num?)?.toInt() ?? 0,
      overageConversationCents:
          (json['overage_conversation_cents'] as num?)?.toInt() ?? 0,
      capReached: json['cap_reached'] as bool? ?? false,
      sdr: json['sdr'] == null
          ? null
          : SdrUsageModel.fromJson(json['sdr'] as Map<String, dynamic>),
    );

Map<String, dynamic> _$UsageModelToJson(UsageModel instance) =>
    <String, dynamic>{
      'ai_tokens_used': instance.aiTokensUsed,
      'ai_input_tokens_used': instance.aiInputTokensUsed,
      'ai_output_tokens_used': instance.aiOutputTokensUsed,
      'addon_tokens': instance.addonTokens,
      'effective_limit': instance.effectiveLimit,
      'conversations_count': instance.conversationsCount,
      'max_ai_tokens_per_month': instance.maxAiTokensPerMonth,
      'max_conversations_per_month': instance.maxConversationsPerMonth,
      'tokens_usage_percent': instance.tokensUsagePercent,
      'conversations_usage_percent': instance.conversationsUsagePercent,
      'plan_name': instance.planName,
      'is_trialing': instance.isTrialing,
      'overage_conversations_count': instance.overageConversationsCount,
      'overage_cents_charged': instance.overageCentsCharged,
      'overage_conversation_cents': instance.overageConversationCents,
      'cap_reached': instance.capReached,
      'sdr': instance.sdr,
    };

SdrUsageModel _$SdrUsageModelFromJson(Map<String, dynamic> json) =>
    SdrUsageModel(
      leads: (json['leads'] as num?)?.toInt() ?? 0,
      enrichments: (json['enrichments'] as num?)?.toInt() ?? 0,
      approachesSent: (json['approaches_sent'] as num?)?.toInt() ?? 0,
      meetings: (json['meetings'] as num?)?.toInt() ?? 0,
    );

Map<String, dynamic> _$SdrUsageModelToJson(SdrUsageModel instance) =>
    <String, dynamic>{
      'leads': instance.leads,
      'enrichments': instance.enrichments,
      'approaches_sent': instance.approachesSent,
      'meetings': instance.meetings,
    };
