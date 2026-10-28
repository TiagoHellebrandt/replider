import 'dart:async';

import 'package:flutter/gestures.dart' show PointerDeviceKind;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/design_system/components/retryable_error.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/domain/models/board_model.dart';
import 'package:app/features/sales/domain/models/lead_model.dart';
import 'package:app/features/sales/presentation/controllers/board_controller.dart';
import 'package:app/features/sales/presentation/widgets/lead_card.dart';
import 'package:app/features/sales/presentation/widgets/lead_detail_panel.dart';
import 'package:app/design_system/components/gradient_pill.dart';
import 'package:app/features/sales/presentation/widgets/bulk_assign_dialog.dart';
import 'package:app/features/sales/presentation/widgets/new_lead_dialog.dart';
import 'package:app/i18n/strings.g.dart';

/// The Kanban board (US-M017 desktop / US-M018 mobile).
///
/// One screen, two interaction models, chosen by width:
///
///   * desktop — columns side by side, drag to move. A pointer makes
///     dragging precise, and seeing the whole pipeline at once is the point
///     of a board.
///   * mobile — one column at a time with a "move to" sheet. Dragging a card
///     across a horizontally scrolling viewport on a phone is a fight
///     between two gestures, and the salesperson loses it while walking
///     between appointments.
class KanbanScreen extends ConsumerStatefulWidget {
  const KanbanScreen({super.key, required this.funnelId, this.openLeadId});

  final String funnelId;

  /// Opened once on arrival when set (US-M024). Once only: re-opening the
  /// panel every rebuild would trap the user behind a sheet they closed.
  final String? openLeadId;

  /// Same breakpoint as the shell, so the navigation and the board never
  /// disagree about which form factor this is.
  static const double desktopBreakpoint = 600;

  @override
  ConsumerState<KanbanScreen> createState() => _KanbanScreenState();
}

class _KanbanScreenState extends ConsumerState<KanbanScreen> {
  /// Guards the one-shot deep link — see KanbanScreen.openLeadId.
  bool _deepLinkOpened = false;

  /// Optimistic overlay: the card must follow the finger before the server
  /// answers, because a drag that snaps back for 300ms every time reads as
  /// a broken app. Cleared whenever fresh server data arrives.
  Board? _optimistic;

  final _searchController = TextEditingController();
  String _query = '';
  BoardFilter _filter = BoardFilter.all;

  String get funnelId => widget.funnelId;

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  /// Moves a card optimistically, rolling back LOUDLY on failure.
  ///
  /// A silent revert is the worst outcome: the salesperson would believe a
  /// deal moved while the pipeline never changed.
  ///
  /// [beforeId]/[afterId] are the drop slot's neighbors (a gap between two
  /// cards); with them the card gets a persisted manual position, and a
  /// same-column drop becomes a REORDER instead of a no-op. Without them
  /// (whole-column fallback, mobile sheet) the card returns to the column's
  /// automatic order.
  Future<void> _moveCard(Board board, String leadId, String stageId,
      {String? beforeId, String? afterId}) async {
    final card = board.cards.where((c) => c.leadId == leadId).toList();
    if (card.isEmpty) return;
    final slot = beforeId != null || afterId != null;
    if (!slot && card.first.stageId == stageId) return;

    setState(() => _optimistic = slot
        ? board.withCardPlaced(leadId, stageId,
            beforeId: beforeId, afterId: afterId)
        : board.withCardMoved(leadId, stageId));
    final messenger = ScaffoldMessenger.of(context);
    try {
      await ref.read(boardServiceProvider).move(
          leadId: leadId,
          stageId: stageId,
          beforeLeadId: beforeId,
          afterLeadId: afterId);
      // Re-read instead of trusting the optimistic copy: the server also
      // rewrote the canonical status, and the column counters depend on it.
      ref.invalidate(boardProvider(funnelId));
    } catch (e) {
      setState(() => _optimistic = null);
      messenger.clearSnackBars();
      messenger.showSnackBar(SnackBar(
        key: const Key('board_move_error'),
        content: Text(parseApiError(e)),
        behavior: SnackBarBehavior.floating,
        margin: const EdgeInsets.all(16),
      ));
    }
  }

  /// "Ordenar por parados": clears one column's manual order, optimistically
  /// and as loudly reversible as a move.
  Future<void> _resetOrder(Board board, String stageId) async {
    setState(() => _optimistic = board.withOrderReset(stageId));
    final messenger = ScaffoldMessenger.of(context);
    try {
      await ref
          .read(boardServiceProvider)
          .resetOrder(funnelId: funnelId, stageId: stageId);
      ref.invalidate(boardProvider(funnelId));
    } catch (e) {
      setState(() => _optimistic = null);
      messenger.clearSnackBars();
      messenger.showSnackBar(SnackBar(
        key: const Key('board_reset_error'),
        content: Text(parseApiError(e)),
        behavior: SnackBarBehavior.floating,
        margin: const EdgeInsets.all(16),
      ));
    }
  }

  /// Opens the lead (US-M019). Refreshing on close is not optional: the
  /// panel can SEND an approach, which changes the lead's status and thus
  /// which column it belongs to.
  Future<void> _openDetail(BoardCard card, String stageName) async {
    await showLeadDetail(context,
        card: card, stageName: stageName, funnelId: funnelId);
    if (mounted) ref.invalidate(boardProvider(funnelId));
  }

