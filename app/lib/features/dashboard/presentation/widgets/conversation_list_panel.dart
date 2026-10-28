import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:app/core/network/websocket_service.dart';
import 'package:app/features/dashboard/presentation/controllers/dashboard_controller.dart';
import 'package:app/features/dashboard/presentation/controllers/inbox_assignment_filter_controller.dart';
import 'package:app/features/dashboard/presentation/widgets/bulk_actions_bar.dart';
import 'package:app/features/dashboard/presentation/widgets/conversation_tile.dart';
import 'package:app/features/home/presentation/controllers/onboarding_progress_controller.dart';
import 'package:app/features/support/presentation/widgets/support_nav_button.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/i18n/strings.g.dart';

/// One inbox, filtered — not one tab per pipe. The salesperson thinks about
/// the clinic, and splitting email from WhatsApp would cut one customer's
/// story in half. Email leads because it is the SDR product's main channel;
/// the rest follow in the order they were shipped.
///
/// Out of the build method because two screens now read the same list: the bar
/// and the filter dialog. Two copies drift, and drift here would be an option
/// the server rejects.
List<({String label, String? value})> get inboxChannelOptions => [
      (label: t.dashboard.channel_all, value: null),
      (label: t.dashboard.channel_email, value: 'email'),
      (label: t.dashboard.channel_whatsapp, value: 'whatsapp'),
      (label: t.dashboard.channel_telegram, value: 'telegram'),
      (label: t.dashboard.channel_instagram, value: 'instagram'),
      (label: t.dashboard.channel_sms, value: 'sms'),
    ];

List<({String label, String? value})> get inboxStatusOptions => [
      (label: t.dashboard.filter_all, value: null),
      (label: t.dashboard.filter_open, value: 'open'),
      (label: t.dashboard.filter_pending, value: 'pending_human'),
      (label: t.dashboard.filter_closed, value: 'closed'),
    ];

/// Must match the backend enum exactly. "" is the default ORDER BY
/// (last_customer_message_at DESC).
List<({String label, String value})> get inboxSortOptions => [
      (label: t.dashboard.sort_recent, value: ''),
      (label: t.dashboard.sort_first_in_conv, value: 'first_in_conv'),
      (label: t.dashboard.sort_oldest, value: 'oldest'),
      // Triage order: a lead who answered outranks a newsletter that arrived a
      // minute ago. Chronological is right for a chat and wrong for a shared
      // commercial mailbox.
      (label: t.dashboard.sort_importance, value: 'importance'),
    ];

/// Mirrors domain.InboxCategory exactly — the value reaches SQL, and the
/// server rejects anything outside the set.
List<({String label, String? value})> get inboxCategoryOptions => [
      (label: t.dashboard.category_all, value: null),
      (label: t.dashboard.category_hot_lead, value: 'hot_lead'),
      (label: t.dashboard.category_customer, value: 'customer'),
      (label: t.dashboard.category_financial, value: 'financial'),
      (label: t.dashboard.category_supplier, value: 'supplier'),
      (label: t.dashboard.category_newsletter, value: 'newsletter'),
      (label: t.dashboard.category_quarantine, value: 'quarantine'),
      (label: t.dashboard.category_other, value: 'other'),
    ];

class ConversationListPanel extends ConsumerStatefulWidget {
  const ConversationListPanel({super.key});

  @override
  ConsumerState<ConversationListPanel> createState() =>
      _ConversationListPanelState();
}

class _ConversationListPanelState extends ConsumerState<ConversationListPanel> {
  bool _isSearching = false;
  final _searchController = TextEditingController();
  final _searchFocusNode = FocusNode();

  /// Debounce for the search box: the provider is a dependency of
  /// conversationsProvider, so an un-debounced onChanged fired one GET
  /// /conversations per keystroke.
  Timer? _searchDebounce;

