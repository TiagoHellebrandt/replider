import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:app/core/utils/account_currency.dart';
import 'package:app/core/analytics/analytics_client.dart';
import 'package:app/core/auth/account_role.dart';
import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/utils/currency_format.dart';
import 'package:app/design_system/components/glass_card.dart';
import 'package:app/design_system/components/gradient_pill.dart';
import 'package:app/design_system/components/retryable_error.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/home/presentation/controllers/command_center_controller.dart';
import 'package:app/features/home/presentation/controllers/onboarding_checklist_dismissal_controller.dart';
import 'package:app/features/home/presentation/controllers/onboarding_progress_controller.dart';
import 'package:app/features/sales/presentation/controllers/approval_queue_controller.dart';
import 'package:app/features/sales/presentation/controllers/lead_detail_controller.dart';
import 'package:app/features/sales/presentation/controllers/radar_controller.dart';
import 'package:app/features/sales/presentation/widgets/bulk_assign_dialog.dart';
import 'package:app/i18n/strings.g.dart';

/// "Início" — the morning briefing (canvas "Início — Briefing").
///
/// One question: what needs me right now? A greeting that already answers it
/// in one line, the overnight hero when there is news, one stat strip, and the
/// "precisa de você" list partitioned by kind — plus, on a wide screen, a
/// right rail with the Radar's first hits, the week's outcome and the
/// activation engine.
///
/// Three rules hold it honest:
///
///  * The ranking comes from the server and is never re-sorted here. Grouping
///    by kind keeps each group in payload order, and the groups themselves
///    follow the rank of their first item — so the top of the screen is still
///    the server's #1.
///  * A section that failed to load says so. An empty "needs você" is a CLAIM,
///    and a query that blew up has not earned the right to make it. The rail
///    cards are the exception the other way: they are garnish, so an errored
///    feed renders NOTHING rather than an empty card.
///  * Exactly one copilot chip, and only when there is something to suggest.
class StartScreen extends ConsumerWidget {
  const StartScreen({super.key});

  /// Below this the right rail folds under the main column.
  static const railBreakpoint = 1180.0;

  /// Below this only the engine card survives the fold — radar and week are
  /// glanceable extras, and on a phone glanceable means "in the way".
  static const stackBreakpoint = 900.0;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final state = ref.watch(commandCenterProvider);
    final role =
        AccountRole.from(ref.watch(authControllerProvider).asData?.value?.role);
    // The Radar's tab (and its server routes) admit attendant and above; a
    // vendedora gets no rail card rather than a 403 on every visit.
    final canRadar = role.satisfies(AccountRole.attendant);

    return Scaffold(
      backgroundColor: colors.backgroundPrimary,
      body: SafeArea(
        child: state.when(
          loading: () => const _StartSkeleton(),
          error: (err, _) => Padding(
            padding: const EdgeInsets.all(24),
            child: RetryableError(
              error: err,
              onRetry: () => ref.invalidate(commandCenterProvider),
            ),
          ),
          data: (data) => RefreshIndicator(
            onRefresh: () async => ref.invalidate(commandCenterProvider),
            child: LayoutBuilder(builder: (context, box) {
              final wide = box.maxWidth >= railBreakpoint;
              final medium = box.maxWidth >= stackBreakpoint;

              final main = Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  if (data.empty)
                    // A brand-new account reading "0 · 0 · 0 · R$ 0" learns
                    // that the product does nothing. Getting the first leads
                    // in is the only thing worth doing, so it takes the
                    // screen.
                    _Activation(suggestion: data.suggestion)
                  else ...[
                    // The reason to open the app this morning goes above the
                    // numbers: it is news, and news read after a dashboard is
                    // news nobody read.
                    if (data.briefing != null)
                      _Briefing(briefing: data.briefing!, canRadar: canRadar),
                    _StatStrip(data: data),
                    const SizedBox(height: 16),
                    if (data.suggestion != null)
                      _SuggestionChip(suggestion: data.suggestion!),
                    _NeedsYou(data: data),
                    const SizedBox(height: 16),
                    _MiniFunnel(stages: data.funnel),
                  ],
                ],
              );

              final rail = <Widget>[
                if (canRadar) const _RailRadar(),
                const _RailWeek(),
                const _RailEngine(),
              ];

              return SingleChildScrollView(
                padding: const EdgeInsets.all(24),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    _Header(data: data),
                    const SizedBox(height: 20),
                    if (wide)
                      Row(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Expanded(child: main),
                          const SizedBox(width: 24),
                          SizedBox(
                            width: 340,
                            child: Column(
                              key: const Key('start_rail'),
                              children: [
                                for (final card in rail)
                                  Padding(
                                    padding: const EdgeInsets.only(bottom: 16),
                                    child: card,
                                  ),
                              ],
                            ),
                          ),
                        ],
                      )
                    else ...[
                      main,
                      // Folded rail: the engine first — it is the one card
                      // that tells a new account what to do next.
                      const SizedBox(height: 16),
                      const _RailEngine(),
                      if (medium) ...[
                        const SizedBox(height: 16),
                        if (canRadar) ...[
                          const _RailRadar(),
                          const SizedBox(height: 16),
                        ],
                        const _RailWeek(),
                      ],
                    ],
                  ],
                ),
              );
            }),
          ),
        ),
      ),
    );
  }
}

/// "Bom dia, {nome}" + the date + a one-line summary of the morning, derived
/// from the loaded payload — never from a second request.
class _Header extends ConsumerWidget {
  const _Header({required this.data});

  final CommandCenter data;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final now = DateTime.now();

    final name =
        (ref.watch(authControllerProvider).asData?.value?.name ?? '').trim();
    final first = name.isEmpty ? '' : name.split(' ').first;
    final greeting = first.isEmpty
        ? t.shell.start_title
        : now.hour < 12
            ? t.start.greeting_morning(name: first)
            : now.hour < 18
                ? t.start.greeting_afternoon(name: first)
                : t.start.greeting_evening(name: first);

    final waiting = data.actions
        .where((a) => a.kind == PendingActionKind.waitingReply)
        .length;
    final today = DateTime(now.year, now.month, now.day);
    final dueToday = data.actions.where((a) {
      final due = a.dueOn;
      if (due == null) return false;
      return !DateTime(due.year, due.month, due.day).isAfter(today);
    }).length;

