// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'team_member_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

TeamMemberModel _$TeamMemberModelFromJson(Map<String, dynamic> json) =>
    TeamMemberModel(
      id: json['id'] as String,
      name: json['name'] as String,
      email: json['email'] as String,
      role: json['role'] as String,
      isActive: json['is_active'] as bool,
      isEmailVerified: json['is_email_verified'] as bool? ?? false,
      inviteSentAt: json['invite_sent_at'] as String?,
      lastSeenAt: json['last_seen_at'] as String?,
      createdAt: json['created_at'] as String,
      updatedAt: json['updated_at'] as String,
    );

Map<String, dynamic> _$TeamMemberModelToJson(TeamMemberModel instance) =>
    <String, dynamic>{
      'id': instance.id,
      'name': instance.name,
      'email': instance.email,
      'role': instance.role,
      'is_active': instance.isActive,
      'is_email_verified': instance.isEmailVerified,
      'invite_sent_at': instance.inviteSentAt,
      'last_seen_at': instance.lastSeenAt,
      'created_at': instance.createdAt,
      'updated_at': instance.updatedAt,
    };