  @override
  void dispose() {
    _searchDebounce?.cancel();
    _searchController.dispose();
    _searchFocusNode.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    // Direct WS listener as a safety net — ensures list updates even if RealtimeListener
    // hasn't been activated yet (e.g., conversations provider not yet built).
    ref.listen(wsEventsProvider, (_, next) {
      next.whenData((event) {
        if (event.event == 'new_message') {
          ref.invalidate(conversationsProvider);
        }
        if (event.event == 'reply_reminder') {
          // Surface a top-of-screen snackbar so the attendant who is
          // already inside the dashboard sees a customer they may have
          // forgotten. Tapping the action opens that conversation. Push
          // notification fires in parallel for anyone with the tab
          // unfocused; this is the in-app counterpart.
          final convID = event.conversationId;
          final raw = event.raw;
          final contactName =
              (raw['contact_name'] as String?) ?? t.dashboard.unknown_contact;
          final minutes = (raw['minutes_waiting'] as int?) ?? 0;
          final messenger = ScaffoldMessenger.maybeOf(context);
          if (messenger != null) {
            messenger.showSnackBar(
              SnackBar(
                behavior: SnackBarBehavior.floating,
                backgroundColor: colors.redAlert,
                content: Text(
                  t.dashboard.reply_reminder_toast(
                    contact: contactName,
                    minutes: minutes,
                  ),
                ),
                action: convID == null
                    ? null
                    : SnackBarAction(
                        label: t.dashboard.reply_reminder_open,
                        textColor: Colors.white,
                        onPressed: () {
                          ref
                              .read(selectedConversationProvider.notifier)
                              .select(convID);
                        },
                      ),
              ),
            );
          }
          ref.invalidate(conversationsProvider);
        }
      });
    });

    final conversationsAsync = ref.watch(conversationsProvider);

    return Column(
      children: [
        // Header
        Container(
          padding: const EdgeInsets.fromLTRB(20, 16, 12, 12),
          decoration: BoxDecoration(
            color: colors.backgroundSecondary,
            border: Border(
              bottom: BorderSide(color: colors.borderLight, width: 0.5),
            ),
          ),
          child: Row(
            children: [
              if (!_isSearching)
                Expanded(
                  child: Text(
                    t.dashboard.conversations_title,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      fontSize: 22,
                      fontWeight: FontWeight.w800,
                      color: colors.textPrimary,
                      letterSpacing: -0.5,
                    ),
                  ),
                ),
              if (_isSearching)
                Expanded(
                  child: Container(
                    height: 36,
                    decoration: BoxDecoration(
                      color: colors.glassLight,
                      borderRadius: BorderRadius.circular(10),
                      border: Border.all(color: colors.borderMedium),
                    ),
                    child: TextField(
                      controller: _searchController,
                      focusNode: _searchFocusNode,
                      onChanged: (v) {
                        _searchDebounce?.cancel();
                        _searchDebounce =
                            Timer(const Duration(milliseconds: 300), () {
                          if (!mounted) return;
                          ref
                              .read(conversationSearchProvider.notifier)
                              .update(v);
                        });
                      },
                      style: TextStyle(
                        fontSize: 14,
                        color: colors.textPrimary,
                      ),
                      decoration: InputDecoration(
                        hintText: t.dashboard.search_conversations,
                        hintStyle:
                            TextStyle(color: colors.textMuted, fontSize: 14),
                        border: InputBorder.none,
                        contentPadding: const EdgeInsets.symmetric(
                          horizontal: 12,
                          vertical: 8,
                        ),
                        isDense: true,
                        prefixIcon: Icon(
                          Icons.search,
                          size: 18,
                          color: colors.textMuted,
                        ),
                      ),
                    ),
                  ),
                ),
              const SizedBox(width: 4),
              IconButton(
                icon: Icon(
                  _isSearching ? Icons.close : Icons.search,
                  color: colors.textSecondary,
                  size: 20,
                ),
                onPressed: () {
                  setState(() {
                    _isSearching = !_isSearching;
                    if (!_isSearching) {
                      _searchController.clear();
                      ref.read(conversationSearchProvider.notifier).clear();
                    } else {
                      Future.microtask(() => _searchFocusNode.requestFocus());
                    }
                  });
                },
                splashRadius: 20,
                tooltip: _isSearching
                    ? t.dashboard.close_search
                    : t.dashboard.search,
              ),
              // Analytics, Vendas, Vendedores and Configurações used to sit
              // here. They are ACCOUNT-level, not inbox-level: reaching
              // Billing should not require opening Conversas first, and a
              // conversation list's header should be about conversations.
              // They now live in the shell's account menu (account_menu.dart).
              // Support stays: it is a CONVERSATION with the platform, and
              // it opens a thread in this very inbox.
              const SupportNavButton(),
            ],
          ),
        ),
        // Status filter chips
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
          decoration: BoxDecoration(
            color: colors.backgroundSecondary,
            border: Border(
              bottom: BorderSide(color: colors.borderLight, width: 0.5),
            ),
          ),
          child: _buildStatusFilters(colors),
        ),
        // Conversation list
        Expanded(
          child: conversationsAsync.when(
            data: (conversations) {
              // Search-results counter: when a query is active, show
              // "N resultados" just above the list. Previously an empty
              // result set rendered as the same "no conversations"
              // empty state as a brand-new account, leaving the user
              // unsure whether the search worked at all.
              final query = _searchController.text.trim();
              final hasQuery = _isSearching && query.isNotEmpty;
              if (conversations.isEmpty) {
                // For a real (non-search) empty inbox, surface an
                // onboarding-aware next-step CTA: brand-new accounts get
                // nudged to connect a channel, set-up accounts get nudged
                // to share their test link to land the first conversation.
                if (!hasQuery) {
                  // Empty BECAUSE OF FILTERS is not an empty inbox: a
                  // mature account filtering "Não atribuídas" was shown
                  // "conecte um canal" and a button to /onboarding.
                  // The assignment scope only counts as a "filter" when it
                  // is the explicit Não-atribuídas bucket: "Minhas" is a
                  // ROLE DEFAULT, and an attendant's genuinely empty inbox
                  // must still get the onboarding CTA.
                  final filtered = ref.read(conversationStatusFilterProvider) !=
                          null ||
                      ref.read(conversationChannelFilterProvider) != null ||
                      ref.read(conversationCategoryFilterProvider) != null ||
                      ref.read(inboxAssignmentFilterProvider) ==
                          InboxAssignmentScope.unassigned;
                  if (filtered) {
                    return Center(
                      child: Column(
                        mainAxisAlignment: MainAxisAlignment.center,
                        children: [
                          Icon(Icons.filter_alt_off,
                              size: 48, color: colors.textDisabled),
                          const SizedBox(height: 12),
                          Text(
                            t.dashboard.no_conversations_for_filters,
                            style: TextStyle(
                                color: colors.textMuted, fontSize: 14),
                          ),
                          const SizedBox(height: 8),
                          TextButton(
                            key: const Key('inbox_clear_filters_empty'),
                            onPressed: () {
                              _clearFilters();
                              ref
                                  .read(inboxAssignmentFilterProvider.notifier)
                                  .set(InboxAssignmentScope.all);
                            },
                            child: Text(t.dashboard.clear_filters),
                          ),
                        ],
                      ),
                    );
                  }
                  return _EmptyInboxCta(colors: colors);
                }
                return Center(
                  child: Column(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Icon(
                        Icons.search_off,
                        size: 48,
                        color: colors.textDisabled,
                      ),
                      const SizedBox(height: 12),
                      Text(
                        t.dashboard.no_conversations_for_query(query: query),
                        style: TextStyle(
                          color: colors.textMuted,
                          fontSize: 14,
                        ),
                      ),
                    ],
                  ),
                );
              }
              // A full page means there may be more behind it; a short one
              // is the end of the list.
              final canLoadMore = conversations.length >=
                  ref.watch(conversationListLimitProvider);
              return Column(
                children: [
                  if (hasQuery)
                    Container(
                      width: double.infinity,
                      padding: const EdgeInsets.symmetric(
                          horizontal: 16, vertical: 6),
                      color: colors.backgroundSecondary.withValues(alpha: 0.5),
                      child: Text(
                        t.dashboard.results_for_query(
                            count: conversations.length.toString(),
                            query: query),
                        style: TextStyle(color: colors.textMuted, fontSize: 11),
                      ),
                    ),
                  BulkActionsBar(
                    conversationIds: conversations.map((c) => c.id).toList(),
                  ),
                  Expanded(
                    child: RefreshIndicator(
                      onRefresh: () async {
                        ref.invalidate(conversationsProvider);
                      },
                      child: ListView.builder(
                        // Stable key so Flutter restores the scroll offset
                        // when the user navigates away and back (mobile flow:
                        // tap a conversation, press back). Previously the
                        // list always rewound to the top.
                        key: const PageStorageKey<String>(
                            'conversation_list_scroll'),
                        // One trailing row when the page came back full:
                        // "carregar mais" grows the server-side limit (the
                        // list stays ONE sorted list, never stitched pages).
                        itemCount: conversations.length + (canLoadMore ? 1 : 0),
                        itemBuilder: (context, index) {
                          if (index == conversations.length) {
                            return Padding(
                              padding: const EdgeInsets.symmetric(vertical: 8),
                              child: Center(
                                child: TextButton.icon(
                                  key: const Key('load_more_conversations'),
                                  onPressed: () => ref
                                      .read(conversationListLimitProvider
                                          .notifier)
                                      .loadMore(),
                                  icon: const Icon(Icons.expand_more, size: 18),
                                  label: Text(t.common.load_more),
                                ),
                              ),
                            );
                          }
                          return ConversationTile(
                              conversation: conversations[index]);
                        },
                      ),
                    ),
                  ),
                ],
              );
            },
            loading: () => const Center(
              child: CircularProgressIndicator(),
            ),
            error: (err, stack) => Center(
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Icon(Icons.error_outline, size: 48, color: colors.redAlert),
                    const SizedBox(height: 12),
                    Text(
                      t.dashboard.error_loading_conversations,
                      style: TextStyle(
                        color: colors.textSecondary,
                        fontSize: 16,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    const SizedBox(height: 8),
                    Text(
                      t.dashboard.check_connection,
                      style: TextStyle(color: colors.textMuted, fontSize: 12),
                      textAlign: TextAlign.center,
                      maxLines: 3,
                      overflow: TextOverflow.ellipsis,
                    ),
                    const SizedBox(height: 16),
                    TextButton.icon(
                      onPressed: () => ref.invalidate(conversationsProvider),
                      icon: const Icon(Icons.refresh),
                      label: Text(t.common.retry),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildStatusFilters(AppColorsExtension colors) {
    final currentFilter = ref.watch(conversationStatusFilterProvider);
    final currentSort = ref.watch(conversationSortProvider);
    // "Mine / Unassigned / All" assignment scope (US-017/US-020).
    // While the persisted preference is still resolving, render "Todas" —
    // the safe superset.
    final scope = ref.watch(inboxAssignmentFilterProvider).asData?.value ??
        InboxAssignmentScope.all;
    final channel = ref.watch(conversationChannelFilterProvider);
    final category = ref.watch(conversationCategoryFilterProvider);

    // How many slices are on. Zero lets the button just say "Filters"; the
    // number is what stops a short list from reading as an empty inbox.
    final active = [
      currentFilter != null,
      channel != null,
      currentSort.isNotEmpty,
      category != null,
    ].where((on) => on).length;

    return Column(
      children: [
        SizedBox(
          width: double.infinity,
          child: SegmentedButton<InboxAssignmentScope>(
            segments: [
              ButtonSegment(
                value: InboxAssignmentScope.mine,
                label: Text(t.dashboard.assignment_mine),
                icon: const Icon(Icons.person_outline, size: 16),
              ),
              ButtonSegment(
                value: InboxAssignmentScope.unassigned,
                label: Text(t.dashboard.assignment_unassigned),
                icon: const Icon(Icons.person_off_outlined, size: 16),
              ),
              ButtonSegment(
                value: InboxAssignmentScope.all,
                label: Text(t.dashboard.assignment_all),
                icon: const Icon(Icons.groups_outlined, size: 16),
              ),
            ],
            selected: {scope},
            onSelectionChanged: (selection) => ref
                .read(inboxAssignmentFilterProvider.notifier)
                .set(selection.first),
            showSelectedIcon: false,
            style: SegmentedButton.styleFrom(
              visualDensity: VisualDensity.compact,
              textStyle: const TextStyle(
                fontSize: 12,
                fontWeight: FontWeight.w600,
              ),
              side: BorderSide(color: colors.borderMedium),
            ),
          ),
        ),
        const SizedBox(height: 8),
        // Status, channel, order and category lived here, always open: four
        // stacked selectors, ~90px of fixed height in a 380px sidebar, above
        // the list that is the reason for the screen. In a 900px window that
        // left a little over three conversations.
        //
        // The scope (mine/unassigned/all) stays: it is the triage used all the
        // time. The rest goes behind a button that SAYS how many are on — a
        // short list caused by a forgotten filter is indistinguishable from an
        // empty inbox, and that was the risk of hiding them.
        Row(
          children: [
            Expanded(
              child: OutlinedButton.icon(
                key: const Key('inbox_filters_button'),
                onPressed: () => _showFilterSheet(context),
                icon: const Icon(Icons.tune, size: 16),
                label: Text(
                  active == 0
                      ? t.dashboard.filters_button
                      : t.dashboard.filters_button_n(count: active),
                  overflow: TextOverflow.ellipsis,
                ),
                style: OutlinedButton.styleFrom(
                  visualDensity: VisualDensity.compact,
                  foregroundColor:
                      active == 0 ? colors.textSecondary : colors.bluePrimary,
                  side: BorderSide(
                      color: active == 0
                          ? colors.borderMedium
                          : colors.bluePrimary),
                  textStyle: const TextStyle(
                      fontSize: 12, fontWeight: FontWeight.w600),
                ),
              ),
            ),
            if (active > 0) ...[
              const SizedBox(width: 8),
              TextButton(
                key: const Key('inbox_filters_clear'),
                onPressed: _clearFilters,
                style: TextButton.styleFrom(
                  visualDensity: VisualDensity.compact,
                  textStyle: const TextStyle(
                      fontSize: 12, fontWeight: FontWeight.w600),
                ),
                child: Text(t.dashboard.filters_clear),
              ),
            ],
          ],
        ),
      ],
    );
  }

  void _clearFilters() {
    ref.read(conversationStatusFilterProvider.notifier).update(null);
    ref.read(conversationChannelFilterProvider.notifier).update(null);
    ref.read(conversationSortProvider.notifier).update('');
    ref.read(conversationCategoryFilterProvider.notifier).update(null);
  }

  Future<void> _showFilterSheet(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return showDialog<void>(
      context: context,
      builder: (_) => AlertDialog(
        key: const Key('inbox_filters_sheet'),
        backgroundColor: colors.backgroundSecondary,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
        title: Text(
          t.dashboard.filters_title,
          style:
              TextStyle(color: colors.textPrimary, fontWeight: FontWeight.w700),
        ),
        content: const SizedBox(width: 320, child: _InboxFilterFields()),
        actions: [
          TextButton(
            key: const Key('inbox_filters_sheet_clear'),
            onPressed: _clearFilters,
            child: Text(t.dashboard.filters_clear,
                style: TextStyle(color: colors.textSecondary)),
          ),
          FilledButton(
            key: const Key('inbox_filters_sheet_close'),
            onPressed: () => Navigator.of(context).pop(),
            child: Text(t.common.close),
          ),
        ],
      ),
    );
  }
}

/// The four slices, labelled.
///
/// Loose in the bar they were four unnamed boxes — "All", "All", "Most
/// recent", "All" — and finding out which one moved what cost a click each. A
/// dialog has room for the label.
class _InboxFilterFields extends ConsumerWidget {
  const _InboxFilterFields();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final status = ref.watch(conversationStatusFilterProvider);
    final channel = ref.watch(conversationChannelFilterProvider);
    final sort = ref.watch(conversationSortProvider);
    final category = ref.watch(conversationCategoryFilterProvider);

    Widget field(String label, Widget child) => Padding(
          padding: const EdgeInsets.only(bottom: 14),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(label,
                  style: TextStyle(
                      color: colors.textSecondary,
                      fontSize: 12,
                      fontWeight: FontWeight.w600)),
              const SizedBox(height: 6),
              child,
            ],
          ),
        );

    return SingleChildScrollView(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          field(
            t.dashboard.filter_label_status,
            _FilterDropdown<String?>(
              key: const Key('inbox_status_filter'),
              colors: colors,
              value: status,
              items: [
                for (final o in inboxStatusOptions)
                  DropdownMenuItem<String?>(
                      value: o.value, child: Text(o.label)),
              ],
              onChanged: (v) =>
                  ref.read(conversationStatusFilterProvider.notifier).update(v),
            ),
          ),
          field(
            t.dashboard.filter_label_channel,
            _FilterDropdown<String?>(
              key: const Key('inbox_channel_filter'),
              colors: colors,
              value: channel,
              items: [
                for (final o in inboxChannelOptions)
                  DropdownMenuItem<String?>(
                      value: o.value, child: Text(o.label)),
              ],
              onChanged: (v) => ref
                  .read(conversationChannelFilterProvider.notifier)
                  .update(v),
            ),
          ),
          field(
            t.dashboard.filter_label_sort,
            _FilterDropdown<String>(
              key: const Key('inbox_sort_filter'),
              colors: colors,
              value: sort,
              items: [
                for (final o in inboxSortOptions)
                  DropdownMenuItem<String>(
                      value: o.value, child: Text(o.label)),
              ],
              onChanged: (v) =>
                  ref.read(conversationSortProvider.notifier).update(v ?? ''),
            ),
          ),
          field(
            t.dashboard.filter_label_category,
            _FilterDropdown<String?>(
              key: const Key('inbox_category_filter'),
              colors: colors,
              value: category,
              items: [
                for (final o in inboxCategoryOptions)
                  DropdownMenuItem<String?>(
                      value: o.value, child: Text(o.label)),
              ],
              onChanged: (v) => ref
                  .read(conversationCategoryFilterProvider.notifier)
                  .update(v),
            ),
          ),
        ],
      ),
    );
  }
}

/// Onboarding-aware empty inbox state. Shown only for a genuinely empty
/// inbox (no active search). Brand-new accounts (`!onboarding_completed`)
/// are nudged to connect a channel; already set-up accounts are nudged to
/// send offers so the first customer reply actually arrives.
class _EmptyInboxCta extends ConsumerWidget {
  final AppColorsExtension colors;

