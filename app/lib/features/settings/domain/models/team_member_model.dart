import 'package:json_annotation/json_annotation.dart';

part 'team_member_model.g.dart';

@JsonSerializable(fieldRename: FieldRename.snake)

/// A user within the account's team.
/// Has a role (admin or attendant) and can be active or deactivated.
class TeamMemberModel {
  final String id;
  final String name;
  final String email;
  final String role;
  final bool isActive;
  @JsonKey(defaultValue: false)
  final bool isEmailVerified;

  /// When the admin invited (or re-invited) this member. Paired with
  /// isEmailVerified=false to render a "Pending" badge + Resend button.
  final String? inviteSentAt;

  /// Last authenticated request from this user. Null until first login.
  final String? lastSeenAt;
  final String createdAt;
  final String updatedAt;

  const TeamMemberModel({
    required this.id,
    required this.name,
    required this.email,
    required this.role,
    required this.isActive,
    this.isEmailVerified = false,
    this.inviteSentAt,
    this.lastSeenAt,
    required this.createdAt,
    required this.updatedAt,
  });

  /// True when the user was invited but hasn't completed signup yet.
  /// Requires a non-null inviteSentAt so the account owner — who is
  /// created unverified at signup but never "invited" — is NOT shown
  /// as Pending. Only members an admin actually invited carry the badge.
  bool get isPendingInvite => !isEmailVerified && inviteSentAt != null;

  factory TeamMemberModel.fromJson(Map<String, dynamic> json) =>
      _$TeamMemberModelFromJson(json);
  Map<String, dynamic> toJson() => _$TeamMemberModelToJson(this);
}