  /// Opens the deep-linked lead once the board knows which column it is
  /// in. A lead that is not on this board (wrong funnel, filtered out) is
  /// silently ignored rather than opening an empty panel.
  void _maybeOpenDeepLink(Board board) {
    final leadId = widget.openLeadId;
    if (_deepLinkOpened || leadId == null || leadId.isEmpty) return;
    final match = board.cards.where((c) => c.leadId == leadId);
    if (match.isEmpty) return;
    _deepLinkOpened = true;
    final card = match.first;
    final stage = board.stages.where((s) => s.id == card.stageId);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) {
        _openDetail(card, stage.isEmpty ? '' : stage.first.name);
      }
    });
  }

  /// Hands over every ownerless lead currently on screen, at once.
  ///
  /// Sixty-four unowned leads, one at a time, is work nobody does — and a field
  /// nobody fills in becomes a field nobody trusts.
  ///
  /// The list comes from whatever the search and the filter left visible, and
  /// it is the list that travels: the number on the confirm button is the
  /// number that changes.
  Future<void> _bulkAssign(Board board) async {
    final ids = [
      for (final card in board.visibleCards(query: _query, filter: _filter))
        if (card.ownerUserId == null || card.ownerUserId!.isEmpty) card.leadId,
    ];
    if (ids.isEmpty) return;

    final res = await showBulkAssignDialog(context,
        leadIds: ids,
        filtered: _query.trim().isNotEmpty || _filter != BoardFilter.all);
    if (res == null || !mounted) return;

    ref.invalidate(boardProvider(funnelId));
    final messenger = ScaffoldMessenger.of(context)..clearSnackBars();
    messenger.showSnackBar(SnackBar(
      key: const Key('bulk_assign_result'),
      // How many went, and how many were no longer there. "Assigned 64" and
      // "assigned 61, three vanished" are different answers.
      content: Text(res.skipped == 0
          ? t.board.bulk_assign_done(count: res.assigned.toString())
          : t.board.bulk_assign_partial(
              count: res.assigned.toString(), skipped: res.skipped.toString())),
      behavior: SnackBarBehavior.floating,
      margin: const EdgeInsets.all(16),
    ));
  }

  /// Adds a lead by hand. Refreshing is required either way: a created
  /// lead needs a card, and a matched one may already sit in a column the
  /// user is not looking at.
  Future<void> _addLead() async {
    final created = await showNewLeadDialog(context, funnelId);
    if (created == null || !mounted) return;
    ref.invalidate(boardProvider(funnelId));
    // Clear first: a snackbar still on screen queues this one behind it, so
    // the answer to THIS submission arrives seconds late — long enough for
    // the previous message to read as the reply. Caught in the browser,
    // adding a lead twice in a row.
    final messenger = ScaffoldMessenger.of(context)..clearSnackBars();
    messenger.showSnackBar(SnackBar(
      key: Key(created ? 'new_lead_created' : 'new_lead_existed'),
      content:
          Text(created ? t.board.new_lead_created : t.board.new_lead_existed),
      behavior: SnackBarBehavior.floating,
      margin: const EdgeInsets.all(16),
    ));
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final state = ref.watch(boardProvider(funnelId));

    return Scaffold(
      backgroundColor: colors.backgroundPrimary,
      // The floating button is phone-only, where the thumb reaches the corner
      // and there is no spare header row. On desktop it hovered over the last
      // column and, with the board scrolled, covered the very cards the person
      // was trying to read — a create button hiding what already exists.
      floatingActionButton:
          MediaQuery.sizeOf(context).width < KanbanScreen.desktopBreakpoint
              ? FloatingActionButton.extended(
                  key: const Key('board_new_lead'),
                  onPressed: _addLead,
                  backgroundColor: colors.bluePrimary,
                  icon: const Icon(Icons.add, size: 20),
                  label: Text(t.board.new_lead),
                )
              : null,
      body: SafeArea(
        child: state.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (err, _) => RetryableError(
            error: err,
            onRetry: () => ref.invalidate(boardProvider(funnelId)),
          ),
          data: (fresh) {
            // Fresh server data always wins over the optimistic overlay.
            final board = _optimistic ?? fresh;
            _maybeOpenDeepLink(board);
            if (board.stages.isEmpty) {
              return Center(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(t.board.empty_board, key: const Key('board_empty')),
                    const SizedBox(height: 12),
                    // The message names the missing thing; the button opens
                    // where it is created.
                    FilledButton(
                      key: const Key('board_empty_configure'),
                      onPressed: () =>
                          context.go('/sales/funnels/${widget.funnelId}'),
                      child: Text(t.board.configure_stages),
                    ),
                  ],
                ),
              );
            }
            return Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                _BoardHeader(
                  board: board,
                  controller: _searchController,
                  query: _query,
                  filter: _filter,
                  onQuery: (q) => setState(() => _query = q),
                  onFilter: (f) => setState(() => _filter = f),
                  onNewLead: _addLead,
                  onBulkAssign: () => _bulkAssign(board),
                  unownedCount: board
                      .visibleCards(query: _query, filter: _filter)
                      .where((c) =>
                          c.ownerUserId == null || c.ownerUserId!.isEmpty)
                      .length,
                  onClear: () {
                    _searchController.clear();
                    setState(() => _query = '');
                  },
                ),
                Expanded(
                  child: LayoutBuilder(
                    builder: (context, constraints) => constraints.maxWidth >=
                            KanbanScreen.desktopBreakpoint
                        ? _DesktopBoard(
                            board: board,
                            query: _query,
                            filter: _filter,
                            onMove: (leadId, stageId, {beforeId, afterId}) =>
                                _moveCard(board, leadId, stageId,
                                    beforeId: beforeId, afterId: afterId),
                            onResetOrder: (stageId) =>
                                _resetOrder(board, stageId),
                            onOpen: _openDetail,
                            onClearSearch: () {
                              _searchController.clear();
                              setState(() => _query = '');
                            },
                          )
                        : _MobileBoard(
                            board: board,
                            query: _query,
                            filter: _filter,
                            onMove: (leadId, stageId, {beforeId, afterId}) =>
                                _moveCard(board, leadId, stageId,
                                    beforeId: beforeId, afterId: afterId),
                            onOpen: _openDetail,
                          ),
                  ),
                ),
              ],
            );
          },
        ),
      ),
    );
  }
}

