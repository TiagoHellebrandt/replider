import 'dart:async';

import 'package:url_launcher/url_launcher.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:app/core/auth/account_role.dart';
import 'package:app/core/error/api_error_parser.dart';
import 'package:app/design_system/components/app_dialogs.dart';
import 'package:app/design_system/components/empty_state_card.dart';
import 'package:app/design_system/components/gradient_pill.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/core/utils/whatsapp_link.dart';
import 'package:app/design_system/components/retryable_error.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/sales/domain/models/approach_review.dart';
import 'package:app/features/settings/domain/models/channel_model.dart';
import 'package:app/features/settings/presentation/controllers/channels_controller.dart';
import 'package:app/features/sales/domain/models/board_model.dart';
import 'package:app/features/sales/presentation/controllers/approval_queue_controller.dart';
import 'package:app/features/sales/presentation/controllers/funnels_controller.dart';
import 'package:app/features/sales/presentation/controllers/lead_detail_controller.dart';
import 'package:app/features/sales/presentation/widgets/approach_send_dialog.dart';
import 'package:app/features/sales/presentation/widgets/bulk_approval_dialog.dart';
import 'package:app/features/sales/presentation/widgets/lead_detail_panel.dart';
import 'package:app/features/sales/presentation/widgets/wa_template_picker.dart';
import 'package:app/i18n/strings.g.dart';

/// The approval queue (US-M020).
///
/// This screen is the product's promise about cold outbound: every message
/// that reaches a stranger passed through a human here. So it is built for
/// SPEED without ever removing the gate — keyboard shortcuts to move through
/// the backlog, but approving still shows the exact text and the recipient
/// and waits for a confirmation.
///
/// The draft is read-only on the card. Editing opens the lead panel, which
/// is the one place approach text is written — two editors for the same
/// field would drift.
class ApprovalQueueScreen extends ConsumerStatefulWidget {
  const ApprovalQueueScreen({super.key, this.showHeader = true});

  /// False when the screen sits under the Prospecção tab (US-M021), which
  /// already names the queue and shows its count — repeating both a line
  /// apart just eats the screen a phone doesn't have.
  final bool showHeader;

  /// How long a discard can be taken back. Nothing is sent to the server
  /// until it elapses, so undo costs a timer instead of a second request
  /// that could itself fail.
  static const undoWindow = Duration(seconds: 5);

  @override
  ConsumerState<ApprovalQueueScreen> createState() =>
      _ApprovalQueueScreenState();
}

class _ApprovalQueueScreenState extends ConsumerState<ApprovalQueueScreen> {
  final _focus = FocusNode();

  /// Leads ticked for a bulk send. Desktop only — on a phone the gesture IS
  /// the flow, and a checkbox column would just steal width.
  final _checked = <String>{};

  /// Undo timers, one per discarded lead. Until one fires, the server has
  /// been told nothing.
  final _discarding = <String, Timer>{};

  /// Leads hidden from the list: the ones still inside their undo window PLUS
  /// the ones whose discard is committing. Without the second group a card
  /// pops back into the queue between the timer firing and the refreshed
  /// list arriving — long enough for the operator to act on a lead they
  /// already dropped. An entry is released only once the server stops
  /// returning it (or the discard fails).
  final _hidden = <String>{};

  int _selected = 0;
  bool _busy = false;

  /// Whether the cold group is unfolded. Collapsed by default when it is
  /// large: the pre-catalogue Places drafts are near-identical, and forty of
  /// them interleaved with real work is how the two drafts that pay go
  /// unread.
  bool _frioExpanded = false;

  /// AI verdicts, per lead, for this session only. Deliberately not cached
  /// across a reload: the review is about a SPECIFIC text, and a verdict shown
  /// next to a draft that has since been rewritten would be a lie with a
  /// checkmark on it.
  final _reviews = <String, ApproachReview>{};
  final _reviewing = <String>{};

  /// Captured eagerly so a discard can still be committed from dispose(),
  /// where `ref` is no longer usable but the user's decision still is.
  late final LeadActions _actions;

  @override
  void initState() {
    super.initState();
    _actions = ref.read(leadActionsProvider);
  }

  @override
  void dispose() {
    // Leaving the screen must not silently cancel a discard the user already
    // made — commit whatever is still in flight.
    for (final entry in _discarding.entries) {
      entry.value.cancel();
      // Fire-and-forget: there is no UI left to report an error to, and the
      // lead simply stays in the queue if the call fails.
      unawaited(_actions.discard(leadId: entry.key).catchError((_) {}));
    }
    _discarding.clear();
    _focus.dispose();
    super.dispose();
  }

  List<QueueLead> _visible(List<QueueLead> all, {required bool flaggedOnly}) {
    // Self-healing: an id the server no longer returns is settled, so stop
    // tracking it rather than leaking the set for the session.
    _hidden.removeWhere(
        (id) => !all.any((l) => l.id == id) && !_discarding.containsKey(id));
    final visible = all
        .where((l) => !_hidden.contains(l.id))
        .where((l) => !flaggedOnly || l.findings.isNotEmpty)
        .toList();
    return deadlineFirst(visible, DateTime.now());
  }

  /// Drops ticks whose card the filter just took off screen.
  ///
  /// The bulk send only ever looks at what is visible, so a lead left ticked
  /// behind a filter would sit in the counter and never be sent — the bar would
  /// say 12 and 9 would go out. Called from build, after the visible set is
  /// known, because that is the only place both halves exist.
  void _pruneChecked(List<QueueLead> visible) {
    if (_checked.isEmpty) return;
    final onScreen = visible.map((l) => l.id).toSet();
    if (_checked.every(onScreen.contains)) return;
    // Already inside build: mutate now and let this frame render the pruned
    // count rather than scheduling a second pass.
    _checked.removeWhere((id) => !onScreen.contains(id));
  }

  QueueLead? _current(List<QueueLead> visible) {
    if (visible.isEmpty) return null;
    return visible[_selected.clamp(0, visible.length - 1)];
  }

  void _move(int delta, List<QueueLead> visible) {
    if (visible.isEmpty) return;
    setState(() =>
        _selected = (_selected + delta).clamp(0, visible.length - 1).toInt());
  }

  /// The list interleaved with its group headers. Headers only appear when
  /// there is more than one source on screen — a single-source queue reads
  /// better without labels stating the obvious.
  List<_QueueRow> _rows(
      List<QueueLead> visible, List<QueueLead> frioAll, bool collapsed) {
    final sources = visible.map(queueSourceOf).toSet();
    final withHeaders =
        sources.length > 1 || (sources.length == 1 && collapsed);
    final rows = <_QueueRow>[];
    QueueSource? prev;
    for (var i = 0; i < visible.length; i++) {
      final src = queueSourceOf(visible[i]);
      if (withHeaders && src != prev) {
        rows.add(_QueueRow.head(src));
        prev = src;
      }
      rows.add(_QueueRow.lead(i));
    }
    if (collapsed) rows.add(_QueueRow.showAll());
    return rows;
  }

