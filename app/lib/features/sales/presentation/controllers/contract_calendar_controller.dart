import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/network/dio_client.dart';

/// One buying window: a public commitment about to run out.
///
/// The instrument nobody in this market watches. A public contract carries a
/// published end date, so the purchase that replaces it is a known event three
/// to six months before any tender exists — and by the time an edital is
/// published, the specification has usually been written around whoever
/// supplied the reference quote. This is the list that lets a seller arrive
/// while the spec is still being written.
class ContractWindow {
  const ContractWindow({
    required this.pncpId,
    required this.buyerName,
    required this.object,
    this.url = '',
    this.buyerCnpj = '',
    this.supplierName = '',
    this.valueCents = 0,
    this.city = '',
    this.uf = '',
    this.expiresAt,
    this.daysLeft,
    this.priceRegistry = false,
    this.ridesOthers = false,
  });

  final String pncpId;

  /// The published record. Every claim built on this row can be checked by the
  /// person who receives it — which is the whole reason to cite it.
  final String url;

  final String buyerName;
  final String buyerCnpj;

  /// Who holds it today. Empty when the record names nobody, which happens.
  final String supplierName;

  final String object;
  final int valueCents;
  final String city;
  final String uf;

  final DateTime? expiresAt;

  /// Negative once the term has run out. A window that closed last week is not
  /// history: the buyer is very probably mid-process right now.
  final int? daysLeft;

  /// Bought under an ata de registro de preços — a twelve-month licence to
  /// sell at a registered price, not a single closed purchase.
  final bool priceRegistry;

  /// This buyer rides other bodies' registries (PNCP's `frutoAdesao`). The
  /// most actionable bit in the record: they can buy from you without running
  /// a tender at all.
  final bool ridesOthers;

  /// Under 60 days the window is a call to make this week.
  bool get isNear => daysLeft != null && daysLeft! >= 0 && daysLeft! <= 60;

  /// Already closed — still worth a call, lower in the list.
  bool get hasClosed => daysLeft != null && daysLeft! < 0;

  String get location {
    if (city.isEmpty) return uf;
    if (uf.isEmpty) return city;
    return '$city/$uf';
  }

  factory ContractWindow.fromJson(Map<String, dynamic> json) => ContractWindow(
        pncpId: json['pncp_id'] as String? ?? '',
        url: json['url'] as String? ?? '',
        buyerName: json['buyer_name'] as String? ?? '',
        buyerCnpj: json['buyer_cnpj'] as String? ?? '',
        supplierName: json['supplier_name'] as String? ?? '',
        object: json['object'] as String? ?? '',
        valueCents: (json['value_cents'] as num?)?.toInt() ?? 0,
        city: json['city'] as String? ?? '',
        uf: json['uf'] as String? ?? '',
        expiresAt: json['expires_at'] == null
            ? null
            : DateTime.tryParse(json['expires_at'] as String)?.toLocal(),
        daysLeft: (json['days_left'] as num?)?.toInt(),
        priceRegistry: json['price_registry'] as bool? ?? false,
        ridesOthers: json['rides_others'] as bool? ?? false,
      );
}

/// The calendar plus what it was narrowed by.
///
/// The narrowing travels so an empty list can explain itself. "Nothing expires
/// in your state in the next six months" and "you have not told us what you
/// sell" are different answers, and a screen that renders both as an empty
/// list teaches people the feature is broken.
class ContractCalendar {
  const ContractCalendar({
    this.entries = const [],
    this.windowDays = 180,
    this.states = const [],
    this.keywords = const [],
    this.scanning = false,
  });

  final List<ContractWindow> entries;
  final int windowDays;
  final List<String> states;
  final List<String> keywords;

  /// True while the sweep has not covered this territory yet. Day-one empty
  /// means "still reading", not "nothing found".
  final bool scanning;

  bool get hasTerritory => states.isNotEmpty;
  bool get hasKeywords => keywords.isNotEmpty;

  factory ContractCalendar.fromJson(Map<String, dynamic> json) =>
      ContractCalendar(
        entries: (json['entries'] as List<dynamic>? ?? const <dynamic>[])
            .whereType<Map<String, dynamic>>()
            .map(ContractWindow.fromJson)
            .toList(),
        windowDays: (json['window_days'] as num?)?.toInt() ?? 180,
        states: (json['states'] as List<dynamic>? ?? const <dynamic>[])
            .map((e) => e.toString())
            .toList(),
        keywords: (json['keywords'] as List<dynamic>? ?? const <dynamic>[])
            .map((e) => e.toString())
            .toList(),
        scanning: json['scanning'] as bool? ?? false,
      );
}

final contractCalendarProvider =
    FutureProvider.autoDispose<ContractCalendar>((ref) async {
  final res = await ref
      .watch(dioProvider)
      .get<Map<String, dynamic>>('/contract-calendar');
  return ContractCalendar.fromJson(res.data ?? const <String, dynamic>{});
});

final calendarActionsProvider = Provider<CalendarActions>(CalendarActions.new);

class CalendarActions {
  const CalendarActions(this._ref);

  final Ref _ref;

  /// Puts one expiring commitment into the funnel.
  ///
  /// Nothing is drafted and nothing is sent. A public body published on PNCP
  /// has no e-mail and no phone, so the card arrives asking for the one thing
  /// it needs — the buyer's contact — which is exactly the work that has to
  /// happen in October for a contract that ends in January.
  ///
  /// Idempotent: clicking twice returns the same card rather than minting a
  /// second conversation with the same hospital.
  Future<void> work({required String pncpId}) async {
    await _ref.read(dioProvider).post<Map<String, dynamic>>(
          '/contract-calendar/${Uri.encodeComponent(pncpId)}/work',
        );
  }
}
