/// What was actually said to this customer, and what they said back.
///
/// A vendedor could not read this: the inbox is attendant-gated and a vendedor
/// deliberately does not satisfy attendant, so the person who sent a proposal had
/// no way to see the reply. This is not the inbox — it is one lead's history,
/// behind that lead's own ownership rules.
class TimelineMessage {
  const TimelineMessage({
    required this.id,
    required this.author,
    required this.text,
    this.authorName = '',
    this.kind = 'text',
    this.channelType = '',
    this.sentAt,
    this.failedAt,
    this.failedReason = '',
  });

  final String id;
  final TimelineAuthor author;

  /// The teammate who wrote it. Empty for the customer and for the bot: the
  /// customer's name is on the card above, and "bot" is not a person.
  final String authorName;

  /// Empty for a media-only message, which reads as its [kind] rather than as a
  /// blank line.
  final String text;
  final String kind;
  final String channelType;
  final DateTime? sentAt;

  /// Stamped when the channel adapter could not deliver this message. A failed
  /// approach must not read like one that reached the customer — the timeline
  /// is the vendedor's only view of the exchange.
  final DateTime? failedAt;
  final String failedReason;

  bool get isFromCustomer => author == TimelineAuthor.customer;
  bool get failed => failedAt != null;

  factory TimelineMessage.fromJson(Map<String, dynamic> json) =>
      TimelineMessage(
        id: json['id'] as String? ?? '',
        author: TimelineAuthor.from(json['author'] as String?),
        authorName: json['author_name'] as String? ?? '',
        text: json['text'] as String? ?? '',
        kind: json['kind'] as String? ?? 'text',
        channelType: json['channel_type'] as String? ?? '',
        sentAt: _date(json['sent_at']),
        failedAt: _date(json['failed_at']),
        failedReason: json['failed_reason'] as String? ?? '',
      );

  static DateTime? _date(dynamic v) {
    if (v is! String || v.trim().isEmpty) return null;
    return DateTime.tryParse(v)?.toLocal();
  }
}

/// Who said a line, in the only three categories that change how a salesperson
/// reads it. "We answered" and "the bot answered" are different facts about the
/// account, so they are not collapsed into one.
enum TimelineAuthor {
  customer,
  bot,
  team;

  static TimelineAuthor from(String? raw) => switch (raw) {
        'customer' => TimelineAuthor.customer,
        'team' => TimelineAuthor.team,
        // Anything unrecognised reads as the bot rather than as a teammate:
        // claiming a person wrote something they did not is the worse mistake.
        _ => TimelineAuthor.bot,
      };
}

class LeadTimeline {
  const LeadTimeline({required this.messages, this.truncated = false});

  final List<TimelineMessage> messages;

  /// The cap cut the history. Shown rather than implied: a list that silently
  /// starts mid-conversation reads as the beginning of the relationship.
  final bool truncated;

  factory LeadTimeline.fromJson(Map<String, dynamic> json) => LeadTimeline(
        messages: (json['messages'] as List<dynamic>? ?? const <dynamic>[])
            .whereType<Map<String, dynamic>>()
            .map(TimelineMessage.fromJson)
            .toList(),
        truncated: json['truncated'] == true,
      );
}
