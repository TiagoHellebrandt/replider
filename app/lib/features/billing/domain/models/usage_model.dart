import 'package:json_annotation/json_annotation.dart';

part 'usage_model.g.dart';

@JsonSerializable(fieldRename: FieldRename.snake)

/// Current billing cycle usage for an account.
/// Tracks AI tokens, conversations, storage against plan limits.
class UsageModel {
  final int aiTokensUsed;
  final int aiInputTokensUsed;
  final int aiOutputTokensUsed;
  final int addonTokens;
  final int effectiveLimit;
  final int conversationsCount;
  final int maxAiTokensPerMonth;
  final int maxConversationsPerMonth;
  final double tokensUsagePercent;
  final double conversationsUsagePercent;
  final String planName;
  final bool isTrialing;

  /// Conversations billed beyond the plan franchise this cycle.
  final int overageConversationsCount;

  /// Overage amount charged so far this cycle, in cents of the
  /// account's billing currency.
  final int overageCentsCharged;

  /// Per-extra-conversation overage rate in cents of the account's
  /// billing currency. 0 on plans without overage metering (e.g. Trial).
  final int overageConversationCents;

  /// True once the cycle overage cap was hit — further conversations
  /// are hard-blocked until the cycle resets.
  final bool capReached;

  /// The month's consumption in the units the customer buys. Null when the server
  /// could not count it — the screen then shows nothing rather than a zero, which
  /// would read as "I used nothing".
  final SdrUsageModel? sdr;

  const UsageModel({
    required this.aiTokensUsed,
    this.aiInputTokensUsed = 0,
    this.aiOutputTokensUsed = 0,
    this.addonTokens = 0,
    this.effectiveLimit = 0,
    required this.conversationsCount,
    required this.maxAiTokensPerMonth,
    required this.maxConversationsPerMonth,
    this.tokensUsagePercent = 0,
    this.conversationsUsagePercent = 0,
    this.planName = 'Trial',
    this.isTrialing = false,
    this.overageConversationsCount = 0,
    this.overageCentsCharged = 0,
    this.overageConversationCents = 0,
    this.capReached = false,
    this.sdr,
  });

  factory UsageModel.fromJson(Map<String, dynamic> json) =>
      _$UsageModelFromJson(json);
  Map<String, dynamic> toJson() => _$UsageModelToJson(this);

  /// The effective token limit (plan max + addon tokens).
  int get computedEffectiveLimit =>
      effectiveLimit > 0 ? effectiveLimit : maxAiTokensPerMonth + addonTokens;

  /// Tokens usage as a 0-1 fraction (backend sends 0-100 percentage).
  double get tokensUsageFraction => tokensUsagePercent / 100;

  /// Conversations usage as a 0-1 fraction (backend sends 0-100 percentage).
  double get conversationsUsageFraction => conversationsUsagePercent / 100;

  /// True once conversations have passed the plan franchise and the
  /// account is being billed per extra conversation.
  bool get isInOverage => overageConversationsCount > 0;
}

/// Lead, pesquisa, abordagem e reunião no mês corrente — contados no mesmo
/// event log the plan's caps use to block. Two different counts would end up
/// disagreeing on the day somebody forgot to increment one of them.
@JsonSerializable(fieldRename: FieldRename.snake)
class SdrUsageModel {
  final int leads;
  final int enrichments;
  final int approachesSent;
  final int meetings;

  const SdrUsageModel({
    this.leads = 0,
    this.enrichments = 0,
    this.approachesSent = 0,
    this.meetings = 0,
  });

  factory SdrUsageModel.fromJson(Map<String, dynamic> json) =>
      _$SdrUsageModelFromJson(json);
  Map<String, dynamic> toJson() => _$SdrUsageModelToJson(this);
}