    final fragments = <(Color, String)>[
      if (data.tiles.pendingApproval > 0)
        (
          colors.orangeAlert,
          t.start.summary_ready(count: data.tiles.pendingApproval)
        ),
      if (waiting > 0)
        (colors.bluePrimary, t.start.summary_waiting(count: waiting)),
      dueToday > 0
          ? (colors.orangeAlert, t.start.summary_due_today(count: dueToday))
          : (colors.textMuted, t.start.summary_due_none),
    ];

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          greeting,
          key: const Key('start_greeting'),
          style: typography.display.copyWith(
              fontSize: 26, color: Colors.white, fontWeight: FontWeight.w700),
        ),
        const SizedBox(height: 6),
        Wrap(
          key: const Key('start_summary'),
          spacing: 12,
          runSpacing: 4,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: [
            Text(
              t.start.date_line(
                weekday: t.start.weekdays[now.weekday - 1],
                day: '${now.day}',
                month: t.command_center.months[now.month - 1],
              ),
              style: typography.body
                  .copyWith(color: colors.textSecondary, fontSize: 13),
            ),
            for (final (dot, label) in fragments)
              Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Container(
                    width: 6,
                    height: 6,
                    decoration:
                        BoxDecoration(color: dot, shape: BoxShape.circle),
                  ),
                  const SizedBox(width: 6),
                  Text(
                    label,
                    style: typography.body
                        .copyWith(color: colors.textSecondary, fontSize: 13),
                  ),
                ],
              ),
          ],
        ),
      ],
    );
  }
}

/// "Your SDR worked overnight" (§6.17, K2) — the hero.
///
/// Shown once per night that actually produced something — the server only
/// sends a briefing for a run with drafts in it, because "0 abordagens
/// prontas" every morning is how a good habit becomes a dismissed banner.
class _Briefing extends ConsumerWidget {
  const _Briefing({required this.briefing, required this.canRadar});

  final OvernightBriefing briefing;
  final bool canRadar;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final gradients = Theme.of(context).extension<AppGradientsExtension>()!;
    final card = colors.backgroundSecondary;

    return Padding(
      padding: const EdgeInsets.only(bottom: 16),
      child: Container(
        key: const Key('start_briefing'),
        padding: const EdgeInsets.all(18),
        decoration: BoxDecoration(
          // A blue wash fading into the card color — the night shift's card
          // is allowed to glow a little.
          gradient: LinearGradient(
            begin: Alignment.topLeft,
            end: Alignment.bottomRight,
            stops: const [0, 0.46],
            colors: [
              Color.alphaBlend(
                  colors.bluePrimary.withValues(alpha: 0.14), card),
              card,
            ],
          ),
          borderRadius: BorderRadius.circular(20),
          border: Border.all(color: colors.bluePrimary.withValues(alpha: 0.28)),
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Container(
              width: 46,
              height: 46,
              decoration: BoxDecoration(
                gradient: gradients.primaryButton,
                borderRadius: BorderRadius.circular(12),
              ),
              child: Icon(
                  briefing.blocked
                      ? Icons.error_outline
                      : Icons.dark_mode_outlined,
                  size: 22,
                  color: Colors.white),
            ),
            const SizedBox(width: 14),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    briefing.blocked
                        ? t.overnight.blocked_title
                        : t.overnight.title,
                    style: typography.body.copyWith(
                        color: Colors.white,
                        fontSize: 15,
                        fontWeight: FontWeight.w700),
                  ),
                  const SizedBox(height: 2),
                  Text(
                    // A blocked night says WHAT stopped it. "0 abordagens
                    // prontas" is the same sentence a quiet night writes,
                    // and telling the two apart is the whole point.
                    briefing.blocked
                        ? t.overnight.blocked_body
                        : t.overnight.drafted(count: briefing.drafted),
                    key: Key(briefing.blocked
                        ? 'start_briefing_blocked'
                        : 'start_briefing_count'),
                    style: typography.body
                        .copyWith(color: colors.textSecondary, fontSize: 13),
                  ),
                  if (briefing.blocked)
                    Padding(
                      padding: const EdgeInsets.only(top: 2),
                      child: Text(
                        briefing.stoppedReason!,
                        key: const Key('start_briefing_blocked_reason'),
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: typography.body
                            .copyWith(color: colors.orangeAlert, fontSize: 11),
                      ),
                    ),
                  if (briefing.budgetHit)
                    Text(
                      // Without this, "40 prontas" when 100 were asked for is
                      // a number that does not add up.
                      t.overnight.budget_hit(count: briefing.skippedBudget),
                      key: const Key('start_briefing_budget'),
                      style: typography.body
                          .copyWith(color: colors.orangeAlert, fontSize: 11),
                    ),
                  const SizedBox(height: 12),
                  Wrap(
                    spacing: 8,
                    runSpacing: 8,
                    children: [
                      GradientPill(
                        key: const Key('start_briefing_review'),
                        // A blocked night has nothing to review — the button
                        // goes where the fix is instead.
                        label: briefing.blocked
                            ? t.overnight.blocked_cta
                            : t.start.review_n(count: '${briefing.drafted}'),
                        height: 42,
                        fontSize: 13,
                        onTap: () => _acknowledge(
                            context,
                            ref,
                            briefing.blocked
                                ? '/sales/products'
                                : '/prospect?tab=queue'),
                      ),
                      if (canRadar)
                        GhostPill(
                          key: const Key('start_briefing_radar'),
                          label: t.start.see_radar,
                          height: 42,
                          fontSize: 13,
                          onTap: () => _acknowledge(
                              context, ref, '/prospect?tab=signals'),
                        ),
                    ],
                  ),
                ],
              ),
            ),
            IconButton(
              key: const Key('start_briefing_dismiss'),
              tooltip: t.overnight.dismiss,
              icon: Icon(Icons.close, size: 16, color: colors.textSecondary),
              onPressed: () => _acknowledge(context, ref, null),
            ),
          ],
        ),
      ),
    );
  }

  /// Marks it read either way: reviewing the queue IS reading the briefing, so
  /// leaving it on screen after the tap would nag somebody who already acted.
  Future<void> _acknowledge(
      BuildContext context, WidgetRef ref, String? destination) async {
    try {
      await ref
          .read(commandCenterActionsProvider)
          .markBriefingSeen(briefing.id);
    } catch (_) {
      // Best-effort: failing to record that it was read must not block the
      // navigation the person asked for. It reappears tomorrow at worst.
    }
    ref.invalidate(commandCenterProvider);
    if (destination != null && context.mounted) context.go(destination);
  }
}

/// The empty state: no pipeline at all.
class _Activation extends ConsumerWidget {
  const _Activation({required this.suggestion});

