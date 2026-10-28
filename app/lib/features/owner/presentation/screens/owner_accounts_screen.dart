import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:app/core/error/api_error_parser.dart';
import 'package:app/design_system/components/app_dialogs.dart';
import 'package:app/design_system/components/empty_state_card.dart';
import 'package:app/design_system/components/glass_card.dart';
import 'package:app/design_system/components/retryable_error.dart';
import 'package:app/design_system/components/settings_section_header.dart';
import 'package:app/design_system/components/status_badge.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/owner/domain/models/owner_account_models.dart';
import 'package:app/features/owner/presentation/controllers/owner_controller.dart';
import 'package:app/features/owner/presentation/widgets/invite_customer_dialog.dart';
import 'package:app/features/owner/presentation/widgets/owner_2fa_step_up.dart';
import 'package:app/i18n/strings.g.dart';

/// How the loaded page of accounts is ordered client-side. Server paging
/// stays by recency; the operator re-sorts the visible page so the
/// heaviest / most at-risk accounts surface without a round-trip.
enum _SortMode { consumption, recent, status }

/// Quick mutating actions exposed straight from a list row so the
/// operator doesn't have to open the detail screen for the common ones.
enum _QuickAction { impersonate, suspend, reactivate }

class OwnerAccountsScreen extends ConsumerStatefulWidget {
  const OwnerAccountsScreen({super.key});

  @override
  ConsumerState<OwnerAccountsScreen> createState() =>
      _OwnerAccountsScreenState();
}

class _OwnerAccountsScreenState extends ConsumerState<OwnerAccountsScreen> {
  final _searchCtrl = TextEditingController();
  String _search = '';
  String _status = '';
  _SortMode _sort = _SortMode.consumption;
  bool _overdueFirst = false;

  static const _statuses = <(String, String)>[
    ('', 'all'),
    ('active', 'active'),
    ('trialing', 'trialing'),
    ('past_due', 'past_due'),
    ('suspended', 'suspended'),
  ];

  @override
  void dispose() {
    _searchCtrl.dispose();
    super.dispose();
  }

  String _statusLabel(String key) {
    switch (key) {
      case '':
        return t.owner.filter_all;
      case 'active':
        return t.owner.filter_active;
      case 'trialing':
        return t.owner.filter_trialing;
      case 'past_due':
        return t.owner.filter_past_due;
      case 'suspended':
        return t.owner.filter_suspended;
    }
    return key;
  }

  String _sortLabel(_SortMode m) {
    switch (m) {
      case _SortMode.consumption:
        return t.owner.sort_consumption;
      case _SortMode.recent:
        return t.owner.sort_recent;
      case _SortMode.status:
        return t.owner.sort_status;
    }
  }

  /// Rank for status-sort + overdue-first: lower sorts earlier.
  int _statusRank(OwnerAccountListing it) {
    if (it.account.isSuspended) return 0;
    switch (it.account.subscriptionStatus) {
      case 'past_due':
        return 1;
      case 'trialing':
        return 2;
      case 'active':
        return 3;
      default:
        return 4;
    }
  }

  List<OwnerAccountListing> _sorted(List<OwnerAccountListing> items) {
    final list = [...items];
    switch (_sort) {
      case _SortMode.consumption:
        list.sort((a, b) {
          final c = b.capPercent.compareTo(a.capPercent);
          return c != 0 ? c : b.tokensUsed.compareTo(a.tokensUsed);
        });
      case _SortMode.recent:
        list.sort((a, b) => b.account.createdAt.compareTo(a.account.createdAt));
      case _SortMode.status:
        list.sort((a, b) => _statusRank(a).compareTo(_statusRank(b)));
    }
    if (_overdueFirst) {
      list.sort((a, b) {
        int risk(OwnerAccountListing it) => it.account.isSuspended ||
                it.account.subscriptionStatus == 'past_due'
            ? 0
            : 1;
        return risk(a).compareTo(risk(b));
      });
    }
    return list;
  }

  // ── Quick-row actions (mirror the detail screen) ──────────────────