AppColorsExtension colors2(BuildContext context) =>
    Theme.of(context).extension<AppColorsExtension>()!;

/// Search, filters and the truncation notice, above both layouts.
///
/// The board had none of this. With sixty leads in a column, finding one
/// meant scrolling a list with no order and no handle — and the salesperson
/// arrives already knowing the name of the clinic they want.
class _BoardHeader extends StatelessWidget {
  const _BoardHeader({
    required this.board,
    required this.controller,
    required this.query,
    required this.filter,
    required this.onQuery,
    required this.onFilter,
    required this.onNewLead,
    required this.onBulkAssign,
    required this.unownedCount,
    required this.onClear,
  });

  final Board board;
  final TextEditingController controller;
  final String query;
  final BoardFilter filter;
  final ValueChanged<String> onQuery;
  final ValueChanged<BoardFilter> onFilter;
  final VoidCallback onNewLead;
  final VoidCallback onBulkAssign;

  /// How many of the visible cards have no owner. Zero hides the button: an
  /// action that would do nothing does not deserve header space.
  final int unownedCount;
  final VoidCallback onClear;

  /// Whether the "no owner" count came from a slice rather than the whole
  /// funnel. Same rule the dialog uses for its caveat.
  bool get _filtered => query.trim().isNotEmpty || filter != BoardFilter.all;

  /// What changes the VIEW: search and filters. Nothing here touches the
  /// database.
  Widget _viewControls(AppColorsExtension colors,
      AppTypographyExtension typography, double available) {
    return Wrap(
      spacing: 12,
      runSpacing: 8,
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        // The pill is the CONTAINER's; the field itself stays borderless,
        // so the theme keeps owning input shapes (the design-system guard).
        Container(
          // Never wider than the space left: pinned at 300 inside a narrow
          // strip, it overflowed.
          width: available.clamp(0.0, 300.0),
          padding: const EdgeInsets.only(left: 14, right: 4),
          decoration: BoxDecoration(
            color: colors.backgroundSecondary,
            borderRadius: BorderRadius.circular(999),
            border: Border.all(color: colors.borderMedium),
          ),
          child: Row(
            children: [
              Icon(Icons.search, size: 16, color: colors.textMuted),
              const SizedBox(width: 8),
              Expanded(
                child: TextField(
                  key: const Key('board_search'),
                  controller: controller,
                  onChanged: onQuery,
                  style: typography.body
                      .copyWith(color: Colors.white, fontSize: 14),
                  decoration: InputDecoration(
                    isDense: true,
                    hintText: t.board.search_hint,
                    border: InputBorder.none,
                    enabledBorder: InputBorder.none,
                    focusedBorder: InputBorder.none,
                    contentPadding: const EdgeInsets.symmetric(vertical: 11),
                  ),
                ),
              ),
              if (query.isNotEmpty)
                IconButton(
                  key: const Key('board_search_clear'),
                  icon: const Icon(Icons.close, size: 16),
                  tooltip: t.board.search_clear,
                  visualDensity: VisualDensity.compact,
                  onPressed: onClear,
                ),
            ],
          ),
        ),
        for (final f in BoardFilter.values)
          ChoiceChip(
            key: Key('board_filter_${f.name}'),
            label: Text(_label(f)),
            selected: filter == f,
            onSelected: (_) => onFilter(f),
          ),
      ],
    );
  }

  /// What changes the DATABASE. Null when there is none — on a phone "new
  /// lead" is the floating button, and with no orphan lead there is nothing to
  /// redistribute.
  Widget? _actions(BuildContext context) {
    final wide =
        MediaQuery.sizeOf(context).width >= KanbanScreen.desktopBreakpoint;
    if (unownedCount == 0 && !wide) return null;
    return Wrap(
      spacing: 8,
      runSpacing: 8,
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        if (unownedCount > 0)
          TextButton.icon(
            key: const Key('board_bulk_assign'),
            onPressed: onBulkAssign,
            style: TextButton.styleFrom(
                foregroundColor: colors2(context).textSecondary),
            icon: const Icon(Icons.person_add_alt, size: 16),
            // The chips count the whole funnel on purpose; this one counts
            // what is in view, because that is what the click will touch.
            // When the two rules diverge, the label says which one it uses.
            label: Text(
                _filtered
                    ? '${t.board.bulk_assign_here} · $unownedCount'
                    : '${t.board.bulk_assign} · $unownedCount',
                style: const TextStyle(fontSize: 12.5)),
          ),
        if (wide)
          GradientPill(
            key: const Key('board_new_lead'),
            onTap: onNewLead,
            icon: Icons.add,
            label: t.board.new_lead,
            fontSize: 13,
            height: 38,
            padding: const EdgeInsets.symmetric(horizontal: 18),
          ),
      ],
    );
  }

  String _label(BoardFilter f) {
    // Counts ignore the text query on purpose: a filter chip has to say how
    // much work is in that pile, not how much of it matches what is being
    // typed right now.
    final count = board.countMatching(f);
    switch (f) {
      case BoardFilter.all:
        return t.board.filter_all(count: count);
      case BoardFilter.responded:
        return t.board.filter_responded(count: count);
      case BoardFilter.awaitingApproval:
        return t.board.filter_awaiting(count: count);
      case BoardFilter.stalled:
        return t.board.filter_stalled(count: count);
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 8),
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        border: Border(bottom: BorderSide(color: colors.borderLight)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Search, filters and actions share the row. Stacked, the header
          // ate a third of the height before the first card appeared — on a
          // board, what matters is the board.
          //
          // What changes the VIEW sits left; what changes the DATABASE sits
          // right. They shared one Wrap at equal weight: a chip that filters
          // and a button that redistributes sixty leads, touching, and
          // indistinguishable until read. That adjacency is what made
          // "All · 64" beside "Assign · 3" read as a contradiction, when they
          // are two counts of different things.
          LayoutBuilder(builder: (context, box) {
            final view = _viewControls(colors, typography, box.maxWidth);
            final act = _actions(context);
            // Side by side only when it fits. In a Row the right-hand group
            // measures whatever it likes and Expanded takes the remainder —
            // which in a 390px window is negative, and Flutter clips 103px of
            // button. When it does not fit, stack: a second row is honest,
            // half a button is not.
            if (box.maxWidth < 720) {
              return Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  view,
                  if (act != null) ...[const SizedBox(height: 8), act],
                ],
              );
            }
            return Row(
              crossAxisAlignment: CrossAxisAlignment.center,
              children: [
                Expanded(child: view),
                if (act != null) ...[const SizedBox(width: 12), act],
              ],
            );
          }),
          if (board.truncated) ...[
            const SizedBox(height: 8),
            Text(
              t.board.truncated_notice(count: boardPageLimit),
              key: const Key('board_truncated'),
              style: typography.body
                  .copyWith(color: colors.orangeAlert, fontSize: 12),
            ),
          ],
        ],
      ),
    );
  }
}