  Future<void> _discardFrio(List<QueueLead> frio) async {
    final ids = frio.map((l) => l.id).toList();
    final ok = await showConfirmDialog(
      context,
      title: t.lead_detail.queue_batch_discard_title,
      message:
          t.lead_detail.queue_batch_discard_message(count: '${ids.length}'),
      confirmText: t.sales.leads.discard,
      destructive: true,
    );
    if (!ok || !mounted) return;
    setState(() => _busy = true);
    try {
      final res = await ref.read(queueBatchActionsProvider).discardAll(ids);
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(
          content: Text(
              t.lead_detail.queue_batch_discarded(count: '${res.processed}'))));
    } on DioException catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(parseApiError(e))));
    } finally {
      if (mounted) setState(() => _busy = false);
      ref.invalidate(approvalQueueProvider);
    }
  }

  Future<void> _redraftFrio(List<QueueLead> frio) async {
    final ids = frio.map((l) => l.id).toList();
    final ok = await showConfirmDialog(
      context,
      title: t.lead_detail.queue_batch_redraft_title,
      message:
          t.lead_detail.queue_batch_redraft_message(count: '${ids.length}'),
      confirmText: t.common.confirm,
    );
    if (!ok || !mounted) return;
    setState(() => _busy = true);
    try {
      final res = await ref.read(queueBatchActionsProvider).redraftAll(ids);
      if (!mounted) return;
      // A stop for missing products is the one refusal worth its own words —
      // it tells the seller the FIX (import the catalogue), not just a count.
      final text = res.stoppedReason == 'APPROACH_NEEDS_PRODUCTS'
          ? t.errors.APPROACH_NEEDS_PRODUCTS
          : t.lead_detail.queue_batch_redrafted(count: '${res.processed}');
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(text)));
    } on DioException catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(parseApiError(e))));
    } finally {
      if (mounted) setState(() => _busy = false);
      ref.invalidate(approvalQueueProvider);
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final state = ref.watch(approvalQueueProvider);
    // Watched (not just read in the helper) so the cards' channel badges
    // appear once the funnel/channel lists arrive — a read-only lookup built
    // before they loaded would never rebuild.
    ref.watch(funnelsListProvider);
    ref.watch(channelsListProvider);

    return Scaffold(
      backgroundColor: colors.backgroundPrimary,
      body: SafeArea(
        child: state.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (err, _) => RetryableError(
            error: err,
            onRetry: () => ref.invalidate(approvalQueueProvider),
          ),
          data: (page) {
            final filter = ref.watch(approvalQueueFilterProvider);
            final filtered =
                _visible(page.items, flaggedOnly: filter.flaggedOnly);
            // Source order is value order: the seller's own book first, then
            // radar signals, cold Places prospects last. Stable within each
            // group, so the server's staleness sort survives.
            final ordered = [
              for (final s in QueueSource.values)
                ...filtered.where((l) => queueSourceOf(l) == s),
            ];
            final frioAll = filtered
                .where((l) => queueSourceOf(l) == QueueSource.frio)
                .toList();
            final collapseFrio = !_frioExpanded && frioAll.length > 3;
            final visible = collapseFrio
                ? [
                    ...ordered
                        .where((l) => queueSourceOf(l) != QueueSource.frio),
                    ...frioAll.take(2),
                  ]
                : ordered;
            _pruneChecked(visible);
            final current = _current(visible);
            final rows = _rows(visible, frioAll, collapseFrio);

            return CallbackShortcuts(
              bindings: {
                // A/E/X act on the highlighted card. They are deliberately
                // plain letters: this screen is a backlog you work through,
                // and a modifier per action doubles the keystrokes.
                const SingleActivator(LogicalKeyboardKey.keyA): () {
                  if (current != null) _approve(current);
                },
                const SingleActivator(LogicalKeyboardKey.keyE): () {
                  if (current != null) _edit(current);
                },
                const SingleActivator(LogicalKeyboardKey.keyX): () {
                  if (current != null) _discard(current);
                },
                const SingleActivator(LogicalKeyboardKey.arrowDown): () =>
                    _move(1, visible),
                const SingleActivator(LogicalKeyboardKey.arrowUp): () =>
                    _move(-1, visible),
              },
              child: Focus(
                focusNode: _focus,
                autofocus: true,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    // Item 1: the queue is where a warming mailbox's daily
                    // limit bites (approve-send fails closed), so the ramp
                    // state lives HERE, before the refusal.
                    const _EmailWarmupBanner(),
                    // Before ANY of it: an account with nowhere to send from
                    // reads forty finished offers and a live blue button.
                    // Production shipped exactly that — 41 drafts, 0 channels.
                    if (!page.canSend) const _SendBlockedBanner(),
                    if (widget.showHeader)
                      Padding(
                        padding: const EdgeInsets.fromLTRB(20, 20, 20, 8),
                        child: Row(
                          children: <Widget>[
                            Expanded(
                              child: Text(
                                t.lead_detail.queue_title,
                                style: typography.display.copyWith(
                                    fontSize: 20,
                                    color: Colors.white,
                                    fontWeight: FontWeight.w600),
                              ),
                            ),
                            // The counter is the point of a queue: it tells the
                            // operator whether they are winning. So it counts
                            // the whole BACKLOG rather than the filtered view —
                            // a number that drops when you pick a funnel is
                            // measuring the filter — minus the cards the
                            // operator just discarded, because that was real
                            // work and a counter that ignores it feels stuck.
                            Text(
                              t.lead_detail.queue_remaining(
                                  count: _backlog(page, visible)),
                              key: const Key('approval_queue_counter'),
                              style: typography.body.copyWith(
                                  color: colors.textSecondary, fontSize: 13),
                            ),
                          ],
                        ),
                      ),
                    // The feedback line the ritual was missing: whether this
                    // week's approvals turned into anything. Renders nothing
                    // while loading or on error — the queue works without it.
                    if (widget.showHeader) const _OutcomeLine(),
                    // The filter bar renders whenever there is anything to
                    // slice, including under an active filter that emptied the
                    // list — otherwise the only control that could undo it
                    // disappears with the last card.
                    if (page.total > 0)
                      _QueueFilterBar(page: page, filter: filter),
                    Expanded(
                      child: visible.isEmpty
                          ? Center(
                              child: Padding(
                                padding: const EdgeInsets.all(24),
                                child: _filterIsNarrowing(filter, page)
                                    // Distinct from an empty queue: the drafts
                                    // exist, the filter is hiding them. Saying
                                    // "nothing to approve" here would send the
                                    // operator home with work still queued.
                                    ? EmptyStateCard(
                                        key: const Key(
                                            'approval_queue_filtered_empty'),
                                        icon: Icons.filter_alt_off_outlined,
                                        title:
                                            t.lead_detail.queue_filtered_empty,
                                        subtitle: t.lead_detail
                                            .queue_filtered_empty_hint,
                                      )
                                    : EmptyStateCard(
                                        key: const Key('approval_queue_empty'),
                                        icon: Icons.inbox_outlined,
                                        title: t.lead_detail.queue_empty,
                                        subtitle:
                                            t.lead_detail.queue_empty_hint,
                                        // On a fresh account there IS no
                                        // funnel to "generate approaches"
                                        // in — the instruction was circular.
                                        // The next step is finding companies
                                        // — for the roles the hunt tab
                                        // admits (it is admin on the
                                        // server); a vendedora's queue
                                        // fills overnight, so hers just
                                        // explains.
                                        ctaLabel: AccountRole.from(ref
                                                    .watch(
                                                        authControllerProvider)
                                                    .asData
                                                    ?.value
                                                    ?.role)
                                                .satisfies(AccountRole.admin)
                                            ? t.lead_detail.queue_empty_cta
                                            : null,
                                        onCtaTap: () =>
                                            context.go('/prospect?tab=signals'),
                                      ),
                              ),
                            )
                          : ListView.builder(
                              key: const Key('approval_queue_list'),
                              padding: const EdgeInsets.fromLTRB(20, 4, 20, 20),
                              itemCount: rows.length,
                              itemBuilder: (context, rowIndex) {
                                final row = rows[rowIndex];
                                if (row.header != null) {
                                  return _GroupHeader(
                                    source: row.header!,
                                    count: row.header == QueueSource.frio
                                        ? frioAll.length
                                        : visible
                                            .where((l) =>
                                                queueSourceOf(l) == row.header)
                                            .length,
                                    busy: _busy,
                                    onDiscardAll: row.header == QueueSource.frio
                                        ? () => _discardFrio(frioAll)
                                        : null,
                                    onRedraftAll: row.header == QueueSource.frio
                                        ? () => _redraftFrio(frioAll)
                                        : null,
                                  );
                                }
                                if (row.showAllFrio) {
                                  return Align(
                                    alignment: Alignment.centerLeft,
                                    child: TextButton(
                                      key: const Key('queue_frio_show_all'),
                                      onPressed: () =>
                                          setState(() => _frioExpanded = true),
                                      style: TextButton.styleFrom(
                                          padding: const EdgeInsets.symmetric(
                                              horizontal: 4, vertical: 6),
                                          minimumSize: Size.zero),
                                      child: Text(
                                        t.lead_detail.queue_frio_show_all(
                                            count: '${frioAll.length - 2}'),
                                        style: TextStyle(
                                            color: colors.textMuted,
                                            fontSize: 12.5),
                                      ),
                                    ),
                                  );
                                }
                                final i = row.leadIndex!;
                                final card = _QueueCard(
                                  lead: visible[i],
                                  channelType: _channelTypeFor(visible[i]),
                                  // Only worth saying when the list spans more
                                  // than one funnel and none is selected.
                                  funnelLabel: page.byFunnel.length > 1 &&
                                          filter.funnelId == null
                                      ? _funnelName(page, visible[i].funnelId)
                                      : null,
                                  selected: visible[i].id == current?.id,
                                  busy: _busy,
                                  onSelect: () => setState(() => _selected = i),
                                  onApprove: () => _approve(visible[i]),
                                  onEdit: () => _edit(visible[i]),
                                  onDiscard: () => _discard(visible[i]),
                                  onReview: () => _review(visible[i]),
                                  review: _reviews[visible[i].id],
                                  reviewing: _reviewing.contains(visible[i].id),
                                  sendBlocked: !page.canSend,
                                );
                                if (!_isPhone(context)) {
                                  // The mockup's route: ONE full card — the
                                  // selected one, "next up" — and every
                                  // other lead as a compact row. Forty
                                  // expanded cards buried the two that pay.
                                  if (visible[i].id != current?.id) {
                                    return _CheckableCard(
                                      lead: visible[i],
                                      checked: _checked.contains(visible[i].id),
                                      onChanged: (on) => setState(() => on
                                          ? _checked.add(visible[i].id)
                                          : _checked.remove(visible[i].id)),
                                      child: _CompactLeadRow(
                                        lead: visible[i],
                                        busy: _busy,
                                        onSelect: () =>
                                            setState(() => _selected = i),
                                        onDiscard: () => _discard(visible[i]),
                                      ),
                                    );
                                  }
                                  return _CheckableCard(
                                    lead: visible[i],
                                    checked: _checked.contains(visible[i].id),
                                    onChanged: (on) => setState(() => on
                                        ? _checked.add(visible[i].id)
                                        : _checked.remove(visible[i].id)),
                                    child: card,
                                  );
                                }
                                return _SwipeableCard(
                                  key: Key('swipe_${visible[i].id}'),
                                  lead: visible[i],
                                  onApprove: () => _approve(visible[i]),
                                  onDiscard: () => _discard(visible[i]),
                                  child: card,
                                );
                              },
                            ),
                    ),
                    // On a phone the card's own buttons sit a screen and a
                    // half down — past the matched products, the anticipated
                    // objections and the whole offer text — on EVERY card.
                    // The desktop has keyboard shortcuts; the phone had
                    // nothing, and the phone is where the seller reads these
                    // between visits. So the action follows the card.
                    if (_isPhone(context) &&
                        _checked.isEmpty &&
                        current != null)
                      _PhoneActionBar(
                        busy: _busy,
                        sendBlocked: !page.canSend,
                        emptyDraft: current.draft.trim().isEmpty,
                        onApprove: () => _approve(current),
                        onEdit: () => _edit(current),
                        onDiscard: () => _discard(current),
                      ),
                    if (_checked.isNotEmpty)
                      _BulkBar(
                        count: _checked.length,
                        busy: _busy || !page.canSend,
                        onClear: () => setState(_checked.clear),
                        onApprove: () => _approveChecked(visible),
                      ),
                    // Shortcuts stop being a secret. They existed and were
                    // discoverable only by pressing a random letter — a
                    // feature nobody can find is a feature nobody has.
                    if (MediaQuery.of(context).size.width >= 900 &&
                        visible.isNotEmpty)
                      const _ShortcutBar(),
                  ],
                ),
              ),
            );
          },
        ),
      ),
    );
  }

  /// A phone is where this screen is actually worked, and a phone has no
  /// keyboard to press A/E/X on. Below this width the cards swipe instead.
  static bool _isPhone(BuildContext context) =>
      MediaQuery.of(context).size.width < 600;

  /// What is still waiting: the server's whole-queue count, less the cards this
  /// session removed (a discard inside its undo window, or one committing).
  ///
  /// Never below what is on screen — a header claiming 0 over three visible
  /// cards is a screen arguing with itself.
  int _backlog(ApprovalQueuePage page, List<QueueLead> visible) {
    final left = page.total - _hidden.length;
    return left < visible.length ? visible.length : left;
  }

  /// The funnel's name as the count list reported it. Null for a funnel the
  /// counts do not mention (they can fail on their own) — better no label than
  /// a ULID on a card.
  static String? _funnelName(ApprovalQueuePage page, String? funnelId) {
    if (funnelId == null) return null;
    for (final f in page.byFunnel) {
      if (f.funnelId == funnelId) {
        return f.funnelName.isEmpty ? null : f.funnelName;
      }
    }
    return null;
  }

  /// Whether an empty list is the operator's own filter rather than an empty
  /// queue. The undo window counts too: hiding the last card is not a backlog
  /// that ran out while it can still come back.
  bool _filterIsNarrowing(ApprovalQueueFilter filter, ApprovalQueuePage page) =>
      filter.funnelId != null ||
      filter.flaggedOnly ||
      (page.items.isNotEmpty && _hidden.isNotEmpty);

  /// Which channel this lead's approach leaves through, resolved funnel →
  /// channel — the same resolution the server does at send time. Falls back
  /// to the address shape (an '@' is an e-mail) when either list is not
  /// loaded, and to null when nothing can be said.
  ChannelType? _channelTypeFor(QueueLead lead) {
    final funnelId = lead.funnelId;
    if (funnelId != null && funnelId.isNotEmpty) {
      final funnels = ref.read(funnelsListProvider).asData?.value;
      String? channelId;
      for (final f in funnels ?? const []) {
        if (f.id == funnelId) {
          channelId = f.channelId as String?;
          break;
        }
      }
      if (channelId != null && channelId.isNotEmpty) {
        final channels = ref.read(channelsListProvider).asData?.value;
        for (final c in channels ?? const <ChannelModel>[]) {
          if (c.id == channelId) return c.type;
        }
      }
    }
    if (lead.recipientAddress.contains('@')) return ChannelType.email;
    return null;
  }

  /// Approving SENDS to a real person, so it confirms with the exact text and
  /// the recipient — the same gate as the lead panel, on purpose. A keyboard
  /// shortcut may make the queue fast; it must not make it automatic.
  ///
  /// The confirmation also collects what the channel needs: the e-mail
  /// subject, or the approved WhatsApp template — approving used to POST only
  /// the text, which sent e-mail on the default subject and had every
  /// WhatsApp approval bounce off Meta with a generic error.
  Future<void> _approve(QueueLead lead) async {
    if (_busy || lead.draft.trim().isEmpty) return;

    final messenger = ScaffoldMessenger.of(context);
    final spec = await showApproachSendDialog(
      context,
      recipient: recipientLine(lead.recipient, lead.recipientAddress),
      text: lead.draft,
      channelType: _channelTypeFor(lead),
      funnelId: lead.funnelId,
    );
    if (spec == null) return;

    setState(() => _busy = true);
    try {
      final tpl = spec.template;
      final scheduledFor = await _actions.approveSend(
        leadId: lead.id,
        text: lead.draft,
        subject: spec.subject,
        templateName: tpl?.name,
        templateLanguage: tpl?.language,
        templateCategory: tpl?.category?.metaValue,
        templateButtons: tpl == null ? null : waTemplateButtons(tpl),
      );
      messenger.showSnackBar(SnackBar(
          content: Text(scheduledFor != null
              ? t.lead_detail
                  .approve_scheduled(when: formatScheduledFor(scheduledFor))
              : t.lead_detail.sent)));
      ref.invalidate(approvalQueueProvider);
    } catch (e) {
      messenger.showSnackBar(SnackBar(
        key: const Key('approval_queue_send_error'),
        content: Text(parseApiError(e)),
        // An error that names a settings screen without a way there is a
        // dead end — the action IS the way there.
        action: _settingsActionFor(parseApiErrorCode(e)),
      ));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  /// A tap target for the refusals whose fix lives on a settings screen.
  SnackBarAction? _settingsActionFor(String? code) {
    final route = switch (code) {
      'BULK_SENDING_NOT_READY' => '/settings/channels',
      'EMAIL_DOMAIN_NOT_VERIFIED' => '/settings/channels',
      'EMAIL_ADDRESS_SUPPRESSED' => '/settings/email-suppressions',
      _ => null,
    };
    if (route == null) return null;
    return SnackBarAction(
      label: t.lead_detail.error_action_configure,
      onPressed: () => context.go(route),
    );
  }

  /// Sends everything ticked, after reviewing each one on its own.
  ///
  /// A bulk action that asks "send 12 approaches?" is not a review — it is a
  /// number, and nobody has read anything. The whole product posture is that a
  /// human sees the exact words before a stranger does, and doing twelve at once
  /// must not be the loophole in it.
  ///
  /// It used to confirm through the generic dialog with all twelve recipients
  /// and messages concatenated into one string. That dialog scrolls, so nothing
  /// was clipped — but it was a wall with no count, no boundaries and no way to
  /// drop one after reading it. The review dialog returns the ids that survived
  /// it, so what gets sent is what was actually kept.
  Future<void> _approveChecked(List<QueueLead> visible) async {
    if (_busy) return;
    final candidates = visible
        .where((l) => _checked.contains(l.id) && l.draft.trim().isNotEmpty)
        .toList();
    if (candidates.isEmpty) return;

    final messenger = ScaffoldMessenger.of(context);
    // WhatsApp leads are marked unselectable IN the review, with the reason
    // — reading and ticking twelve cards to then learn "Enviar 12" meant 0
    // made the review a lie. And any email in the batch requires a subject:
    // the volume path used to silently ship the server default.
    final blocked = {
      for (final l in candidates)
        if (_channelTypeFor(l) == ChannelType.whatsapp) l.id,
    };
    final hasEmail = candidates.any((l) => !blocked.contains(l.id));
    final result = await showBulkApprovalDialog(
      context,
      leads: candidates,
      blocked: blocked,
      blockedReason: t.lead_detail.bulk_needs_template,
      askSubject: hasEmail,
    );
    if (result == null || result.keptIds.isEmpty || !mounted) return;
    final picked =
        candidates.where((l) => result.keptIds.contains(l.id)).toList();

    setState(() => _busy = true);
    var sent = 0;
    // Grouped by WHY, not just who: nine failures that all read "daily cap
    // reached" are one fact, and dropping the reason left the operator
    // retrying sends that could not work today.
    final failedByReason = <String, List<String>>{};
    String? bulkActionCode;
    for (final lead in picked) {
      // Belt for the dialog's blocking: a WhatsApp approach needs a template
      // nobody picked in bulk.
      if (_channelTypeFor(lead) == ChannelType.whatsapp) {
        failedByReason
            .putIfAbsent(t.lead_detail.bulk_needs_template, () => [])
            .add(recipientLine(lead.recipient, lead.recipientAddress));
        continue;
      }
      try {
        // Every BATCH approval leaves through the platform's bulk provider
        // (hybrid model) — never through the customer's warming mailbox. The
        // API refuses with BULK_SENDING_NOT_READY until volume sending is
        // configured on the channel, and that refusal names the fix.
        await _actions.approveSend(
            leadId: lead.id,
            text: lead.draft,
            subject: result.subject,
            viaBulk: true);
        sent++;
      } catch (e) {
        // One refusal must not abort the rest: an opted-out contact or a
        // suppressed address is a fact about ONE lead, and stopping there
        // would leave the operator re-ticking eleven they already reviewed.
        bulkActionCode ??= parseApiErrorCode(e);
        failedByReason
            .putIfAbsent(parseApiError(e), () => [])
            .add(recipientLine(lead.recipient, lead.recipientAddress));
      }
    }
    if (!mounted) return;
    setState(() {
      _busy = false;
      _checked.clear();
    });
    ref.invalidate(approvalQueueProvider);
    final failureLines = [
      for (final entry in failedByReason.entries)
        '${entry.key}: ${entry.value.join(', ')}',
    ];
    messenger.showSnackBar(SnackBar(
      key: const Key('approval_queue_bulk_result'),
      // Named, not counted: "2 falharam" leaves the operator hunting for
      // which two.
      content: Text(failureLines.isEmpty
          ? t.lead_detail.bulk_sent(count: sent)
          : '${t.lead_detail.bulk_sent(count: sent)}\n'
              '${failureLines.join('\n')}'),
      action: _settingsActionFor(bulkActionCode),
    ));
  }

  /// Runs the second-opinion review on the text as it stands.
  ///
  /// A failure is reported as "did not run", never as "clean": a silent
  /// fallback to a green checkmark would tell the operator a model approved a
  /// message no model ever read.
  Future<void> _review(QueueLead lead) async {
    if (_reviewing.contains(lead.id) || lead.draft.trim().isEmpty) return;
    setState(() => _reviewing.add(lead.id));
    try {
      final review = await _actions.review(leadId: lead.id, text: lead.draft);
      if (!mounted) return;
      setState(() => _reviews[lead.id] = review);
    } catch (e) {
      if (!mounted) return;
      setState(() => _reviews[lead.id] =
          const ApproachReview(safe: false, checked: false, risks: []));
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(
        key: const Key('approval_queue_review_error'),
        content: Text(parseApiError(e)),
      ));
    } finally {
      if (mounted) setState(() => _reviewing.remove(lead.id));
    }
  }

  Future<void> _edit(QueueLead lead) async {
    await showLeadDetail(
      context,
      card: BoardCard(
        leadId: lead.id,
        stageId: '',
        status: 'pending_approval',
        score: 0,
        contactLabel: lead.recipient,
        companyName: lead.companyName,
        source: lead.source,
      ),
      stageName: t.lead_detail.queue_title,
      // Without the funnel the panel cannot load the funnel's field schema —
      // the fields that decide whether this lead is qualified vanished from
      // exactly the surface deciding whether to contact them.
      funnelId: lead.funnelId,
    );
    // The panel can send or rewrite the draft; either way the queue is stale.
    if (mounted) ref.invalidate(approvalQueueProvider);
  }

  /// Discards optimistically with a real undo: the card leaves the list at
  /// once, but the server is only told once the window closes.
  void _discard(QueueLead lead) {
    if (_discarding.containsKey(lead.id)) return;

    final messenger = ScaffoldMessenger.of(context);
    setState(() {
      _hidden.add(lead.id);
      _discarding[lead.id] = Timer(ApprovalQueueScreen.undoWindow, () {
        _discarding.remove(lead.id);
        // The offer expired: leaving "Desfazer" on screen would be a button
        // that silently does nothing.
        messenger.clearSnackBars();
        // Stays hidden through the commit — see _hidden.
        unawaited(_commitDiscard(lead.id));
      });
    });

    messenger.clearSnackBars();
    messenger.showSnackBar(SnackBar(
      key: const Key('approval_queue_discard_undo'),
      duration: ApprovalQueueScreen.undoWindow,
      content: Text(t.lead_detail.discarded),
      action: SnackBarAction(
        label: t.lead_detail.undo,
        onPressed: () => _undoDiscard(lead.id),
      ),
    ));
  }

  void _undoDiscard(String leadId) {
    final timer = _discarding.remove(leadId);
    if (timer == null) return; // already committing — nothing to take back
    timer.cancel();
    if (mounted) setState(() => _hidden.remove(leadId));
  }

  Future<void> _commitDiscard(String leadId) async {
    try {
      await _actions.discard(leadId: leadId);
    } catch (e) {
      // Failed: put the card back rather than leaving a lead invisible. This
      // is the safe direction — nothing was sent and nothing was lost.
      if (!mounted) return;
      setState(() => _hidden.remove(leadId));
      final messenger = ScaffoldMessenger.of(context)
        // The "discarded" toast is a lie once the discard failed.
        ..clearSnackBars();
      messenger.showSnackBar(SnackBar(
        key: const Key('approval_queue_discard_error'),
        content: Text(parseApiError(e)),
      ));
    }
    if (mounted) ref.invalidate(approvalQueueProvider);
  }
}