  final CommandCenterSuggestion? suggestion;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    // Both CTAs land on the Radar, whose tab (and server routes) admit
    // attendant and above. A vendedor following either link fell through to
    // her queue — a button that goes somewhere else than it says.
    final canOpenRadar =
        AccountRole.from(ref.watch(authControllerProvider).asData?.value?.role)
            .satisfies(AccountRole.attendant);

    return GlassCard(
      key: const Key('start_activation'),
      padding: const EdgeInsets.all(18),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            t.command_center.activation_title,
            style: typography.body.copyWith(
                color: Colors.white, fontSize: 16, fontWeight: FontWeight.w700),
          ),
          const SizedBox(height: 6),
          Text(
            t.command_center.activation_body,
            style: typography.body
                .copyWith(color: colors.textSecondary, fontSize: 13),
          ),
          const SizedBox(height: 14),
          if (canOpenRadar)
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                FilledButton.icon(
                  key: const Key('start_activation_prospect'),
                  // The "go prospecting" button opens the Hunt (Places search)
                  // — the actual find-a-customer action. Bare /prospect opens
                  // the approval queue, empty on the account this card exists
                  // for; the signals button below covers the Radar, so this one
                  // must not also land there.
                  onPressed: () => context.go('/prospect?tab=hunt'),
                  icon: const Icon(Icons.travel_explore_outlined, size: 16),
                  label: Text(t.shell.start_go_prospecting),
                ),
                OutlinedButton.icon(
                  key: const Key('start_activation_signals'),
                  onPressed: () => context.go('/sales/signals'),
                  icon: const Icon(Icons.campaign_outlined, size: 16),
                  label: Text(t.command_center.activation_signals),
                ),
              ],
            ),
          if (suggestion != null) ...[
            const SizedBox(height: 12),
            _SuggestionChip(suggestion: suggestion!),
          ],
        ],
      ),
    );
  }
}

/// The stat strip: one card, four cells, the old four tiles fused.
class _StatStrip extends StatelessWidget {
  const _StatStrip({required this.data});

  final CommandCenter data;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final tiles = data.tiles;
    final currency = tiles.currency.isNotEmpty
        ? tiles.currency
        : (AccountCurrencyCache.value ?? 'brl');
    final wonZero = tiles.wonGrossCents == 0;

    // Every number leads to the place where it is work. A number that shouts
    // and leads nowhere teaches people to ignore the number — and a number a
    // salesperson cannot act on in three seconds is that number.
    //
    // This strip carried five. Walking the product as a seller, four of the
    // five were unusable on a working account: "69 leads ativos" names no
    // action (the board is one tap below and the mini-funnel already breaks
    // it down), "0 reuniões" is a zero, and "2790 nunca abordados" is the
    // size of Mato Grosso — the Radar's own banner already says "2794
    // hospitais em MT declaram equipamento que seu catálogo atende", in the
    // place where you can act on it, so the tile was a louder duplicate of a
    // sentence that reads better in context.
    //
    // What is left is one action and one scoreboard. A tile that only has
    // something to say when it is non-zero says it then.
    final cells = <Widget>[
      _StatCell(
        keyName: 'start_tile_pending',
        label: t.shell.stat_pending_approval,
        value: '${tiles.pendingApproval}',
        // The one number that means "someone is waiting on us". Shown even
        // at zero: "nothing is waiting" is worth reading.
        valueColor: tiles.pendingApproval > 0 ? colors.orangeAlert : null,
        route: '/prospect?tab=queue',
      ),
      // A meeting on the calendar IS an action. Zero of them is not, so the
      // tile appears when there is something to appear for.
      if (tiles.meetings > 0)
        _StatCell(
          keyName: 'start_tile_meetings',
          label: t.command_center.tile_meetings,
          value: '${tiles.meetings}',
          footnote: _humanPeriod(tiles.periodLabel),
          footnoteKey: 'start_tile_meetings_period',
          route: '/funnel',
        ),
      _StatCell(
        keyName: 'start_tile_won',
        label: t.command_center.tile_won,
        value: formatCurrencyCents(tiles.wonGrossCents, currency),
        // R$ 0 in bold white claims a scoreboard that has not started. Muted
        // plus a way in beats a shout about nothing — and this footnote is
        // the only route to registering a sale.
        valueColor: wonZero ? colors.textMuted : null,
        valueWeight: wonZero ? FontWeight.w600 : FontWeight.w700,
        footnote: wonZero
            ? t.start.register_sale_link
            : _humanPeriod(tiles.periodLabel),
        footnoteKey: wonZero ? 'start_register_sale' : 'start_tile_won_period',
        footnoteColor: wonZero ? colors.bluePrimary : null,
        route: '/funnel?view=portfolio',
      ),
    ];

    final divider = Container(
      width: 1,
      height: 52,
      color: Colors.white.withValues(alpha: 0.07),
    );

    return GlassCard(
      padding: EdgeInsets.zero,
      child: LayoutBuilder(builder: (context, box) {
        // Four across needs ~150px each; below that the labels wrap one
        // character per line and the strip becomes unreadable.
        if (box.maxWidth >= 640) {
          return Row(
            children: [
              for (var i = 0; i < cells.length; i++) ...[
                if (i > 0) divider,
                Expanded(child: cells[i]),
              ],
            ],
          );
        }
        final hDivider = Container(
          height: 1,
          color: Colors.white.withValues(alpha: 0.07),
        );
        return Column(
          children: [
            Row(children: [
              Expanded(child: cells[0]),
              divider,
              Expanded(child: cells[1]),
            ]),
            hDivider,
            Row(children: [
              Expanded(child: cells[2]),
              divider,
              Expanded(child: cells[3]),
            ]),
          ],
        );
      }),
    );
  }
}

class _StatCell extends StatefulWidget {
  const _StatCell({
    required this.keyName,
    required this.label,
    required this.value,
    required this.route,
    this.valueColor,
    this.valueWeight = FontWeight.w700,
    this.footnote = '',
    this.footnoteKey = '',
    this.footnoteColor,
  });

  final String keyName;
  final String label;
  final String value;

  /// Where this number becomes work.
  final String route;
  final Color? valueColor;
  final FontWeight valueWeight;
  final String footnote;
  final String footnoteKey;
  final Color? footnoteColor;

  @override
  State<_StatCell> createState() => _StatCellState();
}