// ── Desktop ────────────────────────────────────────────────────────────────

typedef MoveCallback = void Function(String leadId, String stageId,
    {String? beforeId, String? afterId});
typedef OpenCallback = void Function(BoardCard card, String stageName);

/// Column width on the board, and the gap between columns.
///
/// The width used to be pinned at 280. Six stages make that 1740px, and in a
/// 1440 window (an ordinary laptop) the first column sat behind the navigation:
/// the chips at the top said "All · 1" while every visible column said "No
/// leads here", because the only lead was in the hidden column. Importing a
/// lead and landing in an apparently empty funnel is the worst possible outcome
/// for the step that exists to prove the import worked.
///
/// The minimum width of a readable column.
///
/// It was once 200, to fit eight stages in a 1440 window. They fit, and they
/// were unreadable: a company name broken across two lines, tags wrapping.
/// Eight stages do not fit on one screen, and pretending they do costs the
/// legibility of all of them — the board scrolls, and scrolling stopped being a
/// problem once dragging to the edge started moving it on its own.
const double _columnMinWidth = 280;

/// A ceiling, so a funnel with few stages does not stretch absurdly wide.
const double _columnMaxWidth = 320;
const double _columnGap = 12;
const double _boardPadding = 16;

/// Splits the available width across the stages.
///
/// When they all fit, they fit — nothing scrolls and the hidden column stops
/// existing as a category of bug. When they do not (many stages, a narrow
/// window), it stops at the readable minimum and the board scrolls again, which
/// is the correct degradation: squeezing below that would trade one unreachable
/// column for six unreadable ones.
double _columnWidth(double available, int stages) {
  if (stages <= 0) return _columnMaxWidth;
  // The right margin exists on EVERY column, the last one included, so there
  // are `stages` gaps and not `stages - 1`. Getting it wrong returns 12px of
  // overflow — too little to see, enough to push the last column off.
  final usable = available - _boardPadding * 2 - _columnGap * stages;
  final each = usable / stages;
  return each.clamp(_columnMinWidth, _columnMaxWidth);
}

class _DesktopBoard extends StatefulWidget {
  const _DesktopBoard({
    required this.board,
    required this.query,
    required this.filter,
    required this.onMove,
    required this.onResetOrder,
    required this.onOpen,
    required this.onClearSearch,
  });

  final Board board;
  final String query;
  final BoardFilter filter;
  final MoveCallback onMove;

  /// Clears one column's manual order (the header's "ordenar por parados").
  final ValueChanged<String> onResetOrder;
  final OpenCallback onOpen;
  final VoidCallback onClearSearch;

  @override
  State<_DesktopBoard> createState() => _DesktopBoardState();
}

/// Width of the strip that starts scrolling when a card is dragged to the
/// edge, and how far it moves per frame.
const double _autoScrollEdge = 56;
const double _autoScrollStep = 14;

class _DesktopBoardState extends State<_DesktopBoard> {
  final _scroll = ScrollController();
  Timer? _autoScroll;

  /// A card is in flight somewhere on the board. While it is, every drop
  /// gap grows enough to hover, so the slots are hittable without pixel
  /// hunting; the hovered one opens into the placeholder.
  bool _dragging = false;

  @override
  void dispose() {
    _autoScroll?.cancel();
    _scroll.dispose();
    super.dispose();
  }

