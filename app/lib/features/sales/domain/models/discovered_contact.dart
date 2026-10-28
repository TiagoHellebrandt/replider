/// Somebody the customer already emails, learned from her own Sent folder.
///
/// A CANDIDATE, never a contact: nothing here is in the CRM until she picks it.
/// The screen's whole job is to make that choice easy and honest.
class DiscoveredContact {
  const DiscoveredContact({
    required this.address,
    this.name,
    this.sentCount = 0,
    this.firstSentAt,
    this.lastSentAt,
  });

  final String address;
  final String? name;

  /// How many messages she has sent them. The ranking signal, and the one
  /// number that separates a customer from a stranger who asked a question
  /// once — so it is the first thing the row shows.
  final int sentCount;
  final DateTime? firstSentAt;
  final DateTime? lastSentAt;

  /// What a human recognises. Falls back to the address, because that is the
  /// identity and an empty row would be unreadable.
  String get label {
    final n = name?.trim();
    return (n == null || n.isEmpty) ? address : n;
  }

  factory DiscoveredContact.fromJson(Map<String, dynamic> json) =>
      DiscoveredContact(
        address: (json['address'] as String? ?? '').trim(),
        name: _str(json['name']),
        sentCount: (json['sent_count'] as num?)?.toInt() ?? 0,
        firstSentAt: _date(json['first_sent_at']),
        lastSentAt: _date(json['last_sent_at']),
      );

  static String? _str(dynamic v) {
    if (v is String && v.trim().isNotEmpty) return v.trim();
    return null;
  }

  /// An unparseable date is absent, not epoch: a row claiming contact since
  /// 1970 is worse than one that says nothing about when.
  static DateTime? _date(dynamic v) {
    if (v is! String || v.trim().isEmpty) return null;
    return DateTime.tryParse(v)?.toLocal();
  }
}

/// One page of candidates plus how many are waiting behind it.
class DiscoveredContactsPage {
  const DiscoveredContactsPage({required this.items, required this.total});

  final List<DiscoveredContact> items;

  /// Counted server-side over the whole pending set. Never derived from
  /// [items]: the page is capped, and a total computed from what arrived
  /// shrinks silently as the list grows.
  final int total;

  factory DiscoveredContactsPage.fromJson(Map<String, dynamic> json) =>
      DiscoveredContactsPage(
        items: (json['items'] as List<dynamic>? ?? const <dynamic>[])
            .whereType<Map<String, dynamic>>()
            .map(DiscoveredContact.fromJson)
            .where((c) => c.address.isNotEmpty)
            .toList(),
        total: (json['total'] as num?)?.toInt() ?? 0,
      );
}

/// What one scan of the Sent folder found.
class SentScanResult {
  const SentScanResult({
    required this.mailbox,
    required this.scanned,
    required this.found,
    required this.skipped,
    required this.complete,
  });

  /// The folder that was actually read, echoed so a customer whose Sent folder
  /// has an unusual name can see we found the right one.
  final String mailbox;

  /// Messages read and DISTINCT people found. Both, because "read 4000, found
  /// 62" is the sentence that explains why a long history yields a short list.
  final int scanned;
  final int found;

  /// Addresses left out as automated or as her own. Shown rather than hidden:
  /// a number that does not add up is a question.
  final int skipped;

  /// False when the cap stopped the walk, which the screen says out loud
  /// instead of implying the list is everybody she has ever written to.
  final bool complete;

  factory SentScanResult.fromJson(Map<String, dynamic> json) => SentScanResult(
        mailbox: (json['mailbox'] as String? ?? '').trim(),
        scanned: (json['scanned'] as num?)?.toInt() ?? 0,
        found: (json['found'] as num?)?.toInt() ?? 0,
        skipped: (json['skipped'] as num?)?.toInt() ?? 0,
        complete: json['complete'] == true,
      );
}

/// What a confirmed import produced.
class ImportContactsResult {
  const ImportContactsResult({required this.imported, required this.skipped});

  final int imported;

  /// Selections that were already in the CRM, or that named an address this
  /// account never discovered. Reported so a count that does not match the
  /// selection has an explanation.
  final int skipped;

  factory ImportContactsResult.fromJson(Map<String, dynamic> json) =>
      ImportContactsResult(
        imported: (json['imported'] as num?)?.toInt() ?? 0,
        skipped: (json['skipped'] as num?)?.toInt() ?? 0,
      );
}
