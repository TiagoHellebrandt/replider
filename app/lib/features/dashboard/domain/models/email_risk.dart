/// What the platform found wrong with an inbound message.
///
/// Present only when something WAS found: the clean case is never stored and
/// never sent, so an absent risk means nothing to say — not "we did not
/// check".
library;

/// Where a warning stops being "worth a look" and becomes "do not act on this
/// until you have confirmed by phone". Mirrors the server.
const int kRiskQuarantineThreshold = 50;

class EmailRiskSignal {
  const EmailRiskSignal({required this.kind, required this.detail});

  /// Stable server-side name: `auth_failed`, `lookalike_domain`,
  /// `new_domain`, `reply_to_mismatch`, `first_contact`, `auth_unknown`,
  /// `changed_payment_details`.
  final String kind;

  /// The evidence, in words, already written for a salesperson. Rendered
  /// verbatim: "similar to medline.com.br, which you already use" is something
  /// the reader confirms in ten seconds, and a score is not.
  final String detail;

  /// The one signal that describes money about to move to a new place.
  bool get isPaymentChange => kind == 'changed_payment_details';

  factory EmailRiskSignal.fromJson(Map<String, dynamic> json) =>
      EmailRiskSignal(
        kind: json['kind'] as String? ?? '',
        detail: (json['detail'] as String? ?? '').trim(),
      );
}

class EmailRisk {
  const EmailRisk({required this.score, required this.signals});

  final int score;
  final List<EmailRiskSignal> signals;

  bool get isQuarantined => score >= kRiskQuarantineThreshold;

  /// Whether the money-moving signal is among the findings. Drives the
  /// loudest state, because it is the last moment anyone can stop a transfer.
  bool get hasPaymentChange => signals.any((s) => s.isPaymentChange);

  static EmailRisk? fromJson(Map<String, dynamic>? json) {
    if (json == null) return null;
    final signals = (json['signals'] as List<dynamic>? ?? const <dynamic>[])
        .whereType<Map<String, dynamic>>()
        .map(EmailRiskSignal.fromJson)
        .where((s) => s.detail.isNotEmpty)
        .toList();
    if (signals.isEmpty) return null;
    return EmailRisk(
      score: (json['score'] as num?)?.toInt() ?? 0,
      signals: signals,
    );
  }
}
