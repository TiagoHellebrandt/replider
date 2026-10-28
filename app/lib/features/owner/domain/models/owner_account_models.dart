import 'package:json_annotation/json_annotation.dart';

part 'owner_account_models.g.dart';

/// Listing row from `GET /v1/owner/accounts`. Slim — the detail screen
/// fetches the full account separately. Carries current-period token
/// usage + plan max so the cap-alert badge can render without a
/// second roundtrip per row.
@JsonSerializable(fieldRename: FieldRename.snake, createToJson: false)
class OwnerAccountListing {
  final OwnerAccountSummary account;
  final String planName;
  final String ownerEmail;
  final String ownerName;
  final int tokensUsed;
  final int addonTokens;
  final int conversationsCount;
  final int planMaxTokens;

  /// Worst Meta quality rating (GREEN/YELLOW/RED) across the account's
  /// WhatsApp numbers, cached server-side by the quality worker. Empty
  /// until the first probe lands. RED = number at risk of being banned.
  final String whatsappQuality;

  OwnerAccountListing({
    required this.account,
    this.planName = '',
    this.ownerEmail = '',
    this.ownerName = '',
    this.tokensUsed = 0,
    this.addonTokens = 0,
    this.conversationsCount = 0,
    this.planMaxTokens = 0,
    this.whatsappQuality = '',
  });

  /// 0..100 (clamped) — proportion of the current-period token cap
  /// consumed. 0 when there is no cap (contract plan with cap=0). The
  /// addon bucket counts toward available capacity.
  int get capPercent {
    if (planMaxTokens <= 0) return 0;
    final denom = planMaxTokens + addonTokens;
    if (denom <= 0) return 0;
    final pct = (tokensUsed * 100) ~/ denom;
    if (pct < 0) return 0;
    if (pct > 100) return 100;
    return pct;
  }

  factory OwnerAccountListing.fromJson(Map<String, dynamic> json) =>
      _$OwnerAccountListingFromJson(json);
}

@JsonSerializable(fieldRename: FieldRename.snake, createToJson: false)
class OwnerAccountSummary {
  final String id;
  final String name;
  final String? planId;
  final String subscriptionStatus;
  final bool onboardingCompleted;
  final bool csatEnabled;
  final bool trialBypass;
  final DateTime? trialExpiresAt;
  final DateTime? pastDueAt;
  final DateTime? suspendedAt;
  final String? suspendedReason;
  final int? maxAgentsOverride;
  final int? maxChannelsOverride;
  final int? maxSeatsOverride;
  final int? maxConversationsPerMonthOverride;
  final int? maxAiTokensPerMonthOverride;

  /// Seller attribution (vendedores v3): platform-account user managing
  /// this customer + which commission track ('seller_ref' | 'inbound').
  final String? sellerUserId;
  final String? leadSource;
  final DateTime? sellerLinkedAt;
  final DateTime createdAt;
  final DateTime updatedAt;

  OwnerAccountSummary({
    required this.id,
    required this.name,
    this.planId,
    required this.subscriptionStatus,
    required this.onboardingCompleted,
    required this.csatEnabled,
    required this.trialBypass,
    this.trialExpiresAt,
    this.pastDueAt,
    this.suspendedAt,
    this.suspendedReason,
    this.maxAgentsOverride,
    this.maxChannelsOverride,
    this.maxSeatsOverride,
    this.maxConversationsPerMonthOverride,
    this.maxAiTokensPerMonthOverride,
    this.sellerUserId,
    this.leadSource,
    this.sellerLinkedAt,
    required this.createdAt,
    required this.updatedAt,
  });

  bool get isSuspended => suspendedAt != null;

  factory OwnerAccountSummary.fromJson(Map<String, dynamic> json) =>
      _$OwnerAccountSummaryFromJson(json);
}

@JsonSerializable(fieldRename: FieldRename.snake, createToJson: false)
class OwnerAccountDetail {
  final OwnerAccountSummary account;
  final OwnerPlanInfo? plan;
  final OwnerUsageInfo? usage;
  final OwnerAdminUserInfo? admin;

