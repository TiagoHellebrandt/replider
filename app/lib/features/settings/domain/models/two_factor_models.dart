import 'package:json_annotation/json_annotation.dart';

part 'two_factor_models.g.dart';

@JsonSerializable(fieldRename: FieldRename.snake, createToJson: false)
class TwoFactorStatus {
  final bool enabled;

  TwoFactorStatus({required this.enabled});

  factory TwoFactorStatus.fromJson(Map<String, dynamic> json) =>
      _$TwoFactorStatusFromJson(json);
}

/// Returned by Setup. The QR is rendered from [otpauthUrl]; the
/// raw [secret] is shown for manual entry as a fallback.
@JsonSerializable(fieldRename: FieldRename.snake, createToJson: false)
class TwoFactorSetupResult {
  final String secret;
  final String otpauthUrl;

  TwoFactorSetupResult({required this.secret, required this.otpauthUrl});

  factory TwoFactorSetupResult.fromJson(Map<String, dynamic> json) =>
      _$TwoFactorSetupResultFromJson(json);
}

/// Plaintext recovery codes — shown to the user once.
@JsonSerializable(fieldRename: FieldRename.snake, createToJson: false)
class TwoFactorVerifyResult {
  final List<String> recoveryCodes;

  TwoFactorVerifyResult({required this.recoveryCodes});

  factory TwoFactorVerifyResult.fromJson(Map<String, dynamic> json) =>
      _$TwoFactorVerifyResultFromJson(json);
}
