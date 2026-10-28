// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'owner_account_models.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

OwnerAccountListing _$OwnerAccountListingFromJson(Map<String, dynamic> json) =>
    OwnerAccountListing(
      account:
          OwnerAccountSummary.fromJson(json['account'] as Map<String, dynamic>),
      planName: json['plan_name'] as String? ?? '',
      ownerEmail: json['owner_email'] as String? ?? '',
      ownerName: json['owner_name'] as String? ?? '',
      tokensUsed: (json['tokens_used'] as num?)?.toInt() ?? 0,
      addonTokens: (json['addon_tokens'] as num?)?.toInt() ?? 0,
      conversationsCount: (json['conversations_count'] as num?)?.toInt() ?? 0,
      planMaxTokens: (json['plan_max_tokens'] as num?)?.toInt() ?? 0,
      whatsappQuality: json['whatsapp_quality'] as String? ?? '',
    );

OwnerAccountSummary _$OwnerAccountSummaryFromJson(Map<String, dynamic> json) =>
    OwnerAccountSummary(
      id: json['id'] as String,
      name: json['name'] as String,
      planId: json['plan_id'] as String?,
      subscriptionStatus: json['subscription_status'] as String,
      onboardingCompleted: json['onboarding_completed'] as bool,
      csatEnabled: json['csat_enabled'] as bool,
      trialBypass: json['trial_bypass'] as bool,
      trialExpiresAt: json['trial_expires_at'] == null
          ? null
          : DateTime.parse(json['trial_expires_at'] as String),
      pastDueAt: json['past_due_at'] == null
          ? null
          : DateTime.parse(json['past_due_at'] as String),
      suspendedAt: json['suspended_at'] == null
          ? null
          : DateTime.parse(json['suspended_at'] as String),
      suspendedReason: json['suspended_reason'] as String?,
      maxAgentsOverride: (json['max_agents_override'] as num?)?.toInt(),
      maxChannelsOverride: (json['max_channels_override'] as num?)?.toInt(),
      maxSeatsOverride: (json['max_seats_override'] as num?)?.toInt(),
      maxConversationsPerMonthOverride:
          (json['max_conversations_per_month_override'] as num?)?.toInt(),
      maxAiTokensPerMonthOverride:
          (json['max_ai_tokens_per_month_override'] as num?)?.toInt(),
      sellerUserId: json['seller_user_id'] as String?,
      leadSource: json['lead_source'] as String?,
      sellerLinkedAt: json['seller_linked_at'] == null
          ? null
          : DateTime.parse(json['seller_linked_at'] as String),
      createdAt: DateTime.parse(json['created_at'] as String),
      updatedAt: DateTime.parse(json['updated_at'] as String),
    );

OwnerAccountDetail _$OwnerAccountDetailFromJson(Map<String, dynamic> json) =>
    OwnerAccountDetail(
      account:
          OwnerAccountSummary.fromJson(json['account'] as Map<String, dynamic>),
      plan: json['plan'] == null
          ? null
          : OwnerPlanInfo.fromJson(json['plan'] as Map<String, dynamic>),
      usage: json['usage'] == null
          ? null
          : OwnerUsageInfo.fromJson(json['usage'] as Map<String, dynamic>),
      admin: json['admin'] == null
          ? null
          : OwnerAdminUserInfo.fromJson(json['admin'] as Map<String, dynamic>),
      seller: json['seller'] == null
          ? null
          : OwnerAdminUserInfo.fromJson(json['seller'] as Map<String, dynamic>),
      sellerSelfMatch: json['seller_self_match'] as bool? ?? false,
      internalThreadsActive:
          (json['internal_threads_active'] as num?)?.toInt() ?? 0,
    );

OwnerPlanInfo _$OwnerPlanInfoFromJson(Map<String, dynamic> json) =>
    OwnerPlanInfo(
      id: json['id'] as String,
      name: json['name'] as String,
      maxAgents: (json['max_agents'] as num).toInt(),
      maxChannels: (json['max_channels'] as num).toInt(),
      maxConversationsPerMonth:
          (json['max_conversations_per_month'] as num).toInt(),
      maxAiTokensPerMonth: (json['max_ai_tokens_per_month'] as num).toInt(),
    );