  /// Scrolls for as long as the card hovers over the edge.
  ///
  /// Without it, moving a lead to an off-screen stage meant: drop, scroll,
  /// find the card again, pick it up again. Three steps for the gesture a board
  /// exists to provide.
  void _startAutoScroll(double direction) {
    _autoScroll?.cancel();
    _autoScroll = Timer.periodic(const Duration(milliseconds: 16), (_) {
      if (!_scroll.hasClients) return;
      final next = (_scroll.offset + _autoScrollStep * direction)
          .clamp(0.0, _scroll.position.maxScrollExtent);
      if (next == _scroll.offset) return; // chegou ao fim
      _scroll.jumpTo(next);
    });
  }

  void _stopAutoScroll() {
    _autoScroll?.cancel();
    _autoScroll = null;
  }

  @override
  Widget build(BuildContext context) {
    final board = widget.board;
    return LayoutBuilder(builder: (context, constraints) {
      final width = _columnWidth(constraints.maxWidth, board.stages.length);
      return ScrollConfiguration(
        // Dragging the background pans the board, as in any Kanban. The card
        // stays draggable because it sits deeper in the tree and wins the
        // gesture arena; the scroll only gets what is left over.
        behavior: ScrollConfiguration.of(context).copyWith(dragDevices: {
          PointerDeviceKind.touch,
          PointerDeviceKind.mouse,
          PointerDeviceKind.trackpad,
          PointerDeviceKind.stylus,
        }),
        child: Stack(children: [
          SingleChildScrollView(
            key: const Key('board_desktop_scroll'),
            controller: _scroll,
            // The horizontal scroll is CONTAINED here rather than on the page, so
            // the shell's navigation never slides off screen with the board.
            scrollDirection: Axis.horizontal,
            padding: const EdgeInsets.all(_boardPadding),
            child: SizedBox(
              // A bounded height is what gives the column somewhere to scroll.
              // Without it, it grew with its content inside a parent that only
              // scrolls horizontally, and cards below the fold were
              // unreachable.
              height: constraints.maxHeight - _boardPadding * 2,
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  for (final stage in board.stages)
                    // An EMPTY stage collapses into a 44px ruler — five
                    // zero-count columns were eating half the screen while
                    // the two populated ones scrolled. Still a DragTarget:
                    // dropping a card there is exactly how it stops being
                    // empty.
                    if (board.cardsIn(stage.id).isEmpty)
                      _StubColumn(
                        stage: stage,
                        onAccept: (leadId) => widget.onMove(leadId, stage.id),
                      )
                    else
                      _Column(
                        width: width,
                        showOwnership: board.hasAnyOwner,
                        stage: stage,
                        cards: board.visibleIn(stage.id,
                            query: widget.query, filter: widget.filter),
                        total: board.cardsIn(stage.id).length,
                        query: widget.query,
                        dragging: _dragging,
                        onDragChanged: (d) => setState(() => _dragging = d),
                        onMove: widget.onMove,
                        manualOrder: board.hasManualOrder(stage.id),
                        onResetOrder: () => widget.onResetOrder(stage.id),
                        onOpen: widget.onOpen,
                        onClearSearch: widget.onClearSearch,
                      ),
                ],
              ),
            ),
          ),
          // The strips only exist for a card in flight: a DragTarget reacts to
          // nothing but a drag, so they steal neither a click nor a scroll from
          // anyone.
          _AutoScrollEdge(
            key: const Key('board_autoscroll_left'),
            alignment: Alignment.centerLeft,
            onEnter: () => _startAutoScroll(-1),
            onLeave: _stopAutoScroll,
          ),
          _AutoScrollEdge(
            key: const Key('board_autoscroll_right'),
            alignment: Alignment.centerRight,
            onEnter: () => _startAutoScroll(1),
            onLeave: _stopAutoScroll,
          ),
        ]),
      );
    });
  }
}

/// An edge strip that scrolls the board while a card hovers over it.
class _AutoScrollEdge extends StatelessWidget {
  const _AutoScrollEdge({
    super.key,
    required this.alignment,
    required this.onEnter,
    required this.onLeave,
  });

  final Alignment alignment;
  final VoidCallback onEnter;
  final VoidCallback onLeave;

  @override
  Widget build(BuildContext context) {
    return Align(
      alignment: alignment,
      child: SizedBox(
        width: _autoScrollEdge,
        child: DragTarget<String>(
          // Never accepts the drop: the strip moves the board, the column
          // underneath receives the card. Accepting here would swallow the lead
          // into somewhere that is no stage at all.
          onWillAcceptWithDetails: (_) {
            onEnter();
            return false;
          },
          onLeave: (_) => onLeave(),
          builder: (context, candidate, rejected) => const SizedBox.expand(),
        ),
      ),
    );
  }
}

class _Column extends StatelessWidget {
  const _Column({
    required this.width,
    required this.stage,
    required this.cards,
    required this.total,
    required this.query,
    required this.dragging,
    required this.onDragChanged,
    required this.onMove,
    required this.manualOrder,
    required this.onResetOrder,
    required this.onOpen,
    required this.onClearSearch,
    required this.showOwnership,
  });

  /// See Board.hasAnyOwner — a board where nobody owns anything says nothing
  /// by labelling every card "Sem dono".
  final bool showOwnership;

  final double width;
  final BoardStage stage;
  final List<BoardCard> cards;

  /// Everything in this column, before the search and the filter. The header
  /// shows both numbers: "3 de 47" tells you a filter is on, "47 de 47" tells
  /// you it is not, and a lone "3" tells you nothing.
  final int total;

