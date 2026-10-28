import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/features/sales/domain/models/lead_timeline.dart';

/// The conversation with one lead's customer.
///
/// Keyed by lead rather than by conversation: a customer may have written on
/// several channels, and the salesperson is asking about the PERSON, not about
/// one thread.
final leadTimelineProvider =
    FutureProvider.autoDispose.family<LeadTimeline, String>(
  (ref, leadId) async {
    final dio = ref.watch(dioProvider);
    final res = await dio.get<Map<String, dynamic>>('/leads/$leadId/timeline');
    return LeadTimeline.fromJson(res.data ?? const <String, dynamic>{});
  },
);

/// Answering the customer from the lead.
///
/// `POST /leads/{id}/reply` is lead-scoped on purpose: `/conversations/*` is
/// admin/attendant, so a vendedor could read the reply here and had nowhere to
/// answer it. The server pauses the bot on that thread and sends through the
/// same human path the inbox uses.
class LeadTimelineActions {
  const LeadTimelineActions(this._dio);

  final Dio _dio;

  /// Sends [text] on the thread behind [leadId]. Throws the Dio error so the
  /// caller can show `parseApiError` (409 `LEAD_NO_CONVERSATION` when nobody
  /// has written to this customer yet).
  Future<void> replyToLead(String leadId, String text, {String? subject}) {
    return _dio.post<void>('/leads/$leadId/reply', data: {
      'text': text,
      if (subject != null && subject.isNotEmpty) 'subject': subject,
    });
  }
}

final leadTimelineActionsProvider = Provider<LeadTimelineActions>(
  (ref) => LeadTimelineActions(ref.watch(dioProvider)),
);
