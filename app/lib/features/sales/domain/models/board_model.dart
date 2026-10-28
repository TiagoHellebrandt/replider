import 'dart:convert';

import 'package:app/core/utils/text_search.dart';

/// Kanban board models (US-M017/M018).
///
/// Hand-written rather than json_serializable: the payload is small, and
/// keeping it codegen-free means a board change does not require a
/// build_runner pass across the whole app.

class BoardStage {
  const BoardStage({
    required this.id,
    required this.name,
    required this.position,
    required this.mapsToStatus,
    required this.isTerminal,
  });

  final String id;
  final String name;
  final int position;

  /// The canonical pipeline status this column means. The board is a VIEW:
  /// dropping a card here is what sets the lead's real status.
  final String mapsToStatus;

  /// Won/lost. Rendered differently and excluded from "stuck lead" nudges —
  /// nagging someone about a deal they already closed is how a feature gets
  /// muted.
  final bool isTerminal;

  factory BoardStage.fromJson(Map<String, dynamic> json) => BoardStage(
        id: json['id'] as String? ?? '',
        name: json['name'] as String? ?? '',
        position: (json['position'] as num?)?.toInt() ?? 0,
        mapsToStatus: json['maps_to_status'] as String? ?? '',
        isTerminal: json['is_terminal'] as bool? ?? false,
      );
}

class BoardCard {
  const BoardCard({
    required this.leadId,
    required this.stageId,
    required this.status,
    required this.score,
    this.companyName,
    this.contactLabel,
    this.ownerUserId,
    this.source,
    this.city,
    this.updatedAt,
    this.boardPos,
    this.hasDraft = false,
    this.buyIntent = false,
    this.qualified = false,
  });

  final String leadId;
  final String stageId;
  final String status;
  final int score;

  /// The manually chosen rank inside the column (fractional ranking, server
  /// authoritative). Null = never manually placed: the card keeps the
  /// automatic most-stalled-first order, AFTER every placed one.
  final double? boardPos;

  /// Where the company is. Second only to the name when someone is deciding
  /// which of two similarly-named clinics this is — and the salesperson
  /// plans their week by region, not by column.
  final String? city;

  /// Last time anything happened to this lead. Drives "parado há N dias",
  /// which is the order the work actually matters in: the deal nobody has
  /// touched in three weeks is the one going cold, not the one at the top of
  /// an arbitrary list.
  final DateTime? updatedAt;

  /// Whole days since anything moved. Null when the server sent no timestamp
  /// — an unknown age must not render as "parado há 0 dias", which reads as
  /// "touched today".
  int? get daysIdle {
    final at = updatedAt;
    if (at == null) return null;
    final days = DateTime.now().difference(at).inDays;
    return days < 0 ? 0 : days;
  }

  /// Company name from enrichment (prospecting fills it). Null for a lead
  /// created by hand before anyone enriched it.
  final String? companyName;

  /// Whatever identifies the person: today the contact id, which for legacy
  /// rows is still a phone/email. Shown only as a fallback — the API does
  /// not join the contact's display name yet.
  final String? contactLabel;

  final String? ownerUserId;
  final String? source;

  /// There is an approach draft waiting for somebody to read and approve.
  ///
  /// The server sent `approach_draft` on every card and the board said nothing
  /// about it — the approval queue is the one step of the cycle that does not
  /// happen without a person, and it was precisely the invisible one.
  final bool hasDraft;

  /// The lead said something that reads as buying intent.
  final bool buyIntent;

  /// Every required field of the funnel has been filled in.
  final bool qualified;

  /// The line a salesperson actually reads. Company first: they are selling
  /// to an organization, and "Hospital São Lucas" beats "+5511999990001".
  String get title {
    final c = companyName?.trim();
    if (c != null && c.isNotEmpty) return c;
    final label = contactLabel?.trim();
    if (label != null && label.isNotEmpty) return label;
    return leadId;
  }

  /// [boardPos] uses a sentinel so callers can SET null (clear the manual
  /// rank) as deliberately as they set a value — `?? this.boardPos` would
  /// make clearing impossible.
  static const _keepPos = Object();

  BoardCard copyWith({
    String? stageId,
    String? status,
    Object? boardPos = _keepPos,
  }) =>
      BoardCard(
        leadId: leadId,
        stageId: stageId ?? this.stageId,
        status: status ?? this.status,
        score: score,
        companyName: companyName,
        contactLabel: contactLabel,
        ownerUserId: ownerUserId,
        source: source,
        city: city,
        updatedAt: updatedAt,
        boardPos:
            identical(boardPos, _keepPos) ? this.boardPos : boardPos as double?,
        hasDraft: hasDraft,
        buyIntent: buyIntent,
        qualified: qualified,
      );

