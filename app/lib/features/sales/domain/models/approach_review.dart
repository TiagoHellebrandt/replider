/// What the platform can say about an outbound message before a human sends
/// it, in two layers that catch different things.
///
/// The PATTERN pass ([ApproachFinding]) is regexes: free, instant, certain
/// about the phrasings it knows, blind to everything else. It rides along with
/// the approval queue, so every card carries its verdict unasked.
///
/// The AI pass ([ApproachReview]) is a second model reading the message
/// against the facts the company supplied — it catches what no pattern list
/// can: a claim that is fluent, novel, and false. It costs a model call, so it
/// runs on demand.
///
/// Neither blocks. They report; the person who answers for the message
/// decides.
library;

/// One problem found in a message.
class ApproachFinding {
  const ApproachFinding({required this.kind, required this.quote, this.why});

  /// The server's stable label: `promise` (a deadline, result or capability
  /// committed in the sender's name), `invented_fact` (asserted about the lead
  /// with nothing behind it), `out_of_scope`, `placeholder`, `tone`.
  ///
  /// The distinction is the point: telling someone "você prometeu" when the
  /// real problem is "you asserted something you do not know" sends them to fix the
  /// wrong sentence.
  final String kind;

  /// The offending fragment, verbatim, so the reader finds it without
  /// re-reading the whole message.
  final String quote;

  /// The consequence, not the rule — only the AI pass fills this in.
  final String? why;

  factory ApproachFinding.fromJson(Map<String, dynamic> json) =>
      ApproachFinding(
        kind: json['kind'] as String? ?? 'promise',
        quote: (json['quote'] as String? ?? '').trim(),
        why: _nonEmpty(json['why']),
      );
}

/// The verdict handed to whoever is about to approve.
class ApproachReview {
  const ApproachReview({
    required this.safe,
    required this.checked,
    required this.risks,
  });

  /// False when anything was found. NOT a permission gate — blocking on a
  /// model's opinion would move the decision away from the person who answers
  /// for the message.
  final bool safe;

  /// Whether the AI half actually ran. False means only the pattern pass
  /// happened, so a clean result reads "no known pattern", never "reviewed
  /// and approved". Saying the stronger thing when only the weaker one
  /// happened is the exact failure this product exists to prevent.
  final bool checked;

  final List<ApproachFinding> risks;

  factory ApproachReview.fromJson(Map<String, dynamic> json) => ApproachReview(
        safe: json['safe'] as bool? ?? false,
        checked: json['checked'] as bool? ?? false,
        risks: (json['risks'] as List<dynamic>? ?? const <dynamic>[])
            .whereType<Map<String, dynamic>>()
            .map(ApproachFinding.fromJson)
            .where((r) => r.quote.isNotEmpty)
            .toList(),
      );
}

String? _nonEmpty(dynamic v) {
  if (v is String && v.trim().isNotEmpty) return v.trim();
  return null;
}
