/// What the receiving mail server concluded about who sent a message.
///
/// Three states, never two. "Nobody checked" is a different statement from
/// both "verificado" and "forjado", and collapsing it into either is the bug
/// this exists to remove: before it, an unchecked sender was indistinguishable
/// from a verified one, and anyone could put a clinic's address in the From
/// line and land in the funnel as that lead's reply.
library;

enum SenderAuthenticity {
  /// SPF, DKIM or DMARC passed. The message really came from the domain it
  /// claims.
  verified,

  /// Nobody checked — an older provider, or a payload with no verdict. NOT
  /// evidence of anything, in either direction.
  unknown,

  /// Something was asserted and disproved. This is the shape a forged sender
  /// takes, and it is the one worth interrupting someone over.
  failed;

  /// Reads the verdict the API stamped onto the message.
  ///
  /// Absent or unreadable metadata is [unknown], never [verified]: a missing
  /// verdict must never render as a checkmark.
  static SenderAuthenticity fromMetadata(Map<String, dynamic>? metadata) {
    final auth = metadata?['email_auth'];
    if (auth is! Map) return SenderAuthenticity.unknown;
    if (auth['suspicious'] == true) return SenderAuthenticity.failed;
    if (auth['trustworthy'] == true) return SenderAuthenticity.verified;
    return SenderAuthenticity.unknown;
  }

  /// Whether this is worth showing at all.
  ///
  /// A verified sender gets no badge on purpose: marking the normal case
  /// turns the badge into wallpaper, and then the one that matters — a
  /// failure — reads as more of the same.
  bool get isWorthShowing => this != SenderAuthenticity.verified;
}

/// The individual checks, for the explanation. Kept raw because "spf=fail,
/// dkim=none" is what someone forwards to their IT person, and a summary is
/// not.
class SenderAuthDetail {
  const SenderAuthDetail({
    required this.spf,
    required this.dkim,
    required this.dmarc,
  });

  final String spf;
  final String dkim;
  final String dmarc;

  static SenderAuthDetail? fromMetadata(Map<String, dynamic>? metadata) {
    final auth = metadata?['email_auth'];
    if (auth is! Map) return null;
    return SenderAuthDetail(
      spf: auth['spf'] as String? ?? 'unknown',
      dkim: auth['dkim'] as String? ?? 'unknown',
      dmarc: auth['dmarc'] as String? ?? 'unknown',
    );
  }

  @override
  String toString() => 'SPF: $spf · DKIM: $dkim · DMARC: $dmarc';
}
