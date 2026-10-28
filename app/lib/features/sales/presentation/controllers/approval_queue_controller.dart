import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/features/sales/domain/models/approach_review.dart';
import 'package:app/i18n/strings.g.dart';

/// The approval queue (US-M020).
///
/// One list across every funnel, oldest first — the server decides the order,
/// because the draft that has waited longest is the one going stale, and a
/// per-funnel queue lets a forgotten funnel rot quietly.
class QueueLead {
  const QueueLead({
    required this.id,
    required this.recipient,
    required this.recipientAddress,
    required this.draft,
    this.companyName,
    this.source,
    this.funnelId,
    this.waitingSince,
    this.findings = const [],
    this.matchedProducts = const [],
    this.objections = const [],
    this.scheduledFor,
    this.sendBlockedReason,
    this.whatsappPhone,
    this.tenderValue,
    this.tenderDeadline,
  });

  final String id;

  /// Who the message reaches, as a human recognises them.
  final String recipient;

  /// WHERE it reaches them — the phone or the email, never a name.
  ///
  /// This queue is the product's main send surface, and approving without
  /// seeing the address is not approval: two clinics can share a name, and
  /// a mistyped number keeps whatever name was typed with it.
  final String recipientAddress;

  final String draft;
  final String? companyName;
  final String? source;

  /// Which funnel this draft belongs to, so a card in the cross-funnel view can
  /// say which campaign it came from — the same words read differently
  /// depending on who the list was built for.
  final String? funnelId;

  /// When this approved send will go out (phase 1b) — null for a card still
  /// waiting on a human. A scheduled card is labeled, never hidden: the send
  /// is planned, and planned work the operator cannot see is work they redo.
  final DateTime? scheduledFor;

  /// Why THIS card cannot go out, when the queue as a whole is fine. Today
  /// the only value is `NO_ADDRESS`: the account has a door, this recipient
  /// has no address behind it.
  ///
  /// Per-card because it is per-card work to fix — find this buyer's e-mail —
  /// and because a queue is usually a mix. Blocking the whole screen would
  /// tell someone with 37 sendable offers that nothing can leave.
  final String? sendBlockedReason;

  /// The buyer's phone, sent ONLY on a card the platform cannot deliver to.
  ///
  /// The way out of a blocked card: the product has no door to this contact,
  /// but the seller has a phone in their pocket. Absent on a card that will
  /// send normally — an escape hatch offered beside a working send teaches
  /// people to route around the product's own delivery, and then nothing is
  /// recorded anywhere.
  final String? whatsappPhone;

  /// The tender's published value, already formatted by the server, and when
  /// it closes. Both null unless this offer answers a public notice.
  ///
  /// A queue of fifty identical-looking cards, every one reading "R$ 0" and
  /// none carrying a date, gives the operator no reason to open the first one
  /// rather than the thirty-seventh. Forty days of an untouched backlog is
  /// what that looks like from the outside.
  final String? tenderValue;
  final DateTime? tenderDeadline;

  /// True when this card cannot be sent for a reason of its own.
  bool get isSendBlocked => sendBlockedReason != null;

  /// Days until the tender closes, negative once it has. Null without a
  /// deadline — which is NOT the same as "plenty of time".
  int? daysToDeadline(DateTime now) {
    final due = tenderDeadline;
    if (due == null) return null;
    final today = DateTime(now.year, now.month, now.day);
    return DateTime(due.year, due.month, due.day).difference(today).inDays;
  }

  /// When the draft last changed, which for a queued draft is when it started
  /// waiting.
  ///
  /// Staleness is the one thing a queue has to show. The operator's real
  /// question is not "how many" but "what is going cold", and a list where
  /// yesterday's draft looks exactly like the one written a minute ago cannot
  /// answer it.
  final DateTime? waitingSince;

  /// Pattern-detected problems in [draft], computed server-side at no cost
  /// (regexes over text already in memory), so every card carries its verdict
  /// without anyone asking for it. Empty means no KNOWN pattern matched — not
  /// "an AI read this and approved it", which is the separate on-demand
  /// review.
  final List<ApproachFinding> findings;

  /// Which of the account's products this offer is about, and why — picked
  /// by the matcher against the catalogue (2026-08-19). Shown above the
  /// text so whoever approves sees WHY this product before reading the
  /// draft that sells it. Empty for a lead drafted before the matcher, or
  /// for a funnel that names its own product.
  final List<MatchedProduct> matchedProducts;