  Future<void> _quickAction(OwnerAccountSummary acc, _QuickAction a) async {
    switch (a) {
      case _QuickAction.impersonate:
        await _impersonate(acc.id);
      case _QuickAction.suspend:
        await _suspend(acc.id);
      case _QuickAction.reactivate:
        await _reactivate(acc.id);
    }
  }

  Future<void> _impersonate(String id) async {
    final ok = await showConfirmDialog(
      context,
      title: t.owner.impersonate_confirm_title,
      message: t.owner.impersonate_confirm_msg,
      confirmText: t.owner.impersonate_cta,
    );
    if (!ok) return;
    try {
      final out = await ref.read(ownerActionsProvider.notifier).impersonate(id);
      final actorEmail = ref
          .read(authControllerProvider)
          .maybeWhen(data: (u) => u?.email ?? '', orElse: () => '');
      await ref
          .read(impersonationControllerProvider.notifier)
          .start(out, actorEmail);
      // Refresh /me on the impersonation token so the whole app reflects
      // the impersonated account, not the operator.
      await ref.read(authControllerProvider.notifier).refreshMe();
      if (mounted) context.go('/home');
    } catch (e) {
      _toast(parseApiError(e));
    }
  }

  Future<void> _suspend(String id) async {
    final reason = await showInputDialog(
      context,
      title: t.owner.suspend_dialog_title,
      hint: t.owner.suspend_reason_hint,
      confirmText: t.owner.suspend_confirm_cta,
    );
    if (reason == null || reason.isEmpty) return;
    await _run(
        () => ref.read(ownerActionsProvider.notifier).suspend(id, reason));
  }

  Future<void> _reactivate(String id) async {
    final ok = await showConfirmDialog(
      context,
      title: t.owner.reactivate_confirm_title,
      message: t.owner.reactivate_confirm_msg,
      confirmText: t.owner.reactivate_cta,
    );
    if (!ok) return;
    await _run(() => ref.read(ownerActionsProvider.notifier).reactivate(id));
  }

  Future<void> _run(Future<void> Function() action) async {
    try {
      // An expired 2FA proof asks for the code in place and retries the action,
      // instead of dead-ending the operator into a full re-login.
      await withOwnerStepUp(context, ref, action);
      _toast(t.common.success);
    } catch (e) {
      _toast(parseApiError(e));
    }
  }

