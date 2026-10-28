import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/network/dio_client.dart';

/// Target audiences (§2, D3).
///
/// A saved answer to "who are we selling to". Prospecting without one is a
/// keyword typed from memory every morning — which is how a supplier works
/// the same three cities for a year and never notices the fourth.
class Audience {
  const Audience({
    required this.id,
    required this.name,
    required this.segment,
    this.size = '',
    this.decisionMaker = '',
    this.regions = const [],
    this.keywords = const [],
    this.suggested = false,
    this.unexplored = false,
  });

  final String id;
  final String name;

  /// The kind of business in the customer's own words, not a CNAE code.
  final String segment;
  final String size;

  /// The ROLE to reach. Decides how the approach is written far more than
  /// the segment does: a message to a purchasing manager and one to a
  /// clinical director share almost no sentences.
  final String decisionMaker;

  final List<String> regions;
  final List<String> keywords;

  /// Proposed by a model rather than typed by a human. Shown as such — a
  /// suggestion presented as the customer's own decision is a small lie that
  /// erodes trust in every other suggestion.
  final bool suggested;

  /// The one deliberately adjacent proposal per round: a segment that buys
  /// the same solution for a different reason, and that this company
  /// probably has not considered.
  final bool unexplored;

  /// What actually reaches the prospecting source: one phrase, because that
  /// is all Google Places and its peers accept.
  String get searchKeyword =>
      keywords.isEmpty ? segment : keywords.join(' ').trim();

  String get searchLocation => regions.isEmpty ? '' : regions.first;

  factory Audience.fromJson(Map<String, dynamic> json) => Audience(
        id: json['id'] as String? ?? '',
        name: json['name'] as String? ?? '',
        segment: json['segment'] as String? ?? '',
        size: json['size'] as String? ?? '',
        decisionMaker: json['decision_maker'] as String? ?? '',
        regions: ((json['regions'] as List<dynamic>?) ?? const [])
            .map((e) => e.toString())
            .toList(),
        keywords: ((json['keywords'] as List<dynamic>?) ?? const [])
            .map((e) => e.toString())
            .toList(),
        suggested: json['suggested'] as bool? ?? false,
        unexplored: json['unexplored'] as bool? ?? false,
      );
}

final audiencesProvider = FutureProvider<List<Audience>>((ref) async {
  final res = await ref.watch(dioProvider).get<List<dynamic>>('/audiences');
  return (res.data ?? <dynamic>[])
      .map((e) => Audience.fromJson(e as Map<String, dynamic>))
      .toList();
});

final audienceActionsProvider = Provider<AudienceActions>(
  (ref) => AudienceActions(ref.watch(dioProvider)),
);

class AudienceActions {
  const AudienceActions(this._dio);

  final Dio _dio;

  /// Asks for proposals. Nothing is saved: a suggestion written straight
  /// into somebody's list would be the product deciding who they sell to.
  Future<List<Audience>> suggest() async {
    final res = await _dio.post<List<dynamic>>('/audiences/suggest');
    return (res.data ?? <dynamic>[])
        .map((e) => Audience.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<void> save(Audience audience) async {
    await _dio.put<Map<String, dynamic>>(
      '/audiences/${audience.id.isEmpty ? 'new' : audience.id}',
      data: <String, dynamic>{
        'name': audience.name,
        'segment': audience.segment,
        'size': audience.size,
        'decision_maker': audience.decisionMaker,
        'regions': audience.regions,
        'keywords': audience.keywords,
      },
    );
  }

  Future<void> delete(String id) async {
    await _dio.delete<dynamic>('/audiences/$id');
  }
}
