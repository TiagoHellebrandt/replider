import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/core/network/websocket_service.dart';

/// The home command centre (§6.13, I2).
///
/// One read for the whole screen: four counted tiles, the mini-funnel, the
/// impact-ranked "needs you" list and at most one copilot chip.
///
/// The list is RANKED server-side and this layer never re-sorts it. Two screens
/// ordering the same work differently is how a team stops agreeing about what
/// matters, and the rule ("overdue first, then by money, then by age") lives in
/// one place precisely so it can be argued about in one place.

/// What kind of thing needs a person.
///
/// [unknown] exists so a server that grows a seventh kind renders a row the user
/// can still tap instead of crashing the screen — the deep link comes from the
/// server, so an unrecognised kind is still actionable.
enum PendingActionKind {
  approveDraft('approve_draft'),
  unownedLead('unowned_lead'),
  signatureViewed('signature_viewed'),
  contractAlert('contract_alert'),
  confirmSale('confirm_sale'),
  reorderDue('reorder_due'),
  waitingReply('waiting_reply'),

  /// An establishment in the territory that holds what this account sells and
  /// has never been approached. The coldest row on the screen, and the only
  /// one about ground rather than about somebody waiting.
  untouchedTarget('untouched_target'),
  unknown('');

  const PendingActionKind(this.wire);
  final String wire;

  static PendingActionKind fromWire(String? value) =>
      PendingActionKind.values.firstWhere((k) => k.wire == value,
          orElse: () => PendingActionKind.unknown);
}

class PendingAction {
  const PendingAction({
    required this.kind,
    required this.targetId,
    required this.deepLink,
    this.customerName = '',
    this.detail = '',
    this.moneyAtStakeCents = 0,
    this.currency = '',
    this.dueOn,
    this.sinceAt,
  });

  final PendingActionKind kind;
  final String targetId;

  /// Where the work happens. Built server-side, so a new kind of action needs
  /// no client release to be actionable.
  final String deepLink;

  /// The point of the row. "3 assinaturas pendentes" is a statistic; a name is
  /// a phone call.
  final String customerName;
  final String detail;

  /// Zero means the action carries no amount (a draft, an unowned lead) — NOT
  /// that it is worth nothing.
  final int moneyAtStakeCents;
  final String currency;

  final DateTime? dueOn;

  /// When this started waiting. Drives the "há 4 dias" the row shows.
  final DateTime? sinceAt;

  bool get hasMoney => moneyAtStakeCents > 0;

  /// A deadline already passed. Computed here rather than trusted from a
  /// server flag so it cannot be a day stale in a cached payload.
  bool overdue(DateTime now) => dueOn != null && dueOn!.isBefore(now);

  factory PendingAction.fromJson(Map<String, dynamic> json) => PendingAction(
        kind: PendingActionKind.fromWire(json['kind'] as String?),
        targetId: json['target_id'] as String? ?? '',
        deepLink: json['deep_link'] as String? ?? '',
        customerName: json['customer_name'] as String? ?? '',
        detail: json['detail'] as String? ?? '',
        moneyAtStakeCents: (json['money_at_stake_cents'] as num?)?.toInt() ?? 0,
        currency: json['currency'] as String? ?? '',
        dueOn: DateTime.tryParse(json['due_on'] as String? ?? ''),
        sinceAt: DateTime.tryParse(json['since_at'] as String? ?? ''),
      );
}

class CommandCenterTiles {
  const CommandCenterTiles({
    this.activeLeads = 0,
    this.pendingApproval = 0,
    this.meetings = 0,
    this.wonGrossCents = 0,
    this.currency = '',
    this.periodLabel = '',
    this.untouchedTargets = 0,
  });

  final int activeLeads;
  final int pendingApproval;
  final int meetings;
  final int wonGrossCents;
  final String currency;

  /// The window the last two tiles cover ("2026-07"). Shown because "ganhos"
  /// with no period is a number nobody can check.
  final String periodLabel;

  /// Establishments in the territory holding this account's equipment that
  /// have never been approached.
  ///
  /// The only tile about what is being LOST — the other four count work
  /// already in hand, so an account with one lead and a whole state in front
  /// of it read exactly like one that had worked the state through. Never a
  /// money figure: the registry counts machines, not deals.
  final int untouchedTargets;

