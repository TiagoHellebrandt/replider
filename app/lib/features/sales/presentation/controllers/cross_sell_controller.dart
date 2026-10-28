import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/network/dio_client.dart';

/// Cross-sell (§6.5, F5).
///
/// "Whoever buys gloves buys masks" is a fact sitting unused in every
/// distributor's own sales table. The point is not a recommendation engine —
/// it is one line on the screen when a salesperson opens a customer who buys
/// gloves and has never bought masks.
class CrossSellSuggestion {
  const CrossSellSuggestion({
    required this.suggest,
    required this.because,
    this.confidence = 0,
    this.customers = 0,
  });

  /// What they have NOT bought.
  final String suggest;

  /// What they DO buy, so the salesperson can say why. A suggestion with no
  /// reason is a guess the customer can hear.
  final String because;

  /// The observed rate among this account's own customers.
  final double confidence;
  final int customers;

  factory CrossSellSuggestion.fromJson(Map<String, dynamic> json) =>
      CrossSellSuggestion(
        suggest: json['suggest'] as String? ?? '',
        because: json['because'] as String? ?? '',
        confidence: (json['confidence'] as num?)?.toDouble() ?? 0,
        customers: (json['customers'] as num?)?.toInt() ?? 0,
      );
}

class CrossSellResult {
  const CrossSellResult({this.suggestions = const [], this.note});

  final List<CrossSellSuggestion> suggestions;

  /// Why there is nothing to show. An empty panel with no explanation reads
  /// as broken; "there is not enough of a pattern yet" reads as honest.
  final String? note;

  factory CrossSellResult.fromJson(Map<String, dynamic> json) =>
      CrossSellResult(
        suggestions: ((json['suggestions'] as List<dynamic>?) ?? const [])
            .whereType<Map<String, dynamic>>()
            .map(CrossSellSuggestion.fromJson)
            .toList(),
      );
}

final crossSellProvider =
    FutureProvider.autoDispose.family<CrossSellResult, String>(
  (ref, contactId) async {
    final res = await ref
        .watch(dioProvider)
        .get<Map<String, dynamic>>('/portfolio/$contactId/cross-sell');
    return CrossSellResult.fromJson(res.data ?? const <String, dynamic>{});
  },
);
