import 'package:json_annotation/json_annotation.dart';

part 'effective_plan_limits_model.g.dart';

/// Per-account effective limits resolved through the API's
/// override → plan → unlimited cascade. Distinct from [PlanModel] which
/// carries the public catalogue values: this surfaces the Partner /
/// Enterprise contract overrides that live on the `accounts` row.
///
/// JSON contract: a value of -1 signals "unlimited". The API folds
/// `domain.UnlimitedLimit` (1<<31) onto -1 so clients don't have to
/// special-case huge numbers. Use [isUnlimited] / [unlimitedTokens] to
/// check before formatting for display.
@JsonSerializable(fieldRename: FieldRename.snake)
class EffectivePlanLimitsModel {
  final String? planId;
  final String planName;
  final bool isContractPlan;
  final int maxAgents;
  final int maxChannels;
  final int maxSeats;
  final int maxConversationsPerMonth;
  final int maxAiTokensPerMonth;
  final int maxStorageMb;
  final int maxBroadcastMessagesPerMonth;
  final int maxRecipientsPerCampaign;
  final int overageConversationCents;
  final double overageCapMultiplier;

  /// What the customer actually buys today (US-M013). These caps were already
  /// ENFORCED by the server — the research one blocks enrichment mid-task — and
  /// never reached the screen: whoever hit one found out by clicking.
  ///
  /// `0` means "the plan does not sell this", and the row disappears. It is not
  /// the same as unlimited, which is `-1`.
  @JsonKey(defaultValue: 0)
  final int maxLeadsPerMonth;
  @JsonKey(defaultValue: 0)
  final int maxEnrichmentsPerMonth;
  @JsonKey(defaultValue: 0)
  final int maxAiDraftsPerMonth;
  @JsonKey(defaultValue: 0)
  final int maxDocumentExtractionsPerMonth;
  @JsonKey(defaultValue: 0)
  final int maxEmailMailboxes;
  @JsonKey(defaultValue: 0)
  final int maxFunnels;

  const EffectivePlanLimitsModel({
    this.planId,
    required this.planName,
    required this.isContractPlan,
    required this.maxAgents,
    required this.maxChannels,
    required this.maxSeats,
    required this.maxConversationsPerMonth,
    required this.maxAiTokensPerMonth,
    required this.maxStorageMb,
    required this.maxBroadcastMessagesPerMonth,
    required this.maxRecipientsPerCampaign,
    required this.overageConversationCents,
    required this.overageCapMultiplier,
    this.maxLeadsPerMonth = 0,
    this.maxEnrichmentsPerMonth = 0,
    this.maxAiDraftsPerMonth = 0,
    this.maxDocumentExtractionsPerMonth = 0,
    this.maxEmailMailboxes = 0,
    this.maxFunnels = 0,
  });

  factory EffectivePlanLimitsModel.fromJson(Map<String, dynamic> json) =>
      _$EffectivePlanLimitsModelFromJson(json);
  Map<String, dynamic> toJson() => _$EffectivePlanLimitsModelToJson(this);

  /// `int32` max, which the backend uses in places as a stand-in for "no
  /// ceiling". Rendered literally it reaches the customer as `2147.5M` — a
  /// quantity nobody was ever sold, sitting next to their real usage as though
  /// it meant something.
  static const int int32Sentinel = 2147483647;

  /// A limit is "unlimited" at the explicit `-1`, and also at anything from the
  /// int32 sentinel up. Treating the sentinel as a real number is how it ends
  /// up on screen; no plan meaningfully caps anything at two billion.
  static bool isUnlimited(int v) => v == -1 || v >= int32Sentinel;
  bool get unlimitedTokens => isUnlimited(maxAiTokensPerMonth);
}
