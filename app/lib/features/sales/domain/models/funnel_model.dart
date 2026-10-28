import 'package:json_annotation/json_annotation.dart';

part 'funnel_model.g.dart';

/// One field the AI should invisibly collect during a sales conversation
/// (mirrors the Go `domain.MetadataFieldDef`). Persisted as one element of the
/// funnel's `metadata_schema` JSONB array. The API serialises with snake_case
/// tags, so `FieldRename.snake` covers `ai_hint`/`enum_options`; only the
/// `required` Dart keyword needs an explicit `@JsonKey`.
@JsonSerializable(fieldRename: FieldRename.snake)
class MetadataFieldModel {
  final String key;
  final String label;
  final String type;
  @JsonKey(name: 'required')
  final bool isRequired;
  final String aiHint;
  final List<String>? enumOptions;

  const MetadataFieldModel({
    required this.key,
    this.label = '',
    this.type = 'text',
    this.isRequired = false,
    this.aiHint = '',
    this.enumOptions,
  });

  factory MetadataFieldModel.fromJson(Map<String, dynamic> json) =>
      _$MetadataFieldModelFromJson(json);
  Map<String, dynamic> toJson() => _$MetadataFieldModelToJson(this);
}

/// Sales funnel (mirrors the Go `domain.SalesFunnel`, US-002/US-006). Ties an
/// optional product/agent/channel to a dynamic metadata schema and webhook
/// config. The config UI (US-009 metadata schema, US-010 webhook) round-trips
/// the whole object so editing one section never wipes another.
@JsonSerializable(fieldRename: FieldRename.snake)
class FunnelModel {
  final String id;
  final String accountId;
  final String name;
  final String? productId;
  final String? agentId;
  final String? channelId;
  final String personaPrompt;
  final List<MetadataFieldModel> metadataSchema;
  final String webhookUrl;
  final String webhookSecret;
  final bool triggerOnRequiredComplete;
  final bool triggerOnBuyIntent;
  final bool active;

  /// The account's one undeletable "Clientes" funnel. Parsed so the UI can
  /// hide Delete instead of offering an action the server refuses (in
  /// English, since the code was unmapped).
  final bool isDefault;
  final String? createdAt;
  final String? updatedAt;

  const FunnelModel({
    this.id = '',
    this.accountId = '',
    this.name = '',
    this.productId,
    this.agentId,
    this.channelId,
    this.personaPrompt = '',
    this.metadataSchema = const [],
    this.webhookUrl = '',
    this.webhookSecret = '',
    this.triggerOnRequiredComplete = false,
    this.triggerOnBuyIntent = false,
    this.active = true,
    this.isDefault = false,
    this.createdAt,
    this.updatedAt,
  });

  bool get isNew => id.isEmpty;

  FunnelModel copyWith({
    String? name,
    String? productId,
    String? agentId,
    String? channelId,
    String? personaPrompt,
    List<MetadataFieldModel>? metadataSchema,
    String? webhookUrl,
    String? webhookSecret,
    bool? triggerOnRequiredComplete,
    bool? triggerOnBuyIntent,
    bool? active,
  }) {
    return FunnelModel(
      id: id,
      accountId: accountId,
      name: name ?? this.name,
      productId: productId ?? this.productId,
      agentId: agentId ?? this.agentId,
      channelId: channelId ?? this.channelId,
      personaPrompt: personaPrompt ?? this.personaPrompt,
      metadataSchema: metadataSchema ?? this.metadataSchema,
      webhookUrl: webhookUrl ?? this.webhookUrl,
      webhookSecret: webhookSecret ?? this.webhookSecret,
      triggerOnRequiredComplete:
          triggerOnRequiredComplete ?? this.triggerOnRequiredComplete,
      triggerOnBuyIntent: triggerOnBuyIntent ?? this.triggerOnBuyIntent,
      active: active ?? this.active,
      createdAt: createdAt,
      updatedAt: updatedAt,
    );
  }

  /// Builds the body the API's `funnelReq` binds (read-only fields like id /
  /// account_id / timestamps are omitted; the handler ignores extras anyway).
  ///
  /// `channel_id` is only sent when set: the server treats an absent/null
  /// channel as "unchanged" (PATCH semantics), so a rename from this form
  /// never erases the link the API made to the account's email channel.
  Map<String, dynamic> toRequestBody() => {
        'name': name,
        'product_id': productId,
        'agent_id': agentId,
        if (channelId != null) 'channel_id': channelId,
        'persona_prompt': personaPrompt,
        'metadata_schema': metadataSchema.map((f) => f.toJson()).toList(),
        'webhook_url': webhookUrl,
        'webhook_secret': webhookSecret,
        'trigger_on_required_complete': triggerOnRequiredComplete,
        'trigger_on_buy_intent': triggerOnBuyIntent,
        'active': active,
      };

  factory FunnelModel.fromJson(Map<String, dynamic> json) =>
      _$FunnelModelFromJson(json);
  Map<String, dynamic> toJson() => _$FunnelModelToJson(this);
}