  /// The pushback the matcher expects from THIS customer, already answered
  /// (objeções v1). Shown under the product so whoever approves goes into
  /// the conversation armed. Empty for pre-objections drafts.
  final List<AnticipatedObjection> objections;

  /// What the operator reads first: the organisation they are selling to,
  /// falling back to whatever identifies the person.
  String get title {
    final company = companyName?.trim();
    if (company != null && company.isNotEmpty) return company;
    return recipient;
  }

  factory QueueLead.fromJson(Map<String, dynamic> json) {
    final enrichment = _asMap(json['enrichment']);
    return QueueLead(
      id: json['id'] as String? ?? '',
      recipient: json['contact_label'] as String? ??
          json['contact_id'] as String? ??
          '',
      recipientAddress: json['contact_address'] as String? ?? '',
      draft: json['approach_draft'] as String? ?? '',
      companyName: _str(enrichment['company_name']),
      source: _str(json['source']),
      funnelId: _str(json['funnel_id']),
      waitingSince: _date(json['updated_at']),
      findings:
          (json['approach_findings'] as List<dynamic>? ?? const <dynamic>[])
              .whereType<Map<String, dynamic>>()
              .map(ApproachFinding.fromJson)
              .where((f) => f.quote.isNotEmpty)
              .toList(),
      matchedProducts: MatchedProduct.listFrom(json['matched_products']),
      objections: AnticipatedObjection.listFrom(json['anticipated_objections']),
      scheduledFor: _date(json['scheduled_for']),
      sendBlockedReason: _blockedReason(json['send_blocked_reason']),
      whatsappPhone: (json['whatsapp_phone'] as String?)?.trim(),
      tenderValue: _str(enrichment['edital_valor_estimado']),
      tenderDeadline: _date(enrichment['edital_prazo']),
    );
  }

  static Map<String, dynamic> _asMap(dynamic v) {
    if (v is Map<String, dynamic>) return v;
    if (v is String && v.trim().isNotEmpty) {
      try {
        final decoded = jsonDecode(v);
        if (decoded is Map<String, dynamic>) return decoded;
      } catch (_) {}
    }
    return const {};
  }

  static String? _str(dynamic v) {
    if (v is String && v.trim().isNotEmpty) return v.trim();
    return null;
  }

  /// An unparseable timestamp is absent, not epoch: the card would otherwise
  /// claim the draft has been waiting since 1970 and pin it to the top of a
  /// list sorted by staleness.
  static DateTime? _date(dynamic v) {
    if (v is! String || v.trim().isEmpty) return null;
    return DateTime.tryParse(v)?.toLocal();
  }
}

/// How long a draft has been waiting, in the coarsest unit that still says
/// something. Null when the timestamp never arrived — the card then shows no
/// age rather than a made-up one.
String? formatQueueWait(DateTime? since, {DateTime? now}) {
  if (since == null) return null;
  final elapsed = (now ?? DateTime.now()).difference(since);
  // A clock skew or a draft stamped a second in the future reads as brand new,
  // never as a negative age.
  if (elapsed.inMinutes < 1) {
    return t.lead_detail.queue_waiting_minutes(count: '1');
  }
  if (elapsed.inHours < 1) {
    return t.lead_detail.queue_waiting_minutes(count: '${elapsed.inMinutes}');
  }
  if (elapsed.inHours < 24) {
    return t.lead_detail.queue_waiting_hours(count: '${elapsed.inHours}');
  }
  return t.lead_detail.queue_waiting_days(count: '${elapsed.inDays}');
}

/// One funnel's share of the backlog, counted over the WHOLE queue.
class QueueFunnelCount {
  const QueueFunnelCount({
    required this.funnelId,
    required this.funnelName,
    required this.total,
  });

  final String funnelId;

  /// Empty when the funnel row is gone but its drafts are not. Rendered as
  /// "no funnel" rather than hidden — somebody is still waiting on those.
  final String funnelName;
  final int total;

  factory QueueFunnelCount.fromJson(Map<String, dynamic> json) =>
      QueueFunnelCount(
        funnelId: json['funnel_id'] as String? ?? '',
        funnelName: (json['funnel_name'] as String? ?? '').trim(),
        total: (json['total'] as num?)?.toInt() ?? 0,
      );
}

/// A page of the queue, plus the shape of the backlog behind it.
///
/// [total] and [byFunnel] come from the server counted over the whole queue,
/// never derived from [items]: the page is capped, so counting what arrived
/// would tell the operator the backlog is 200 when it is 340 and give no chip
/// at all to the funnel holding the rest.
class ApprovalQueuePage {
  const ApprovalQueuePage({
    required this.items,
    required this.total,
    required this.byFunnel,
    this.sendBlockedReason,
  });