  /// Seller (platform-account user) linked to this customer, if any.
  final OwnerAdminUserInfo? seller;

  /// Backend flag (US-013): the linked seller looks like the account's
  /// own admin (same user id or same email) — self-referral to be
  /// reviewed by the owner.
  @JsonKey(defaultValue: false)
  final bool sellerSelfMatch;

  /// OPEN internal CS threads this account's users hold with the
  /// platform (canal-interno US-010).
  @JsonKey(defaultValue: 0)
  final int internalThreadsActive;

  OwnerAccountDetail({
    required this.account,
    this.plan,
    this.usage,
    this.admin,
    this.seller,
    this.sellerSelfMatch = false,
    this.internalThreadsActive = 0,
  });

  factory OwnerAccountDetail.fromJson(Map<String, dynamic> json) =>
      _$OwnerAccountDetailFromJson(json);
}

@JsonSerializable(fieldRename: FieldRename.snake, createToJson: false)
class OwnerPlanInfo {
  final String id;
  final String name;
  final int maxAgents;
  final int maxChannels;
  final int maxConversationsPerMonth;
  @JsonKey(name: 'max_ai_tokens_per_month')
  final int maxAiTokensPerMonth;

  OwnerPlanInfo({
    required this.id,
    required this.name,
    required this.maxAgents,
    required this.maxChannels,
    required this.maxConversationsPerMonth,
    required this.maxAiTokensPerMonth,
  });

  factory OwnerPlanInfo.fromJson(Map<String, dynamic> json) =>
      _$OwnerPlanInfoFromJson(json);
}

@JsonSerializable(fieldRename: FieldRename.snake, createToJson: false)
class OwnerUsageInfo {
  final int? aiTokensUsed;
  final int? addonTokens;
  final int? conversationsCount;
  final int? messagesCount;

  OwnerUsageInfo({
    this.aiTokensUsed,
    this.addonTokens,
    this.conversationsCount,
    this.messagesCount,
  });

  factory OwnerUsageInfo.fromJson(Map<String, dynamic> json) =>
      _$OwnerUsageInfoFromJson(json);
}

@JsonSerializable(fieldRename: FieldRename.snake, createToJson: false)
class OwnerAdminUserInfo {
  final String id;
  final String name;
  final String email;
  final bool isEmailVerified;
  final DateTime? lastSeenAt;

  OwnerAdminUserInfo({
    required this.id,
    required this.name,
    required this.email,
    required this.isEmailVerified,
    this.lastSeenAt,
  });

  factory OwnerAdminUserInfo.fromJson(Map<String, dynamic> json) =>
      _$OwnerAdminUserInfoFromJson(json);
}

@JsonSerializable(fieldRename: FieldRename.snake, createToJson: false)
class TopAccountUsage {
  final String id;
  final String name;
  final String? planId;
  final String planName;
  final int tokensUsed;
  final int conversationsCount;
  final DateTime? suspendedAt;

  TopAccountUsage({
    required this.id,
    required this.name,
    this.planId,
    this.planName = '',
    this.tokensUsed = 0,
    this.conversationsCount = 0,
    this.suspendedAt,
  });

  factory TopAccountUsage.fromJson(Map<String, dynamic> json) =>
      _$TopAccountUsageFromJson(json);
}

/// Wrapper around `GET /v1/owner/accounts/top` ({items: [...]}).
class TopAccountsResponse {
  final List<TopAccountUsage> items;

  TopAccountsResponse({required this.items});

  factory TopAccountsResponse.fromJson(Map<String, dynamic> json) {
    final items = (json['items'] as List?)
            ?.map((e) => TopAccountUsage.fromJson(e as Map<String, dynamic>))
            .toList() ??
        const [];
    return TopAccountsResponse(items: items);
  }
}

/// One day in the 30-day usage trend rollup. Plotted as a bar in the
/// owner-panel mini chart.
@JsonSerializable(fieldRename: FieldRename.snake, createToJson: false)
class OwnerUsageTrendDay {
  final DateTime day;
  final int tokens;
  final int conversationCount;