  /// Everything a search should look at, folded once. Name, city and the
  /// contact string together: the salesperson remembers "aquela de Sorocaba"
  /// at least as often as the clinic's exact legal name.
  ///
  /// Accents are folded on BOTH sides (see [foldForSearch]) — most of this
  /// text was typed by somebody else, and "clínica" used to find nothing
  /// while "Clinica Ficticia" sat on the board.
  String get searchHaystack => foldForSearch([
        companyName,
        contactLabel,
        city,
      ].whereType<String>().join(' '));

  factory BoardCard.fromJson(Map<String, dynamic> json) {
    final lead = (json['lead'] as Map<String, dynamic>?) ?? const {};
    final enrichment = _asMap(lead['enrichment']);
    return BoardCard(
      leadId: lead['id'] as String? ?? '',
      stageId: json['stage_id'] as String? ?? '',
      status: lead['status'] as String? ?? '',
      score: (lead['score'] as num?)?.toInt() ?? 0,
      companyName: _companyFromEnrichment(lead['enrichment']),
      // contact_label is the server-side join (name → phone → email); the
      // raw id is the last resort, not the first choice.
      contactLabel:
          lead['contact_label'] as String? ?? lead['contact_id'] as String?,
      ownerUserId: lead['owner_user_id'] as String?,
      source: lead['source'] as String?,
      city: _cityFrom(enrichment, _asMap(lead['collected_metadata'])),
      updatedAt: DateTime.tryParse(lead['updated_at'] as String? ?? ''),
      boardPos: (lead['board_pos'] as num?)?.toDouble(),
      hasDraft: (lead['approach_draft'] as String? ?? '').trim().isNotEmpty,
      buyIntent: (lead['buy_intent_at'] as String? ?? '').isNotEmpty,
      qualified: (lead['qualified_at'] as String? ?? '').isNotEmpty,
    );
  }

  /// The city, from wherever it landed. Prospecting writes a full `address`
  /// and enrichment may write `city`; a hand-typed lead has neither. There is
  /// no city COLUMN, so this is a best-effort read — it returns null rather
  /// than guessing from a string it cannot parse.
  static String? _cityFrom(
      Map<String, dynamic> enrichment, Map<String, dynamic> metadata) {
    for (final map in [enrichment, metadata]) {
      for (final key in ['city', 'cidade', 'municipio']) {
        final v = map[key];
        if (v is String && v.trim().isNotEmpty) return v.trim();
      }
    }
    // Google Places addresses come as "Rua X, 123 - Centro, Sorocaba - SP".
    // The city is the second-to-last comma group; anything shorter is an
    // address we cannot read, and a wrong city on a card is worse than none.
    final address = enrichment['address'];
    if (address is String) {
      final parts = address
          .split(',')
          .map((p) => p.trim())
          .where((p) => p.isNotEmpty)
          .toList();
      // Walk from the end, skipping groups that are actually a CEP —
      // "…, São Paulo - SP, 08070-001" was rendering the POSTAL CODE as
      // the city on every Places card.
      final cep = RegExp(r'^\d{5}-?\d{3}$');
      for (var i = parts.length - 1; i >= 1; i--) {
        final part = parts[i];
        if (cep.hasMatch(part)) continue;
        // "Sorocaba - SP" → keep as-is; a bare neighborhood-looking group
        // (no UF suffix) only counts when it is the classic second-to-last.
        if (RegExp(r' - [A-Z]{2}$').hasMatch(part) || i == parts.length - 2) {
          return part;
        }
      }
    }
    return null;
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

  /// Enrichment arrives as a JSON object, but older rows (and some MCP
  /// writes) carry it as a raw string — reading only the map shape would
  /// silently blank the card's title for exactly the leads prospecting
  /// created.
  static String? _companyFromEnrichment(dynamic enrichment) {
    Map<String, dynamic>? map;
    if (enrichment is Map<String, dynamic>) {
      map = enrichment;
    } else if (enrichment is String && enrichment.trim().isNotEmpty) {
      try {
        final decoded = jsonDecode(enrichment);
        if (decoded is Map<String, dynamic>) map = decoded;
      } catch (_) {
        return null;
      }
    }
    if (map == null) return null;
    for (final key in ['company_name', 'legal_name', 'trade_name']) {
      final v = map[key];
      if (v is String && v.trim().isNotEmpty) return v.trim();
    }
    return null;
  }
}

/// How a salesperson slices a board they cannot read in one glance.
///
/// Deliberately four, named after work rather than after data: "who replied"
/// and "idle for more than 7 days" are the two piles anyone actually works
/// from, and a filter menu with twelve options is a menu nobody opens.
enum BoardFilter {
  all,
  responded,
  awaitingApproval,
  stalled;

