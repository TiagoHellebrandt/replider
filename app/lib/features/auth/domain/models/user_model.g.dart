// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'user_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

UserModel _$UserModelFromJson(Map<String, dynamic> json) => UserModel(
      id: json['id'] as String,
      name: json['name'] as String,
      email: json['email'] as String,
      token: json['token'] as String,
      isNew: json['is_new'] as bool? ?? false,
      isTrialing: json['is_trialing'] as bool? ?? false,
      planName: json['plan_name'] as String? ?? 'Trial',
      role: json['role'] as String? ?? 'attendant',
      picture: json['picture'] as String?,
      trialDaysRemaining: (json['trial_days_remaining'] as num?)?.toInt(),
      subscriptionStatus: json['subscription_status'] as String?,
      isContractPlan: json['is_contract_plan'] as bool? ?? false,
      openAIKeyMasked: json['openai_key_masked'] as String? ?? '',
      geminiKeyMasked: json['gemini_key_masked'] as String? ?? '',
      impersonatingActor: json['impersonating_actor'] as String?,
      totpEnabled: json['totp_enabled'] as bool? ?? false,
      trialBypass: json['trial_bypass'] as bool? ?? false,
      isEmailVerified: json['is_email_verified'] as bool? ?? true,
      accountCurrency: json['account_currency'] as String? ?? 'brl',
      isPlatformAccount: json['is_platform_account'] as bool? ?? false,
      isSeller: json['is_seller'] as bool? ?? false,
      signature: json['signature'] as String?,
      accountName: json['account_name'] as String? ?? '',
    );

Map<String, dynamic> _$UserModelToJson(UserModel instance) => <String, dynamic>{
      'id': instance.id,
      'name': instance.name,
      'email': instance.email,
      'account_name': instance.accountName,
      'token': instance.token,
      'is_new': instance.isNew,
      'is_trialing': instance.isTrialing,
      'plan_name': instance.planName,
      'role': instance.role,
      'picture': instance.picture,
      'trial_days_remaining': instance.trialDaysRemaining,
      'subscription_status': instance.subscriptionStatus,
      'is_contract_plan': instance.isContractPlan,
      'openai_key_masked': instance.openAIKeyMasked,
      'gemini_key_masked': instance.geminiKeyMasked,
      'impersonating_actor': instance.impersonatingActor,
      'totp_enabled': instance.totpEnabled,
      'trial_bypass': instance.trialBypass,
      'is_email_verified': instance.isEmailVerified,
      'account_currency': instance.accountCurrency,
      'is_platform_account': instance.isPlatformAccount,
      'is_seller': instance.isSeller,
      'signature': instance.signature,
    };