  OwnerUsageTrendDay({
    required this.day,
    this.tokens = 0,
    this.conversationCount = 0,
  });

  factory OwnerUsageTrendDay.fromJson(Map<String, dynamic> json) =>
      _$OwnerUsageTrendDayFromJson(json);
}

/// One Stripe invoice from the owner-panel read-only invoice list. Maps
/// 1:1 to `domain.StripeInvoiceSummary` on the API side. Amounts are in
/// the smallest currency unit (cents). `hostedUrl` / `invoicePdf` are
/// nullable when the invoice was issued before Stripe rolled the
/// hosted-checkout assets.
@JsonSerializable(fieldRename: FieldRename.snake, createToJson: false)
class OwnerInvoice {
  final String id;
  final String number;
  final String status;
  final int amountDue;
  final int amountPaid;
  final String currency;
  final int created;
  final String? hostedUrl;
  @JsonKey(name: 'invoice_pdf')
  final String? invoicePdf;

  OwnerInvoice({
    required this.id,
    this.number = '',
    this.status = '',
    this.amountDue = 0,
    this.amountPaid = 0,
    this.currency = 'brl',
    this.created = 0,
    this.hostedUrl,
    this.invoicePdf,
  });

  DateTime get createdAt => DateTime.fromMillisecondsSinceEpoch(created * 1000);

  factory OwnerInvoice.fromJson(Map<String, dynamic> json) =>
      _$OwnerInvoiceFromJson(json);
}

/// Owner-panel system health snapshot from `GET /v1/owner/system-health`.
@JsonSerializable(fieldRename: FieldRename.snake, createToJson: false)
class OwnerSystemHealth {
  final int accountsTotal;
  final int accountsActive;
  final int accountsTrialing;
  final int accountsSuspended;
  final int usersActive;
  final int conversations24h;
  final int messages24h;

  OwnerSystemHealth({
    this.accountsTotal = 0,
    this.accountsActive = 0,
    this.accountsTrialing = 0,
    this.accountsSuspended = 0,
    this.usersActive = 0,
    this.conversations24h = 0,
    this.messages24h = 0,
  });

  factory OwnerSystemHealth.fromJson(Map<String, dynamic> json) =>
      _$OwnerSystemHealthFromJson(json);
}

/// Slim conversation row from `GET /v1/owner/accounts/:id/conversations`.
/// Only the triage fields surface — message bodies stay opaque so the
/// support flow nudges operators toward impersonation when they need
/// the full transcript.
@JsonSerializable(fieldRename: FieldRename.snake, createToJson: false)
class OwnerConversationRow {
  final String id;
  final String channelId;
  final String? agentId;
  final String contactName;
  final String status;
  final bool isBotPaused;
  final String channelType;
  final int messageCount;
  final DateTime? lastCustomerMessageAt;
  final DateTime createdAt;

  OwnerConversationRow({
    required this.id,
    required this.channelId,
    this.agentId,
    required this.contactName,
    required this.status,
    required this.isBotPaused,
    required this.channelType,
    required this.messageCount,
    this.lastCustomerMessageAt,
    required this.createdAt,
  });

  factory OwnerConversationRow.fromJson(Map<String, dynamic> json) =>
      _$OwnerConversationRowFromJson(json);
}

/// Response of `POST /v1/owner/accounts/:id/impersonate`. The token is
/// dropped into FlutterSecureStorage temporarily so the user can act as
/// the target account; expires_at gates a "session expiring" countdown
/// in the banner.
@JsonSerializable(fieldRename: FieldRename.snake, createToJson: false)
class ImpersonateResponse {
  final String token;
  final DateTime expiresAt;
  final String impersonatingUser;
  final String accountId;
  final String accountName;

  ImpersonateResponse({
    required this.token,
    required this.expiresAt,
    required this.impersonatingUser,
    required this.accountId,
    required this.accountName,
  });

  factory ImpersonateResponse.fromJson(Map<String, dynamic> json) =>
      _$ImpersonateResponseFromJson(json);
}