/// How to slice a backlog of sixty.
///
/// Two of the three controls reach the SQL (funnel, order) so they narrow the
/// whole queue; "flagged only" works over what is loaded and the caveat line
/// below says when that is less than everything. The chips carry counts read
/// server-side over the ENTIRE queue, which is what makes them a map of where
/// the work is rather than a description of the current page.
class _QueueFilterBar extends ConsumerWidget {
  const _QueueFilterBar({required this.page, required this.filter});

  final ApprovalQueuePage page;
  final ApprovalQueueFilter filter;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final notifier = ref.read(approvalQueueFilterProvider.notifier);

    // With one funnel in the backlog there is nothing to choose between, and a
    // lone chip next to "All" is two buttons for one list.
    final showFunnels = page.byFunnel.length > 1;

    return Padding(
      key: const Key('approval_queue_filter_bar'),
      padding: const EdgeInsets.fromLTRB(20, 4, 20, 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (showFunnels)
            SingleChildScrollView(
              scrollDirection: Axis.horizontal,
              child: Row(
                children: [
                  _Chip(
                    key: const Key('approval_queue_funnel_all'),
                    label: '${t.lead_detail.queue_filter_all} · ${page.total}',
                    selected: filter.funnelId == null,
                    onTap: () => notifier.selectFunnel(null),
                  ),
                  for (final f in page.byFunnel)
                    Padding(
                      padding: const EdgeInsets.only(left: 8),
                      child: _Chip(
                        key: Key('approval_queue_funnel_${f.funnelId}'),
                        label:
                            '${f.funnelName.isEmpty ? t.lead_detail.queue_no_funnel : f.funnelName} · ${f.total}',
                        selected: filter.funnelId == f.funnelId,
                        // Tapping the selected chip clears it: the way back to
                        // the whole queue should not require finding "All".
                        onTap: () => notifier.selectFunnel(
                            filter.funnelId == f.funnelId ? null : f.funnelId),
                      ),
                    ),
                ],
              ),
            ),
          if (showFunnels) const SizedBox(height: 8),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              _Chip(
                key: const Key('approval_queue_sort'),
                label: filter.newestFirst
                    ? t.lead_detail.queue_sort_newest
                    : t.lead_detail.queue_sort_oldest,
                icon: filter.newestFirst
                    ? Icons.arrow_downward
                    : Icons.arrow_upward,
                selected: filter.newestFirst,
                onTap: () => notifier.setNewestFirst(!filter.newestFirst),
              ),
              _Chip(
                key: const Key('approval_queue_flagged_only'),
                label: t.lead_detail.queue_flagged_only,
                icon: Icons.error_outline,
                selected: filter.flaggedOnly,
                onTap: () => notifier.setFlaggedOnly(!filter.flaggedOnly),
              ),
            ],
          ),
          // Said out loud rather than inferred from a short list: every filter
          // above is honest only about the drafts that arrived.
          if (page.isPartial) ...[
            const SizedBox(height: 8),
            Text(
              t.lead_detail.queue_partial(
                  shown: page.items.length.toString(),
                  total: page.total.toString()),
              key: const Key('approval_queue_partial'),
              style: typography.body
                  .copyWith(color: colors.orangeAlert, fontSize: 12),
            ),
          ],
        ],
      ),
    );
  }
}