  void _toast(String msg) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(msg)));
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final spacing = Theme.of(context).extension<AppSpacingExtension>()!;
    final accessAsync = ref.watch(ownerAccessStatusProvider);
    final access = accessAsync.maybeWhen(
      data: (s) => s,
      orElse: () => OwnerAccessStatus.allowed,
    );
    if (access != OwnerAccessStatus.allowed) {
      return Scaffold(
        backgroundColor: colors.backgroundPrimary,
        appBar: _appBar(colors),
        body: _OwnerAccessGate(access: access),
      );
    }
    final async = ref.watch(
      ownerAccountsProvider(search: _search, status: _status),
    );

    return Scaffold(
      backgroundColor: colors.backgroundPrimary,
      appBar: _appBar(colors),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: () => context.push('/owner/accounts/new'),
        backgroundColor: colors.bluePrimary,
        foregroundColor: Colors.white,
        icon: const Icon(Icons.add),
        label: Text(t.owner.create_contract_cta),
      ),
      body: ListView(
        padding: EdgeInsets.only(bottom: spacing.xxl * 3),
        children: [
          _SystemHealthBar(
            colors: colors,
            typography: typography,
            spacing: spacing,
            activeStatus: _status,
            onSelect: (s) => setState(() => _status = s),
          ),
          _TopAccountsStrip(
            colors: colors,
            typography: typography,
            spacing: spacing,
          ),
          Padding(
            padding: EdgeInsets.fromLTRB(spacing.lg, spacing.md, spacing.lg, 0),
            child: _FilterChips(
              statuses: _statuses,
              current: _status,
              labelFor: _statusLabel,
              colors: colors,
              onSelected: (s) => setState(() => _status = s),
            ),
          ),
          Padding(
            padding: EdgeInsets.fromLTRB(spacing.lg, spacing.md, spacing.lg, 0),
            child: Row(
              children: [
                Expanded(child: _searchField(colors)),
                SizedBox(width: spacing.sm),
                _sortButton(colors),
              ],
            ),
          ),
          SizedBox(height: spacing.sm),
          async.when(
            loading: () => const Padding(
              padding: EdgeInsets.all(40),
              child: Center(child: CircularProgressIndicator()),
            ),
            error: (e, _) => Padding(
              padding: EdgeInsets.all(spacing.lg),
              child: RetryableError(
                error: e,
                onRetry: () => ref.invalidate(ownerAccountsProvider),
              ),
            ),
            data: (data) {
              if (data.items.isEmpty) {
                return Padding(
                  padding: const EdgeInsets.all(24),
                  child: EmptyStateCard(
                    icon: Icons.business_outlined,
                    title: t.owner.empty_title,
                    subtitle: t.owner.empty_desc,
                  ),
                );
              }
              final items = _sorted(data.items);
              return Column(
                children: [
                  for (final it in items)
                    Padding(
                      padding: EdgeInsets.fromLTRB(
                          spacing.lg, 0, spacing.lg, spacing.sm),
                      child: _AccountTile(
                        item: it,
                        colors: colors,
                        typography: typography,
                        spacing: spacing,
                        onAction: _quickAction,
                      ),
                    ),
                ],
              );
            },
          ),
        ],
      ),
    );
  }

  PreferredSizeWidget _appBar(AppColorsExtension colors) {
    return AppBar(
      backgroundColor: colors.backgroundSecondary,
      elevation: 0,
      title: Text(
        t.owner.accounts_title,
        style: TextStyle(
          fontSize: 20,
          fontWeight: FontWeight.w700,
          color: colors.textPrimary,
        ),
      ),
      leading: IconButton(
        icon: Icon(Icons.arrow_back, color: colors.textSecondary),
        onPressed: () => context.canPop() ? context.pop() : context.go('/home'),
      ),
      actions: [
        // The sales-led door: a one-shot invite is how a new customer's
        // account gets permission to exist (docs/fluxo-criacao-conta.md).
        IconButton(
          key: const Key('owner_invite_action'),
          icon: Icon(Icons.person_add_alt_1_outlined,
              color: colors.textSecondary),
          tooltip: t.owner.invite_title,
          onPressed: () => showDialog<void>(
            context: context,
            builder: (_) => const InviteCustomerDialog(),
          ),
        ),
        IconButton(
          icon: Icon(Icons.request_quote_outlined, color: colors.textSecondary),
          tooltip: t.owner.provider_costs_title,
          onPressed: () => context.push('/owner/provider-costs'),
        ),
      ],
    );
  }

  Widget _searchField(AppColorsExtension colors) {
    return TextField(
      controller: _searchCtrl,
      onSubmitted: (v) => setState(() => _search = v.trim()),
      style: TextStyle(color: colors.textPrimary, fontSize: 14),
      decoration: InputDecoration(
        isDense: true,
        filled: true,
        fillColor: colors.glassMedium.withValues(alpha: 0.5),
        hintText: t.owner.search_hint,
        hintStyle: TextStyle(color: colors.textMuted, fontSize: 14),
        prefixIcon: Icon(Icons.search, color: colors.textSecondary, size: 20),
        suffixIcon: _searchCtrl.text.isEmpty
            ? null
            : IconButton(
                icon: Icon(Icons.close, size: 18, color: colors.textSecondary),
                onPressed: () {
                  _searchCtrl.clear();
                  setState(() => _search = '');
                },
              ),
      ),
    );
  }

  Widget _sortButton(AppColorsExtension colors) {
    return Container(
      decoration: BoxDecoration(
        color: colors.glassMedium.withValues(alpha: 0.5),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: colors.borderMedium),
      ),
      child: PopupMenuButton<Object>(
        tooltip: t.owner.sort_label,
        icon: Icon(Icons.sort, color: colors.textSecondary, size: 20),
        color: colors.backgroundSecondary,
        onSelected: (v) {
          if (v is _SortMode) setState(() => _sort = v);
          if (v == 'overdue') setState(() => _overdueFirst = !_overdueFirst);
        },
        itemBuilder: (_) => [
          for (final m in _SortMode.values)
            CheckedPopupMenuItem(
              value: m,
              checked: _sort == m,
              child: Text(_sortLabel(m)),
            ),
          const PopupMenuDivider(),
          CheckedPopupMenuItem(
            value: 'overdue',
            checked: _overdueFirst,
            child: Text(t.owner.overdue_first),
          ),
        ],
      ),
    );
  }
}