OwnerUsageInfo _$OwnerUsageInfoFromJson(Map<String, dynamic> json) =>
    OwnerUsageInfo(
      aiTokensUsed: (json['ai_tokens_used'] as num?)?.toInt(),
      addonTokens: (json['addon_tokens'] as num?)?.toInt(),
      conversationsCount: (json['conversations_count'] as num?)?.toInt(),
      messagesCount: (json['messages_count'] as num?)?.toInt(),
    );

OwnerAdminUserInfo _$OwnerAdminUserInfoFromJson(Map<String, dynamic> json) =>
    OwnerAdminUserInfo(
      id: json['id'] as String,
      name: json['name'] as String,
      email: json['email'] as String,
      isEmailVerified: json['is_email_verified'] as bool,
      lastSeenAt: json['last_seen_at'] == null
          ? null
          : DateTime.parse(json['last_seen_at'] as String),
    );

TopAccountUsage _$TopAccountUsageFromJson(Map<String, dynamic> json) =>
    TopAccountUsage(
      id: json['id'] as String,
      name: json['name'] as String,
      planId: json['plan_id'] as String?,
      planName: json['plan_name'] as String? ?? '',
      tokensUsed: (json['tokens_used'] as num?)?.toInt() ?? 0,
      conversationsCount: (json['conversations_count'] as num?)?.toInt() ?? 0,
      suspendedAt: json['suspended_at'] == null
          ? null
          : DateTime.parse(json['suspended_at'] as String),
    );

OwnerUsageTrendDay _$OwnerUsageTrendDayFromJson(Map<String, dynamic> json) =>
    OwnerUsageTrendDay(
      day: DateTime.parse(json['day'] as String),
      tokens: (json['tokens'] as num?)?.toInt() ?? 0,
      conversationCount: (json['conversation_count'] as num?)?.toInt() ?? 0,
    );

OwnerInvoice _$OwnerInvoiceFromJson(Map<String, dynamic> json) => OwnerInvoice(
      id: json['id'] as String,
      number: json['number'] as String? ?? '',
      status: json['status'] as String? ?? '',
      amountDue: (json['amount_due'] as num?)?.toInt() ?? 0,
      amountPaid: (json['amount_paid'] as num?)?.toInt() ?? 0,
      currency: json['currency'] as String? ?? 'brl',
      created: (json['created'] as num?)?.toInt() ?? 0,
      hostedUrl: json['hosted_url'] as String?,
      invoicePdf: json['invoice_pdf'] as String?,
    );

OwnerSystemHealth _$OwnerSystemHealthFromJson(Map<String, dynamic> json) =>
    OwnerSystemHealth(
      accountsTotal: (json['accounts_total'] as num?)?.toInt() ?? 0,
      accountsActive: (json['accounts_active'] as num?)?.toInt() ?? 0,
      accountsTrialing: (json['accounts_trialing'] as num?)?.toInt() ?? 0,
      accountsSuspended: (json['accounts_suspended'] as num?)?.toInt() ?? 0,
      usersActive: (json['users_active'] as num?)?.toInt() ?? 0,
      conversations24h: (json['conversations24h'] as num?)?.toInt() ?? 0,
      messages24h: (json['messages24h'] as num?)?.toInt() ?? 0,
    );

OwnerConversationRow _$OwnerConversationRowFromJson(
        Map<String, dynamic> json) =>
    OwnerConversationRow(
      id: json['id'] as String,
      channelId: json['channel_id'] as String,
      agentId: json['agent_id'] as String?,
      contactName: json['contact_name'] as String,
      status: json['status'] as String,
      isBotPaused: json['is_bot_paused'] as bool,
      channelType: json['channel_type'] as String,
      messageCount: (json['message_count'] as num).toInt(),
      lastCustomerMessageAt: json['last_customer_message_at'] == null
          ? null
          : DateTime.parse(json['last_customer_message_at'] as String),
      createdAt: DateTime.parse(json['created_at'] as String),
    );

ImpersonateResponse _$ImpersonateResponseFromJson(Map<String, dynamic> json) =>
    ImpersonateResponse(
      token: json['token'] as String,
      expiresAt: DateTime.parse(json['expires_at'] as String),
      impersonatingUser: json['impersonating_user'] as String,
      accountId: json['account_id'] as String,
      accountName: json['account_name'] as String,
    );
