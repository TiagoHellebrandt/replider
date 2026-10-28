import 'package:json_annotation/json_annotation.dart';

part 'tag_model.g.dart';

@JsonSerializable(fieldRename: FieldRename.snake)

/// A colored label for organizing conversations.
/// Can be applied to conversations for filtering and reporting.
class TagModel {
  final String id;
  final String accountId;
  final String name;
  final String color;
  final DateTime createdAt;

  TagModel({
    required this.id,
    required this.accountId,
    required this.name,
    required this.color,
    required this.createdAt,
  });

  factory TagModel.fromJson(Map<String, dynamic> json) =>
      _$TagModelFromJson(json);

  Map<String, dynamic> toJson() => _$TagModelToJson(this);
}