class _Chip extends StatelessWidget {
  const _Chip({
    super.key,
    required this.label,
    required this.selected,
    required this.onTap,
    this.icon,
  });

  final String label;
  final bool selected;
  final VoidCallback onTap;
  final IconData? icon;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
        decoration: BoxDecoration(
          color: selected
              ? colors.bluePrimary.withValues(alpha: 0.18)
              : colors.backgroundSecondary,
          borderRadius: BorderRadius.circular(20),
          border: Border.all(
            color: selected ? colors.bluePrimary : colors.borderMedium,
          ),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (icon != null) ...[
              Icon(icon,
                  size: 14,
                  color: selected ? Colors.white : colors.textSecondary),
              const SizedBox(width: 6),
            ],
            Text(
              label,
              style: typography.body.copyWith(
                color: selected ? Colors.white : colors.textSecondary,
                fontSize: 12,
                fontWeight: selected ? FontWeight.w600 : FontWeight.w400,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _QueueCard extends StatelessWidget {
  const _QueueCard({
    required this.lead,
    required this.selected,
    required this.busy,
    required this.onSelect,
    required this.onApprove,
    required this.onEdit,
    required this.onDiscard,
    required this.onReview,
    required this.review,
    required this.reviewing,
    this.funnelLabel,
    this.channelType,
    this.sendBlocked = false,
  });

  final QueueLead lead;

  /// Which channel the approach leaves through, when the screen could resolve
  /// it. A phone number can be WhatsApp or SMS and an approver deserves the
  /// answer on the card, not after the send.
  final ChannelType? channelType;
  final bool selected;
  final bool busy;
  final VoidCallback onSelect;
  final VoidCallback onApprove;
  final VoidCallback onEdit;
  final VoidCallback onDiscard;
  final VoidCallback onReview;
  final ApproachReview? review;
  final bool reviewing;

  /// True when the account has nowhere to send from. The draft is real and
  /// worth reading; it just cannot leave, and the button says so instead of
  /// failing after the click.
  final bool sendBlocked;

  /// The funnel this draft belongs to, or null when the queue is showing one
  /// funnel anyway — repeating it on sixty cards says nothing.
  final String? funnelLabel;

  /// "na fila há 3d · Clínicas", collapsing to whichever half exists.
  String? _context() {
    final parts = <String>[
      if (formatQueueWait(lead.waitingSince) case final wait?) wait,
      if (funnelLabel case final label? when label.isNotEmpty) label,
    ];
    return parts.isEmpty ? null : parts.join(' · ');
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    return GestureDetector(
      onTap: onSelect,
      child: Container(
        key: Key('approval_queue_card_${lead.id}'),
        margin: const EdgeInsets.only(bottom: 12),
        padding: EdgeInsets.all(selected ? 20 : 16),
        decoration: BoxDecoration(
          color: colors.backgroundSecondary,
          borderRadius: BorderRadius.circular(selected ? 20 : 16),
          border: Border.all(
            // The highlight is what makes A/E/X unambiguous — a shortcut
            // whose target is invisible is a trap.
            color: selected
                ? colors.bluePrimary.withValues(alpha: 0.5)
                : colors.borderMedium,
            width: selected ? 1.5 : 1,
          ),
          boxShadow: selected
              ? [
                  BoxShadow(
                      color: Colors.black.withValues(alpha: 0.35),
                      blurRadius: 28,
                      offset: const Offset(0, 12)),
                ]
              : null,
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    lead.title,
                    style: typography.body.copyWith(
                        color: Colors.white,
                        fontSize: selected ? 18 : 15,
                        fontWeight: FontWeight.w700),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
                // Where the words came from, as a quiet chip — the mockup's
                // "origem:" tag, carrying the same honest sentence as before.
                Flexible(
                  child: Container(
                    padding:
                        const EdgeInsets.symmetric(horizontal: 10, vertical: 3),
                    decoration: BoxDecoration(
                      borderRadius: BorderRadius.circular(999),
                      border: Border.all(color: colors.borderMedium),
                    ),
                    child: Text(
                      lead.source?.trim().isNotEmpty == true
                          ? t.lead_detail
                              .written_by_ai(source: lead.source!.trim())
                          : t.lead_detail.written_by_ai_unknown,
                      style: typography.body
                          .copyWith(color: colors.textMuted, fontSize: 11),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 2),
            // ONE muted meta line, the mockup's: address · channel · age.
            // The name is already the title — repeating it here read as two
            // recipients. Two Texts so each half keeps its test key.
            Wrap(
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                Text(
                  '${lead.recipient.trim() == lead.title.trim() ? lead.recipientAddress : recipientLine(lead.recipient, lead.recipientAddress)}'
                  // The door it leaves through, beside who it reaches — the
                  // two halves of "where is this going?".
                  '${channelType != null ? ' · ${channelType!.label}' : ''}',
                  key: Key('approval_queue_recipient_${lead.id}'),
                  style: typography.body
                      .copyWith(color: colors.textSecondary, fontSize: 12),
                ),
                if (_context() != null)
                  Text(
                    ' · ${_context()!}',
                    key: Key('approval_queue_context_${lead.id}'),
                    style: typography.body
                        .copyWith(color: colors.textMuted, fontSize: 12),
                  ),
              ],
            ),
            // What is at stake and when it closes — the two facts that make
            // one card worth opening before another.
            //
            // The queue used to show fifty near-identical cards, every one
            // reading nothing about value or time, and it stayed untouched for
            // forty days. A list with no first item is a list nobody starts.
            if (lead.tenderDeadline != null || lead.tenderValue != null) ...[
              const SizedBox(height: 8),
              _TenderStrip(lead: lead),
            ],
            // Finished, correct, and addressed to nobody. Said on the card
            // because the repair is per-card work: find this buyer's e-mail.
            if (lead.isSendBlocked) ...[
              const SizedBox(height: 8),
              _NoAddressStrip(lead: lead),
            ],
            // An approved send waiting for the mailbox's budget: labeled,
            // never hidden. Approving again just refreshes the schedule.
            if (lead.scheduledFor != null) ...[
              const SizedBox(height: 8),
              Container(
                key: Key('queue_scheduled_badge_${lead.id}'),
                padding:
                    const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                decoration: BoxDecoration(
                  color: colors.bluePrimary.withValues(alpha: 0.12),
                  borderRadius: BorderRadius.circular(999),
                  border: Border.all(
                      color: colors.bluePrimary.withValues(alpha: 0.4)),
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Icon(Icons.schedule, size: 13, color: colors.bluePrimary),
                    const SizedBox(width: 5),
                    Text(
                      t.lead_detail.queue_scheduled_badge(
                          when: formatScheduledFor(lead.scheduledFor!)),
                      style: typography.body.copyWith(
                          color: colors.textSecondary, fontSize: 11.5),
                    ),
                  ],
                ),
              ),
            ],
            // WHICH product, and WHY — before the text that sells it. The
            // reason is the matcher's one sentence citing the fact it read;
            // an approver who disagrees with the pick should not have to
            // reverse-engineer it from the prose.
            if (lead.matchedProducts.isNotEmpty) ...[
              const SizedBox(height: 8),
              for (final p in lead.matchedProducts)
                Padding(
                  padding: const EdgeInsets.only(bottom: 4),
                  child: Row(
                    key:
                        Key('approval_queue_product_${lead.id}_${p.productId}'),
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Icon(Icons.inventory_2_outlined,
                          size: 14, color: colors.cyanGlow),
                      const SizedBox(width: 6),
                      Expanded(
                        child: RichText(
                          text: TextSpan(
                            style: typography.body.copyWith(
                                color: colors.textSecondary, fontSize: 12),
                            children: [
                              TextSpan(
                                text: p.name,
                                style: TextStyle(
                                    color: colors.textPrimary,
                                    fontWeight: FontWeight.w600),
                              ),
                              if (p.reason.isNotEmpty)
                                TextSpan(text: ' — ${p.reason}'),
                            ],
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
            ],
            // The pushback the matcher expects, with the counter ready —
            // whoever approves goes into the conversation armed instead of
            // discovering the objection live (objeções v1).
            if (lead.objections.isNotEmpty) ...[
              const SizedBox(height: 4),
              for (final o in lead.objections)
                Padding(
                  padding: const EdgeInsets.only(bottom: 4),
                  child: Row(
                    key: Key('approval_queue_objection_${lead.id}_${o.label}'),
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Icon(Icons.shield_outlined,
                          size: 14, color: colors.orangeAlert),
                      const SizedBox(width: 6),
                      Expanded(
                        child: RichText(
                          text: TextSpan(
                            style: typography.body.copyWith(
                                color: colors.textSecondary, fontSize: 12),
                            children: [
                              TextSpan(
                                text: '"${o.label}"',
                                style: TextStyle(
                                    color: colors.textPrimary,
                                    fontWeight: FontWeight.w600),
                              ),
                              if (o.counter.isNotEmpty)
                                TextSpan(text: ' → ${o.counter}'),
                            ],
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
            ],
            const SizedBox(height: 12),
            // The EXACT text that would be sent, verbatim and unshortened:
            // an approval based on a truncated preview is not an approval.
            // Plain on the card, like a letter — the mockup dropped the
            // box-inside-a-box.
            SelectableText(
              lead.draft,
              key: Key('approval_queue_text_${lead.id}'),
              style: typography.body.copyWith(
                  color: Colors.white.withValues(alpha: 0.92),
                  fontSize: selected ? 14.5 : 13,
                  height: 1.6),
            ),
            const SizedBox(height: 12),
            _ApproachVerdict(
              lead: lead,
              review: review,
              reviewing: reviewing,
              onReview: onReview,
            ),
            const SizedBox(height: 12),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                GradientPill(
                  key: Key('approval_queue_approve_${lead.id}'),
                  // `_approve` refuses an empty draft and says nothing about
                  // it. A live button that swallows the click is worse than
                  // an obviously dead one, and this is the send button. Same
                  // reasoning for an account with no channel: the send cannot
                  // succeed, so the button must not offer it.
                  onTap: busy || sendBlocked || lead.draft.trim().isEmpty
                      ? null
                      : onApprove,
                  icon: Icons.send,
                  label: t.lead_detail.approve_send,
                  fontSize: 14,
                  height: 42,
                  padding: const EdgeInsets.symmetric(horizontal: 22),
                ),
                GhostPill(
                  key: Key('approval_queue_edit_${lead.id}'),
                  onTap: busy ? null : onEdit,
                  label: t.lead_detail.edit,
                  fontSize: 14,
                  height: 42,
                  padding: const EdgeInsets.symmetric(horizontal: 26),
                ),
                SizedBox(
                  height: 42,
                  child: TextButton.icon(
                    key: Key('approval_queue_discard_${lead.id}'),
                    onPressed: busy ? null : onDiscard,
                    style: TextButton.styleFrom(
                        foregroundColor: colors.textMuted,
                        padding: const EdgeInsets.symmetric(horizontal: 14),
                        textStyle: const TextStyle(
                            fontSize: 14, fontWeight: FontWeight.w500)),
                    icon: const Icon(Icons.close, size: 15),
                    label: Text(t.lead_detail.discard),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

/// What the platform can honestly say about this message before it goes out.
///
/// Two layers, never conflated. The pattern pass ships with every card at no
/// cost and speaks only for the phrasings it knows. The AI pass costs a model
/// call, so it runs on demand — and until it has run, the card says so.
///
/// The distinction is the whole feature. A green line reading "reviewed" when
/// only a regex looked at the text would be exactly the kind of confident
/// wrong claim this product exists to catch in outbound messages; making it
/// about our own UI does not make it true.
class _ApproachVerdict extends StatelessWidget {
  const _ApproachVerdict({
    required this.lead,
    required this.review,
    required this.reviewing,
    required this.onReview,
  });

  final QueueLead lead;
  final ApproachReview? review;
  final bool reviewing;
  final VoidCallback onReview;

  /// AI risks not already reported by the pattern pass. The same sentence
  /// flagged twice under two names makes the reader hunt twice.
  List<ApproachFinding> get _extraRisks {
    final seen = lead.findings.map((f) => f.quote.toLowerCase()).toSet();
    return (review?.risks ?? const <ApproachFinding>[])
        .where((r) => !seen.contains(r.quote.toLowerCase()))
        .toList();
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    final all = [...lead.findings, ..._extraRisks];
    final clean = all.isEmpty;
    final accent = clean ? colors.textMuted : colors.orangeAlert;

    return Container(
      key: Key('approval_queue_verdict_${lead.id}'),
      width: double.infinity,
      // A clean verdict is one quiet line; only FINDINGS earn a frame —
      // the amber box exists to interrupt, and interrupting for "nothing
      // found" trains the eye to skip it.
      padding: clean
          ? const EdgeInsets.symmetric(vertical: 2)
          : const EdgeInsets.all(12),
      decoration: clean
          ? null
          : BoxDecoration(
              color: colors.backgroundPrimary,
              borderRadius: BorderRadius.circular(12),
              border: Border.all(color: colors.orangeAlert),
            ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // The verdict and its "second opinion" control share ONE line —
          // a dangling right-aligned link below read as a separate section.
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              Icon(
                clean ? Icons.check_circle_outline : Icons.error_outline,
                size: 16,
                color: clean ? colors.limeGlow : accent,
              ),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  clean
                      ? t.lead_detail.check_clean
                      : t.lead_detail.check_found(count: all.length),
                  key: Key('approval_queue_verdict_line_${lead.id}'),
                  style: typography.body.copyWith(
                      color: clean ? colors.textSecondary : Colors.white,
                      fontSize: 12.5,
                      height: 1.4),
                ),
              ),
              if (reviewing)
                Text(
                  t.lead_detail.review_running,
                  style: typography.body
                      .copyWith(color: colors.textMuted, fontSize: 12),
                )
              else if (review == null)
                TextButton(
                  key: Key('approval_queue_review_${lead.id}'),
                  // Dead on an empty draft: _review returned on the spot,
                  // a live button that does nothing.
                  onPressed: lead.draft.trim().isEmpty ? null : onReview,
                  style: TextButton.styleFrom(
                      padding: const EdgeInsets.symmetric(horizontal: 8),
                      minimumSize: Size.zero,
                      tapTargetSize: MaterialTapTargetSize.shrinkWrap),
                  child: Text(t.lead_detail.review_action,
                      style: const TextStyle(fontSize: 12.5)),
                ),
            ],
          ),
          for (final f in all) ...[
            const SizedBox(height: 8),
            _FindingRow(finding: f),
          ],
          if (review != null && !reviewing) ...[
            const SizedBox(height: 4),
            Text(
              _reviewLine(),
              key: Key('approval_queue_review_line_${lead.id}'),
              style: typography.body
                  .copyWith(color: colors.textSecondary, fontSize: 12),
            ),
          ],
        ],
      ),
    );
  }

  /// `checked == false` means the AI half never ran (no provider, a transport
  /// failure, an unparseable answer). It gets its own sentence — reporting it
  /// as "clean" would claim a review that did not happen.
  String _reviewLine() {
    final r = review!;
    if (!r.checked) return t.lead_detail.review_unavailable;
    if (r.risks.isEmpty) return t.lead_detail.review_clean;
    return t.lead_detail.review_found(count: r.risks.length);
  }
}

class _FindingRow extends StatelessWidget {
  const _FindingRow({required this.finding});

  final ApproachFinding finding;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    return Padding(
      padding: const EdgeInsets.only(left: 24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // A missing-attachment finding "quotes" the whole message, because
          // the problem is the message as a whole rather than one sentence.
          // Repeating it under the bubble it came from would be noise.
          if (finding.kind != 'missing_attachment')
            Text(
              '“${finding.quote}”',
              style: typography.body
                  .copyWith(color: Colors.white, fontSize: 13, height: 1.4),
            ),
          Text(
            finding.why ?? kindLabel(finding.kind),
            style: typography.body
                .copyWith(color: colors.textSecondary, fontSize: 12),
          ),
        ],
      ),
    );
  }
}

/// The plain-language name of a finding kind. An unknown kind falls back to
/// the promise wording rather than showing the raw server token — a screen
/// that leaks `invented_fact` at a salesperson has stopped explaining itself.
String kindLabel(String kind) {
  switch (kind) {
    case 'invented_fact':
      return t.lead_detail.check_kind_invented_fact;
    case 'out_of_scope':
      return t.lead_detail.check_kind_out_of_scope;
    case 'placeholder':
      return t.lead_detail.check_kind_placeholder;
    case 'tone':
      return t.lead_detail.check_kind_tone;
    case 'missing_attachment':
      return t.lead_detail.check_kind_missing_attachment;
    default:
      return t.lead_detail.check_kind_promise;
  }
}

/// The keyboard shortcuts, written down.
///
/// Desktop only: on a phone there is no keyboard to press and the row would
/// be four dead chips eating the thumb zone.
class _ShortcutBar extends StatelessWidget {
  const _ShortcutBar();

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    return Container(
      key: const Key('approval_queue_shortcuts'),
      width: double.infinity,
      padding: const EdgeInsets.fromLTRB(20, 12, 20, 12),
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        border: Border(top: BorderSide(color: colors.borderLight)),
      ),
      child: Wrap(
        spacing: 16,
        runSpacing: 8,
        children: [
          _ShortcutChip(keyCap: 'A', label: t.lead_detail.shortcut_approve),
          _ShortcutChip(keyCap: 'E', label: t.lead_detail.shortcut_edit),
          _ShortcutChip(keyCap: 'X', label: t.lead_detail.shortcut_discard),
          _ShortcutChip(keyCap: '↑ ↓', label: t.lead_detail.shortcut_move),
        ],
      ),
    );
  }
}

class _ShortcutChip extends StatelessWidget {
  const _ShortcutChip({required this.keyCap, required this.label});

  final String keyCap;
  final String label;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
          decoration: BoxDecoration(
            color: colors.backgroundTertiary,
            borderRadius: BorderRadius.circular(8),
            border: Border.all(color: colors.borderMedium),
          ),
          child: Text(
            keyCap,
            style: typography.body.copyWith(
                color: Colors.white, fontSize: 13, fontWeight: FontWeight.w600),
          ),
        ),
        const SizedBox(width: 8),
        Text(
          label,
          style: typography.body
              .copyWith(color: colors.textSecondary, fontSize: 13),
        ),
      ],
    );
  }
}

/// One card, swipeable.
///
/// Right approves, left discards — the gesture pair everyone already knows
/// from every triage app they use. The point is the queue: a backlog worked
/// with a thumb, one flick per lead, instead of aim-tap-scroll-aim-tap.
///
/// The important part is what right does NOT do. A swipe does not send: it
/// opens the same confirmation as the button, showing the exact text and the
/// recipient. Making the fastest gesture on the screen also the one that
/// puts a message in front of a stranger, with no reading step, is exactly
/// the accident this product exists to prevent — and a thumb brushing a list
/// while scrolling is not consent.
///
/// Left is different, and allowed to be immediate, because a discard is
/// undoable for five seconds and nothing has been sent.
class _SwipeableCard extends StatelessWidget {
  const _SwipeableCard({
    super.key,
    required this.lead,
    required this.onApprove,
    required this.onDiscard,
    required this.child,
  });

  final QueueLead lead;
  final VoidCallback onApprove;
  final VoidCallback onDiscard;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    return Dismissible(
      key: Key('dismissible_${lead.id}'),
      // A deliberate asymmetry: a discard is cheap and reversible, so a
      // normal flick does it. Approving crosses a threshold nearly to the
      // edge, because the gesture that reaches a real person should take
      // more of a decision than the one that does not.
      dismissThresholds: const {
        DismissDirection.startToEnd: 0.55,
        DismissDirection.endToStart: 0.35,
      },
      background: _swipeBackground(
        colors: colors,
        alignment: Alignment.centerLeft,
        color: colors.whatsappGreen,
        icon: Icons.send,
        label: t.lead_detail.approve_send,
      ),
      secondaryBackground: _swipeBackground(
        colors: colors,
        alignment: Alignment.centerRight,
        color: colors.redAlert,
        icon: Icons.close,
        label: t.lead_detail.discard,
      ),
      confirmDismiss: (direction) async {
        if (direction == DismissDirection.startToEnd) {
          // A card with no draft has nothing to approve — the swipe used
          // to complete the gesture and silently do nothing, a dead
          // primary interaction on the phone.
          if (lead.draft.trim().isEmpty) return false;
          // Never removes the card itself: _approve opens the confirmation
          // and the list refreshes only if the send actually happened.
          onApprove();
          return false;
        }
        onDiscard();
        // Also false — the discard is hidden by the undo bookkeeping, which
        // already knows how to put the card back if the operator changes
        // their mind. Letting Dismissible remove it too would leave a hole
        // no undo could fill.
        return false;
      },
      child: child,
    );
  }

  Widget _swipeBackground({
    required AppColorsExtension colors,
    required Alignment alignment,
    required Color color,
    required IconData icon,
    required String label,
  }) {
    return Container(
      margin: const EdgeInsets.only(bottom: 10),
      padding: const EdgeInsets.symmetric(horizontal: 20),
      alignment: alignment,
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.18),
        borderRadius: BorderRadius.circular(16),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, color: color, size: 18),
          const SizedBox(width: 6),
          Text(label,
              style: TextStyle(
                  color: color, fontSize: 12, fontWeight: FontWeight.w700)),
        ],
      ),
    );
  }
}

/// A tick box beside a card, for the bulk send.
class _CheckableCard extends StatelessWidget {
  const _CheckableCard({
    required this.lead,
    required this.checked,
    required this.onChanged,
    required this.child,
  });

  final QueueLead lead;
  final bool checked;
  final ValueChanged<bool> onChanged;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.only(top: 14, right: 4),
          child: Checkbox(
            key: Key('queue_check_${lead.id}'),
            value: checked,
            onChanged: (v) => onChanged(v ?? false),
          ),
        ),
        Expanded(child: child),
      ],
    );
  }
}

