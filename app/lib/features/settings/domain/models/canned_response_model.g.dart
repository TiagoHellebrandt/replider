// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'canned_response_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

CannedResponseModel _$CannedResponseModelFromJson(Map<String, dynamic> json) =>
    CannedResponseModel(
      id: json['id'] as String,
      accountId: json['account_id'] as String,
      shortcut: json['shortcut'] as String,
      title: json['title'] as String,
      content: json['content'] as String,
      category: json['category'] as String,
      visibility: json['visibility'] as String? ?? 'account',
      ownerUserId: json['owner_user_id'] as String?,
      createdAt: DateTime.parse(json['created_at'] as String),
      updatedAt: DateTime.parse(json['updated_at'] as String),
    );

Map<String, dynamic> _$CannedResponseModelToJson(
        CannedResponseModel instance) =>
    <String, dynamic>{
      'id': instance.id,
      'account_id': instance.accountId,
      'shortcut': instance.shortcut,
      'title': instance.title,
      'content': instance.content,
      'category': instance.category,
      'visibility': instance.visibility,
      'owner_user_id': instance.ownerUserId,
      'created_at': instance.createdAt.toIso8601String(),
      'updated_at': instance.updatedAt.toIso8601String(),
    };
