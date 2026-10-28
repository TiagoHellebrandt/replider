import 'package:json_annotation/json_annotation.dart';

part 'note_model.g.dart';

@JsonSerializable(fieldRename: FieldRename.snake)

/// An internal note attached to a conversation by an agent.
/// Not visible to the customer. Used for handoff context.
class NoteModel {
  final String id;
  final String conversationId;
  final String userId;
  final String userName;
  final String content;
  final DateTime createdAt;

  NoteModel({
    required this.id,
    required this.conversationId,
    required this.userId,
    required this.userName,
    required this.content,
    required this.createdAt,
  });

  factory NoteModel.fromJson(Map<String, dynamic> json) =>
      _$NoteModelFromJson(json);
  Map<String, dynamic> toJson() => _$NoteModelToJson(this);
}