/// The phone's action bar: approve, edit or skip the card in view.
///
/// A sibling of the desktop's keyboard-shortcut strip, for the device that
/// has no keyboard. It acts on the SELECTED card — the one the list is
/// showing — so the button and the text on screen can never disagree.
class _PhoneActionBar extends StatelessWidget {
  const _PhoneActionBar({
    required this.busy,
    required this.sendBlocked,
    required this.emptyDraft,
    required this.onApprove,
    required this.onEdit,
    required this.onDiscard,
  });

  final bool busy;
  final bool sendBlocked;
  final bool emptyDraft;
  final VoidCallback onApprove;
  final VoidCallback onEdit;
  final VoidCallback onDiscard;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    // Same three refusals as the card's own button, for the same reasons —
    // a live control that swallows the tap is worse than a dead one.
    final blocked = busy || sendBlocked || emptyDraft;

    return Container(
      key: const Key('approval_queue_phone_actions'),
      padding: const EdgeInsets.fromLTRB(16, 10, 16, 10),
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        border: Border(top: BorderSide(color: colors.borderMedium)),
      ),
      child: SafeArea(
        top: false,
        child: Row(
          children: [
            IconButton(
              key: const Key('approval_queue_phone_discard'),
              onPressed: busy ? null : onDiscard,
              tooltip: t.lead_detail.discard,
              icon: const Icon(Icons.close, size: 20),
              color: colors.textMuted,
            ),
            IconButton(
              key: const Key('approval_queue_phone_edit'),
              onPressed: busy ? null : onEdit,
              tooltip: t.lead_detail.edit,
              icon: const Icon(Icons.edit_outlined, size: 19),
              color: colors.textSecondary,
            ),
            const SizedBox(width: 8),
            // The only labelled control: on 390px the two secondary actions
            // have to be icons or the primary one has no room for its own
            // words. Both keep their tooltip.
            Expanded(
              // FittedBox because the label is a translation: "Aprovar e
              // enviar" fits a 390px phone with the two icon buttons beside
              // it, and the next language's word for it may not. Shrinking
              // beats clipping a send button's own name.
              child: GradientPill(
                key: const Key('approval_queue_phone_approve'),
                onTap: blocked ? null : onApprove,
                icon: Icons.send,
                label: t.lead_detail.approve_send,
                fontSize: 14,
                height: 44,
                padding: const EdgeInsets.symmetric(horizontal: 8),
                fitLabel: true,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Said once, at the top: these drafts are finished and cannot leave.
///
/// A banner rather than an empty state, because the queue is NOT empty — the
/// work is done and worth reading. What is missing is the door, and the
/// button that opens it is one tap away.
class _SendBlockedBanner extends StatelessWidget {
  const _SendBlockedBanner();

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 12, 20, 0),
      child: Container(
        key: const Key('queue_send_blocked_banner'),
        padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 13),
        decoration: BoxDecoration(
          color: colors.glassLight,
          borderRadius: BorderRadius.circular(16),
          border: Border.all(color: colors.borderLight),
        ),
        child: Row(
          children: [
            Icon(Icons.link_off, size: 18, color: colors.textSecondary),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    t.lead_detail.queue_blocked_title,
                    style: const TextStyle(
                        fontSize: 14, fontWeight: FontWeight.w700),
                  ),
                  const SizedBox(height: 2),
                  Text(
                    t.lead_detail.queue_blocked_body,
                    style:
                        TextStyle(fontSize: 12.5, color: colors.textSecondary),
                  ),
                ],
              ),
            ),
            const SizedBox(width: 12),
            GradientPill(
              key: const Key('queue_send_blocked_connect'),
              onTap: () => context.go('/settings/channels'),
              label: t.lead_detail.queue_blocked_cta,
              fontSize: 13,
              height: 38,
              padding: const EdgeInsets.symmetric(horizontal: 18),
            ),
          ],
        ),
      ),
    );
  }
}

