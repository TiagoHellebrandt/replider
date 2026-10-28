// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'two_factor_models.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

TwoFactorStatus _$TwoFactorStatusFromJson(Map<String, dynamic> json) =>
    TwoFactorStatus(
      enabled: json['enabled'] as bool,
    );

TwoFactorSetupResult _$TwoFactorSetupResultFromJson(
        Map<String, dynamic> json) =>
    TwoFactorSetupResult(
      secret: json['secret'] as String,
      otpauthUrl: json['otpauth_url'] as String,
    );

TwoFactorVerifyResult _$TwoFactorVerifyResultFromJson(
        Map<String, dynamic> json) =>
    TwoFactorVerifyResult(
      recoveryCodes: (json['recovery_codes'] as List<dynamic>)
          .map((e) => e as String)
          .toList(),
    );