class _StatCellState extends State<_StatCell> {
  bool _hover = false;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    return MouseRegion(
      onEnter: (_) => setState(() => _hover = true),
      onExit: (_) => setState(() => _hover = false),
      child: InkWell(
        key: Key('${widget.keyName}_open'),
        onTap: () => context.go(widget.route),
        child: SizedBox(
          height: 84,
          child: Stack(
            children: [
              Padding(
                padding:
                    const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Text(
                      widget.value,
                      key: Key(widget.keyName),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: typography.display.copyWith(
                        fontSize: 23,
                        fontWeight: widget.valueWeight,
                        color: widget.valueColor ?? Colors.white,
                      ),
                    ),
                    Text(
                      widget.label,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: typography.body
                          .copyWith(color: colors.textSecondary, fontSize: 12),
                    ),
                    if (widget.footnote.isNotEmpty)
                      Text(
                        widget.footnote,
                        key: Key(widget.footnoteKey),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: typography.body.copyWith(
                            color: widget.footnoteColor ?? colors.textTertiary,
                            fontSize: 11),
                      ),
                  ],
                ),
              ),
              if (_hover)
                Positioned(
                  top: 8,
                  right: 10,
                  child: Text('›',
                      style:
                          TextStyle(color: colors.textSecondary, fontSize: 14)),
                ),
            ],
          ),
        ),
      ),
    );
  }
}

/// The single copilot chip. One, never a list: a screen with three suggestions
/// is a screen where all three get ignored.
class _SuggestionChip extends StatelessWidget {
  const _SuggestionChip({required this.suggestion});

  final CommandCenterSuggestion suggestion;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Align(
        alignment: Alignment.centerLeft,
        child: ActionChip(
          key: const Key('start_suggestion'),
          avatar: Icon(Icons.auto_awesome, size: 16, color: colors.bluePrimary),
          label: Text(suggestion.label),
          // The prompt travels in the deep link: a suggestion the user has to
          // retype is a suggestion nobody taps.
          onPressed: () => context.go(
              '/copilot?prompt=${Uri.encodeQueryComponent(suggestion.prompt)}'),
        ),
      ),
    );
  }
}

/// "Precisa de você", already ranked by the server, partitioned by kind.
class _NeedsYou extends ConsumerStatefulWidget {
  const _NeedsYou({required this.data});

  final CommandCenter data;

  @override
  ConsumerState<_NeedsYou> createState() => _NeedsYouState();
}

class _NeedsYouState extends ConsumerState<_NeedsYou> {
  /// The selected chip. Filters the LOADED list only — never a new request.
  PendingActionKind? _filter;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final data = widget.data;

    // Partition preserving the server's rank: each group keeps payload order,
    // and the groups follow the rank of their first item. NEVER re-sorted —
    // two screens ordering the same work differently is how a team stops
    // agreeing about what matters.
    final groups = <PendingActionKind, List<PendingAction>>{};
    for (final action in data.actions) {
      groups.putIfAbsent(action.kind, () => []).add(action);
    }
    final visibleKinds = _filter == null
        ? groups.keys.toList()
        : groups.keys.where((k) => k == _filter).toList();

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Wrap(
          spacing: 8,
          runSpacing: 6,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: [
            Text(
              t.shell.needs_you_title,
              style: typography.body.copyWith(
                  color: colors.textSecondary,
                  fontSize: 12,
                  fontWeight: FontWeight.w600),
            ),
            if (data.actions.isNotEmpty) ...[
              _chip(
                key: 'start_chip_all',
                label: t.start.filter_all(count: data.actions.length),
                selected: _filter == null,
                onTap: () => setState(() => _filter = null),
              ),
              for (final entry in groups.entries)
                _chip(
                  key: 'start_chip_${entry.key.wire}',
                  label: '${_labelFor(entry.key)} ${entry.value.length}',
                  selected: _filter == entry.key,
                  onTap: () => setState(() => _filter = entry.key),
                ),
              Text(
                t.start.sorted_by_urgency,
                style: typography.body
                    .copyWith(color: colors.textMuted, fontSize: 11),
              ),
            ],
          ],
        ),
        const SizedBox(height: 8),
        if (data.warnings.isNotEmpty)
          // An empty list is a claim. If a section did not load, say so
          // instead of implying nothing needs anybody.
          Padding(
            padding: const EdgeInsets.only(bottom: 8),
            child: Row(
              children: [
                Icon(Icons.error_outline, size: 14, color: colors.orangeAlert),
                const SizedBox(width: 6),
                Expanded(
                  child: Text(
                    t.command_center.partial_load,
                    key: const Key('start_partial'),
                    style: typography.body
                        .copyWith(color: colors.orangeAlert, fontSize: 11),
                  ),
                ),
              ],
            ),
          ),
        if (data.actions.isEmpty)
          Text(
            t.shell.needs_you_clear,
            key: const Key('start_needs_clear'),
            style: typography.body
                .copyWith(color: colors.textSecondary, fontSize: 13),
          )
        else
          GlassCard(
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                for (final kind in visibleKinds) ...[
                  _GroupHeader(kind: kind, actions: groups[kind]!),
                  for (final action in groups[kind]!)
                    _ActionRow(action: action),
                ],
                if (data.hiddenActions > 0)
                  // Six rows with no total would read as "that is all of it".
                  Center(
                    child: Padding(
                      padding: const EdgeInsets.symmetric(vertical: 8),
                      child: Text(
                        t.command_center
                            .more_actions(count: data.hiddenActions),
                        key: const Key('start_more_actions'),
                        style: typography.body.copyWith(
                            color: colors.bluePrimary, fontSize: 12.5),
                      ),
                    ),
                  ),
              ],
            ),
          ),
      ],
    );
  }

  Widget _chip({
    required String key,
    required String label,
    required bool selected,
    required VoidCallback onTap,
  }) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return InkWell(
      key: Key(key),
      onTap: onTap,
      borderRadius: BorderRadius.circular(999),
      child: Container(
        height: 26,
        padding: const EdgeInsets.symmetric(horizontal: 10),
        alignment: Alignment.center,
        decoration: BoxDecoration(
          color: selected
              ? colors.bluePrimary.withValues(alpha: 0.14)
              : Colors.white.withValues(alpha: 0.05),
          borderRadius: BorderRadius.circular(999),
          border: Border.all(
              color: selected ? colors.bluePrimary : colors.borderLight),
        ),
        child: Text(
          label,
          style: TextStyle(
            fontSize: 11.5,
            fontWeight: FontWeight.w600,
            color: selected ? colors.bluePrimary : colors.textSecondary,
          ),
        ),
      ),
    );
  }
}