/// The bar that appears once anything is ticked.
class _BulkBar extends StatelessWidget {
  const _BulkBar({
    required this.count,
    required this.busy,
    required this.onClear,
    required this.onApprove,
  });

  final int count;
  final bool busy;
  final VoidCallback onClear;
  final VoidCallback onApprove;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    return Container(
      key: const Key('approval_queue_bulk_bar'),
      padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 10),
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        border: Border(top: BorderSide(color: colors.borderMedium)),
      ),
      child: Row(
        children: [
          Expanded(
            child: Text(
              t.lead_detail.bulk_selected(count: count),
              style: TextStyle(color: colors.textSecondary, fontSize: 13),
            ),
          ),
          TextButton(
            key: const Key('approval_queue_bulk_clear'),
            onPressed: busy ? null : onClear,
            child: Text(t.common.cancel),
          ),
          const SizedBox(width: 8),
          FilledButton.icon(
            key: const Key('approval_queue_bulk_send'),
            onPressed: busy ? null : onApprove,
            icon: const Icon(Icons.send, size: 16),
            label: Text(t.lead_detail.bulk_send),
          ),
        ],
      ),
    );
  }
}

/// Warmup state of the account's email mailbox, surfaced where the daily
/// limit actually bites: the approve button below it (Item 1 — the gate
/// fails closed, so the queue says where the ramp is before the refusal).
class _EmailWarmupBanner extends ConsumerStatefulWidget {
  const _EmailWarmupBanner();

