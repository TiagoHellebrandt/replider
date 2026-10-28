import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/network/dio_client.dart';

/// Public buying signals (§2.1).
///
/// A hospital that publishes a tender has said in writing that it intends to
/// buy a specific thing, by a specific date, with a budget. That is a
/// stronger intent signal than anything an enrichment vendor sells — and it
/// is public, free, and nobody in this market uses it.
class BuyingSignal {
  const BuyingSignal({
    required this.id,
    required this.title,
    required this.organization,
    this.source = 'pncp',
    this.description = '',
    this.url = '',
    this.city = '',
    this.state = '',
    this.valueCents = 0,
    this.daysLeft,
  });

  final String id;
  final String title;
  final String organization;
  final String source;
  final String description;
  final String url;
  final String city;
  final String state;

  /// Zero means the notice did not disclose a value, which is common in
  /// public tenders and is NOT the same as R$ 0,00.
  final int valueCents;

  /// Null when there is no published deadline. Three days is a reason to
  /// write today; thirty is a reason to prepare properly — that difference
  /// is the whole value of the signal.
  final int? daysLeft;

  /// Under three days the card changes colour. Not decoration: a tender is
  /// worth acting on today and worthless next week.
  bool get isUrgent => daysLeft != null && daysLeft! <= 3;

  String get location {
    if (city.isEmpty) return state;
    if (state.isEmpty) return city;
    return '$city/$state';
  }

  factory BuyingSignal.fromJson(Map<String, dynamic> json) => BuyingSignal(
        id: json['id'] as String? ?? '',
        title: json['title'] as String? ?? '',
        organization: json['organization'] as String? ?? '',
        source: json['source'] as String? ?? 'pncp',
        description: json['description'] as String? ?? '',
        url: json['url'] as String? ?? '',
        city: json['city'] as String? ?? '',
        state: json['state'] as String? ?? '',
        valueCents: (json['value_cents'] as num?)?.toInt() ?? 0,
        daysLeft: (json['days_left'] as num?)?.toInt(),
      );
}

final signalsProvider = FutureProvider<List<BuyingSignal>>((ref) async {
  final res = await ref.watch(dioProvider).get<List<dynamic>>('/signals');
  return (res.data ?? <dynamic>[])
      .map((e) => BuyingSignal.fromJson(e as Map<String, dynamic>))
      .toList();
});

/// Whether a signal source answered the last time the poll asked.
///
/// The radar renders what is stored, so an outage used to look exactly like a
/// quiet week: when PNCP refused every request from 06:20 on 2026-09-10 the
/// screen went on showing eight-day-old tenders as if they were today's.
/// Staleness cannot be derived from the signals themselves — a healthy source
/// can publish nothing for days — so the server reports it separately.
class SignalSourceHealth {
  const SignalSourceHealth({
    required this.source,
    required this.reachable,
    this.lastSuccessAt,
  });

  final String source;
  final bool reachable;

  /// Null when the source has never answered.
  final DateTime? lastSuccessAt;

  /// How the source is named to a person. The API's own key is a slug.
  String get label => source.toUpperCase();

  factory SignalSourceHealth.fromJson(Map<String, dynamic> json) =>
      SignalSourceHealth(
        source: json['source'] as String? ?? '',
        // Absent means we were told nothing; treat that as fine. Reading a
        // missing field as "down" would have every screen cry outage the
        // moment a payload changes shape.
        reachable: json['reachable'] as bool? ?? true,
        lastSuccessAt: json['last_success_at'] == null
            ? null
            : DateTime.tryParse(json['last_success_at'] as String)?.toLocal(),
      );
}

/// Never throws upward: the banner is a courtesy, and a radar that refuses to
/// render because it could not check on its sources is worse than a radar
/// that does not mention them.
final signalSourcesProvider =
    FutureProvider<List<SignalSourceHealth>>((ref) async {
  try {
    final res =
        await ref.watch(dioProvider).get<List<dynamic>>('/signal-sources');
    return (res.data ?? <dynamic>[])
        .map((e) => SignalSourceHealth.fromJson(e as Map<String, dynamic>))
        .toList();
  } catch (_) {
    return const <SignalSourceHealth>[];
  }
});

/// One account's standing interest: what to look for.
class SignalWatch {
  const SignalWatch({
    required this.id,
    required this.keywords,
    this.source = 'pncp',
    this.states = const [],
    this.minValueCents = 0,
    this.isActive = true,
    this.derivedFromCatalog = false,
  });

  final String id;
  final String source;

  /// ANY keyword matching counts. A supplier of monitors sells to "monitor
  /// multiparâmetro" and to "monitorização"; demanding every term finds
  /// nothing.
  final List<String> keywords;
  final List<String> states;
  final int minValueCents;
  final bool isActive;

  /// Whether the keywords were derived from the product catalogue (the
  /// server seeder) or typed by a person. The screen's promise — "combina
  /// com o que você vende" — is only honest for derived watches, so the
  /// origin is shown.
  final bool derivedFromCatalog;

  factory SignalWatch.fromJson(Map<String, dynamic> json) => SignalWatch(
        id: json['id'] as String? ?? '',
        source: json['source'] as String? ?? 'pncp',
        keywords: ((json['keywords'] as List<dynamic>?) ?? const [])
            .map((e) => e.toString())
            .toList(),
        states: ((json['states'] as List<dynamic>?) ?? const [])
            .map((e) => e.toString())
            .toList(),
        minValueCents: (json['min_value_cents'] as num?)?.toInt() ?? 0,
        isActive: json['is_active'] as bool? ?? true,
        derivedFromCatalog: json['derived_from_catalog'] as bool? ?? false,
      );
}

final signalWatchesProvider = FutureProvider<List<SignalWatch>>((ref) async {
  final res =
      await ref.watch(dioProvider).get<List<dynamic>>('/signal-watches');
  return (res.data ?? <dynamic>[])
      .map((e) => SignalWatch.fromJson(e as Map<String, dynamic>))
      .toList();
});

final signalActionsProvider = Provider<SignalActions>(
  (ref) => SignalActions(ref.watch(dioProvider)),
);

class SignalActions {
  const SignalActions(this._dio);

  final Dio _dio;

  /// Records "not for us". Kept rather than deleted, so the next collection
  /// cycle does not bring the same notice straight back.
  Future<void> dismiss(String signalId) async {
    await _dio.post<dynamic>('/signals/$signalId/dismiss');
  }

  Future<void> saveWatch({
    String? id,
    required List<String> keywords,
    List<String> states = const [],
    int minValueCents = 0,
    String source = 'pncp',
  }) async {
    await _dio.put<Map<String, dynamic>>(
      '/signal-watches/${id ?? 'new'}',
      data: <String, dynamic>{
        'source': source,
        'keywords': keywords,
        'states': states,
        'min_value_cents': minValueCents,
      },
    );
  }

  Future<void> deleteWatch(String id) async {
    await _dio.delete<dynamic>('/signal-watches/$id');
  }
}
