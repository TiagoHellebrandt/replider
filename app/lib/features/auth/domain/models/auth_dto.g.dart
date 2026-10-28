// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'auth_dto.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

AuthenticateUserDTO _$AuthenticateUserDTOFromJson(Map<String, dynamic> json) =>
    AuthenticateUserDTO(
      email: json['email'] as String,
      password: json['password'] as String,
    );

Map<String, dynamic> _$AuthenticateUserDTOToJson(
        AuthenticateUserDTO instance) =>
    <String, dynamic>{
      'email': instance.email,
      'password': instance.password,
    };

AuthenticateUserResponse _$AuthenticateUserResponseFromJson(
        Map<String, dynamic> json) =>
    AuthenticateUserResponse(
      token: json['token'] as String?,
      isNew: json['is_new'] as bool?,
      mfaRequired: json['mfa_required'] as bool?,
      mfaSession: json['mfa_session'] as String?,
    );

Map<String, dynamic> _$AuthenticateUserResponseToJson(
        AuthenticateUserResponse instance) =>
    <String, dynamic>{
      'token': instance.token,
      'is_new': instance.isNew,
      'mfa_required': instance.mfaRequired,
      'mfa_session': instance.mfaSession,
    };

VerifyMfaLoginDTO _$VerifyMfaLoginDTOFromJson(Map<String, dynamic> json) =>
    VerifyMfaLoginDTO(
      mfaSession: json['mfa_session'] as String,
      code: json['code'] as String,
    );

Map<String, dynamic> _$VerifyMfaLoginDTOToJson(VerifyMfaLoginDTO instance) =>
    <String, dynamic>{
      'mfa_session': instance.mfaSession,
      'code': instance.code,
    };

VerifyMfaLoginResponse _$VerifyMfaLoginResponseFromJson(
        Map<String, dynamic> json) =>
    VerifyMfaLoginResponse(
      token: json['token'] as String,
      isNew: json['is_new'] as bool?,
    );

Map<String, dynamic> _$VerifyMfaLoginResponseToJson(
        VerifyMfaLoginResponse instance) =>
    <String, dynamic>{
      'token': instance.token,
      'is_new': instance.isNew,
    };

CreateAccountDTO _$CreateAccountDTOFromJson(Map<String, dynamic> json) =>
    CreateAccountDTO(
      accountName: json['account_name'] as String,
      adminEmail: json['admin_email'] as String,
      adminName: json['admin_name'] as String,
      password: json['password'] as String,
      termsAccepted: json['terms_accepted'] as bool,
      utmSource: json['utm_source'] as String?,
      utmMedium: json['utm_medium'] as String?,
      utmCampaign: json['utm_campaign'] as String?,
      referralCode: json['referral_code'] as String?,
      locale: json['locale'] as String?,
      inviteToken: json['invite_token'] as String?,
    );

Map<String, dynamic> _$CreateAccountDTOToJson(CreateAccountDTO instance) =>
    <String, dynamic>{
      'account_name': instance.accountName,
      'admin_email': instance.adminEmail,
      'admin_name': instance.adminName,
      'password': instance.password,
      'terms_accepted': instance.termsAccepted,
      'utm_source': instance.utmSource,
      'utm_medium': instance.utmMedium,
      'utm_campaign': instance.utmCampaign,
      'invite_token': instance.inviteToken,
      'referral_code': instance.referralCode,
      'locale': instance.locale,
    };

HandleCreateAccountResponse _$HandleCreateAccountResponseFromJson(
        Map<String, dynamic> json) =>
    HandleCreateAccountResponse(
      message: json['message'] as String?,
    );

Map<String, dynamic> _$HandleCreateAccountResponseToJson(
        HandleCreateAccountResponse instance) =>
    <String, dynamic>{
      'message': instance.message,
    };

ForgotPasswordDTO _$ForgotPasswordDTOFromJson(Map<String, dynamic> json) =>
    ForgotPasswordDTO(
      email: json['email'] as String,
    );

Map<String, dynamic> _$ForgotPasswordDTOToJson(ForgotPasswordDTO instance) =>
    <String, dynamic>{
      'email': instance.email,
    };

ResetPasswordDTO _$ResetPasswordDTOFromJson(Map<String, dynamic> json) =>
    ResetPasswordDTO(
      token: json['token'] as String,
      newPassword: json['new_password'] as String,
    );

Map<String, dynamic> _$ResetPasswordDTOToJson(ResetPasswordDTO instance) =>
    <String, dynamic>{
      'token': instance.token,
      'new_password': instance.newPassword,
    };

SocialAuthDTO _$SocialAuthDTOFromJson(Map<String, dynamic> json) =>
    SocialAuthDTO(
      idToken: json['id_token'] as String,
      locale: json['locale'] as String?,
      referralCode: json['referral_code'] as String?,
    );

Map<String, dynamic> _$SocialAuthDTOToJson(SocialAuthDTO instance) =>
    <String, dynamic>{
      'id_token': instance.idToken,
      'locale': instance.locale,
      'referral_code': instance.referralCode,
    };

SocialAuthResponse _$SocialAuthResponseFromJson(Map<String, dynamic> json) =>
    SocialAuthResponse(
      token: json['token'] as String,
      isNew: json['is_new'] as bool?,
    );

Map<String, dynamic> _$SocialAuthResponseToJson(SocialAuthResponse instance) =>
    <String, dynamic>{
      'token': instance.token,
      'is_new': instance.isNew,
    };

GetMeResponse _$GetMeResponseFromJson(Map<String, dynamic> json) =>
    GetMeResponse(
      user: json['user'] as Map<String, dynamic>,
      account: json['account'] as Map<String, dynamic>,
      isPlatformAccount: json['is_platform_account'] as bool? ?? false,
      isSeller: json['is_seller'] as bool? ?? false,
    );

Map<String, dynamic> _$GetMeResponseToJson(GetMeResponse instance) =>
    <String, dynamic>{
      'user': instance.user,
      'account': instance.account,
      'is_platform_account': instance.isPlatformAccount,
      'is_seller': instance.isSeller,
    };