  @override
  ConsumerState<_EmailWarmupBanner> createState() => _EmailWarmupBannerState();
}

class _EmailWarmupBannerState extends ConsumerState<_EmailWarmupBanner> {
  @override
  void initState() {
    super.initState();
    // The channels list is lazy; the queue may be the first screen to need
    // it. Loading an already-loaded list is a cheap refresh, not a loop.
    Future.microtask(() {
      // The tab this banner lives in can be swapped out within the first
      // frame (the role-gated tab set arrives with the session) — ref after
      // unmount throws.
      if (!mounted) return;
      final current = ref.read(channelsListProvider).asData?.value;
      if (current == null || current.isEmpty) {
        ref.read(channelsListProvider.notifier).load();
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final channels =
        ref.watch(channelsListProvider).asData?.value ?? const <ChannelModel>[];
    EmailWarmupStatus? warming;
    String mailboxName = '';
    for (final c in channels) {
      final w = c.emailWarmup;
      if (w != null && w.warming) {
        warming = w;
        // With two mailboxes "a caixa" is ambiguous — always name it.
        mailboxName = c.name;
        break;
      }
    }
    if (warming == null) return const SizedBox.shrink();

    // The mockup's route strip: the warm-up budget as the day's RHYTHM
    // ("envio 3 de 8"), not an amber warning to skim past. Zero left is
    // stated as the plan working — approvals now schedule for tomorrow.
    final exhausted = warming.remaining <= 0;
    final sent = warming.sentToday;
    final cap = warming.dailyCap;

    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 12, 20, 0),
      child: Container(
        key: const Key('queue_email_warmup_banner'),
        padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 13),
        decoration: BoxDecoration(
          color: colors.glassLight,
          borderRadius: BorderRadius.circular(16),
          border: Border.all(color: colors.borderLight),
        ),
        child: Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    exhausted
                        ? t.lead_detail.route_strip_exhausted
                        : t.lead_detail
                            .route_strip_title(n: '${sent + 1}', cap: '$cap'),
                    key: Key(exhausted
                        ? 'queue_warmup_exhausted'
                        : 'queue_warmup_progress'),
                    style: const TextStyle(
                        fontSize: 14, fontWeight: FontWeight.w700),
                  ),
                  const SizedBox(height: 2),
                  Text(
                    t.lead_detail.route_strip_sub(
                        mailbox: mailboxName,
                        day: '${warming.day}',
                        total: '${warming.rampDays}'),
                    style: TextStyle(color: colors.textMuted, fontSize: 11.5),
                  ),
                ],
              ),
            ),
            const SizedBox(width: 16),
            // Progress dots when the cap is small enough to count at a
            // glance; a bar once it grows past what a row of dots can say.
            if (cap > 0 && cap <= 12)
              Row(
                children: [
                  for (var i = 0; i < cap; i++)
                    Container(
                      width: 22,
                      height: 8,
                      margin: const EdgeInsets.only(left: 5),
                      decoration: BoxDecoration(
                        borderRadius: BorderRadius.circular(999),
                        color: i < sent
                            ? colors.bluePrimary
                            : (i == sent && !exhausted
                                ? colors.bluePrimary.withValues(alpha: 0.45)
                                : colors.borderLight),
                        border: i == sent && !exhausted
                            ? Border.all(color: colors.bluePrimary)
                            : null,
                      ),
                    ),
                ],
              )
            else if (cap > 0)
              SizedBox(
                width: 140,
                child: ClipRRect(
                  borderRadius: BorderRadius.circular(999),
                  child: LinearProgressIndicator(
                    value: (sent / cap).clamp(0, 1).toDouble(),
                    minHeight: 8,
                    backgroundColor: colors.borderLight,
                    color: colors.bluePrimary,
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

/// One row of the interleaved list: a group header, a lead (by its index in
/// the VISIBLE list, so selection and keyboard nav keep working), or the
/// "show all" unfold at the end of a collapsed cold group.
class _QueueRow {
  const _QueueRow._(this.header, this.leadIndex, this.showAllFrio);
  const _QueueRow.head(QueueSource source) : this._(source, null, false);
  const _QueueRow.lead(int index) : this._(null, index, false);
  const _QueueRow.showAll() : this._(null, null, true);

  final QueueSource? header;
  final int? leadIndex;
  final bool showAllFrio;
}

class _GroupHeader extends StatelessWidget {
  const _GroupHeader({
    required this.source,
    required this.count,
    required this.busy,
    this.onDiscardAll,
    this.onRedraftAll,
  });

  final QueueSource source;
  final int count;
  final bool busy;
  final VoidCallback? onDiscardAll;
  final VoidCallback? onRedraftAll;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final label = switch (source) {
      QueueSource.carteira => t.lead_detail.queue_group_carteira,
      QueueSource.sinal => t.lead_detail.queue_group_sinal,
      QueueSource.frio => t.lead_detail.queue_group_frio,
    };
    return Padding(
      padding: const EdgeInsets.fromLTRB(4, 14, 4, 6),
      child: Row(
        key: Key('queue_group_${source.name}'),
        children: [
          Text(
            label.toUpperCase(),
            style: TextStyle(
              color: colors.textMuted,
              fontSize: 11,
              fontWeight: FontWeight.w700,
              letterSpacing: 0.6,
            ),
          ),
          const SizedBox(width: 6),
          Text('($count)',
              style: TextStyle(color: colors.textMuted, fontSize: 11)),
          const Spacer(),
          if (onDiscardAll != null)
            OutlinedButton(
              key: const Key('queue_frio_discard_all'),
              onPressed: busy ? null : onDiscardAll,
              style: OutlinedButton.styleFrom(
                foregroundColor: colors.redAlert,
                side: BorderSide(color: colors.redAlert.withValues(alpha: 0.5)),
                padding:
                    const EdgeInsets.symmetric(horizontal: 14, vertical: 6),
                minimumSize: Size.zero,
                tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(999)),
              ),
              child: Text(t.lead_detail.queue_frio_discard_all(count: '$count'),
                  style: const TextStyle(
                      fontSize: 12, fontWeight: FontWeight.w600)),
            ),
          if (onRedraftAll != null) ...[
            const SizedBox(width: 8),
            GhostPill(
              key: const Key('queue_frio_redraft_all'),
              label: t.lead_detail.queue_frio_redraft_all,
              fontSize: 12,
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 6),
              onTap: busy ? null : onRedraftAll,
            ),
          ],
        ],
      ),
    );
  }
}

