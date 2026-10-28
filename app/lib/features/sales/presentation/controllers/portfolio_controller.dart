import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/features/sales/presentation/controllers/sales_record_controller.dart';

/// One customer as the portfolio sees them (§6.5, F2).
///
/// Consumables are recurring revenue: a customer whose repurchase works does
/// not churn, and one who has to remember to reorder eventually remembers a
/// competitor instead.
class PortfolioCustomer {
  const PortfolioCustomer({
    required this.contactId,
    required this.contactName,
    this.purchases = 0,
    this.grossCents = 0,
    this.netCents = 0,
    this.daysUntilDue,
    this.daysSinceContact,
    this.urgency = 0,
    this.lastPurchaseAt,
    this.nextDueAt,
  });

  final String contactId;
  final String contactName;
  final int purchases;
  final int grossCents;

  /// What the company kept. Leads over gross in the UI: what a customer is
  /// worth is not what they were invoiced.
  final int netCents;

  /// NEGATIVE when they have already run out — deliberately not clamped,
  /// because "acabou há 12 dias" is the most actionable thing on the screen
  /// and zero would hide it. Null when no sale carried a cycle.
  final int? daysUntilDue;

  /// The relationship's pulse. Null when nobody ever spoke to them, which is
  /// a different thing from "spoke long ago".
  final int? daysSinceContact;

  final int urgency;
  final DateTime? lastPurchaseAt;
  final DateTime? nextDueAt;

  /// Already out of stock. The most expensive state in the model: somebody
  /// is selling to them today.
  bool get overdue => daysUntilDue != null && daysUntilDue! < 0;

  /// Inside the window where an approach still arrives before they run out.
  bool get dueSoon =>
      daysUntilDue != null && daysUntilDue! >= 0 && daysUntilDue! <= 7;

  /// Silence long enough to be a risk on its own.
  bool get stale => daysSinceContact != null && daysSinceContact! >= 60;

  factory PortfolioCustomer.fromJson(Map<String, dynamic> json) =>
      PortfolioCustomer(
        contactId: json['contact_id'] as String? ?? '',
        contactName: json['contact_name'] as String? ?? '',
        purchases: (json['purchases'] as num?)?.toInt() ?? 0,
        grossCents: (json['gross_cents'] as num?)?.toInt() ?? 0,
        netCents: (json['net_cents'] as num?)?.toInt() ?? 0,
        daysUntilDue: (json['days_until_due'] as num?)?.toInt(),
        daysSinceContact: (json['days_since_contact'] as num?)?.toInt(),
        urgency: (json['urgency'] as num?)?.toInt() ?? 0,
        lastPurchaseAt:
            DateTime.tryParse(json['last_purchase_at'] as String? ?? ''),
        nextDueAt: DateTime.tryParse(json['next_due_at'] as String? ?? ''),
      );
}

final portfolioProvider = FutureProvider<List<PortfolioCustomer>>((ref) async {
  // The API's hard cap. Since this is the customer list (not only the
  // buyers), the default page of 50 would hide most of an imported base; a
  // search box over 200 rows beats a paginator for the accounts we have.
  final res = await ref.watch(dioProvider).get<List<dynamic>>('/portfolio',
      queryParameters: <String, dynamic>{'limit': 200});
  return (res.data ?? <dynamic>[])
      .map((e) => PortfolioCustomer.fromJson(e as Map<String, dynamic>))
      .toList();
});

/// One customer plus everything they bought.
class PortfolioDetail {
  const PortfolioDetail({required this.customer, this.sales = const []});

  final PortfolioCustomer customer;
  final List<Sale> sales;

  factory PortfolioDetail.fromJson(Map<String, dynamic> json) =>
      PortfolioDetail(
        customer: PortfolioCustomer.fromJson(json),
        sales: ((json['sales'] as List<dynamic>?) ?? const [])
            .whereType<Map<String, dynamic>>()
            .map(Sale.fromJson)
            .toList(),
      );
}

final portfolioDetailProvider =
    FutureProvider.autoDispose.family<PortfolioDetail, String>(
  (ref, contactId) async {
    final res = await ref
        .watch(dioProvider)
        .get<Map<String, dynamic>>('/portfolio/$contactId');
    return PortfolioDetail.fromJson(res.data ?? const <String, dynamic>{});
  },
);