  /// A lead untouched for longer than this is the one going cold. Seven days
  /// because that is a working week — long enough not to nag, short enough
  /// that the deal is still warm when it surfaces.
  static const stalledAfterDays = 7;

  bool matches(BoardCard card) {
    switch (this) {
      case BoardFilter.all:
        return true;
      case BoardFilter.responded:
        return card.status == 'responded';
      case BoardFilter.awaitingApproval:
        return card.status == 'pending_approval';
      case BoardFilter.stalled:
        final idle = card.daysIdle;
        return idle != null && idle > stalledAfterDays;
    }
  }
}

class Board {
  const Board({
    required this.stages,
    required this.cards,
    this.truncated = false,
  });

  final List<BoardStage> stages;
  final List<BoardCard> cards;

  /// True when the server hit its page cap, so [cards] is not the whole
  /// funnel. The board says so out loud: a search that silently covers only
  /// part of the pipeline would answer "that company does not exist" about one
  /// that does.
  final bool truncated;

  /// Whether ownership is worth showing on this board at all.
  ///
  /// "Sem dono" on every card of a board where nobody owns anything is one
  /// repetition per lead of a fact that names no action — sixty-nine of them
  /// on the account this was found on. A single-seat account never assigns.
  ///
  /// Derived from the board instead of from the team size: no extra request,
  /// and it turns itself on the moment ownership starts to mean something.
  /// One predicate, both layouts — desktop and mobile must not disagree
  /// about whether a column is worth a label.
  bool get hasAnyOwner =>
      cards.any((c) => c.ownerUserId != null && c.ownerUserId!.isNotEmpty);

  List<BoardCard> cardsIn(String stageId) =>
      cards.where((c) => c.stageId == stageId).toList();

  /// Whether anyone manually ordered this column. One placed card is enough:
  /// the header then says "ordem manual" and offers the reset, because a
  /// half-manual column is no longer honestly "mais parados primeiro".
  bool hasManualOrder(String stageId) =>
      cards.any((c) => c.stageId == stageId && c.boardPos != null);

  /// The cards of one column, filtered, searched, and ordered.
  ///
  /// Manually placed cards come FIRST, by their chosen position — that is
  /// the order a finger just made. The never-placed keep the automatic
  /// oldest-first rule after them: a column sorted by anything else hides
  /// the leads that are dying, which are exactly the ones a human should see
  /// first. Cards with no timestamp sink to the bottom rather than claiming
  /// to be the most urgent.
  /// Everything the search and the filter leave visible, across all stages.
  ///
  /// The bulk redistribution counts from here: the number on the button has to
  /// come from the SAME rule that drew the columns, or the dialog promises a
  /// total the screen does not show.
  List<BoardCard> visibleCards({
    String query = '',
    BoardFilter filter = BoardFilter.all,
  }) {
    final q = foldForSearch(query.trim());
    return cards
        .where(filter.matches)
        .where((c) => q.isEmpty || c.searchHaystack.contains(q))
        .toList();
  }

  List<BoardCard> visibleIn(
    String stageId, {
    String query = '',
    BoardFilter filter = BoardFilter.all,
  }) {
    final q = foldForSearch(query.trim());
    final out = cards
        .where((c) => c.stageId == stageId)
        .where(filter.matches)
        .where((c) => q.isEmpty || c.searchHaystack.contains(q))
        .toList();
    out.sort(compareForColumn);
    return out;
  }

  /// The column ordering rule, in one place: placed first by position, the
  /// rest by idle time (oldest first, unknown last).
  static int compareForColumn(BoardCard a, BoardCard b) {
    final ap = a.boardPos, bp = b.boardPos;
    if (ap != null || bp != null) {
      if (ap == null) return 1;
      if (bp == null) return -1;
      return ap.compareTo(bp);
    }
    final ax = a.updatedAt, bx = b.updatedAt;
    if (ax == null && bx == null) return a.leadId.compareTo(b.leadId);
    if (ax == null) return 1;
    if (bx == null) return -1;
    final byIdle = ax.compareTo(bx);
    // Deterministic tiebreak by id: Dart's sort is unstable, and the server
    // seeds positions with the same id tiebreak — the two orders must agree
    // or the first manual drop lands one slot off.
    return byIdle != 0 ? byIdle : a.leadId.compareTo(b.leadId);
  }