/// Clickable platform-health strip in a glass card. Total / Active /
/// Suspended double as quick status filters; the 24h volume stats are
/// read-only. Hidden while loading, SizedBox.shrink on error so a Stripe
/// blip doesn't break the screen.
class _SystemHealthBar extends ConsumerWidget {
  final AppColorsExtension colors;
  final AppTypographyExtension typography;
  final AppSpacingExtension spacing;
  final String activeStatus;
  final ValueChanged<String> onSelect;

  const _SystemHealthBar({
    required this.colors,
    required this.typography,
    required this.spacing,
    required this.activeStatus,
    required this.onSelect,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final async = ref.watch(ownerSystemHealthProvider);
    return async.maybeWhen(
      data: (h) => Padding(
        padding: EdgeInsets.fromLTRB(spacing.lg, spacing.md, spacing.lg, 0),
        child: GlassCard(
          padding: EdgeInsets.symmetric(
              horizontal: spacing.sm, vertical: spacing.md),
          child: Row(
            children: [
              _tile(t.owner.health_accounts, h.accountsTotal, filter: ''),
              _divider(),
              _tile(t.owner.health_active, h.accountsActive,
                  filter: 'active', accent: colors.cyanGlow),
              _divider(),
              _tile(t.owner.health_suspended, h.accountsSuspended,
                  filter: 'suspended',
                  accent: h.accountsSuspended > 0 ? colors.redAlert : null),
              _divider(),
              _tile(t.owner.health_conv_24h, h.conversations24h),
              _divider(),
              _tile(t.owner.health_msg_24h, h.messages24h),
            ],
          ),
        ),
      ),
      orElse: () => const SizedBox.shrink(),
    );
  }

  Widget _divider() =>
      Container(width: 1, height: 28, color: colors.borderMedium);

  Widget _tile(String label, int value, {String? filter, Color? accent}) {
    final selected = filter != null && filter == activeStatus;
    final content = Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          '$value',
          style: typography.body.copyWith(
            color:
                selected ? colors.bluePrimary : (accent ?? colors.textPrimary),
            fontSize: 16,
            fontWeight: FontWeight.w700,
          ),
        ),
        SizedBox(height: spacing.xs / 2),
        Text(
          label,
          style: typography.body.copyWith(
            color: selected ? colors.bluePrimary : colors.textSecondary,
            fontSize: 10,
            fontWeight: selected ? FontWeight.w600 : FontWeight.w400,
          ),
        ),
      ],
    );
    return Expanded(
      child: filter == null
          ? Padding(
              padding: EdgeInsets.symmetric(vertical: spacing.xs),
              child: content)
          : InkWell(
              borderRadius: BorderRadius.circular(10),
              onTap: () => onSelect(filter),
              child: Container(
                padding: EdgeInsets.symmetric(vertical: spacing.xs),
                decoration: BoxDecoration(
                  color: selected
                      ? colors.bluePrimary.withValues(alpha: 0.12)
                      : Colors.transparent,
                  borderRadius: BorderRadius.circular(10),
                ),
                child: content,
              ),
            ),
    );
  }
}

/// Horizontal strip of the heaviest accounts this period as glass mini
/// cards (rank + name + tok/conv). Quick-glance so the operator spots a
/// customer near a cap before churning.
class _TopAccountsStrip extends ConsumerWidget {
  final AppColorsExtension colors;
  final AppTypographyExtension typography;
  final AppSpacingExtension spacing;

