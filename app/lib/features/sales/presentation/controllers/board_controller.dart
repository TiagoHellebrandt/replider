import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/features/sales/domain/models/board_model.dart';

/// Kanban board data (US-M017/M018).
///
/// A plain FutureProvider + a service, rather than a family AsyncNotifier:
/// the board needs exactly one read and one mutation, and the optimistic
/// move belongs to the screen anyway (it is the widget that must roll the
/// card back under the user's finger). Keeping the provider dumb also keeps
/// it codegen-free.
/// The server's hard page cap for list endpoints. Asking for it explicitly
/// beats the 50 default: a funnel with 120 leads was silently rendering 50 of
/// them, and a board that hides 70 deals without saying so is worse than one
/// that admits it is partial.
const boardPageLimit = 200;

final boardProvider =
    FutureProvider.family<Board, String>((ref, funnelId) async {
  final dio = ref.watch(dioProvider);
  final res = await dio.get<Map<String, dynamic>>(
    '/funnels/$funnelId/board',
    queryParameters: <String, dynamic>{'limit': boardPageLimit},
  );
  final data = res.data ?? const <String, dynamic>{};
  final cards = (data['cards'] as List<dynamic>?) ?? const <dynamic>[];
  // A full page means there is probably more behind it. Probably, not
  // certainly — the board says "there may be more", never invents a total.
  return Board.fromJson(data, truncated: cards.length >= boardPageLimit);
});

final boardServiceProvider = Provider<BoardService>(
  (ref) => BoardService(ref.watch(dioProvider)),
);

class BoardService {
  const BoardService(this._dio);

  final Dio _dio;

  /// Moves a card. The server writes the stage AND the canonical status
  /// together (US-M008), so the caller must refresh rather than assume its
  /// optimistic copy is complete.
  ///
  /// [beforeLeadId]/[afterLeadId] are the drop slot's neighbors as rendered
  /// (before = the card above, after = the one below; either absent at an
  /// edge). With at least one, the card gets a persisted manual position; a
  /// same-stage move with neighbors is a pure reorder. With neither, the
  /// card returns to the column's automatic order.
  Future<void> move({
    required String leadId,
    required String stageId,
    String? beforeLeadId,
    String? afterLeadId,
  }) async {
    await _dio.post<Map<String, dynamic>>(
      '/leads/$leadId/move',
      data: <String, dynamic>{
        'stage_id': stageId,
        if (beforeLeadId != null) 'before_lead_id': beforeLeadId,
        if (afterLeadId != null) 'after_lead_id': afterLeadId,
      },
    );
  }

  /// Clears one column's manual order ("ordenar por parados"), restoring the
  /// automatic most-stalled-first sort. Moves nothing through the pipeline.
  Future<void> resetOrder({
    required String funnelId,
    required String stageId,
  }) async {
    await _dio.post<Map<String, dynamic>>(
      '/leads/board-order/reset',
      data: <String, dynamic>{'funnel_id': funnelId, 'stage_id': stageId},
    );
  }

  /// Hands several leads at once to the same person.
  ///
  /// Sends the LIST of ids, not a filter: the screen counts and sends exactly
  /// what it counted, so the number confirmed in the dialog is the number that
  /// changes. A filter evaluated server-side would also catch whatever entered
  /// the funnel between the count and the click.
  Future<({int assigned, int skipped})> bulkAssign({
    required List<String> leadIds,
    required String? ownerUserId,
  }) async {
    final res = await _dio.post<Map<String, dynamic>>(
      '/leads/bulk-assign',
      data: <String, dynamic>{
        'lead_ids': leadIds,
        'owner_user_id': ownerUserId,
      },
    );
    final data = res.data ?? const <String, dynamic>{};
    return (
      assigned: (data['assigned'] as num?)?.toInt() ?? 0,
      skipped: ((data['skipped'] as List<dynamic>?) ?? const []).length,
    );
  }

  /// Registers a lead by hand — a referral, a card from a conference.
  ///
  /// Returns false when the phone was already in this funnel: the server is
  /// idempotent and answers 200-not-created instead of minting a second
  /// card for the same clinic, and the caller says so rather than pretending
  /// something new happened.
  /// A lead is a company with somebody in it (2026-08-19). [company] is the
  /// establishment, [name] the person; at least one of [phone]/[email] is
  /// required by the server, the rest is optional and lands in the lead's
  /// enrichment under the researcher's own keys.
  Future<bool> createLead({
    required String funnelId,
    required String name,
    required String phone,
    String company = '',
    String role = '',
    String email = '',
    String city = '',
    Map<String, String> metadata = const {},
  }) async {
    final res = await _dio.post<Map<String, dynamic>>(
      '/funnels/$funnelId/leads',
      data: <String, dynamic>{
        'name': name,
        'phone': phone,
        if (company.isNotEmpty) 'company': company,
        if (role.isNotEmpty) 'role': role,
        if (email.isNotEmpty) 'email': email,
        if (city.isNotEmpty) 'city': city,
        // Only what was answered: sending an empty key would store a collected
        // field with a blank value, which the qualification queue counts as
        // answered.
        if (metadata.isNotEmpty) 'metadata': metadata,
      },
    );
    return (res.data?['created'] as bool?) ?? true;
  }
}