  final String query;

  /// A card is in flight somewhere on the board (not necessarily this
  /// column) — the drop gaps grow while it is.
  final bool dragging;
  final ValueChanged<bool> onDragChanged;
  final MoveCallback onMove;

  /// Somebody manually ordered this column: the caption says so and offers
  /// the way back to "mais parados primeiro".
  final bool manualOrder;
  final VoidCallback onResetOrder;
  final OpenCallback onOpen;
  final VoidCallback onClearSearch;

  /// Turns the drop slot at [index] (a gap in the rendered stack) into the
  /// neighbor pair the server persists: before = the card above the gap,
  /// after = the one below, both never the dragged card itself.
  void _dropAt(int index, String leadId) {
    final from = cards.indexWhere((c) => c.leadId == leadId);
    // The gaps touching the card's own spot are where it already is:
    // dropping there is a no-op, not a request.
    if (from != -1 && (index == from || index == from + 1)) return;
    final others = [
      for (final c in cards)
        if (c.leadId != leadId) c.leadId,
    ];
    var at = index;
    if (from != -1 && from < index) at = index - 1;
    if (at > others.length) at = others.length;
    onMove(
      leadId,
      stage.id,
      beforeId: at > 0 ? others[at - 1] : null,
      afterId: at < others.length ? others[at] : null,
    );
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    // The whole-column target survives as the FALLBACK: a card released on
    // the header or padding still lands in the column, just without a manual
    // position (the gaps sit deeper in the tree, so a drop on a slot never
    // reaches here).
    return DragTarget<String>(
      key: Key('board_column_${stage.id}'),
      onAcceptWithDetails: (details) => onMove(details.data, stage.id),
      builder: (context, candidate, rejected) {
        final hovering = candidate.isNotEmpty;
        return Container(
          width: width,
          margin: const EdgeInsets.only(right: _columnGap),
          padding: const EdgeInsets.all(12),
          decoration: BoxDecoration(
            color: colors.backgroundSecondary,
            borderRadius: BorderRadius.circular(16),
            border: Border.all(
              // A visible drop target: without it the user cannot tell
              // whether releasing here will do anything.
              color: hovering ? colors.bluePrimary : colors.borderMedium,
              width: hovering ? 2 : 1,
            ),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _ColumnHeader(
                stage: stage,
                count: cards.length,
                total: total,
                manualOrder: manualOrder,
                onResetOrder: onResetOrder,
              ),
              const SizedBox(height: 10),
              // The card stack scrolls on its own. The column used to grow
              // with its content inside a parent that only scrolled
              // horizontally: past the height of the screen, the rest of the
              // stage could not be reached.
              Expanded(
                child: ListView(
                  key: Key('board_column_list_${stage.id}'),
                  padding: EdgeInsets.zero,
                  children: [
                    if (cards.isEmpty)
                      Padding(
                        padding: const EdgeInsets.symmetric(vertical: 24),
                        child: query.trim().isEmpty
                            ? Center(
                                child: Text(
                                  t.board.empty_column,
                                  style: TextStyle(
                                      color: colors.textSecondary,
                                      fontSize: 12),
                                ),
                              )
                            : _NoMatch(query: query, onClear: onClearSearch),
                      )
                    else ...[
                      // Every gap is a drop slot, the card's own edges
                      // included, so reordering WITHIN the column is the same
                      // gesture as moving across — the placeholder shows
                      // exactly where the card will land.
                      for (var i = 0; i < cards.length; i++) ...[
                        _DropGap(
                          key: Key('board_gap_${stage.id}_$i'),
                          expanded: dragging,
                          onAccept: (leadId) => _dropAt(i, leadId),
                        ),
                        Draggable<String>(
                          // Only HORIZONTAL movement grabs the card. Without
                          // this it won the arena in every direction and a full
                          // stack lost drag-scrolling entirely: there is no
                          // background left to grab when the cards fill the
                          // column. A drag, once started, moves freely — which
                          // is how the same gesture also reorders vertically.
                          affinity: Axis.horizontal,
                          data: cards[i].leadId,
                          onDragStarted: () => onDragChanged(true),
                          onDragEnd: (_) => onDragChanged(false),
                          feedback: Material(
                            color: Colors.transparent,
                            child: SizedBox(
                              width: 256,
                              child: LeadCardTile(
                                  card: cards[i], showOwnership: showOwnership),
                            ),
                          ),
                          childWhenDragging: Opacity(
                            opacity: 0.35,
                            child: LeadCardTile(
                                card: cards[i], showOwnership: showOwnership),
                          ),
                          child: LeadCardTile(
                            card: cards[i],
                            showOwnership: showOwnership,
                            onTap: () => onOpen(cards[i], stage.name),
                          ),
                        ),
                      ],
                      _DropGap(
                        key: Key('board_gap_${stage.id}_${cards.length}'),
                        expanded: dragging,
                        tail: true,
                        onAccept: (leadId) => _dropAt(cards.length, leadId),
                      ),
                    ],
                  ],
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}

/// One drop slot between two cards.
///
/// Invisible at rest; while a card is in flight it grows enough to hover
/// without pixel-hunting, and the hovered slot opens into a dashed-looking
/// placeholder that pushes the cards apart — the empty space IS the answer
/// to "where will this land?".
class _DropGap extends StatelessWidget {
  const _DropGap({
    super.key,
    required this.onAccept,
    required this.expanded,
    this.tail = false,
  });

  final ValueChanged<String> onAccept;

  /// A drag is in flight somewhere on the board.
  final bool expanded;

  /// The slot below the last card — a little taller, because "the end of the
  /// stack" is the drop people aim at least precisely.
  final bool tail;

  @override
  Widget build(BuildContext context) {
    return DragTarget<String>(
      onWillAcceptWithDetails: (_) => true,
      onAcceptWithDetails: (d) => onAccept(d.data),
      builder: (context, candidates, _) {
        final hovering = candidates.isNotEmpty;
        final double height =
            hovering ? 72 : (expanded ? (tail ? 48 : 22) : (tail ? 12 : 0));
        return AnimatedContainer(
          duration: const Duration(milliseconds: 150),
          curve: Curves.easeOut,
          height: height,
          margin: EdgeInsets.only(bottom: hovering ? 8 : 0),
          decoration: hovering
              ? BoxDecoration(
                  borderRadius: BorderRadius.circular(12),
                  color: Colors.white.withValues(alpha: 0.04),
                  border: Border.all(
                    color: Colors.white.withValues(alpha: 0.18),
                    width: 1.5,
                  ),
                )
              : null,
        );
      },
    );
  }
}

String _statusLabel(String wire) => LeadPipelineStatus.fromWire(wire).label;

class _ColumnHeader extends StatelessWidget {
  const _ColumnHeader({
    required this.stage,
    required this.count,
    required this.total,
    required this.manualOrder,
    required this.onResetOrder,
  });

  final BoardStage stage;
  final int count;
  final int total;

  /// At least one card here was manually placed: the caption must stop
  /// claiming "mais parados primeiro" and offer the way back to it.
  final bool manualOrder;
  final VoidCallback onResetOrder;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Expanded(
              // The tooltip says what the stage MEANS in pipeline terms
              // (which status it maps to, and that terminal stages end the
              // journey) — a custom column name alone answered neither, and
              // leads also move here automatically on send/reply.
              child: Tooltip(
                message: stage.isTerminal
                    ? t.board.column_hint_terminal(
                        status: _statusLabel(stage.mapsToStatus))
                    : t.board
                        .column_hint(status: _statusLabel(stage.mapsToStatus)),
                child: Text(
                  stage.name,
                  key: Key('board_column_title_${stage.id}'),
                  style: typography.body.copyWith(
                    color:
                        stage.isTerminal ? colors.textSecondary : Colors.white,
                    fontWeight: FontWeight.w600,
                    fontSize: 13,
                  ),
                ),
              ),
            ),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
              decoration: BoxDecoration(
                color: colors.glassLight,
                borderRadius: BorderRadius.circular(999),
              ),
              child: Text(
                '$count',
                key: Key('board_column_count_${stage.id}'),
                style: typography.body
                    .copyWith(color: colors.textSecondary, fontSize: 12),
              ),
            ),
          ],
        ),
        // "0 of 0 · most idle first" is a sort caption over nothing: the empty
        // column already says what needs saying, just below. The line only
        // earns its place when something in it is actually ordered — and it
        // must say WHICH order: once a hand placed a card, "mais parados
        // primeiro" would be a lie, so the caption flips to "ordem manual"
        // and carries the way back.
        if (total > 0) ...[
          const SizedBox(height: 2),
          Row(
            children: [
              Flexible(
                child: Text(
                  manualOrder
                      ? t.board
                          .column_showing_manual(showing: count, total: total)
                      : t.board.column_showing(showing: count, total: total),
                  key: Key('board_column_showing_${stage.id}'),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: typography.body
                      .copyWith(color: colors.textMuted, fontSize: 12),
                ),
              ),
              if (manualOrder) ...[
                const SizedBox(width: 6),
                InkWell(
                  key: Key('board_order_reset_${stage.id}'),
                  onTap: onResetOrder,
                  borderRadius: BorderRadius.circular(4),
                  child: Text(
                    t.board.order_reset,
                    style: typography.body.copyWith(
                      color: colors.textSecondary,
                      fontSize: 12,
                      decoration: TextDecoration.underline,
                    ),
                  ),
                ),
              ],
            ],
          ),
        ],
      ],
    );
  }
}

/// Searching in a column that has no match must not look like an empty
/// column: one means "nothing here", the other means "not here — look
/// elsewhere", and only the second has an obvious next move.
class _NoMatch extends StatelessWidget {
  const _NoMatch({required this.query, required this.onClear});