/// "Esta semana: 12 enviadas · 3 respostas · 1 proposta" — the only line on
/// this screen that says whether the ritual is working.
class _OutcomeLine extends ConsumerWidget {
  const _OutcomeLine();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final outcome = ref.watch(offersOutcomeProvider).asData?.value;
    if (outcome == null) return const SizedBox.shrink();
    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 0, 20, 4),
      child: Text(
        t.lead_detail.queue_outcome_line(
          sent: '${outcome.sent}',
          replies: '${outcome.replies}',
          proposals: '${outcome.proposals}',
        ),
        key: const Key('queue_outcome_line'),
        style: TextStyle(color: colors.textMuted, fontSize: 12),
      ),
    );
  }
}

/// One lead as a single line: who, the first words, how long it has waited.
/// Clicking anywhere makes it the full card above — the compact form is for
/// scanning, never for deciding blind (approve lives only on the open card).
class _CompactLeadRow extends StatelessWidget {
  const _CompactLeadRow({
    required this.lead,
    required this.busy,
    required this.onSelect,
    required this.onDiscard,
  });

  final QueueLead lead;
  final bool busy;
  final VoidCallback onSelect;
  final VoidCallback onDiscard;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return GestureDetector(
      onTap: onSelect,
      behavior: HitTestBehavior.opaque,
      child: Container(
        key: Key('approval_queue_card_${lead.id}'),
        margin: const EdgeInsets.only(bottom: 6),
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 9),
        decoration: BoxDecoration(
          color: colors.glassLight,
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: colors.borderLight),
        ),
        child: Row(
          children: [
            SizedBox(
              width: 180,
              child: Text(
                lead.title,
                style:
                    const TextStyle(fontSize: 13, fontWeight: FontWeight.w600),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Text(
                lead.draft,
                style: TextStyle(color: colors.textMuted, fontSize: 12),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
            ),
            if (lead.findings.isNotEmpty) ...[
              const SizedBox(width: 8),
              Icon(Icons.error_outline, size: 14, color: colors.orangeAlert),
            ],
            if (lead.scheduledFor != null) ...[
              const SizedBox(width: 8),
              Icon(Icons.schedule, size: 14, color: colors.bluePrimary),
            ],
            if (formatQueueWait(lead.waitingSince) case final wait?) ...[
              const SizedBox(width: 8),
              Text(wait,
                  style: TextStyle(color: colors.textMuted, fontSize: 11)),
            ],
            const SizedBox(width: 4),
            IconButton(
              key: Key('approval_queue_discard_${lead.id}'),
              onPressed: busy ? null : onDiscard,
              icon: Icon(Icons.close, size: 16, color: colors.textMuted),
              tooltip: t.sales.leads.discard,
              visualDensity: VisualDensity.compact,
            ),
          ],
        ),
      ),
    );
  }
}

/// The tender's clock and its size, as one quiet line.
///
/// Deliberately NOT a big number: the operator needs to rank cards, not read a
/// dashboard. Colour carries the urgency, the words carry the fact.
class _TenderStrip extends StatelessWidget {
  const _TenderStrip({required this.lead});

  final QueueLead lead;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final days = lead.daysToDeadline(DateTime.now());

    // Under three days is a reason to act today; expired is a reason not to
    // write at all. Everything between is ordinary.
    final urgent = days != null && days >= 0 && days <= 3;
    final expired = days != null && days < 0;
    final tone = expired
        ? colors.textMuted
        : urgent
            ? colors.redAlert
            : colors.textSecondary;

    final parts = <String>[
      if (days != null)
        expired
            ? t.lead_detail.queue_tender_deadline_past
            : days == 0
                ? t.lead_detail.queue_tender_deadline_today
                : t.lead_detail.queue_tender_deadline_days(count: days),
      if (lead.tenderValue case final value? when value.isNotEmpty)
        t.lead_detail.queue_tender_value(value: value),
    ];
    if (parts.isEmpty) return const SizedBox.shrink();

    return Row(
      key: Key('queue_tender_strip_${lead.id}'),
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(expired ? Icons.event_busy : Icons.gavel_outlined,
            size: 13, color: tone),
        const SizedBox(width: 6),
        Flexible(
          child: Text(
            parts.join(' · '),
            style: typography.body.copyWith(
                color: tone,
                fontSize: 12,
                fontWeight: urgent ? FontWeight.w700 : FontWeight.w400),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
        ),
      ],
    );
  }
}

/// "Mandar pelo meu WhatsApp" — the escape hatch on a blocked card.
///
/// It opens the seller's own WhatsApp with the approved text already in the
/// box. No integration: the Business API bills per conversation and needs
/// templates approved weeks in advance, and neither is worth paying for a
/// message this seller can send in ten seconds from the phone in their pocket.
///
/// Because nothing leaves this product, the card asks afterwards. A queue that
/// keeps showing an offer already delivered is the same lie as one that hides
/// it — and the seller is the only witness to what happened.
class _SendByMyWhatsApp extends ConsumerStatefulWidget {
  const _SendByMyWhatsApp({required this.lead});

  final QueueLead lead;

  @override
  ConsumerState<_SendByMyWhatsApp> createState() => _SendByMyWhatsAppState();
}

class _SendByMyWhatsAppState extends ConsumerState<_SendByMyWhatsApp> {
  bool _busy = false;

  Future<void> _open() async {
    final url =
        whatsappLink(widget.lead.whatsappPhone ?? '', widget.lead.draft);
    if (url == null) return;
    await launchUrl(Uri.parse(url), mode: LaunchMode.externalApplication);
    if (!mounted) return;

    // Asked, never assumed. Opening WhatsApp is not sending, and a lead moved
    // on an intention the seller abandoned is a customer nobody will call
    // again.
    final sent = await showConfirmDialog(
      context,
      title: t.lead_detail.queue_wa_confirm_title,
      message: t.lead_detail.queue_wa_confirm_body,
      confirmText: t.lead_detail.queue_wa_confirm_yes,
      cancelText: t.lead_detail.queue_wa_confirm_no,
    );
    if (!sent || !mounted) return;

    setState(() => _busy = true);
    try {
      await ref.read(queueManualSendProvider).markSentByHand(widget.lead.id);
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(parseApiError(e))));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        TextButton.icon(
          key: Key('queue_wa_send_${widget.lead.id}'),
          onPressed: _busy ? null : _open,
          icon: const Icon(Icons.chat_bubble_outline, size: 15),
          label: Text(t.lead_detail.queue_wa_send,
              style: const TextStyle(fontSize: 12.5)),
          style: TextButton.styleFrom(
            foregroundColor: colors.whatsappGreen,
            padding: const EdgeInsets.symmetric(horizontal: 8),
            minimumSize: const Size(0, 32),
            tapTargetSize: MaterialTapTargetSize.shrinkWrap,
          ),
        ),
        const SizedBox(width: 6),
        Flexible(
          child: Text(
            t.lead_detail.queue_wa_hint,
            style: TextStyle(color: colors.textMuted, fontSize: 11),
          ),
        ),
      ],
    );
  }
}

/// The offer is written and there is nowhere to post it.
///
/// A card rather than a disabled button: the work left is not "approve", it is
/// "find the buyer's e-mail", and that is a task with an end. The account-level
/// banner cannot say this — the account has a channel; this recipient has no
/// address behind it.
class _NoAddressStrip extends StatelessWidget {
  const _NoAddressStrip({required this.lead});

  final QueueLead lead;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    return Container(
      key: Key('queue_no_address_${lead.id}'),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        color: colors.redAlert.withValues(alpha: 0.10),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: colors.redAlert.withValues(alpha: 0.35)),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(Icons.markunread_mailbox_outlined,
              size: 15, color: colors.redAlert),
          const SizedBox(width: 9),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  t.lead_detail.queue_no_address_title,
                  style: typography.body.copyWith(
                      color: colors.textPrimary,
                      fontSize: 12.5,
                      fontWeight: FontWeight.w700),
                ),
                const SizedBox(height: 2),
                Text(
                  t.lead_detail.queue_no_address_body,
                  style: typography.body
                      .copyWith(color: colors.textSecondary, fontSize: 12),
                ),
                // The way out, when the record left a phone instead of an
                // address. Hidden when there is no dialable number — a button
                // that opens a chat with nobody is worse than no button.
                if (whatsappLink(lead.whatsappPhone ?? '', lead.draft) != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 8),
                    child: _SendByMyWhatsApp(lead: lead),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// Puts the cards with a closing date in front of the ones without, nearest
/// first, and leaves the server's ordering alone underneath.
///
/// A queue is worked top-down, so its order IS its priority. The server orders
/// by age, which is the right default for drafts that are all alike — but an
/// offer answering a tender that closes on Thursday is not alike, and burying
/// it under fifty older cards with no deadline is how a real opportunity
/// expires in a list somebody scrolled past.
///
/// Stable, so the age order survives inside each group. Client-side over the
/// loaded page like the flagged filter, and for the same reason: there is no
/// column to sort on server-side, and a partial page announces itself.
///
/// Top-level so it can be tested without pumping the whole screen.
List<QueueLead> deadlineFirst(List<QueueLead> items, DateTime now) {
  final withDate = <QueueLead>[];
  final withoutDate = <QueueLead>[];
  for (final lead in items) {
    (lead.tenderDeadline == null ? withoutDate : withDate).add(lead);
  }
  withDate.sort((a, b) {
    final da = a.daysToDeadline(now)!;
    final db = b.daysToDeadline(now)!;
    // An expired tender sinks below every live one: writing about a closed
    // notice proves nobody read it. It stays in the list — the operator still
    // has to discard it — just not at the top.
    final expiredA = da < 0, expiredB = db < 0;
    if (expiredA != expiredB) return expiredA ? 1 : -1;
    return da.compareTo(db);
  });
  return [...withDate, ...withoutDate];
}