/// One group's header: what these rows are, how many, and — for the unowned
/// group — the door to hand them all over at once.
class _GroupHeader extends ConsumerWidget {
  const _GroupHeader({required this.kind, required this.actions});

  final PendingActionKind kind;
  final List<PendingAction> actions;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    // Same gate as the server's POST /leads/bulk-assign
    // (RequireRole admin/owner/gestor): a vendedora would confirm a dialog
    // into a 403.
    final canDistribute = kind == PendingActionKind.unownedLead &&
        AccountRole.from(ref.watch(authControllerProvider).asData?.value?.role)
            .satisfies(AccountRole.gestor);

    return Padding(
      padding: const EdgeInsets.only(top: 10, bottom: 4),
      child: Row(
        children: [
          Expanded(
            child: Text.rich(
              TextSpan(children: [
                TextSpan(
                  text: _labelFor(kind),
                  style: typography.body.copyWith(
                      color: Colors.white,
                      fontSize: 12,
                      fontWeight: FontWeight.w700),
                ),
                TextSpan(
                  text: '  ${actions.length}',
                  style: typography.body
                      .copyWith(color: colors.textMuted, fontSize: 12),
                ),
              ]),
            ),
          ),
          if (canDistribute)
            GhostPill(
              key: const Key('start_distribute_unowned'),
              label: t.start.distribute_team,
              height: 26,
              fontSize: 11.5,
              padding: const EdgeInsets.symmetric(horizontal: 10),
              onTap: () => _distribute(context, ref),
            ),
        ],
      ),
    );
  }

  /// The same bulk hand-over the Kanban header offers, on the same dialog —
  /// sixty-four unowned leads, one at a time, is work nobody does.
  Future<void> _distribute(BuildContext context, WidgetRef ref) async {
    final ids = [for (final a in actions) a.targetId];
    final res =
        await showBulkAssignDialog(context, leadIds: ids, filtered: false);
    if (res == null || !context.mounted) return;
    ref.invalidate(commandCenterProvider);
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      key: const Key('bulk_assign_result'),
      content: Text(res.skipped == 0
          ? t.board.bulk_assign_done(count: res.assigned.toString())
          : t.board.bulk_assign_partial(
              count: res.assigned.toString(), skipped: res.skipped.toString())),
    ));
  }
}

class _ActionRow extends ConsumerWidget {
  const _ActionRow({required this.action});

  final PendingAction action;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final now = DateTime.now();
    final overdue = action.overdue(now);
    final tint = _tintFor(action.kind, colors);
    final age = _ageLabel(action, now);
    final waitedLong =
        action.sinceAt != null && now.difference(action.sinceAt!).inDays > 7;
    final hot = overdue || waitedLong;

    return InkWell(
      key: Key('start_action_${action.targetId}'),
      onTap: () => context.go(action.deepLink),
      borderRadius: BorderRadius.circular(10),
      child: SizedBox(
        height: 56,
        child: Row(
          children: [
            Container(
              width: 34,
              height: 34,
              decoration: BoxDecoration(
                color: tint.withValues(alpha: 0.13),
                borderRadius: BorderRadius.circular(10),
              ),
              child: Icon(_iconFor(action.kind), size: 17, color: tint),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    // The customer's name is the headline. Without it the row
                    // is a statistic; with it, it is a phone call.
                    action.customerName.isEmpty
                        ? _labelFor(action.kind)
                        : action.customerName,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: typography.body.copyWith(
                        color: Colors.white,
                        fontSize: 14,
                        fontWeight: FontWeight.w600),
                  ),
                  Text(
                    action.detail.isEmpty
                        ? _labelFor(action.kind)
                        : action.detail,
                    key: Key('start_action_sub_${action.targetId}'),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: typography.body
                        .copyWith(color: colors.textSecondary, fontSize: 12),
                  ),
                ],
              ),
            ),
            if (action.hasMoney)
              Padding(
                padding: const EdgeInsets.only(left: 8),
                child: Text(
                  formatCurrencyCents(
                      action.moneyAtStakeCents,
                      action.currency.isEmpty
                          ? (AccountCurrencyCache.value ?? 'brl')
                          : action.currency),
                  key: Key('start_action_money_${action.targetId}'),
                  style: typography.body.copyWith(
                      color: Colors.white,
                      fontSize: 13,
                      fontWeight: FontWeight.w600),
                ),
              ),
            if (age != null) ...[
              const SizedBox(width: 8),
              Container(
                height: 24,
                padding: const EdgeInsets.symmetric(horizontal: 9),
                alignment: Alignment.center,
                decoration: BoxDecoration(
                  color: hot
                      ? colors.orangeAlert.withValues(alpha: 0.12)
                      : Colors.white.withValues(alpha: 0.06),
                  borderRadius: BorderRadius.circular(999),
                ),
                child: Text(
                  age,
                  key: Key('start_action_age_${action.targetId}'),
                  style: TextStyle(
                    fontSize: 11,
                    fontWeight: FontWeight.w600,
                    color: hot ? colors.orangeAlert : colors.textSecondary,
                  ),
                ),
              ),
            ],
            const SizedBox(width: 8),
            _primaryAction(context, ref),
            IconButton(
              key: Key('start_action_snooze_${action.targetId}'),
              icon: Icon(Icons.schedule, size: 16, color: colors.textSecondary),
              // "Depois", not "feito": the row is derived from live state, so
              // the only honest thing this button can do without doing the
              // work is hide it for a while.
              tooltip: t.command_center.snooze,
              onPressed: () => _snooze(context, ref),
            ),
          ],
        ),
      ),
    );
  }

  /// The row's own verb. Every route comes from the server's deep link — the
  /// same place the row tap goes — so a new kind needs no client release.
  Widget _primaryAction(BuildContext context, WidgetRef ref) {
    switch (action.kind) {
      case PendingActionKind.waitingReply:
        // A customer waiting is THE action of the screen, so it gets the loud
        // pill. The deep link is the conversation surface the server chose.
        return GradientPill(
          key: Key('start_action_go_${action.targetId}'),
          label: t.start.action_reply,
          height: 30,
          fontSize: 12,
          onTap: () => context.go(action.deepLink),
        );
      case PendingActionKind.approveDraft:
        return _ghost(context, t.overnight.review);
      case PendingActionKind.untouchedTarget:
        // The verb is "look at it", not "act on it": nothing has been promised
        // to this hospital, and a pill saying "abordar" would imply the
        // message is already written.
        return _ghost(context, t.command_center.action_see_target);
      case PendingActionKind.reorderDue:
        return _ghost(context, t.radar.prepare_offer);
      case PendingActionKind.unownedLead:
        // "Assumir" only for who the server lets: PATCH /leads/:id/owner is
        // RequireRole(admin/owner/gestor). Below that, the honest verb is
        // "open".
        final me = ref.watch(authControllerProvider).asData?.value;
        if (me != null &&
            AccountRole.from(me.role).satisfies(AccountRole.gestor)) {
          return GhostPill(
            key: Key('start_action_go_${action.targetId}'),
            label: t.start.action_assume,
            height: 30,
            fontSize: 12,
            onTap: () => _assume(context, ref, me.id),
          );
        }
        return _ghost(context, t.shell.needs_open);
      case PendingActionKind.signatureViewed:
      case PendingActionKind.contractAlert:
      case PendingActionKind.confirmSale:
      case PendingActionKind.unknown:
        return _ghost(context, t.shell.needs_open);
    }
  }

  Widget _ghost(BuildContext context, String label) => GhostPill(
        key: Key('start_action_go_${action.targetId}'),
        label: label,
        height: 30,
        fontSize: 12,
        onTap: () => context.go(action.deepLink),
      );

  /// Takes the lead for myself — the one-tap version of the distribute
  /// dialog, using the owner endpoint the lead panel already uses.
  Future<void> _assume(
      BuildContext context, WidgetRef ref, String userId) async {
    try {
      await ref
          .read(leadActionsProvider)
          .assignOwner(leadId: action.targetId, ownerUserId: userId);
      ref.invalidate(commandCenterProvider);
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text(t.board.bulk_assign_done(count: '1'))));
      }
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(parseApiError(e))));
      }
    }
  }

  Future<void> _snooze(BuildContext context, WidgetRef ref) async {
    try {
      await ref.read(commandCenterActionsProvider).snooze(action);
      ref.invalidate(commandCenterProvider);
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
              content: Text(t.command_center
                  .snoozed(days: '${CommandCenterActions.snoozeHours ~/ 24}'))),
        );
      }
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(parseApiError(e))));
      }
    }
  }
}