  const _TopAccountsStrip({
    required this.colors,
    required this.typography,
    required this.spacing,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final async = ref.watch(ownerTopAccountsProvider());
    return async.maybeWhen(
      data: (resp) {
        if (resp.items.isEmpty) return const SizedBox.shrink();
        return Padding(
          padding: EdgeInsets.fromLTRB(spacing.lg, spacing.lg, 0, 0),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Padding(
                padding: EdgeInsets.only(right: spacing.lg),
                child: SettingsSectionHeader(label: t.owner.top_label),
              ),
              SizedBox(height: spacing.sm),
              SizedBox(
                height: 72,
                child: ListView.separated(
                  scrollDirection: Axis.horizontal,
                  padding: EdgeInsets.only(right: spacing.lg),
                  itemCount: resp.items.length,
                  separatorBuilder: (_, __) => SizedBox(width: spacing.sm),
                  itemBuilder: (_, i) {
                    final it = resp.items[i];
                    return GlassCard(
                      width: 196,
                      padding: EdgeInsets.all(spacing.md),
                      child: InkWell(
                        borderRadius: BorderRadius.circular(12),
                        onTap: () => context.push('/owner/accounts/${it.id}'),
                        child: Row(
                          children: [
                            _RankChip(rank: i + 1, colors: colors),
                            SizedBox(width: spacing.sm),
                            Expanded(
                              child: Column(
                                mainAxisSize: MainAxisSize.min,
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Text(
                                    it.name,
                                    maxLines: 1,
                                    overflow: TextOverflow.ellipsis,
                                    style: typography.body.copyWith(
                                      color: colors.textPrimary,
                                      fontSize: 13,
                                      fontWeight: FontWeight.w600,
                                    ),
                                  ),
                                  SizedBox(height: spacing.xs),
                                  Text(
                                    '${it.tokensUsed} tok · ${it.conversationsCount} conv',
                                    maxLines: 1,
                                    overflow: TextOverflow.ellipsis,
                                    style: typography.body.copyWith(
                                      color: colors.textSecondary,
                                      fontSize: 11,
                                    ),
                                  ),
                                ],
                              ),
                            ),
                          ],
                        ),
                      ),
                    );
                  },
                ),
              ),
            ],
          ),
        );
      },
      orElse: () => const SizedBox.shrink(),
    );
  }
}

class _RankChip extends StatelessWidget {
  final int rank;
  final AppColorsExtension colors;

  const _RankChip({required this.rank, required this.colors});

  @override
  Widget build(BuildContext context) {
    // Podium tint for the top three, muted for the rest.
    final accent = switch (rank) {
      1 => colors.orangeAlert,
      2 => colors.cyanGlow,
      3 => colors.bluePrimary,
      _ => colors.textMuted,
    };
    return Container(
      width: 26,
      height: 26,
      alignment: Alignment.center,
      decoration: BoxDecoration(
        color: accent.withValues(alpha: 0.15),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Text(
        '$rank',
        style: TextStyle(
          color: accent,
          fontSize: 12,
          fontWeight: FontWeight.w700,
        ),
      ),
    );
  }
}

class _FilterChips extends StatelessWidget {
  final List<(String, String)> statuses;
  final String current;
  final String Function(String) labelFor;
  final AppColorsExtension colors;
  final ValueChanged<String> onSelected;

  const _FilterChips({
    required this.statuses,
    required this.current,
    required this.labelFor,
    required this.colors,
    required this.onSelected,
  });

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: 36,
      child: ListView(
        scrollDirection: Axis.horizontal,
        children: [
          for (final s in statuses)
            Padding(
              padding: const EdgeInsets.only(right: 6),
              child: ChoiceChip(
                label: Text(labelFor(s.$1)),
                selected: current == s.$1,
                onSelected: (_) => onSelected(s.$1),
                backgroundColor: colors.glassMedium.withValues(alpha: 0.5),
                selectedColor: colors.bluePrimary.withValues(alpha: 0.18),
                labelStyle: TextStyle(
                  color: current == s.$1
                      ? colors.bluePrimary
                      : colors.textSecondary,
                  fontWeight: FontWeight.w600,
                  fontSize: 12,
                ),
                side: BorderSide(
                  color: current == s.$1
                      ? colors.bluePrimary.withValues(alpha: 0.5)
                      : colors.borderLight,
                ),
                showCheckmark: false,
              ),
            ),
        ],
      ),
    );
  }
}