  final String query;
  final VoidCallback onClear;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          t.board.column_no_match(query: query),
          style: typography.body
              .copyWith(color: Colors.white, fontSize: 13, height: 1.4),
        ),
        const SizedBox(height: 4),
        Text(
          t.board.column_no_match_hint,
          style: typography.body
              .copyWith(color: colors.textSecondary, fontSize: 12, height: 1.4),
        ),
        const SizedBox(height: 8),
        TextButton(onPressed: onClear, child: Text(t.board.search_clear)),
      ],
    );
  }
}

// ── Mobile ─────────────────────────────────────────────────────────────────

class _MobileBoard extends StatefulWidget {
  const _MobileBoard({
    required this.board,
    required this.query,
    required this.filter,
    required this.onMove,
    required this.onOpen,
  });

  final Board board;
  final String query;
  final BoardFilter filter;
  final MoveCallback onMove;
  final OpenCallback onOpen;

  @override
  State<_MobileBoard> createState() => _MobileBoardState();
}

class _MobileBoardState extends State<_MobileBoard> {
  late final PageController _pages = PageController();
  int _index = 0;

  @override
  void dispose() {
    _pages.dispose();
    super.dispose();
  }

  void _jumpTo(int index) {
    setState(() => _index = index);
    _pages.animateToPage(index,
        duration: const Duration(milliseconds: 220), curve: Curves.easeOut);
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final stages = widget.board.stages;

    return Column(
      children: [
        // Stage chips double as a position indicator: with one column
        // visible at a time, "where am I in the pipeline?" is otherwise
        // invisible.
        SizedBox(
          height: 52,
          child: ListView.separated(
            key: const Key('board_stage_chips'),
            scrollDirection: Axis.horizontal,
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
            itemCount: stages.length,
            separatorBuilder: (_, __) => const SizedBox(width: 8),
            itemBuilder: (context, i) {
              final selected = i == _index;
              // The chip counts what the current filter/search LEAVES in
              // that stage, so tapping through never lands on a column the
              // chip promised had cards.
              final shown = widget.board
                  .visibleIn(stages[i].id,
                      query: widget.query, filter: widget.filter)
                  .length;
              return ChoiceChip(
                key: Key('board_chip_${stages[i].id}'),
                label: Text('${stages[i].name} ($shown)'),
                selected: selected,
                onSelected: (_) => _jumpTo(i),
              );
            },
          ),
        ),
        Expanded(
          child: PageView.builder(
            key: const Key('board_pageview'),
            controller: _pages,
            onPageChanged: (i) => setState(() => _index = i),
            itemCount: stages.length,
            itemBuilder: (context, i) {
              final stage = stages[i];
              final cards = widget.board.visibleIn(stage.id,
                  query: widget.query, filter: widget.filter);
              if (cards.isEmpty) {
                return Center(
                  child: Text(
                    widget.query.trim().isEmpty
                        ? t.board.empty_column
                        : t.board.column_no_match(query: widget.query),
                    textAlign: TextAlign.center,
                    style: TextStyle(color: colors.textSecondary),
                  ),
                );
              }
              return ListView.builder(
                // The bottom padding is the floating button: without it "New
                // lead" covered the stage's last card, and the hidden card was
                // always the most idle one — the stack is ordered by exactly
                // that.
                padding: const EdgeInsets.fromLTRB(12, 12, 12, 88),
                itemCount: cards.length,
                itemBuilder: (context, j) => LeadCardTile(
                  card: cards[j],
                  showOwnership: widget.board.hasAnyOwner,
                  onTap: () => widget.onOpen(cards[j], stage.name),
                  // No drag on mobile: dragging across a horizontally
                  // paging viewport is two gestures fighting.
                  onMove: () => _showMoveSheet(cards[j]),
                ),
              );
            },
          ),
        ),
      ],
    );
  }

