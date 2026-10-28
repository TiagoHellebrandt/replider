import 'package:json_annotation/json_annotation.dart';

import 'package:app/core/utils/currency_format.dart';

part 'plan_model.g.dart';

@JsonSerializable(fieldRename: FieldRename.snake)

/// A subscription plan defining resource limits and pricing.
/// Fetched from `GET /v1/plans` (database-backed, not hardcoded).
class PlanModel {
  final String id;
  final String name;
  final int price; // in cents
  final String currency;
  final String aiProvider;
  final String aiModel;
  final double inputMultiplier;
  final double outputMultiplier;
  final int maxAgents;
  final int maxChannels;

  /// Number of human attendant seats included. Defaults to 0 for older
  /// API responses that pre-date the seat-cap rollout (migration 000029).
  @JsonKey(defaultValue: 0)
  final int maxSeats;
  final int maxConversationsPerMonth;
  final int maxAiTokensPerMonth;
  final int maxStorageMb;

  /// Broadcast / campaign caps. Both default to 0 — the billing UI hides
  /// the row when 0 to avoid implying "unlimited" on plans that simply
  /// don't permit broadcasts (Solo / Trial). Migration 000017 + 000062
  /// own the canonical values per plan.
  @JsonKey(name: 'max_broadcast_messages_per_month', defaultValue: 0)
  final int maxBroadcastMessagesPerMonth;
  @JsonKey(name: 'max_recipients_per_campaign', defaultValue: 0)
  final int maxRecipientsPerCampaign;

  /// Overage metering. Per-extra-conversation rate in cents, capped at
  /// `overageCapMultiplier` × plan price per cycle. Both default to 0
  /// for plans without overage billing (Trial / contract plans where
  /// the contract embeds the spend).
  @JsonKey(name: 'overage_conversation_cents', defaultValue: 0)
  final int overageConversationCents;
  @JsonKey(name: 'overage_cap_multiplier', defaultValue: 0.0)
  final double overageCapMultiplier;

  /// Null for contract-billed plans (Enterprise / Partner). Used by the
  /// billing screen to swap the Stripe checkout button for a "Falar com
  /// especialista" CTA instead.
  final String? stripePriceId;

  /// Stripe price for the annual interval (20% off, US-169). Null when
  /// the plan has no annual option — USD tiers ship monthly-only
  /// (migration 000109) and the backend silently falls back to the
  /// MONTHLY price on an annual checkout, so the UI must not offer an
  /// annual discount unless this is set. See [hasAnnualBilling].
  final String? stripePriceIdAnnual;
  final String createdAt;
  final String updatedAt;

  const PlanModel({
    required this.id,
    required this.name,
    required this.price,
    required this.currency,
    required this.aiProvider,
    required this.aiModel,
    required this.inputMultiplier,
    required this.outputMultiplier,
    required this.maxAgents,
    required this.maxChannels,
    this.maxSeats = 0,
    required this.maxConversationsPerMonth,
    required this.maxAiTokensPerMonth,
    required this.maxStorageMb,
    this.maxBroadcastMessagesPerMonth = 0,
    this.maxRecipientsPerCampaign = 0,
    this.overageConversationCents = 0,
    this.overageCapMultiplier = 0.0,
    this.stripePriceId,
    this.stripePriceIdAnnual,
    required this.createdAt,
    required this.updatedAt,
  });

  factory PlanModel.fromJson(Map<String, dynamic> json) =>
      _$PlanModelFromJson(json);
  Map<String, dynamic> toJson() => _$PlanModelToJson(this);

  /// Plans without a Stripe price are sold under contract: the dashboard
  /// hides the self-serve checkout button and surfaces the "Falar com
  /// especialista" CTA instead.
  bool get isContractPlan => stripePriceId == null || stripePriceId!.isEmpty;

  /// True when the plan can actually be billed yearly. Gates the
  /// monthly/annual toggle: showing a 20%-off annual price for a plan
  /// without an annual Stripe price would charge the full monthly rate
  /// (backend fallback in Plan.ResolveStripePriceID).
  bool get hasAnnualBilling =>
      stripePriceIdAnnual != null && stripePriceIdAnnual!.isNotEmpty;

  /// Returns price formatted as currency string (e.g. "R$ 49,90" or "$49.90")
  String get formattedPrice => formatCurrencyCents(price, currency);

  /// Annual billing discount fraction (US-169). 20% off the monthly rate,
  /// matching the API seed (`annualDiscount` in cmd/seed_stripe_prices)
  /// and the landing-page copy (web ANNUAL_DISCOUNT).
  static const double annualDiscount = 0.20;

  /// The 20%-off monthly-equivalent price shown when annual billing is
  /// selected in the upgrade modal. The customer is billed 12× this once
  /// a year, but showing the per-month figure keeps it comparable to the
  /// monthly column.
  String get formattedAnnualMonthlyPrice =>
      formatCurrencyCents((price * (1 - annualDiscount)).round(), currency);
}

/// Contact-sales channels surfaced beside contract-only plans. Mirrors
/// `domain.ContactSalesChannels` from the API.
@JsonSerializable(fieldRename: FieldRename.snake)
class ContactSalesChannels {
  final String email;
  final String whatsapp;
  final String whatsappLink;
  final String whatsappText;

  /// The calendar page a prospect books a call on. Empty when the API has
  /// no BOOKING_URL configured; the card then leads with WhatsApp.
  @JsonKey(defaultValue: '')
  final String bookingUrl;

  const ContactSalesChannels({
    required this.email,
    required this.whatsapp,
    required this.whatsappLink,
    required this.whatsappText,
    this.bookingUrl = '',
  });

  factory ContactSalesChannels.fromJson(Map<String, dynamic> json) =>
      _$ContactSalesChannelsFromJson(json);
  Map<String, dynamic> toJson() => _$ContactSalesChannelsToJson(this);
}
