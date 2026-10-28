import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/network/dio_client.dart';

/// A recorded sale (§5, F1).
///
/// The one record three features read: the portfolio (what this customer buys
/// and when they will need it again), commissions (what the salesperson
/// earned) and contracts (what was agreed).
///
/// Gross, cost and tax are kept APART because a commission paid on gross is a
/// commission paid on money the company never had — in distribution the cost
/// of goods is most of the invoice, and 5% of gross can exceed the whole
/// margin.
class Sale {
  const Sale({
    required this.id,
    required this.contactId,
    required this.grossCents,
    this.costCents = 0,
    this.taxCents = 0,
    this.netCents = 0,
    this.currency = 'brl',
    this.cycleDays = 0,
    this.underwater = false,
    this.soldAt,
    this.nextPurchaseDue,
    this.items = const [],
  });

  final String id;
  final String contactId;
  final int grossCents;
  final int costCents;
  final int taxCents;

  /// What the company kept. Computed server-side so the client cannot
  /// disagree with the number a payout is calculated from.
  final int netCents;

  final String currency;

  /// How long until this customer is likely to need it again. Zero means
  /// nobody said, and the repurchase engine will never resurface it.
  final int cycleDays;

  /// Cost more than it brought in. Shown, never blocked: discounts, loss
  /// leaders and typos all look like this, and only a person can tell which.
  final bool underwater;

  final DateTime? soldAt;
  final DateTime? nextPurchaseDue;
  final List<SaleItem> items;

  factory Sale.fromJson(Map<String, dynamic> json) => Sale(
        id: json['id'] as String? ?? '',
        contactId: json['contact_id'] as String? ?? '',
        grossCents: (json['gross_cents'] as num?)?.toInt() ?? 0,
        costCents: (json['cost_cents'] as num?)?.toInt() ?? 0,
        taxCents: (json['tax_cents'] as num?)?.toInt() ?? 0,
        netCents: (json['net_cents'] as num?)?.toInt() ?? 0,
        currency: json['currency'] as String? ?? 'brl',
        cycleDays: (json['cycle_days'] as num?)?.toInt() ?? 0,
        underwater: json['underwater'] as bool? ?? false,
        soldAt: DateTime.tryParse(json['sold_at'] as String? ?? ''),
        nextPurchaseDue:
            DateTime.tryParse(json['next_purchase_due'] as String? ?? ''),
        items: ((json['items'] as List<dynamic>?) ?? const [])
            .whereType<Map<String, dynamic>>()
            .map(SaleItem.fromJson)
            .toList(),
      );
}

class SaleItem {
  const SaleItem({
    required this.description,
    this.quantity = 0,
    this.productId = '',
    this.unitCents = 0,
  });

  final String description;
  final int quantity;

  /// The catalogue product, when the line is one. This is what the reorder
  /// engine and the cross-sell key on — a free description is a fallback.
  final String productId;
  final int unitCents;

  factory SaleItem.fromJson(Map<String, dynamic> json) => SaleItem(
        description: json['description'] as String? ?? '',
        quantity: (json['quantity'] as num?)?.toInt() ?? 0,
        productId: json['product_id'] as String? ?? '',
        unitCents: (json['unit_cents'] as num?)?.toInt() ?? 0,
      );

  Map<String, dynamic> toJson() => <String, dynamic>{
        'description': description,
        'quantity': quantity,
        if (productId.isNotEmpty) 'product_id': productId,
        if (unitCents > 0) 'unit_cents': unitCents,
      };
}

final salesProvider = FutureProvider<List<Sale>>((ref) async {
  final res = await ref.watch(dioProvider).get<List<dynamic>>('/sales');
  return (res.data ?? <dynamic>[])
      .map((e) => Sale.fromJson(e as Map<String, dynamic>))
      .toList();
});

/// What a model proposes after reading a conversation.
///
/// Deliberately not a [Sale]: it is a suggestion awaiting confirmation, and
/// giving it the same type is how one gets recorded by accident.
class SaleProposal {
  const SaleProposal({
    required this.found,
    this.question = '',
    this.grossCents = 0,
    this.cycleDays = 0,
    this.items = const [],
  });

  /// False when the conversation holds no closed sale. An explicit "I did not
  /// find a sale here" beats a blank form.
  final bool found;

  /// The one sentence the person reads: "fechou 200 cx de luvas, ciclo ~30d?".
  /// Written by the model rather than assembled from fields, because the
  /// whole interaction IS this question.
  final String question;

  final int grossCents;
  final int cycleDays;
  final List<SaleItem> items;

  factory SaleProposal.fromJson(Map<String, dynamic> json) => SaleProposal(
        found: json['found'] as bool? ?? false,
        question: json['question'] as String? ?? '',
        grossCents: (json['gross_cents'] as num?)?.toInt() ?? 0,
        cycleDays: (json['cycle_days'] as num?)?.toInt() ?? 0,
        items: ((json['items'] as List<dynamic>?) ?? const [])
            .whereType<Map<String, dynamic>>()
            .map(SaleItem.fromJson)
            .toList(),
      );
}

final saleActionsProvider = Provider<SaleActions>(
  (ref) => SaleActions(ref.watch(dioProvider)),
);

class SaleActions {
  const SaleActions(this._dio);

  final Dio _dio;

  /// Records a sale. [contactId] or [leadId] — from a lead the server
  /// resolves the customer itself and marks the deal won; sending the lead's
  /// own id as the customer used to come back as CONTACT_NOT_FOUND.
  Future<Sale> register({
    String contactId = '',
    String leadId = '',
    required int grossCents,
    int costCents = 0,
    int taxCents = 0,
    int cycleDays = 0,
    DateTime? soldAt,
    List<SaleItem> items = const [],
    String notes = '',
    bool fromAI = false,
  }) async {
    final res =
        await _dio.post<Map<String, dynamic>>('/sales', data: <String, dynamic>{
      if (contactId.isNotEmpty) 'contact_id': contactId,
      if (leadId.isNotEmpty) 'lead_id': leadId,
      'gross_cents': grossCents,
      'cost_cents': costCents,
      'tax_cents': taxCents,
      'cycle_days': cycleDays,
      // Absent means "now" on the server; a backfilled sale carries its day.
      if (soldAt != null) 'sold_at': soldAt.toUtc().toIso8601String(),
      'items': items.map((i) => i.toJson()).toList(),
      'notes': notes,
      // Recorded as extracted AND confirmed by this person: the model
      // proposed it, they said yes, and both halves belong in the record.
      if (fromAI) 'source': 'ai_extracted',
    });
    return Sale.fromJson(res.data ?? const <String, dynamic>{});
  }

  /// Asks the server to read a conversation. Records nothing — the answer is
  /// a question to put in front of a person.
  Future<SaleProposal> extract(String conversationId) async {
    final res = await _dio.post<Map<String, dynamic>>(
        '/conversations/$conversationId/extract-sale');
    return SaleProposal.fromJson(res.data ?? const <String, dynamic>{});
  }
}