  factory CommandCenterTiles.fromJson(Map<String, dynamic> json) =>
      CommandCenterTiles(
        activeLeads: (json['active_leads'] as num?)?.toInt() ?? 0,
        pendingApproval: (json['pending_approval'] as num?)?.toInt() ?? 0,
        meetings: (json['meetings'] as num?)?.toInt() ?? 0,
        wonGrossCents: (json['won_gross_cents'] as num?)?.toInt() ?? 0,
        currency: json['currency'] as String? ?? '',
        periodLabel: json['period_label'] as String? ?? '',
        untouchedTargets: (json['untouched_targets'] as num?)?.toInt() ?? 0,
      );
}

class CommandCenterStage {
  const CommandCenterStage({
    required this.status,
    required this.label,
    required this.count,
  });

  final String status;
  final String label;
  final int count;

  factory CommandCenterStage.fromJson(Map<String, dynamic> json) =>
      CommandCenterStage(
        status: json['status'] as String? ?? '',
        label: json['label'] as String? ?? '',
        count: (json['count'] as num?)?.toInt() ?? 0,
      );
}

class CommandCenterSuggestion {
  const CommandCenterSuggestion({required this.label, required this.prompt});

  final String label;

  /// Sent to the copilot on tap. A suggestion the user has to retype is a
  /// suggestion nobody taps.
  final String prompt;

  factory CommandCenterSuggestion.fromJson(Map<String, dynamic> json) =>
      CommandCenterSuggestion(
        label: json['label'] as String? ?? '',
        prompt: json['prompt'] as String? ?? '',
      );
}

/// What the overnight run produced (§6.17, K2).
///
/// The reason to open the app in the morning: the queue filled itself while
/// nobody was watching.
class OvernightBriefing {
  const OvernightBriefing({
    required this.id,
    required this.drafted,
    this.skippedBudget = 0,
    this.usedBatch = false,
    this.stoppedReason,
  });

  final String id;
  final int drafted;

  /// How many the plan's monthly allowance did not cover. Shown alongside the
  /// count, because "40 prontas" when 100 were asked for needs the missing 60
  /// explained rather than left as a number that does not add up.
  final int skippedBudget;

  /// Whether the provider's batch endpoint did the work. The discount argument
  /// rests on it, so a run without it must not imply one.
  final bool usedBatch;

  /// Why the night gave up, when it did. Null covers both a normal run and
  /// a legitimately quiet one — an account with nobody left to write to has
  /// not failed.
  ///
  /// Until 2026-09-09 there was no such field and the briefing query filtered
  /// on `drafted > 0`, so nine consecutive broken nights were invisible and
  /// the card kept showing the last SUCCESSFUL run as if it were today's.
  final String? stoppedReason;

  bool get budgetHit => skippedBudget > 0;

  /// True when the night produced nothing because something about the account
  /// made every draft impossible.
  bool get blocked => (stoppedReason?.trim().isNotEmpty ?? false);

  factory OvernightBriefing.fromJson(Map<String, dynamic> json) =>
      OvernightBriefing(
        id: json['id'] as String? ?? '',
        drafted: (json['drafted'] as num?)?.toInt() ?? 0,
        skippedBudget: (json['skipped_budget'] as num?)?.toInt() ?? 0,
        usedBatch: json['used_batch'] as bool? ?? false,
        stoppedReason:
            (json['stopped_reason'] as String?)?.trim().isEmpty ?? true
                ? null
                : (json['stopped_reason'] as String).trim(),
      );
}

class CommandCenter {
  const CommandCenter({
    this.tiles = const CommandCenterTiles(),
    this.funnel = const [],
    this.actions = const [],
    this.totalActions = 0,
    this.suggestion,
    this.briefing,
    this.empty = false,
    this.warnings = const [],
  });

  final CommandCenterTiles tiles;
  final List<CommandCenterStage> funnel;

  /// Already ranked. Never re-sorted here.
  final List<PendingAction> actions;

  /// How many actions exist in total. Larger than [actions] means the list was
  /// truncated, and the screen says so — six rows with no total would read as
  /// "that is all of it".
  final int totalActions;

  /// At most one. Null is a legitimate state: a filler chip trains people to
  /// ignore the slot.
  final CommandCenterSuggestion? suggestion;

  /// The overnight run's news, when there is any. Null most mornings — the
  /// server only sends a briefing for a night that actually drafted something.
  final OvernightBriefing? briefing;

  /// No pipeline at all, so the screen shows the activation checklist instead
  /// of a wall of zeros a new user would read as "this product does nothing".
  final bool empty;