class _AccountTile extends StatelessWidget {
  final OwnerAccountListing item;
  final AppColorsExtension colors;
  final AppTypographyExtension typography;
  final AppSpacingExtension spacing;
  final Future<void> Function(OwnerAccountSummary, _QuickAction) onAction;

  const _AccountTile({
    required this.item,
    required this.colors,
    required this.typography,
    required this.spacing,
    required this.onAction,
  });

  Color get _statusColor {
    final acc = item.account;
    if (acc.isSuspended) return colors.redAlert;
    switch (acc.subscriptionStatus) {
      case 'active':
        return colors.cyanGlow;
      case 'past_due':
        return colors.redAlert;
      default:
        return colors.orangeAlert;
    }
  }

  Color _capColor() {
    final p = item.capPercent;
    if (p >= 95) return colors.redAlert;
    if (p >= 80) return colors.orangeAlert;
    return colors.cyanGlow;
  }

  @override
  Widget build(BuildContext context) {
    final acc = item.account;
    final statusLabel =
        acc.isSuspended ? t.owner.badge_suspended : acc.subscriptionStatus;
    final initial = acc.name.isNotEmpty ? acc.name[0].toUpperCase() : '?';
    final hasCap = item.planMaxTokens > 0;

    return GlassCard(
      padding: EdgeInsets.zero,
      child: InkWell(
        borderRadius: BorderRadius.circular(16),
        onTap: () => context.push('/owner/accounts/${acc.id}'),
        child: Padding(
          padding: EdgeInsets.all(spacing.md),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Container(
                    width: 40,
                    height: 40,
                    alignment: Alignment.center,
                    decoration: BoxDecoration(
                      color: _statusColor.withValues(alpha: 0.15),
                      borderRadius: BorderRadius.circular(12),
                    ),
                    child: Text(
                      initial,
                      style: TextStyle(
                        color: _statusColor,
                        fontWeight: FontWeight.w700,
                        fontSize: 18,
                      ),
                    ),
                  ),
                  SizedBox(width: spacing.md),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          acc.name,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: typography.body.copyWith(
                            color: colors.textPrimary,
                            fontSize: 15,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                        if (item.ownerEmail.isNotEmpty) ...[
                          SizedBox(height: spacing.xs / 2),
                          Text(
                            item.ownerEmail,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: typography.body.copyWith(
                              color: colors.textSecondary,
                              fontSize: 12,
                            ),
                          ),
                        ],
                      ],
                    ),
                  ),
                  _actionsMenu(context, acc),
                ],
              ),
              SizedBox(height: spacing.sm),
              Wrap(
                spacing: spacing.xs + 2,
                runSpacing: spacing.xs,
                children: [
                  if (item.planName.isNotEmpty)
                    StatusBadge(
                        label: item.planName, color: colors.bluePrimary),
                  StatusBadge(label: statusLabel, color: _statusColor),
                  if (item.whatsappQuality == 'RED')
                    StatusBadge(
                        label: t.owner.wa_risk_ban, color: colors.redAlert)
                  else if (item.whatsappQuality == 'YELLOW')
                    StatusBadge(
                        label: t.owner.wa_quality_warning,
                        color: colors.orangeAlert),
                ],
              ),
              SizedBox(height: spacing.sm),
              _usageBar(hasCap),
            ],
          ),
        ),
      ),
    );
  }

  Widget _usageBar(bool hasCap) {
    if (!hasCap) {
      return Row(
        children: [
          Icon(Icons.all_inclusive, size: 13, color: colors.textMuted),
          SizedBox(width: spacing.xs),
          Expanded(
            child: Text(
              '${t.owner.usage_no_cap} · ${item.tokensUsed} tok · ${item.conversationsCount} conv',
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: typography.body
                  .copyWith(color: colors.textMuted, fontSize: 11),
            ),
          ),
        ],
      );
    }
    final capColor = _capColor();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        ClipRRect(
          borderRadius: BorderRadius.circular(6),
          child: LinearProgressIndicator(
            value: item.capPercent / 100,
            minHeight: 6,
            backgroundColor: colors.borderLight,
            valueColor: AlwaysStoppedAnimation(capColor),
          ),
        ),
        SizedBox(height: spacing.xs),
        Text(
          '${item.capPercent}% · ${item.tokensUsed}/${item.planMaxTokens} tok · ${item.conversationsCount} conv',
          style: typography.body.copyWith(
            color: item.capPercent >= 80 ? capColor : colors.textMuted,
            fontSize: 11,
            fontWeight:
                item.capPercent >= 80 ? FontWeight.w600 : FontWeight.w400,
          ),
        ),
      ],
    );
  }

  Widget _actionsMenu(BuildContext context, OwnerAccountSummary acc) {
    return PopupMenuButton<_QuickAction>(
      tooltip: t.owner.actions_menu,
      icon: Icon(Icons.more_vert, color: colors.textMuted, size: 20),
      color: colors.backgroundSecondary,
      onSelected: (a) => onAction(acc, a),
      itemBuilder: (_) => [
        PopupMenuItem(
          value: _QuickAction.impersonate,
          child: _menuRow(Icons.login, t.owner.impersonate_cta, colors),
        ),
        if (acc.isSuspended)
          PopupMenuItem(
            value: _QuickAction.reactivate,
            child: _menuRow(Icons.play_arrow, t.owner.reactivate_cta, colors),
          )
        else
          PopupMenuItem(
            value: _QuickAction.suspend,
            child: _menuRow(Icons.block, t.owner.suspend_cta, colors,
                danger: true),
          ),
      ],
    );
  }

  Widget _menuRow(IconData icon, String label, AppColorsExtension colors,
      {bool danger = false}) {
    final c = danger ? colors.redAlert : colors.textPrimary;
    return Row(
      children: [
        Icon(icon, size: 18, color: c),
        const SizedBox(width: 12),
        Flexible(
          child: Text(
            label,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(color: c),
          ),
        ),
      ],
    );
  }
}