IconData _iconFor(PendingActionKind kind) => switch (kind) {
      PendingActionKind.approveDraft => Icons.rule_outlined,
      PendingActionKind.unownedLead => Icons.person_off_outlined,
      PendingActionKind.signatureViewed => Icons.draw_outlined,
      PendingActionKind.contractAlert => Icons.event_busy_outlined,
      PendingActionKind.confirmSale => Icons.fact_check_outlined,
      PendingActionKind.reorderDue => Icons.autorenew_outlined,
      PendingActionKind.waitingReply => Icons.mark_chat_unread_outlined,
      PendingActionKind.untouchedTarget => Icons.travel_explore_outlined,
      PendingActionKind.unknown => Icons.notifications_outlined,
    };

Color _tintFor(PendingActionKind kind, AppColorsExtension colors) =>
    switch (kind) {
      PendingActionKind.unownedLead ||
      PendingActionKind.contractAlert ||
      PendingActionKind.signatureViewed =>
        colors.orangeAlert,
      PendingActionKind.waitingReply => colors.bluePrimary,
      PendingActionKind.reorderDue ||
      PendingActionKind.confirmSale =>
        colors.limeGlow,
      PendingActionKind.approveDraft => colors.indigoDark,
      // Muted: this is ground, not somebody waiting. A cold row painted the
      // same colour as an unanswered customer teaches people to distrust the
      // colour.
      PendingActionKind.untouchedTarget => colors.textMuted,
      PendingActionKind.unknown => colors.bluePrimary,
    };

String _labelFor(PendingActionKind kind) => switch (kind) {
      PendingActionKind.approveDraft => t.command_center.kind_approve_draft,
      PendingActionKind.unownedLead => t.command_center.kind_unowned_lead,
      PendingActionKind.signatureViewed =>
        t.command_center.kind_signature_viewed,
      PendingActionKind.contractAlert => t.command_center.kind_contract_alert,
      PendingActionKind.confirmSale => t.command_center.kind_confirm_sale,
      PendingActionKind.reorderDue => t.command_center.kind_reorder_due,
      PendingActionKind.waitingReply => t.command_center.kind_waiting_reply,
      PendingActionKind.untouchedTarget =>
        t.command_center.kind_untouched_target,
      PendingActionKind.unknown => t.command_center.kind_unknown,
    };

/// How late, or how long it has waited — the age chip's text. Null when the
/// action carries neither a deadline nor a start.
String? _ageLabel(PendingAction action, DateTime now) {
  final due = action.dueOn;
  if (due != null) {
    // Date-only on BOTH sides. Subtracting timestamps and taking inDays
    // truncates toward zero, so a deadline three days ago at 08:00 read as
    // "2 days overdue" — off by one in the direction that makes a late thing
    // look less late.
    final today = DateTime(now.year, now.month, now.day);
    final dueDate = DateTime(due.year, due.month, due.day);
    final days = dueDate.difference(today).inDays;
    if (days < 0) return t.command_center.overdue_days(count: -days);
    if (days == 0) return t.command_center.due_today;
    return t.command_center.due_in_days(count: days);
  }
  final since = action.sinceAt;
  if (since != null) {
    final days = now.difference(since).inDays;
    if (days >= 1) return t.command_center.waiting_days(count: days);
    return t.command_center.waiting_today;
  }
  return null;
}

// ── Right rail ─────────────────────────────────────────────────────────────

/// "Radar hoje": the feed's first three hits, as an appetizer for the tab.
///
/// Renders NOTHING while loading, on error or on an empty feed — the rail is
/// garnish, and an empty card would be a claim the data has not earned.
class _RailRadar extends ConsumerWidget {
  const _RailRadar();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final feed = ref.watch(radarProvider).asData?.value;
    if (feed == null || feed.items.isEmpty) return const SizedBox.shrink();

