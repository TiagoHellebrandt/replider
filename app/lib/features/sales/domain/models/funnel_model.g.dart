// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'funnel_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

MetadataFieldModel _$MetadataFieldModelFromJson(Map<String, dynamic> json) =>
    MetadataFieldModel(
      key: json['key'] as String,
      label: json['label'] as String? ?? '',
      type: json['type'] as String? ?? 'text',
      isRequired: json['required'] as bool? ?? false,
      aiHint: json['ai_hint'] as String? ?? '',
      enumOptions: (json['enum_options'] as List<dynamic>?)
          ?.map((e) => e as String)
          .toList(),
    );

Map<String, dynamic> _$MetadataFieldModelToJson(MetadataFieldModel instance) =>
    <String, dynamic>{
      'key': instance.key,
      'label': instance.label,
      'type': instance.type,
      'required': instance.isRequired,
      'ai_hint': instance.aiHint,
      'enum_options': instance.enumOptions,
    };

FunnelModel _$FunnelModelFromJson(Map<String, dynamic> json) => FunnelModel(
      id: json['id'] as String? ?? '',
      accountId: json['account_id'] as String? ?? '',
      name: json['name'] as String? ?? '',
      productId: json['product_id'] as String?,
      agentId: json['agent_id'] as String?,
      channelId: json['channel_id'] as String?,
      personaPrompt: json['persona_prompt'] as String? ?? '',
      metadataSchema: (json['metadata_schema'] as List<dynamic>?)
              ?.map(
                  (e) => MetadataFieldModel.fromJson(e as Map<String, dynamic>))
              .toList() ??
          const [],
      webhookUrl: json['webhook_url'] as String? ?? '',
      webhookSecret: json['webhook_secret'] as String? ?? '',
      triggerOnRequiredComplete:
          json['trigger_on_required_complete'] as bool? ?? false,
      triggerOnBuyIntent: json['trigger_on_buy_intent'] as bool? ?? false,
      active: json['active'] as bool? ?? true,
      isDefault: json['is_default'] as bool? ?? false,
      createdAt: json['created_at'] as String?,
      updatedAt: json['updated_at'] as String?,
    );

Map<String, dynamic> _$FunnelModelToJson(FunnelModel instance) =>
    <String, dynamic>{
      'id': instance.id,
      'account_id': instance.accountId,
      'name': instance.name,
      'product_id': instance.productId,
      'agent_id': instance.agentId,
      'channel_id': instance.channelId,
      'persona_prompt': instance.personaPrompt,
      'metadata_schema': instance.metadataSchema,
      'webhook_url': instance.webhookUrl,
      'webhook_secret': instance.webhookSecret,
      'trigger_on_required_complete': instance.triggerOnRequiredComplete,
      'trigger_on_buy_intent': instance.triggerOnBuyIntent,
      'active': instance.active,
      'is_default': instance.isDefault,
      'created_at': instance.createdAt,
      'updated_at': instance.updatedAt,
    };