  final List<QueueLead> items;
  final int total;
  final List<QueueFunnelCount> byFunnel;

  /// Why these drafts cannot go out, or null.
  ///
  /// The queue used to render "Aprovar e enviar" as an enabled primary button
  /// on an account with no channel at all — 41 written offers and a failure
  /// that only arrived after the click.
  final String? sendBlockedReason;

  /// True when the drafts are real but nothing can leave yet.
  bool get canSend => sendBlockedReason == null;

  /// True when the server had more drafts than it returned. The screen says so
  /// out loud, because every filter below is honest only about what is loaded.
  bool get isPartial => total > items.length;
}

/// What the operator asked the queue to show.
///
/// Funnel and order are SERVER-side (they reach the SQL, so they narrow the
/// whole queue rather than the page). [flaggedOnly] is client-side over what is
/// loaded — the promise detector runs per draft on the server but there is no
/// column to filter on, and a partial page announces itself via
/// [ApprovalQueuePage.isPartial].
class ApprovalQueueFilter {
  const ApprovalQueueFilter({
    this.funnelId,
    this.newestFirst = false,
    this.flaggedOnly = false,
  });

  /// Null means every funnel — the default, on purpose: a per-funnel queue
  /// lets a funnel nobody opened today rot quietly.
  final String? funnelId;
  final bool newestFirst;
  final bool flaggedOnly;

  ApprovalQueueFilter copyWith({
    String? funnelId,
    bool clearFunnel = false,
    bool? newestFirst,
    bool? flaggedOnly,
  }) =>
      ApprovalQueueFilter(
        funnelId: clearFunnel ? null : (funnelId ?? this.funnelId),
        newestFirst: newestFirst ?? this.newestFirst,
        flaggedOnly: flaggedOnly ?? this.flaggedOnly,
      );
}

class ApprovalQueueFilterNotifier extends Notifier<ApprovalQueueFilter> {
  @override
  ApprovalQueueFilter build() => const ApprovalQueueFilter();

  void selectFunnel(String? funnelId) => state = funnelId == null
      ? state.copyWith(clearFunnel: true)
      : state.copyWith(funnelId: funnelId);

  void setNewestFirst(bool value) => state = state.copyWith(newestFirst: value);

  void setFlaggedOnly(bool value) => state = state.copyWith(flaggedOnly: value);
}

final approvalQueueFilterProvider =
    NotifierProvider<ApprovalQueueFilterNotifier, ApprovalQueueFilter>(
        ApprovalQueueFilterNotifier.new);

/// The queue's page size.
///
/// Deliberately the server's maximum rather than its default: this is a backlog
/// the operator works through, and the client-side "flagged only" filter is
/// honest only over what is loaded. A queue past this is announced, not hidden.
const approvalQueuePageSize = 200;

final approvalQueueProvider =
    FutureProvider.autoDispose<ApprovalQueuePage>((ref) async {
  final dio = ref.watch(dioProvider);
  // Only the server-side halves of the filter are watched: toggling "flagged
  // only" must not spend a round-trip re-reading rows the client already has.
  final filter = ref.watch(approvalQueueFilterProvider
      .select((f) => (funnelId: f.funnelId, newestFirst: f.newestFirst)));

  final res = await dio.get<Map<String, dynamic>>(
    '/leads/approval-queue',
    queryParameters: <String, dynamic>{
      'limit': approvalQueuePageSize,
      if (filter.funnelId != null) 'funnel_id': filter.funnelId,
      if (filter.newestFirst) 'sort': 'newest',
    },
  );

  final body = res.data ?? const <String, dynamic>{};
  final items = (body['items'] as List<dynamic>? ?? const <dynamic>[])
      .whereType<Map<String, dynamic>>()
      .map(QueueLead.fromJson)
      .toList();
  return ApprovalQueuePage(
    items: items,
    // Falls back to what arrived rather than to 0: an absent count must not
    // render as "the queue is empty" over a list of visible cards.
    total: _totalFloor((body['total'] as num?)?.toInt(), items.length),
    byFunnel: (body['by_funnel'] as List<dynamic>? ?? const <dynamic>[])
        .whereType<Map<String, dynamic>>()
        .map(QueueFunnelCount.fromJson)
        .where((c) => c.total > 0)
        .toList(),
    sendBlockedReason: _blockedReason(body['send_blocked_reason']),
  );
});

