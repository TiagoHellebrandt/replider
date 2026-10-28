import 'package:json_annotation/json_annotation.dart';

part 'canned_response_model.g.dart';

@JsonSerializable(fieldRename: FieldRename.snake)

/// A pre-written response template for quick agent replies.
/// Searchable by shortcut key or content.
class CannedResponseModel {
  final String id;
  final String accountId;
  final String shortcut;
  final String title;
  final String content;
  final String category;

  /// 'account' (team-wide, admin-managed) or 'personal' (owner-only,
  /// US-018). Defaults to 'account' so cached/legacy payloads keep
  /// today's behaviour.
  @JsonKey(defaultValue: 'account')
  final String visibility;

  /// Set only for personal responses — the owning user's id.
  final String? ownerUserId;

  final DateTime createdAt;
  final DateTime updatedAt;

  bool get isPersonal => visibility == 'personal';

  CannedResponseModel({
    required this.id,
    required this.accountId,
    required this.shortcut,
    required this.title,
    required this.content,
    required this.category,
    this.visibility = 'account',
    this.ownerUserId,
    required this.createdAt,
    required this.updatedAt,
  });

  factory CannedResponseModel.fromJson(Map<String, dynamic> json) =>
      _$CannedResponseModelFromJson(json);

  Map<String, dynamic> toJson() => _$CannedResponseModelToJson(this);
}