  /// Sections that failed to load, by name. Rendered as a caveat: an empty
  /// "needs you" is a CLAIM, and if a section did not load we have not earned
  /// the right to make it.
  final List<String> warnings;

  int get hiddenActions =>
      totalActions > actions.length ? totalActions - actions.length : 0;

  factory CommandCenter.fromJson(Map<String, dynamic> json) => CommandCenter(
        tiles: CommandCenterTiles.fromJson(
            (json['tiles'] as Map<String, dynamic>?) ?? const {}),
        funnel: ((json['funnel'] as List<dynamic>?) ?? const [])
            .map((e) => CommandCenterStage.fromJson(e as Map<String, dynamic>))
            .toList(),
        actions: ((json['actions'] as List<dynamic>?) ?? const [])
            .map((e) => PendingAction.fromJson(e as Map<String, dynamic>))
            .toList(),
        totalActions: (json['total_actions'] as num?)?.toInt() ?? 0,
        suggestion: json['suggestion'] == null
            ? null
            : CommandCenterSuggestion.fromJson(
                json['suggestion'] as Map<String, dynamic>),
        briefing: json['briefing'] == null
            ? null
            : OvernightBriefing.fromJson(
                json['briefing'] as Map<String, dynamic>),
        empty: json['empty'] as bool? ?? false,
        warnings: ((json['warnings'] as List<dynamic>?) ?? const [])
            .map((e) => e.toString())
            .toList(),
      );
}

/// `retry: null`, like the other repository reads: a failed load keeps its error
/// and its retry button instead of blinking back to a skeleton behind an
/// invisible backoff timer.
final commandCenterProvider = FutureProvider<CommandCenter>((ref) async {
  // Real-time: the tiles and the queue move when a message arrives, so the
  // screen re-reads on the events the hub already broadcasts rather than
  // polling. A dropped socket therefore degrades to pull-to-refresh, never to
  // silently stale numbers — the period label and the timestamps let the user
  // see how fresh the screen is.
  ref.watch(commandCenterRefreshProvider);
  final res = await ref
      .watch(dioProvider)
      .get<Map<String, dynamic>>('/me/command-center');
  return CommandCenter.fromJson(res.data ?? <String, dynamic>{});
}, retry: null);

/// A counter bumped by the WebSocket events that can change what the command
/// centre says.
///
/// A counter rather than `ref.invalidate` from a listener: the provider WATCHES
/// this, so a burst of ten inbound messages coalesces into the rebuilds Riverpod
/// schedules instead of ten overlapping requests.
class CommandCenterRefresh extends Notifier<int> {
  @override
  int build() {
    // Only the events that can actually move a tile or the queue. Subscribing
    // to everything would re-read the whole payload on a typing indicator.
    final subscription = ref.watch(webSocketServiceProvider).events.listen((e) {
      switch (e.event) {
        case 'new_message':
        case 'conversation_updated':
          state = state + 1;
      }
    });
    ref.onDispose(subscription.cancel);
    return 0;
  }
}

final commandCenterRefreshProvider =
    NotifierProvider<CommandCenterRefresh, int>(CommandCenterRefresh.new);

final commandCenterActionsProvider = Provider<CommandCenterActions>(
  (ref) => CommandCenterActions(ref.watch(dioProvider)),
);

class CommandCenterActions {
  const CommandCenterActions(this._dio);

  final Dio _dio;

  /// Acknowledges the morning briefing so it does not reappear all day.
  Future<void> markBriefingSeen(String runId) =>
      _dio.post<dynamic>('/me/overnight-drafting/$runId/seen');

  /// Snoozes one row for THIS user.
  ///
  /// Named snooze, not complete: the rows are derived from live state, so the
  /// only honest thing this can do without doing the work is hide it for a
  /// while. It always comes back.
  /// How long a snooze lasts. Named here rather than typed twice, because
  /// the screen has to TELL the person the number it just chose.
  static const snoozeHours = 72;

  Future<void> snooze(
    PendingAction action, {
    int hours = snoozeHours,
    String note = '',
  }) =>
      _dio.post<dynamic>(
        '/me/pending-actions/dismiss',
        data: <String, dynamic>{
          'kind': action.kind.wire,
          'target_id': action.targetId,
          'snooze_hours': hours,
          if (note.isNotEmpty) 'note': note,
        },
      );
}