/// An absent or blank reason means "nothing is blocking" — the permissive
/// answer, so an older server that does not send the field never disables a
/// queue that would have sent fine.
String? _blockedReason(dynamic raw) {
  if (raw is! String) return null;
  final trimmed = raw.trim();
  return trimmed.isEmpty ? null : trimmed;
}

/// "Clínica Aurora — +5511999900101", collapsing to whichever half exists.
/// Shared by the queue card and its send confirmation so the two can never
/// disagree about who is being messaged.
String recipientLine(String label, String address) {
  final l = label.trim();
  final a = address.trim();
  if (a.isEmpty || a == l) return l;
  if (l.isEmpty) return a;
  return '$l — $a';
}

/// One product the matcher picked for a lead: `{product_id, name, reason}`
/// as the API stores it on `leads.matched_products`.
class MatchedProduct {
  const MatchedProduct(
      {required this.productId, required this.name, this.reason = ''});

  final String productId;
  final String name;
  final String reason;

  static List<MatchedProduct> listFrom(dynamic raw) {
    var list = raw;
    if (list is String && list.trim().isNotEmpty) {
      try {
        list = jsonDecode(list);
      } catch (_) {
        return const [];
      }
    }
    if (list is! List) return const [];
    return list
        .whereType<Map>()
        .map((m) => MatchedProduct(
              productId: (m['product_id'] as String?)?.trim() ?? '',
              name: (m['name'] as String?)?.trim() ?? '',
              reason: (m['reason'] as String?)?.trim() ?? '',
            ))
        .where((p) => p.name.isNotEmpty)
        .toList();
  }
}

/// One anticipated objection `{label, counter}` as the API stores it on
/// `leads.anticipated_objections`.
class AnticipatedObjection {
  const AnticipatedObjection({required this.label, this.counter = ''});

  final String label;
  final String counter;

  static List<AnticipatedObjection> listFrom(dynamic raw) {
    var list = raw;
    if (list is String && list.trim().isNotEmpty) {
      try {
        list = jsonDecode(list);
      } catch (_) {
        return const [];
      }
    }
    if (list is! List) return const [];
    return list
        .whereType<Map>()
        .map((m) => AnticipatedObjection(
              label: (m['label'] as String?)?.trim() ?? '',
              counter: (m['counter'] as String?)?.trim() ?? '',
            ))
        .where((o) => o.label.isNotEmpty)
        .toList();
  }
}

/// The server deliberately degrades to `total: 0` with items present when
/// the by-funnel count query fails. Trusting that zero hid the filter bar
/// and blanked the tab badge over a visible backlog — the floor keeps the
/// UI honest about what it can already see.
int _totalFloor(int? serverTotal, int visible) {
  final t = serverTotal ?? 0;
  return t < visible ? visible : t;
}

/// Where a queued draft came from, in the order the money is likely to be.
///
/// The queue mixes three very different kinds of work: a reorder for an
/// existing customer, a tender the radar matched, and a cold Places prospect.
/// Deciding them interleaved means the seller reads forty generic cold drafts
/// to find the two that pay — so the screen groups by source, likeliest first.
enum QueueSource { carteira, sinal, frio }

QueueSource queueSourceOf(QueueLead lead) {
  switch (lead.source) {
    case 'pncp_signal':
      return QueueSource.sinal;
    // Both of these are strangers. `google_places` is a business found on a
    // map; `cnes_park` is an establishment read out of a government
    // registry. Neither has ever heard of us.
    case 'google_places':
    case 'cnes_park':
      return QueueSource.frio;
    default:
      // Imported customers, manual leads and legacy rows: the seller's own
      // book. Unknown sources land here on purpose — "carteira" renders
      // them prominently, which is the safe failure for a mislabeled draft.
      return QueueSource.carteira;
  }
}

/// The result of one batch call, in the words the snackbar needs.
class QueueBatchOutcome {
  const QueueBatchOutcome(
      {required this.processed, required this.failed, this.stoppedReason});
  final int processed;
  final int failed;
  final String? stoppedReason;
}

/// The one action that records something the PLATFORM did not do.
///
/// A blocked card can still be delivered — by the seller, from their own
/// WhatsApp. Nothing leaves this product when that happens, so the queue would
/// otherwise hold the same finished offer for ever, and every count built on
/// it would be wrong in the direction that hurts: work done, reported as
/// pending.
final queueManualSendProvider =
    Provider.autoDispose<QueueManualSend>(QueueManualSend.new);