class _OwnerAccessGate extends ConsumerWidget {
  final OwnerAccessStatus access;

  const _OwnerAccessGate({required this.access});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final isEnrollment = access == OwnerAccessStatus.requiresEnrollment;
    final title =
        isEnrollment ? t.owner.gate_enroll_title : t.owner.gate_refresh_title;
    final body =
        isEnrollment ? t.owner.gate_enroll_body : t.owner.gate_refresh_body;
    final ctaLabel =
        isEnrollment ? t.owner.gate_enroll_cta : t.owner.gate_refresh_cta;
    return Center(
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 480),
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.shield_outlined, size: 48, color: colors.bluePrimary),
              const SizedBox(height: 16),
              Text(
                title,
                style: typography.display.copyWith(
                  color: colors.textPrimary,
                  fontSize: 20,
                  fontWeight: FontWeight.w700,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 12),
              Text(
                body,
                style: typography.body.copyWith(
                  color: colors.textSecondary,
                  fontSize: 14,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 24),
              ElevatedButton(
                key: const Key('owner_gate_cta'),
                onPressed: () async {
                  if (isEnrollment) {
                    // Nothing enrolled yet — that genuinely lives in Profile.
                    context.go('/settings/profile');
                    return;
                  }
                  // A lapsed proof is re-proved right here: the dialog stamps a
                  // fresh one and invalidates the probe, so the panel renders
                  // its content instead of sending the operator to the login
                  // screen and back.
                  await showOwner2FAStepUp(context, ref);
                },
                style: ElevatedButton.styleFrom(
                  backgroundColor: colors.bluePrimary,
                  foregroundColor: Colors.white,
                  padding:
                      const EdgeInsets.symmetric(horizontal: 24, vertical: 12),
                ),
                child: Text(ctaLabel),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
