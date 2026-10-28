import 'package:json_annotation/json_annotation.dart';

import 'package:app/core/auth/account_role.dart';

part 'user_model.g.dart';

@JsonSerializable()
class UserModel {
  final String id;
  final String name;
  final String email;

  /// The account's display name from /me. Used to prefill the onboarding's
  /// company step — but only when it differs from the USER's name: social
  /// signups inherit the person's name as the account name, and prefilling
  /// that would put "Tiago" where the clinic belongs.
  @JsonKey(name: 'account_name')
  final String accountName;
  final String token;
  @JsonKey(name: 'is_new')
  final bool isNew;
  @JsonKey(name: 'is_trialing')
  final bool isTrialing;
  @JsonKey(name: 'plan_name')
  final String planName;
  final String role;

  /// Profile picture URL. Currently populated only for users who signed in
  /// via Google (from the `picture` claim of the id_token). `null` for
  /// email/password users — UI should fall back to an initials avatar.
  final String? picture;

  /// Days remaining in the free trial, computed server-side from the
  /// account's trial_expires_at. Null when the account isn't on trial
  /// (already on a paid plan) or for legacy accounts created before the
  /// trial-window migration. Used to render the countdown banner.
  @JsonKey(name: 'trial_days_remaining')
  final int? trialDaysRemaining;

  /// Raw account subscription state from Stripe — `trialing`, `active`,
  /// `past_due`, `canceled`, `incomplete`. Drives the read-only banner
  /// on Settings: trial-expired or inactive accounts can browse their
  /// agents/channels/knowledge but can't write — the API returns 402 on
  /// any POST/PUT/PATCH/DELETE. UI uses [isPaidActive] to disable the
  /// edit buttons up front instead of letting the user fill a form and
  /// hit a wall on save.
  @JsonKey(name: 'subscription_status')
  final String? subscriptionStatus;

  /// True for Partner / Enterprise accounts (plan with no Stripe price).
  /// Drives the BYOK API-keys card visibility + the "Falar com
  /// especialista" CTAs in billing. Defaults to false so legacy cached
  /// rows behave like self-serve users.
  @JsonKey(name: 'is_contract_plan')
  final bool isContractPlan;

  /// Masked previews of the BYOK keys (first 4 + last 4 chars). Empty
  /// string when no key is configured. Always empty for self-serve
  /// plans — the API doesn't echo platform env keys here.
  @JsonKey(name: 'openai_key_masked')
  final String openAIKeyMasked;
  @JsonKey(name: 'gemini_key_masked')
  final String geminiKeyMasked;

  /// Set when the current JWT was minted via the owner-panel
  /// impersonate endpoint. Holds the operator email. Empty for
  /// regular sessions. The dashboard banner uses this as the
  /// server-truth source — local storage holds a mirror for
  /// availability between /me refreshes.
  @JsonKey(name: 'impersonating_actor')
  final String? impersonatingActor;

  /// Whether the user has TOTP 2FA enrolled. Carried on /me so the
  /// Profile 2FA card renders its state instantly from the cached user
  /// instead of waiting on a dedicated GET /me/2fa/status round-trip.
  /// Defaults to false so legacy cached rows render as "disabled".
  @JsonKey(name: 'totp_enabled')
  final bool totpEnabled;

  /// True when the account is exempt from the trial/subscription gate
  /// (owner-provisioned contract deals, Meta reviewers). Suppresses every
  /// "trial expired / upgrade" banner — see [isPaidActive]. Defaults to
  /// false so legacy cached rows behave like self-serve users.
  @JsonKey(name: 'trial_bypass')
  final bool trialBypass;

  /// Whether the user has confirmed their email. Carried on /me so the
  /// dashboard can render a "verify your email" nudge for an owner who
  /// signed up but never clicked the link. Defaults to TRUE so a legacy
  /// cached row (or a backend that omits the field) never false-flags a
  /// verified user with the banner — the real false value only arrives
  /// from a fresh /me.
  @JsonKey(name: 'is_email_verified')
  final bool isEmailVerified;