class QueueManualSend {
  QueueManualSend(this._ref);
  final Ref _ref;

  /// Moves the lead to `contacted` on the seller's word.
  ///
  /// Only ever called after a human confirms they actually sent it. The status
  /// means "this buyer has been contacted", which is true — it does not claim
  /// the platform was the one that did it, and no send is recorded, so the
  /// e-mail metrics stay honest about what left the mailbox.
  Future<void> markSentByHand(String leadId) async {
    await _ref.read(dioProvider).patch<Map<String, dynamic>>(
      '/leads/$leadId/status',
      data: <String, dynamic>{'status': 'contacted'},
    );
    _ref.invalidate(approvalQueueProvider);
  }
}

/// Batch actions over the queue (Ofertas → Hoje, phase 1).
///
/// Both loop server-side per lead through the same rules as clicking one by
/// one; the redraft additionally loops CLIENT-side because the server caps
/// each call at 8 live AI calls and returns the remainder.
class QueueBatchActions {
  QueueBatchActions(this._ref);
  final Ref _ref;

  Future<QueueBatchOutcome> discardAll(List<String> leadIds) async {
    final dio = _ref.read(dioProvider);
    final res = await dio.post<Map<String, dynamic>>(
      '/leads/batch-discard',
      data: {'lead_ids': leadIds},
    );
    final body = res.data ?? const <String, dynamic>{};
    return QueueBatchOutcome(
      processed: (body['discarded'] as num?)?.toInt() ?? 0,
      failed: (body['skipped'] as num?)?.toInt() ?? 0,
    );
  }

  Future<QueueBatchOutcome> redraftAll(
    List<String> leadIds, {
    void Function(int done, int total)? onProgress,
  }) async {
    final dio = _ref.read(dioProvider);
    var remaining = List<String>.from(leadIds);
    var redrafted = 0;
    final failed = <String>{};
    String? stopped;
    // 30 round-trips × 8 drafts covers a 200-lead page with headroom; the
    // bound exists so a server that keeps returning the same remainder can
    // never spin this loop forever.
    for (var i = 0; i < 30 && remaining.isNotEmpty; i++) {
      final res = await dio.post<Map<String, dynamic>>(
        '/leads/batch-redraft',
        data: {'lead_ids': remaining},
      );
      final body = res.data ?? const <String, dynamic>{};
      redrafted += (body['redrafted'] as num?)?.toInt() ?? 0;
      failed.addAll((body['failed_lead_ids'] as List<dynamic>? ?? const [])
          .whereType<String>());
      final next = (body['remaining_lead_ids'] as List<dynamic>? ?? const [])
          .whereType<String>()
          .toList();
      stopped = (body['stopped_reason'] as String?)?.trim();
      onProgress?.call(redrafted + failed.length, leadIds.length);
      if (stopped != null && stopped.isNotEmpty) break;
      if (next.length >= remaining.length) break; // no forward progress
      remaining = next;
    }
    return QueueBatchOutcome(
      processed: redrafted,
      failed: failed.length,
      stoppedReason: (stopped != null && stopped.isNotEmpty) ? stopped : null,
    );
  }
}

final queueBatchActionsProvider =
    Provider.autoDispose<QueueBatchActions>(QueueBatchActions.new);

/// This week's outcome line — the feedback the approval ritual was missing.
class OffersOutcome {
  const OffersOutcome(
      {required this.sent, required this.replies, required this.proposals});
  final int sent;
  final int replies;
  final int proposals;
}

final offersOutcomeProvider =
    FutureProvider.autoDispose<OffersOutcome>((ref) async {
  final dio = ref.watch(dioProvider);
  final res = await dio.get<Map<String, dynamic>>('/me/offers/outcome');
  final body = res.data ?? const <String, dynamic>{};
  return OffersOutcome(
    sent: (body['sent'] as num?)?.toInt() ?? 0,
    replies: (body['replies'] as num?)?.toInt() ?? 0,
    proposals: (body['proposals'] as num?)?.toInt() ?? 0,
  );
});

/// "29/08 às 08:00", local time — shared by the snackbar and the card badge
/// so the two never disagree about when a scheduled send goes out.
String formatScheduledFor(DateTime at) {
  final local = at.toLocal();
  final day = local.day.toString().padLeft(2, '0');
  final month = local.month.toString().padLeft(2, '0');
  final hour = local.hour.toString().padLeft(2, '0');
  final minute = local.minute.toString().padLeft(2, '0');
  return '$day/$month às $hour:$minute';
}