  const _EmptyInboxCta({required this.colors});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // Default to the "connect a channel" nudge while the progress is
    // loading or unavailable — that's the safe path for a fresh account.
    final completed = ref
            .watch(onboardingProgressControllerProvider)
            .asData
            ?.value
            ?.onboardingCompleted ??
        false;

    final hint = completed
        ? t.dashboard.empty_inbox_hint_share
        : t.dashboard.empty_inbox_hint_setup;
    final cta = completed
        ? t.dashboard.empty_inbox_cta_share
        : t.dashboard.empty_inbox_cta_setup;
    final icon =
        completed ? Icons.local_offer_outlined : Icons.chat_bubble_outline;

    void onPressed() {
      if (completed) {
        // Channel connected but no replies yet — the next SDR action is to
        // send offers, so point them at the queue rather than a "share link".
        context.go('/prospect');
      } else {
        // Resume the onboarding flow at the channel-connection step.
        context.go('/onboarding');
      }
    }

    // Scrollable: the panel is short on a phone and got shorter when the
    // triage filter joined the header, so a fixed Column overflowed. An
    // empty state that clips its own call to action is worse than one that
    // scrolls.
    return SingleChildScrollView(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(icon, size: 48, color: colors.textDisabled),
            const SizedBox(height: 12),
            Text(
              t.dashboard.no_conversations_found,
              style: TextStyle(
                color: colors.textSecondary,
                fontSize: 16,
                fontWeight: FontWeight.w700,
              ),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 8),
            Text(
              hint,
              style: TextStyle(color: colors.textMuted, fontSize: 13),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 20),
            FilledButton.icon(
              onPressed: onPressed,
              icon: Icon(
                completed ? Icons.ios_share : Icons.add_link,
                size: 18,
              ),
              label: Text(cta),
            ),
          ],
        ),
      ),
    );
  }
}