  Future<void> _showMoveSheet(BoardCard card) async {
    final stages = widget.board.stages;
    await showModalBottomSheet<void>(
      context: context,
      backgroundColor: Theme.of(context)
          .extension<AppColorsExtension>()!
          .backgroundSecondary,
      builder: (sheetContext) => SafeArea(
        child: ListView(
          key: const Key('board_move_sheet'),
          shrinkWrap: true,
          children: [
            Padding(
              padding: const EdgeInsets.all(16),
              child: Text(t.board.move_to,
                  style: const TextStyle(
                      color: Colors.white, fontWeight: FontWeight.w600)),
            ),
            for (final stage in stages)
              ListTile(
                key: Key('board_move_option_${stage.id}'),
                // 44px minimum: this list is the only way to move a card
                // on a phone.
                minVerticalPadding: 12,
                title: Text(stage.name,
                    style: const TextStyle(color: Colors.white)),
                trailing: stage.id == card.stageId
                    ? const Icon(Icons.check, size: 18)
                    : null,
                onTap: () {
                  Navigator.of(sheetContext).pop();
                  if (stage.id != card.stageId) {
                    widget.onMove(card.leadId, stage.id);
                  }
                },
              ),
          ],
        ),
      ),
    );
  }
}

/// An empty stage as a slim vertical ruler: name rotated, zero on top, and
/// a live DragTarget — dropping a card here is how the stage stops being
/// empty, so collapsing must never cost the drop.
class _StubColumn extends StatelessWidget {
  const _StubColumn({required this.stage, required this.onAccept});

  final BoardStage stage;
  final ValueChanged<String> onAccept;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return DragTarget<String>(
      onWillAcceptWithDetails: (_) => true,
      onAcceptWithDetails: (d) => onAccept(d.data),
      builder: (context, candidates, _) {
        final hovering = candidates.isNotEmpty;
        return Container(
          key: Key('board_stub_${stage.id}'),
          width: 44,
          margin: const EdgeInsets.only(right: 12),
          decoration: BoxDecoration(
            color: hovering
                ? colors.bluePrimary.withValues(alpha: 0.12)
                : colors.glassLight,
            borderRadius: BorderRadius.circular(16),
            border: Border.all(
              color: hovering ? colors.bluePrimary : colors.borderLight,
            ),
          ),
          padding: const EdgeInsets.symmetric(vertical: 14),
          child: Column(
            children: [
              Text('0',
                  style: TextStyle(
                      fontSize: 11,
                      fontWeight: FontWeight.w700,
                      color: colors.textMuted)),
              const SizedBox(height: 10),
              Expanded(
                child: RotatedBox(
                  quarterTurns: 1,
                  child: Text(
                    stage.name,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                        fontSize: 11.5,
                        fontWeight: FontWeight.w600,
                        color: colors.textMuted),
                  ),
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}