    return GlassCard(
      key: const Key('start_rail_radar'),
      padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  t.start.rail_radar_title,
                  style: typography.body.copyWith(
                      color: Colors.white,
                      fontSize: 13,
                      fontWeight: FontWeight.w700),
                ),
              ),
              InkWell(
                key: const Key('start_rail_radar_open'),
                onTap: () => context.go('/prospect?tab=signals'),
                child: Text(
                  t.start.rail_radar_open,
                  style: typography.body
                      .copyWith(color: colors.bluePrimary, fontSize: 12),
                ),
              ),
            ],
          ),
          const SizedBox(height: 10),
          for (final item in feed.items.take(3))
            InkWell(
              key: Key('start_rail_radar_${item.id}'),
              onTap: () => context.go('/prospect?tab=signals'),
              borderRadius: BorderRadius.circular(8),
              child: Padding(
                padding: const EdgeInsets.symmetric(vertical: 6),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Container(
                      width: 28,
                      height: 28,
                      decoration: BoxDecoration(
                        color: _radarTint(item.type, colors)
                            .withValues(alpha: 0.13),
                        borderRadius: BorderRadius.circular(8),
                      ),
                      child: Icon(_radarIcon(item.type),
                          size: 14, color: _radarTint(item.type, colors)),
                    ),
                    const SizedBox(width: 10),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            item.title,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: typography.body.copyWith(
                                color: Colors.white,
                                fontSize: 12.5,
                                fontWeight: FontWeight.w600),
                          ),
                          Text(
                            item.context,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: typography.body.copyWith(
                                color: colors.textSecondary, fontSize: 11.5),
                          ),
                          if (item.deadline != null)
                            Text(
                              _radarDeadline(item.deadline!),
                              style: typography.body.copyWith(
                                  color: colors.orangeAlert,
                                  fontSize: 11,
                                  fontWeight: FontWeight.w600),
                            ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            ),
        ],
      ),
    );
  }

  static IconData _radarIcon(RadarItemType type) => switch (type) {
        RadarItemType.licitacao => Icons.description_outlined,
        RadarItemType.parque => Icons.precision_manufacturing_outlined,
        RadarItemType.recompra => Icons.replay_outlined,
        RadarItemType.data => Icons.event_outlined,
        RadarItemType.crossSell => Icons.add_shopping_cart_outlined,
        RadarItemType.contrato => Icons.event_busy_outlined,
        RadarItemType.ata => Icons.receipt_long_outlined,
      };

  static Color _radarTint(RadarItemType type, AppColorsExtension colors) =>
      switch (type) {
        RadarItemType.licitacao || RadarItemType.contrato => colors.orangeAlert,
        RadarItemType.parque => colors.bluePrimary,
        RadarItemType.recompra ||
        RadarItemType.crossSell ||
        RadarItemType.ata =>
          colors.limeGlow,
        RadarItemType.data => colors.bluePrimary,
      };

  static String _radarDeadline(DateTime deadline) {
    final days = deadline.difference(DateTime.now()).inDays;
    return days <= 0
        ? t.radar.deadline_today
        : t.radar.deadline_days(count: '$days');
  }
}

/// "Sua semana": the outcome line the approval ritual earns, as numbers.
///
/// The server counts sent, replies and proposals — the three facts the queue
/// can claim. Hidden entirely while loading or on error.
class _RailWeek extends ConsumerWidget {
  const _RailWeek();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final outcome = ref.watch(offersOutcomeProvider).asData?.value;
    if (outcome == null) return const SizedBox.shrink();

    Widget number(String keyName, int value, String label,
        {bool muteZero = false}) {
      final muted = muteZero && value == 0;
      return Expanded(
        child: Column(
          children: [
            Text(
              '$value',
              key: Key(keyName),
              style: typography.display.copyWith(
                  fontSize: 19,
                  fontWeight: FontWeight.w700,
                  color: muted ? colors.textMuted : Colors.white),
            ),
            Text(
              label,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: typography.body
                  .copyWith(color: colors.textMuted, fontSize: 11),
            ),
          ],
        ),
      );
    }

    return GlassCard(
      key: const Key('start_rail_week'),
      padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            t.start.rail_week_title,
            style: typography.body.copyWith(
                color: Colors.white, fontSize: 13, fontWeight: FontWeight.w700),
          ),
          const SizedBox(height: 12),
          Row(
            children: [
              number('start_week_sent', outcome.sent, t.radar.week_sent),
              number(
                  'start_week_replies', outcome.replies, t.radar.week_replies),
              number('start_week_proposals', outcome.proposals,
                  t.radar.week_proposals,
                  muteZero: true),
            ],
          ),
        ],
      ),
    );
  }
}

/// "Ligue o motor": the six SALES milestones as a rail card — the same
/// provider, the same 24h dismissal, the same CTAs the old checklist panel
/// carried; only the furniture changed.
class _RailEngine extends ConsumerWidget {
  const _RailEngine();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final gradients = Theme.of(context).extension<AppGradientsExtension>()!;
    final progress =
        ref.watch(onboardingProgressControllerProvider).asData?.value;
    if (progress == null) return const SizedBox.shrink();
    if (progress.completionPercentage >= 100) return const SizedBox.shrink();

    // Dismissed for the active 24h window — local pref (survives reload
    // before the server echoes back) OR the server-side stamp (US-112).
    final locallyDismissed =
        ref.watch(onboardingChecklistDismissalProvider).asData?.value ?? false;
    if (locallyDismissed || progress.isDismissActive) {
      return const SizedBox.shrink();
    }

    final isAdmin =
        ref.watch(authControllerProvider).asData?.value?.isAdmin ?? false;

    // The six the server's percentage counts — one source of truth, so the
    // card cannot disagree with its own bar.
    final items = <(bool, String, String?, String?, String)>[
      (
        progress.channelConnected,
        t.home.onboarding_checklist_channel,
        t.home.onboarding_checklist_channel_cta,
        '/settings/channels',
        'channel',
      ),
      (
        progress.productsAdded,
        t.home.onboarding_checklist_products,
        t.home.onboarding_checklist_products_cta,
        '/sales/products/import',
        'products',
      ),
      (
        progress.customersAdded,
        t.home.onboarding_checklist_customers,
        t.home.onboarding_checklist_customers_cta,
        '/contacts/import',
        'customers',
      ),
      (
        progress.proposalMade,
        t.home.onboarding_checklist_proposal,
        t.home.onboarding_checklist_proposal_cta,
        '/funnel',
        'proposal',
      ),
      // The only milestone that is not our doing: somebody answered. No CTA,
      // because there is nothing to click.
      (
        progress.firstInboundReceived,
        t.home.onboarding_checklist_inbound,
        null,
        null,
        'inbound',
      ),
      // The seller's seat — only for whoever can actually invite:
      // /settings/team silently redirects non-admins away.
      if (isAdmin)
        (
          progress.teamInvited,
          t.home.onboarding_checklist_team,
          t.home.onboarding_checklist_team_cta,
          '/settings/team',
          'team',
        ),
    ];
    final done = items.where((i) => i.$1).length +
        (!isAdmin && progress.teamInvited ? 1 : 0);
    final total = items.length + (isAdmin ? 0 : 1);

