/// A priced offer made to one customer at one moment.
///
/// Mirrors the API's `proposalResponse`. Two fields deserve their own note
/// because getting either wrong misstates money:
///
///  * [currency] travels with every amount. A client that renders cents without
///    it is right until the first account that sells in the other one.
///  * [totalCents] comes from the server, not from summing [items] here. Two
///    implementations of the same arithmetic disagree the first time one of them
///    changes; the server owns the number that will be sent.
class Proposal {
  const Proposal({
    required this.id,
    required this.leadId,
    required this.status,
    required this.currency,
    required this.body,
    required this.items,
    required this.totalCents,
    required this.rendered,
    this.sentAt,
  });

  final String id;
  final String leadId;
  final String status;
  final String currency;

  /// The salesperson's own words. The priced lines are appended by the server.
  final String body;
  final List<ProposalItem> items;
  final int totalCents;

  /// Exactly what the approval queue will show. Rendered server-side so the
  /// review screen never reproduces the formatting and disagrees with it.
  final String rendered;
  final DateTime? sentAt;

  /// A sent offer is a statement the customer is holding; editing it would make
  /// our record disagree with their copy. The server refuses too — this is so
  /// the UI does not offer the action in the first place.
  bool get editable => status == 'draft';

  factory Proposal.fromJson(Map<String, dynamic> json) => Proposal(
        id: json['id'] as String? ?? '',
        leadId: json['lead_id'] as String? ?? '',
        status: json['status'] as String? ?? 'draft',
        currency: json['currency'] as String? ?? '',
        body: json['body'] as String? ?? '',
        items: (json['items'] as List<dynamic>? ?? const <dynamic>[])
            .whereType<Map<String, dynamic>>()
            .map(ProposalItem.fromJson)
            .toList(),
        totalCents: (json['total_cents'] as num?)?.toInt() ?? 0,
        rendered: json['rendered'] as String? ?? '',
        sentAt: _date(json['sent_at']),
      );

  /// An unparseable date is absent, not epoch: a proposal claiming it was sent
  /// in 1970 is worse than one that says nothing about when.
  static DateTime? _date(dynamic v) {
    if (v is! String || v.trim().isEmpty) return null;
    return DateTime.tryParse(v)?.toLocal();
  }
}

/// One line: a product, how many, and what it cost at the moment of the offer.
class ProposalItem {
  const ProposalItem({
    required this.productId,
    required this.name,
    required this.quantity,
    required this.unitPriceCents,
    required this.totalCents,
  });

  /// Empty when the product has since been deleted. The line survives it — the
  /// name and price are snapshots, so the record stays readable.
  final String productId;
  final String name;
  final int quantity;
  final int unitPriceCents;
  final int totalCents;

  factory ProposalItem.fromJson(Map<String, dynamic> json) => ProposalItem(
        productId: json['product_id'] as String? ?? '',
        name: json['name'] as String? ?? '',
        quantity: (json['quantity'] as num?)?.toInt() ?? 0,
        unitPriceCents: (json['unit_price_cents'] as num?)?.toInt() ?? 0,
        totalCents: (json['total_cents'] as num?)?.toInt() ?? 0,
      );
}

/// One requested line, before the server prices it.
class ProposalLine {
  const ProposalLine({required this.productId, required this.quantity});

  final String productId;
  final int quantity;

  Map<String, dynamic> toJson() =>
      <String, dynamic>{'product_id': productId, 'quantity': quantity};
}
