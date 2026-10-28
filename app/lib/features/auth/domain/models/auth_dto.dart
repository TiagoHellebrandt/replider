import 'package:json_annotation/json_annotation.dart';

part 'auth_dto.g.dart';

@JsonSerializable()
class AuthenticateUserDTO {
  final String email;
  final String password;
  const AuthenticateUserDTO({required this.email, required this.password});
  Map<String, dynamic> toJson() => _$AuthenticateUserDTOToJson(this);
}

@JsonSerializable()
class AuthenticateUserResponse {
  /// Empty when [mfaRequired] is true — the API holds the real session
  /// JWT until the second leg of the login flow.
  final String? token;
  @JsonKey(name: 'is_new')
  final bool? isNew;

  /// True when the user has 2FA enabled. The client must show the MFA
  /// step and trade [mfaSession] back via /auth/2fa/verify-login.
  @JsonKey(name: 'mfa_required')
  final bool? mfaRequired;
  @JsonKey(name: 'mfa_session')
  final String? mfaSession;

  const AuthenticateUserResponse({
    this.token,
    this.isNew,
    this.mfaRequired,
    this.mfaSession,
  });
  factory AuthenticateUserResponse.fromJson(Map<String, dynamic> json) =>
      _$AuthenticateUserResponseFromJson(json);
}

@JsonSerializable()
class VerifyMfaLoginDTO {
  @JsonKey(name: 'mfa_session')
  final String mfaSession;
  final String code;
  const VerifyMfaLoginDTO({required this.mfaSession, required this.code});
  Map<String, dynamic> toJson() => _$VerifyMfaLoginDTOToJson(this);
}

@JsonSerializable()
class VerifyMfaLoginResponse {
  final String token;
  @JsonKey(name: 'is_new')
  final bool? isNew;
  const VerifyMfaLoginResponse({required this.token, this.isNew});
  factory VerifyMfaLoginResponse.fromJson(Map<String, dynamic> json) =>
      _$VerifyMfaLoginResponseFromJson(json);
}

@JsonSerializable()
class CreateAccountDTO {
  @JsonKey(name: 'account_name')
  final String accountName;
  @JsonKey(name: 'admin_email')
  final String adminEmail;
  @JsonKey(name: 'admin_name')
  final String adminName;
  final String password;

  /// LGPD: must be true. Controlled by a checkbox next to the Terms +
  /// Privacy Policy links on the register screen.
  @JsonKey(name: 'terms_accepted')
  final bool termsAccepted;
  // First-touch acquisition attribution forwarded from the marketing site's
  // register CTA (utm_*). Null when the user arrived without UTM params.
  @JsonKey(name: 'utm_source')
  final String? utmSource;
  @JsonKey(name: 'utm_medium')
  final String? utmMedium;
  @JsonKey(name: 'utm_campaign')
  final String? utmCampaign;

  /// One-shot signup invite from /register?invite=… (sales-led flow). With
  /// the public door closed it is the only way in; the account is born on
  /// the invite's contracted plan.
  @JsonKey(name: 'invite_token')
  final String? inviteToken;
  // Referral code forwarded from a ?ref=CODE signup link (US-174). Null
  // when the user arrived without a referral.
  @JsonKey(name: 'referral_code')
  final String? referralCode;
  // Device locale tag at signup (e.g. 'en-US', 'pt-BR', 'de-DE').
  // Decides the account's immutable billing currency server-side —
  // only a real en-US maps to USD, everything else defaults to BRL
  // (US-017 review fixes). Never the Slang UI locale: that collapses
  // every non-Portuguese device to 'en'.
  final String? locale;

  const CreateAccountDTO({
    required this.accountName,
    required this.adminEmail,
    required this.adminName,
    required this.password,
    required this.termsAccepted,
    this.utmSource,
    this.utmMedium,
    this.utmCampaign,
    this.referralCode,
    this.locale,
    this.inviteToken,
  });
  Map<String, dynamic> toJson() => _$CreateAccountDTOToJson(this);
}

@JsonSerializable()
class HandleCreateAccountResponse {
  final String? message;
  const HandleCreateAccountResponse({this.message});
  factory HandleCreateAccountResponse.fromJson(Map<String, dynamic> json) =>
      _$HandleCreateAccountResponseFromJson(json);
}