/// Compact dropdown used for the status + sort selectors at the top
/// of the conversation list. Uses [MenuAnchor] instead of the framework
/// [DropdownButton] so we can track open state and flip the trailing
/// caret between `arrow_drop_down` and `arrow_drop_up`.
class _FilterDropdown<T> extends StatefulWidget {
  final AppColorsExtension colors;
  final T? value;
  final List<DropdownMenuItem<T>> items;
  final ValueChanged<T?> onChanged;

  const _FilterDropdown({
    super.key,
    required this.colors,
    required this.value,
    required this.items,
    required this.onChanged,
  });

  @override
  State<_FilterDropdown<T>> createState() => _FilterDropdownState<T>();
}

class _FilterDropdownState<T> extends State<_FilterDropdown<T>> {
  final MenuController _menu = MenuController();
  bool _open = false;

  DropdownMenuItem<T>? _selectedItem() {
    for (final item in widget.items) {
      if (item.value == widget.value) return item;
    }
    return widget.items.isNotEmpty ? widget.items.first : null;
  }

  @override
  Widget build(BuildContext context) {
    final colors = widget.colors;
    final selected = _selectedItem();
    return LayoutBuilder(
      builder: (context, constraints) {
        return MenuAnchor(
          controller: _menu,
          onOpen: () => setState(() => _open = true),
          onClose: () => setState(() => _open = false),
          style: MenuStyle(
            backgroundColor: WidgetStatePropertyAll(colors.backgroundSecondary),
            padding: const WidgetStatePropertyAll(EdgeInsets.zero),
            shape: const WidgetStatePropertyAll(RoundedRectangleBorder(
              borderRadius: BorderRadius.all(Radius.circular(10)),
            )),
          ),
          menuChildren: widget.items
              .map(
                (item) => MenuItemButton(
                  onPressed: () => widget.onChanged(item.value),
                  style: MenuItemButton.styleFrom(
                    minimumSize: Size(constraints.maxWidth, 36),
                    alignment: Alignment.centerLeft,
                    foregroundColor: colors.textPrimary,
                    textStyle: const TextStyle(
                      fontSize: 12,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  child: DefaultTextStyle(
                    style: TextStyle(
                      color: colors.textPrimary,
                      fontSize: 12,
                      fontWeight: FontWeight.w600,
                    ),
                    child: item.child,
                  ),
                ),
              )
              .toList(),
          builder: (context, controller, _) {
            return InkWell(
              borderRadius: BorderRadius.circular(8),
              onTap: () =>
                  controller.isOpen ? controller.close() : controller.open(),
              child: Container(
                height: 34,
                padding: const EdgeInsets.only(left: 10, right: 4),
                decoration: BoxDecoration(
                  color: colors.glassMedium,
                  borderRadius: BorderRadius.circular(8),
                  border: Border.all(color: colors.borderMedium),
                ),
                child: Row(
                  children: [
                    Expanded(
                      child: DefaultTextStyle(
                        style: TextStyle(
                          color: colors.textPrimary,
                          fontSize: 12,
                          fontWeight: FontWeight.w600,
                        ),
                        overflow: TextOverflow.ellipsis,
                        maxLines: 1,
                        child: selected?.child ?? const SizedBox.shrink(),
                      ),
                    ),
                    Icon(
                      _open ? Icons.expand_less : Icons.expand_more,
                      size: 18,
                      color: colors.textMuted,
                    ),
                  ],
                ),
              ),
            );
          },
        );
      },
    );
  }
}