  /// The account's immutable billing currency (`brl` | `usd`, US-001).
  /// Sourced from `/me` `account.currency`. Drives which price surfaces
  /// render (addon packs are BRL-only) — the plans list itself is already
  /// pinned server-side. Defaults to `brl` so legacy cached rows keep
  /// today's behaviour until the next /me refresh.
  @JsonKey(name: 'account_currency', defaultValue: 'brl')
  final String accountCurrency;

  /// True only for users of the Replider platform account (US-001).
  /// Gates the seller (vendedores) UI — convert-lead action on
  /// conversations, /seller panel. Defaults to false so cached rows of
  /// regular customers never flash seller controls.
  @JsonKey(name: 'is_platform_account')
  final bool isPlatformAccount;

  /// Explicit owner-granted seller flag (review-fixes US-014). The
  /// seller panel entry needs BOTH [isPlatformAccount] and this — a
  /// regular platform attendant is not a payable seller. Defaults to
  /// false so cached rows never flash seller controls.
  @JsonKey(name: 'is_seller')
  final bool isSeller;

  /// Optional personal sign-off the attendant can append to MANUAL
  /// replies from the composer (US-018). Never applied to AI responses.
  /// Null/empty hides the composer's insert-signature affordance.
  final String? signature;

  /// The seller's own WhatsApp number (E.164). With it set, texting the
  /// account's WhatsApp channel from this number works the approval
  /// queue (OFERTAS / APROVAR / PULAR) instead of opening a conversation.

  const UserModel({
    required this.id,
    required this.name,
    required this.email,
    required this.token,
    this.isNew = false,
    this.isTrialing = false,
    this.planName = 'Trial',
    this.role = 'attendant',
    this.picture,
    this.trialDaysRemaining,
    this.subscriptionStatus,
    this.isContractPlan = false,
    this.openAIKeyMasked = '',
    this.geminiKeyMasked = '',
    this.impersonatingActor,
    this.totpEnabled = false,
    this.trialBypass = false,
    this.isEmailVerified = true,
    this.accountCurrency = 'brl',
    this.isPlatformAccount = false,
    this.isSeller = false,
    this.signature,
    this.accountName = '',
  });

  /// True when this is an impersonation session — the JWT was minted
  /// by the owner panel as another account's admin user.
  bool get isImpersonating =>
      impersonatingActor != null && impersonatingActor!.isNotEmpty;

  /// True when the user may reach the account-level surfaces. Asks the
  /// role-satisfaction table, not string equality — an `owner` outranks an
  /// admin and must not be locked out of what an admin can do.
  bool get isAdmin => roleIsAdmin(role);

  /// True when this user authenticated via Google (the only flow that
  /// currently populates [picture]).
  bool get isGoogleUser => picture != null && picture!.isNotEmpty;

  /// True when the account has access to paid features (active sub OR
  /// still inside the trial window). False when the trial has elapsed
  /// or the subscription is past_due/canceled. UI uses this to flip
  /// edit buttons into a "Plano expirado" read-only state.
  bool get isPaidActive {
    // Contract / owner-provisioned accounts are never billing-locked,
    // even if their trial window lapsed or the status reads 'trialing'.
    if (trialBypass || isContractPlan) return true;
    if (subscriptionStatus == 'active') return true;
    if (subscriptionStatus == 'trialing') {
      // Trial only counts as active while there are days remaining.
      // Null means we couldn't compute it server-side; fall back to
      // the explicit isTrialing flag, which will be true on a fresh
      // signup before the migration backfill runs.
      return (trialDaysRemaining ?? 1) > 0;
    }
    return false;
  }

  factory UserModel.fromJson(Map<String, dynamic> json) =>
      _$UserModelFromJson(json);
  Map<String, dynamic> toJson() => _$UserModelToJson(this);

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is UserModel &&
          runtimeType == other.runtimeType &&
          id == other.id &&
          name == other.name &&
          email == other.email &&
          token == other.token &&
          picture == other.picture;

  @override
  int get hashCode =>
      id.hashCode ^
      name.hashCode ^
      email.hashCode ^
      token.hashCode ^
      picture.hashCode;
}