@JsonSerializable()
class ForgotPasswordDTO {
  final String email;
  const ForgotPasswordDTO({required this.email});
  Map<String, dynamic> toJson() => _$ForgotPasswordDTOToJson(this);
}

@JsonSerializable()
class ResetPasswordDTO {
  final String token;
  @JsonKey(name: 'new_password')
  final String newPassword;
  const ResetPasswordDTO({required this.token, required this.newPassword});
  Map<String, dynamic> toJson() => _$ResetPasswordDTOToJson(this);
}

@JsonSerializable()
class SocialAuthDTO {
  @JsonKey(name: 'id_token')
  final String idToken;
  // Device locale tag (e.g. 'en-US', 'pt-BR'). Only used server-side
  // when this social login creates a brand-new account — decides its
  // immutable billing currency (only a real en-US → USD, US-017).
  final String? locale;
  // ?ref=CODE from the signup link, forwarded so social signups get the
  // same seller/referral attribution as email/password ones (US-003).
  // Only consulted server-side when the login creates a new account.
  @JsonKey(name: 'referral_code')
  final String? referralCode;
  const SocialAuthDTO({required this.idToken, this.locale, this.referralCode});
  Map<String, dynamic> toJson() => _$SocialAuthDTOToJson(this);
}

@JsonSerializable()
class SocialAuthResponse {
  final String token;
  @JsonKey(name: 'is_new')
  final bool? isNew;

  const SocialAuthResponse({required this.token, this.isNew});
  factory SocialAuthResponse.fromJson(Map<String, dynamic> json) =>
      _$SocialAuthResponseFromJson(json);
}

@JsonSerializable()
class GetMeResponse {
  final Map<String, dynamic> user;
  final Map<String, dynamic> account;

  /// True only for users of the Replider platform account (US-001).
  /// Gates the seller (vendedores) UI: convert-lead action, /seller panel.
  @JsonKey(name: 'is_platform_account', defaultValue: false)
  final bool isPlatformAccount;

  /// Explicit owner-granted seller flag (review-fixes US-014) — the
  /// seller panel needs BOTH flags.
  @JsonKey(name: 'is_seller', defaultValue: false)
  final bool isSeller;

  const GetMeResponse({
    required this.user,
    required this.account,
    this.isPlatformAccount = false,
    this.isSeller = false,
  });
  factory GetMeResponse.fromJson(Map<String, dynamic> json) =>
      _$GetMeResponseFromJson(json);

  /// Subscription status straight from the account record. Typically one
  /// of "trialing", "active", "past_due", "canceled" — we don't enum it
  /// on the client to avoid breaking forward compatibility if the API
  /// introduces a new state.
  String get subscriptionStatus =>
      (account['subscription_status'] as String?) ?? 'trialing';

  /// Days remaining in the free trial, computed from `trial_expires_at`.
  /// Null when the field is absent (already on a paid plan, or legacy
  /// account created before the trial-window migration). Negative values
  /// are clamped to 0 so the banner reads "trial ended" instead of
  /// "-3 days remaining".
  int? get trialDaysRemaining {
    final raw = account['trial_expires_at'];
    if (raw is! String || raw.isEmpty) return null;
    final when = DateTime.tryParse(raw);
    if (when == null) return null;
    final diff = when.difference(DateTime.now()).inHours / 24;
    final days = diff.ceil();
    return days < 0 ? 0 : days;
  }

  /// True when the account is on a contract-billed plan (Partner /
  /// Enterprise). Drives the settings UI: hides Stripe checkout,
  /// surfaces the BYOK API-keys card, swaps upgrade buttons for the
  /// "Falar com especialista" CTA.
  bool get isContractPlan => (account['is_contract_plan'] as bool?) ?? false;

  /// Masked previews of the BYOK keys (first 4 + last 4 chars). Empty
  /// string means "no key configured". Always empty for self-serve
  /// accounts so the platform's env keys never leak through /me.
  String get openAIKeyMasked => (account['openai_key_masked'] as String?) ?? '';
  String get geminiKeyMasked => (account['gemini_key_masked'] as String?) ?? '';
}