  /// How many cards a filter+query leaves across the WHOLE board, so the
  /// header can say "11" next to a filter before anyone clicks it.
  int countMatching(BoardFilter filter, {String query = ''}) {
    final q = foldForSearch(query.trim());
    return cards
        .where(filter.matches)
        .where((c) => q.isEmpty || c.searchHaystack.contains(q))
        .length;
  }

  factory Board.fromJson(Map<String, dynamic> json, {bool truncated = false}) =>
      Board(
        truncated: truncated,
        stages: ((json['stages'] as List<dynamic>?) ?? const [])
            .map((e) => BoardStage.fromJson(e as Map<String, dynamic>))
            .toList(),
        cards: ((json['cards'] as List<dynamic>?) ?? const [])
            .map((e) => BoardCard.fromJson(e as Map<String, dynamic>))
            .toList(),
      );

  /// Applies a plain column drop locally so the card follows the finger
  /// immediately. The server is still the authority — the controller rolls
  /// this back if the request fails.
  ///
  /// Clears the manual position, mirroring the server: a drop with no slot
  /// hands the card back to the automatic order (and, having just been
  /// touched, it lands at the bottom of it).
  Board withCardMoved(String leadId, String stageId) {
    final stage = stages.where((s) => s.id == stageId).firstOrNull;
    return Board(
      stages: stages,
      truncated: truncated,
      cards: [
        for (final c in cards)
          if (c.leadId == leadId)
            c.copyWith(
                stageId: stageId, status: stage?.mapsToStatus, boardPos: null)
          else
            c,
      ],
    );
  }

  /// Step used when a card is placed at an edge of the placed set — the same
  /// value the server uses, so the optimistic order and the persisted one
  /// agree.
  static const double boardPosStep = 1024;

  /// Applies a slot drop locally: the card gets the same fractional rank the
  /// server will compute, so the optimistic board already sorts the way the
  /// authoritative one will.
  Board withCardPlaced(String leadId, String stageId,
      {String? beforeId, String? afterId}) {
    // A named neighbor that has no position yet means the column is still
    // (partly) in automatic order. The server materialises the WHOLE column
    // into positions before computing the midpoint — mirror it, or the
    // optimistic drop lands at the top while the server lands it right
    // (the original bug: first drag in an automatic column jumped to #1).
    var board = this;
    double? posOf(String? id) => id == null
        ? null
        : board.cards.where((c) => c.leadId == id).firstOrNull?.boardPos;
    final needsSeed = (beforeId != null && posOf(beforeId) == null) ||
        (afterId != null && posOf(afterId) == null);
    if (needsSeed) {
      final column = board.cards.where((c) => c.stageId == stageId).toList()
        ..sort(compareForColumn);
      final seeded = <String, double>{
        for (var i = 0; i < column.length; i++)
          column[i].leadId: (i + 1) * boardPosStep,
      };
      board = Board(
        stages: board.stages,
        truncated: board.truncated,
        cards: [
          for (final c in board.cards)
            seeded.containsKey(c.leadId)
                ? c.copyWith(boardPos: seeded[c.leadId])
                : c,
        ],
      );
    }
    final before = posOf(beforeId);
    final after = posOf(afterId);
    double? maxPos;
    for (final c in board.cards) {
      final p = c.boardPos;
      if (c.stageId != stageId || c.leadId == leadId || p == null) continue;
      if (maxPos == null || p > maxPos) maxPos = p;
    }
    final double pos;
    if (before != null && after != null) {
      pos = (before + after) / 2;
    } else if (before != null) {
      pos = before + boardPosStep;
    } else if (after != null) {
      pos = after - boardPosStep;
    } else {
      pos = maxPos == null ? 0 : maxPos + boardPosStep;
    }

    final stage = board.stages.where((s) => s.id == stageId).firstOrNull;
    return Board(
      stages: board.stages,
      truncated: board.truncated,
      cards: [
        for (final c in board.cards)
          if (c.leadId == leadId)
            c.copyWith(
              stageId: stageId,
              // A same-column reorder keeps the status — the server does too.
              status: c.stageId == stageId ? null : stage?.mapsToStatus,
              boardPos: pos,
            )
          else
            c,
      ],
    );
  }

  /// Clears one column's manual order locally ("ordenar por parados").
  Board withOrderReset(String stageId) => Board(
        stages: stages,
        truncated: truncated,
        cards: [
          for (final c in cards)
            if (c.stageId == stageId && c.boardPos != null)
              c.copyWith(boardPos: null)
            else
              c,
        ],
      );
}

extension _FirstOrNull<T> on Iterable<T> {
  T? get firstOrNull => isEmpty ? null : first;
}