    return GlassCard(
      key: const Key('start_rail_engine'),
      padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  t.start.rail_engine_title,
                  style: typography.body.copyWith(
                      color: Colors.white,
                      fontSize: 13,
                      fontWeight: FontWeight.w700),
                ),
              ),
              Text(
                t.start.of_total(current: '$done', total: '$total'),
                key: const Key('start_engine_count'),
                style: typography.body
                    .copyWith(color: colors.textSecondary, fontSize: 12),
              ),
              const SizedBox(width: 4),
              InkWell(
                key: const Key('start_engine_dismiss'),
                onTap: () => _dismiss(ref, progress.completionPercentage),
                borderRadius: BorderRadius.circular(999),
                child: Padding(
                  padding: const EdgeInsets.all(4),
                  child: Icon(Icons.close, size: 14, color: colors.textMuted),
                ),
              ),
            ],
          ),
          const SizedBox(height: 10),
          ClipRRect(
            borderRadius: BorderRadius.circular(3),
            child: SizedBox(
              height: 6,
              child: Stack(
                children: [
                  Container(color: colors.glassMedium),
                  FractionallySizedBox(
                    widthFactor:
                        (progress.completionPercentage / 100).clamp(0.0, 1.0),
                    child: Container(
                      decoration:
                          BoxDecoration(gradient: gradients.primaryButton),
                    ),
                  ),
                ],
              ),
            ),
          ),
          const SizedBox(height: 10),
          for (final (isDone, label, cta, route, tag) in items)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 3),
              child: Row(
                children: [
                  Icon(
                    isDone ? Icons.check_circle : Icons.radio_button_unchecked,
                    size: 16,
                    color: isDone ? colors.limeGlow : colors.textMuted,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      label,
                      style: typography.body.copyWith(
                        color: isDone ? colors.textMuted : Colors.white,
                        fontSize: 12.5,
                        decoration: isDone ? TextDecoration.lineThrough : null,
                      ),
                    ),
                  ),
                  if (!isDone && cta != null && route != null)
                    InkWell(
                      key: Key('start_engine_$tag'),
                      onTap: () {
                        ref.read(analyticsClientProvider).track(
                          OnboardingEvents.checklistItemClicked,
                          {'item_name': tag},
                        );
                        context.go(route);
                      },
                      child: Padding(
                        padding: const EdgeInsets.symmetric(
                            horizontal: 4, vertical: 2),
                        child: Text(
                          cta,
                          style: typography.body.copyWith(
                              color: colors.bluePrimary, fontSize: 12),
                        ),
                      ),
                    ),
                ],
              ),
            ),
        ],
      ),
    );
  }

  /// The 24h snooze the old panel had, verbatim: local pref for the instant
  /// reload, server stamp for every other device (US-112).
  void _dismiss(WidgetRef ref, int percentage) {
    ref.read(analyticsClientProvider).track(
      OnboardingEvents.checklistDismissed,
      {'progress_percentage': percentage},
    );
    ref.read(onboardingChecklistDismissalProvider.notifier).dismiss();
    ref.read(onboardingProgressControllerProvider.notifier).dismissForNow();
  }
}

/// The mini-funnel: the board's shape at a glance, one tap from the board.
class _MiniFunnel extends StatelessWidget {
  const _MiniFunnel({required this.stages});

  final List<CommandCenterStage> stages;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    if (stages.isEmpty) return const SizedBox.shrink();

    final peak =
        stages.map((s) => s.count).fold<int>(0, (a, b) => a > b ? a : b);

    return GlassCard(
      padding: EdgeInsets.zero,
      child: InkWell(
        key: const Key('start_mini_funnel'),
        onTap: () => context.go('/funnel'),
        borderRadius: BorderRadius.circular(16),
        child: Padding(
          padding: const EdgeInsets.all(14),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Expanded(
                    child: Text(
                      t.command_center.funnel_title,
                      style: typography.body.copyWith(
                          color: Colors.white,
                          fontSize: 13,
                          fontWeight: FontWeight.w600),
                    ),
                  ),
                  Icon(Icons.chevron_right, color: colors.textSecondary),
                ],
              ),
              const SizedBox(height: 10),
              for (final stage in stages)
                Padding(
                  padding: const EdgeInsets.only(bottom: 6),
                  child: Row(
                    children: [
                      SizedBox(
                        width: 96,
                        child: Text(
                          stage.label,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: typography.body.copyWith(
                              color: colors.textSecondary, fontSize: 11),
                        ),
                      ),
                      Expanded(
                        child: ClipRRect(
                          borderRadius: BorderRadius.circular(3),
                          child: LinearProgressIndicator(
                            // Against the widest stage: the shape is the
                            // information, and a bar scaled to the total would
                            // flatten every real funnel into nothing.
                            value: peak == 0 ? 0 : stage.count / peak,
                            minHeight: 5,
                            backgroundColor: colors.borderLight,
                            valueColor: AlwaysStoppedAnimation<Color>(
                                colors.bluePrimary),
                          ),
                        ),
                      ),
                      SizedBox(
                        width: 40,
                        child: Text(
                          '${stage.count}',
                          key: Key('start_funnel_${stage.status}'),
                          textAlign: TextAlign.right,
                          style: typography.body
                              .copyWith(color: Colors.white, fontSize: 12),
                        ),
                      ),
                    ],
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }
}

class _StartSkeleton extends StatelessWidget {
  const _StartSkeleton();

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return ListView(
      key: const Key('start_skeleton'),
      padding: const EdgeInsets.all(24),
      children: [
        for (var i = 0; i < 4; i++)
          Padding(
            padding: const EdgeInsets.only(bottom: 12),
            child: Container(
              height: i == 0 ? 84 : 120,
              decoration: BoxDecoration(
                color: colors.glassLight,
                borderRadius: BorderRadius.circular(16),
              ),
            ),
          ),
      ],
    );
  }
}

/// "2026-07" is how a database says July. It is not how a person reads the
/// footnote under a money figure.
///
/// Anything that does not parse is printed unchanged: the label exists to say
/// which window the number covers, and a window nobody can name is still
/// better than no window at all.
String _humanPeriod(String raw) {
  final parts = raw.split('-');
  if (parts.length != 2) return raw;
  final year = int.tryParse(parts[0]);
  final month = int.tryParse(parts[1]);
  if (year == null || month == null || month < 1 || month > 12) return raw;
  return t.command_center.month_of_year(
    month: t.command_center.months[month - 1],
    year: '$year',
  );
}
