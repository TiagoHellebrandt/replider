import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/utils/number_format.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/billing/domain/models/plan_model.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/components/app_dialogs.dart';
import 'package:app/design_system/components/glass_card.dart';
import 'package:app/design_system/components/primary_gradient_button.dart';
import 'package:app/design_system/components/retryable_error.dart';
import 'package:app/design_system/components/status_badge.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/owner/domain/models/owner_account_models.dart';
import 'package:app/features/owner/presentation/controllers/owner_controller.dart';
import 'package:app/features/owner/presentation/widgets/owner_2fa_step_up.dart';
import 'package:app/i18n/strings.g.dart';

/// Owner-panel account detail. A hero header (avatar + status badge +
/// actions menu) sits above a 4-tab body: Overview / Usage / Invoices /
/// Conversations. Each tab lazily watches its own provider so a Stripe
/// or analytics blip degrades one tab, not the screen.
class OwnerAccountDetailScreen extends ConsumerStatefulWidget {
  final String accountId;

  const OwnerAccountDetailScreen({super.key, required this.accountId});

  @override
  ConsumerState<OwnerAccountDetailScreen> createState() =>
      _OwnerAccountDetailScreenState();
}

class _OwnerAccountDetailScreenState
    extends ConsumerState<OwnerAccountDetailScreen>
    with SingleTickerProviderStateMixin {
  late final TabController _tab;

  String get accountId => widget.accountId;

  @override
  void initState() {
    super.initState();
    _tab = TabController(length: 4, vsync: this);
  }

  @override
  void dispose() {
    _tab.dispose();
    super.dispose();
  }

  Future<void> _suspend() async {
    final reason = await showInputDialog(
      context,
      title: t.owner.suspend_dialog_title,
      hint: t.owner.suspend_reason_hint,
      confirmText: t.owner.suspend_confirm_cta,
    );
    if (reason == null || reason.isEmpty) return;
    await _run(() =>
        ref.read(ownerActionsProvider.notifier).suspend(accountId, reason));
  }

  Future<void> _reactivate() async {
    final ok = await showConfirmDialog(
      context,
      title: t.owner.reactivate_confirm_title,
      message: t.owner.reactivate_confirm_msg,
      confirmText: t.owner.reactivate_cta,
    );
    if (!ok) return;
    await _run(
        () => ref.read(ownerActionsProvider.notifier).reactivate(accountId));
  }

  Future<void> _grantCredits() async {
    final raw = await showInputDialog(
      context,
      title: t.owner.grant_dialog_title,
      hint: t.owner.grant_hint,
      initialValue: '10000',
      keyboardType: TextInputType.number,
    );
    if (raw == null) return;
    final tokens = int.tryParse(raw) ?? 0;
    if (tokens <= 0) return;
    await _run(() => ref
        .read(ownerActionsProvider.notifier)
        .grantCredits(accountId, tokens));
  }

  Future<void> _extendTrial() async {
    final raw = await showInputDialog(
      context,
      title: t.owner.extend_dialog_title,
      hint: t.owner.extend_hint,
      initialValue: '14',
      keyboardType: TextInputType.number,
    );
    if (raw == null) return;
    final days = int.tryParse(raw) ?? 0;
    if (days <= 0) return;
    await _run(() =>
        ref.read(ownerActionsProvider.notifier).extendTrial(accountId, days));
  }

  Future<void> _impersonate() async {
    final ok = await showConfirmDialog(
      context,
      title: t.owner.impersonate_confirm_title,
      message: t.owner.impersonate_confirm_msg,
      confirmText: t.owner.impersonate_cta,
    );
    if (!ok) return;
    try {
      final out =
          await ref.read(ownerActionsProvider.notifier).impersonate(accountId);
      // Pull the operator's email from the cached auth user so the
      // banner can attribute who is acting (audit/debug).
      final actorEmail = ref
          .read(authControllerProvider)
          .maybeWhen(data: (u) => u?.email ?? '', orElse: () => '');
      await ref
          .read(impersonationControllerProvider.notifier)
          .start(out, actorEmail);
      // Refresh /me on the impersonation token so the whole app (settings,
      // plan, billing) reflects the impersonated account, not the operator.
      await ref.read(authControllerProvider.notifier).refreshMe();
      if (mounted) context.go('/home');
    } catch (e) {
      _toast(parseApiError(e));
    }
  }

  Future<void> _applyContract(OwnerAccountSummary acc) async {
    final result = await showDialog<_ApplyContractResult>(
      context: context,
      builder: (_) => _ApplyContractDialog(initial: acc),
    );
    if (result == null) return;
    await _run(() => ref.read(ownerActionsProvider.notifier).applyContract(
          accountId,
          planId: result.planId,
          trialBypass: result.trialBypass,
          platformKeyAllowed: result.platformKeyAllowed,
          overrides: result.overrides,
        ));
  }

  Future<void> _editSeller(OwnerAccountDetail d) async {
    final result = await showDialog<_SellerResult>(
      context: context,
      builder: (_) => _SellerDialog(
        currentEmail: d.seller?.email ?? '',
        currentLeadSource: d.account.leadSource,
      ),
    );
    if (result == null) return;
    await _run(() => ref.read(ownerActionsProvider.notifier).setSeller(
          accountId,
          sellerEmail: result.unlink ? '' : result.email,
          leadSource: result.unlink ? '' : result.leadSource,
        ));
  }

  Future<void> _editOverrides(OwnerAccountSummary acc) async {
    final result = await showDialog<Map<String, int?>>(
      context: context,
      builder: (_) => _OverridesDialog(initial: acc),
    );
    if (result == null) return;
    await _run(() => ref
        .read(ownerActionsProvider.notifier)
        .setOverrides(accountId, result));
  }

  /// Run a mutating action, surfacing success/error as a snackbar.
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
    final async = ref.watch(ownerAccountDetailProvider(accountId));

    return Scaffold(
      backgroundColor: colors.backgroundPrimary,
      appBar: AppBar(
        backgroundColor: colors.backgroundSecondary,
        elevation: 0,
        title: Text(
          t.owner.detail_title,
          style: TextStyle(
            fontSize: 20,
            fontWeight: FontWeight.w700,
            color: colors.textPrimary,
          ),
        ),
        leading: IconButton(
          icon: Icon(Icons.arrow_back, color: colors.textSecondary),
          onPressed: () =>
              context.canPop() ? context.pop() : context.go('/owner/accounts'),
        ),
      ),
      body: async.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (e, _) => RetryableError(
          error: e,
          onRetry: () => ref.invalidate(ownerAccountDetailProvider(accountId)),
        ),
        data: (d) => Column(
          children: [
            _HeroHeader(
              detail: d,
              colors: colors,
              typography: typography,
              onAction: (a) => _dispatch(a, d.account),
            ),
            TabBar(
              controller: _tab,
              isScrollable: true,
              tabAlignment: TabAlignment.start,
              labelColor: colors.textPrimary,
              unselectedLabelColor: colors.textSecondary,
              indicatorColor: colors.bluePrimary,
              tabs: [
                Tab(text: t.owner.tab_overview),
                Tab(text: t.owner.tab_usage),
                Tab(text: t.owner.tab_invoices),
                Tab(text: t.owner.tab_conversations),
              ],
            ),
            Expanded(
              child: TabBarView(
                controller: _tab,
                children: [
                  _overviewTab(d, colors, typography),
                  _usageTab(d, colors, typography),
                  _invoicesTab(colors, typography),
                  _conversationsTab(colors, typography),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  void _dispatch(_OwnerAction a, OwnerAccountSummary acc) {
    switch (a) {
      case _OwnerAction.grant:
        _grantCredits();
      case _OwnerAction.extend:
        _extendTrial();
      case _OwnerAction.impersonate:
        _impersonate();
      case _OwnerAction.applyContract:
        _applyContract(acc);
      case _OwnerAction.editOverrides:
        _editOverrides(acc);
      case _OwnerAction.suspend:
        _suspend();
      case _OwnerAction.reactivate:
        _reactivate();
    }
  }

  // ---- Tabs -------------------------------------------------------------

  Widget _overviewTab(
    OwnerAccountDetail d,
    AppColorsExtension colors,
    AppTypographyExtension typography,
  ) {
    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        _Section(
          title: t.owner.detail_title,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _kv(t.owner.field_id, d.account.id, typography, colors,
                  mono: true),
              _kv(t.owner.field_status, d.account.subscriptionStatus,
                  typography, colors),
              if (d.account.suspendedAt != null)
                _kv(
                  t.owner.field_suspended_reason,
                  d.account.suspendedReason ?? '—',
                  typography,
                  colors,
                  highlight: colors.redAlert,
                ),
              _kv(t.owner.field_owner, d.admin?.email ?? '—', typography,
                  colors),
              _kv(t.owner.field_plan, d.plan?.name ?? '—', typography, colors),
              _kv(t.owner.field_created, _fmtDate(d.account.createdAt),
                  typography, colors),
              _kv(t.owner.field_internal_threads, '${d.internalThreadsActive}',
                  typography, colors),
            ],
          ),
        ),
        const SizedBox(height: 16),
        _Section(
          title: t.owner.seller_label,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (d.seller == null)
                Padding(
                  padding: const EdgeInsets.only(bottom: 8),
                  child: Text(t.owner.seller_none,
                      style: typography.body
                          .copyWith(color: colors.textSecondary)),
                )
              else ...[
                _kv(
                    t.owner.seller_label,
                    '${d.seller!.name} (${d.seller!.email})',
                    typography,
                    colors),
                _kv(
                    t.owner.seller_source_label,
                    d.account.leadSource == 'seller_ref'
                        ? t.owner.seller_source_seller_ref
                        : d.account.leadSource == 'inbound'
                            ? t.owner.seller_source_inbound
                            : '—',
                    typography,
                    colors),
                if (d.account.sellerLinkedAt != null)
                  _kv(t.owner.seller_linked_at,
                      _fmtDate(d.account.sellerLinkedAt!), typography, colors),
                if (d.sellerSelfMatch)
                  Padding(
                    padding: const EdgeInsets.only(top: 8),
                    child: Row(
                      children: [
                        Icon(Icons.warning_amber_rounded,
                            size: 16, color: colors.orangeAlert),
                        const SizedBox(width: 6),
                        Expanded(
                          child: Text(
                            t.owner.seller_self_match_warning,
                            style: typography.body
                                .copyWith(color: colors.orangeAlert),
                          ),
                        ),
                      ],
                    ),
                  ),
              ],
              const SizedBox(height: 8),
              OutlinedButton.icon(
                icon: const Icon(Icons.person_add_alt, size: 16),
                label: Text(t.owner.seller_edit_cta),
                onPressed: () => _editSeller(d),
              ),
            ],
          ),
        ),
        const SizedBox(height: 16),
        _Section(
          title: t.owner.overrides_label,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _kv(
                  t.owner.cap_max_agents,
                  _ovOrPlan(d.account.maxAgentsOverride, d.plan?.maxAgents),
                  typography,
                  colors),
              _kv(
                  t.owner.cap_max_channels,
                  _ovOrPlan(d.account.maxChannelsOverride, d.plan?.maxChannels),
                  typography,
                  colors),
              _kv(
                  t.owner.cap_max_conversations,
                  _ovOrPlan(d.account.maxConversationsPerMonthOverride,
                      d.plan?.maxConversationsPerMonth),
                  typography,
                  colors),
              _kv(
                  t.owner.cap_max_tokens,
                  _ovOrPlan(d.account.maxAiTokensPerMonthOverride,
                      d.plan?.maxAiTokensPerMonth),
                  typography,
                  colors),
              const SizedBox(height: 8),
              OutlinedButton.icon(
                icon: const Icon(Icons.edit, size: 16),
                label: Text(t.owner.edit_overrides_cta),
                onPressed: () => _editOverrides(d.account),
              ),
            ],
          ),
        ),
        const SizedBox(height: 16),
        if (d.account.isSuspended)
          PrimaryGradientButton(
            text: t.owner.reactivate_cta,
            onPressed: _reactivate,
          )
        else
          OutlinedButton.icon(
            icon: Icon(Icons.block, color: colors.redAlert),
            label: Text(t.owner.suspend_cta,
                style: TextStyle(color: colors.redAlert)),
            onPressed: _suspend,
          ),
      ],
    );
  }

  Widget _usageTab(
    OwnerAccountDetail d,
    AppColorsExtension colors,
    AppTypographyExtension typography,
  ) {
    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        if (d.usage != null)
          _Section(
            title: t.owner.usage_label,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                _kv(
                    t.owner.usage_tokens,
                    formatCompactNumber(d.usage!.aiTokensUsed ?? 0),
                    typography,
                    colors),
                _kv(
                    t.owner.usage_addon,
                    formatCompactNumber(d.usage!.addonTokens ?? 0),
                    typography,
                    colors),
                _kv(
                    t.owner.usage_conversations,
                    formatCompactNumber(d.usage!.conversationsCount ?? 0),
                    typography,
                    colors),
                _kv(
                    t.owner.usage_messages,
                    formatCompactNumber(d.usage!.messagesCount ?? 0),
                    typography,
                    colors),
              ],
            ),
          ),
        if (d.usage != null) const SizedBox(height: 16),
        _UsageTrendChart(
            accountId: accountId, typography: typography, colors: colors),
      ],
    );
  }

  Widget _invoicesTab(
    AppColorsExtension colors,
    AppTypographyExtension typography,
  ) {
    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        _InvoicesSection(
            accountId: accountId, typography: typography, colors: colors),
      ],
    );
  }

  Widget _conversationsTab(
    AppColorsExtension colors,
    AppTypographyExtension typography,
  ) {
    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        _ConversationsSection(
          accountId: accountId,
          typography: typography,
          colors: colors,
          onTapRow: _impersonate,
        ),
      ],
    );
  }

  String _ovOrPlan(int? override, int? planVal) {
    if (override != null) return '$override (override)';
    return planVal == null ? '—' : '$planVal';
  }

  Widget _kv(
    String k,
    String v,
    AppTypographyExtension typography,
    AppColorsExtension colors, {
    Color? highlight,
    bool mono = false,
  }) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 130,
            child: Text(
              k,
              style: typography.body.copyWith(
                color: colors.textSecondary,
                fontSize: 12,
              ),
            ),
          ),
          Expanded(
            child: SelectableText(
              v,
              style: typography.body.copyWith(
                color: highlight ?? colors.textPrimary,
                fontSize: 13,
                fontWeight: FontWeight.w500,
                fontFamily: mono ? 'monospace' : null,
              ),
            ),
          ),
        ],
      ),
    );
  }

  String _fmtDate(DateTime d) =>
      '${d.year}-${d.month.toString().padLeft(2, '0')}-${d.day.toString().padLeft(2, '0')}';
}

/// Actions surfaced in the hero overflow menu. Suspend/reactivate are
/// mutually exclusive on the account's current state.
enum _OwnerAction {
  grant,
  extend,
  impersonate,
  applyContract,
  editOverrides,
  suspend,
  reactivate
}

class _HeroHeader extends StatelessWidget {
  final OwnerAccountDetail detail;
  final AppColorsExtension colors;
  final AppTypographyExtension typography;
  final ValueChanged<_OwnerAction> onAction;

  const _HeroHeader({
    required this.detail,
    required this.colors,
    required this.typography,
    required this.onAction,
  });

  ({Color color, String label}) _statusBadge(OwnerAccountSummary acc) {
    if (acc.isSuspended) {
      return (color: colors.redAlert, label: t.owner.badge_suspended);
    }
    final s = acc.subscriptionStatus;
    final color = switch (s) {
      'active' => colors.cyanGlow,
      'trialing' => colors.orangeAlert,
      'past_due' => colors.redAlert,
      _ => colors.textMuted,
    };
    return (color: color, label: s.toUpperCase());
  }

  @override
  Widget build(BuildContext context) {
    final acc = detail.account;
    final badge = _statusBadge(acc);
    final initial =
        acc.name.trim().isEmpty ? '?' : acc.name.trim()[0].toUpperCase();
    final email = detail.admin?.email ?? '';
    final planName = detail.plan?.name ?? '';

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.fromLTRB(16, 12, 8, 16),
      decoration: BoxDecoration(
        color: colors.glassMedium.withValues(alpha: 0.4),
        border: Border(bottom: BorderSide(color: colors.borderMedium)),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 52,
            height: 52,
            decoration: BoxDecoration(
              color: colors.bluePrimary.withValues(alpha: 0.18),
              shape: BoxShape.circle,
            ),
            alignment: Alignment.center,
            child: Text(
              initial,
              style: typography.display.copyWith(
                color: colors.bluePrimary,
                fontSize: 22,
                fontWeight: FontWeight.w700,
              ),
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Flexible(
                      child: Text(
                        acc.name,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: typography.display.copyWith(
                          color: colors.textPrimary,
                          fontSize: 18,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                    ),
                    const SizedBox(width: 8),
                    StatusBadge(label: badge.label, color: badge.color),
                  ],
                ),
                if (email.isNotEmpty) ...[
                  const SizedBox(height: 4),
                  Text(
                    email,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: typography.body.copyWith(
                      color: colors.textSecondary,
                      fontSize: 12,
                    ),
                  ),
                ],
                if (planName.isNotEmpty) ...[
                  const SizedBox(height: 2),
                  Text(
                    planName,
                    style: typography.body.copyWith(
                      color: colors.textMuted,
                      fontSize: 11,
                    ),
                  ),
                ],
              ],
            ),
          ),
          PopupMenuButton<_OwnerAction>(
            icon: Icon(Icons.more_vert, color: colors.textSecondary),
            tooltip: t.owner.actions_menu,
            onSelected: onAction,
            itemBuilder: (_) => [
              PopupMenuItem(
                value: _OwnerAction.grant,
                child: _menuRow(Icons.add_circle_outline, t.owner.grant_cta),
              ),
              PopupMenuItem(
                value: _OwnerAction.extend,
                child: _menuRow(Icons.schedule, t.owner.extend_cta),
              ),
              PopupMenuItem(
                value: _OwnerAction.impersonate,
                child: _menuRow(Icons.login, t.owner.impersonate_cta),
              ),
              PopupMenuItem(
                value: _OwnerAction.applyContract,
                child: _menuRow(
                    Icons.workspace_premium_outlined, t.owner.apply_plan_cta),
              ),
              PopupMenuItem(
                value: _OwnerAction.editOverrides,
                child: _menuRow(Icons.tune, t.owner.edit_overrides_cta),
              ),
              const PopupMenuDivider(),
              if (acc.isSuspended)
                PopupMenuItem(
                  value: _OwnerAction.reactivate,
                  child: _menuRow(Icons.play_circle_outline,
                      t.owner.reactivate_cta, colors.cyanGlow),
                )
              else
                PopupMenuItem(
                  value: _OwnerAction.suspend,
                  child: _menuRow(
                      Icons.block, t.owner.suspend_cta, colors.redAlert),
                ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _menuRow(IconData icon, String label, [Color? color]) {
    return Row(
      children: [
        Icon(icon, size: 18, color: color),
        const SizedBox(width: 10),
        Flexible(
          child: Text(
            label,
            overflow: TextOverflow.ellipsis,
            style: color == null ? null : TextStyle(color: color),
          ),
        ),
      ],
    );
  }
}

class _Section extends StatelessWidget {
  final String title;
  final Widget child;
  const _Section({required this.title, required this.child});

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    return GlassCard(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              title,
              style: typography.body.copyWith(
                color: colors.textPrimary,
                fontSize: 15,
                fontWeight: FontWeight.w700,
              ),
            ),
            const SizedBox(height: 12),
            child,
          ],
        ),
      ),
    );
  }
}

class _OverridesDialog extends StatefulWidget {
  final OwnerAccountSummary initial;

  const _OverridesDialog({required this.initial});

  @override
  State<_OverridesDialog> createState() => _OverridesDialogState();
}

class _OverridesDialogState extends State<_OverridesDialog> {
  late final TextEditingController _agents;
  late final TextEditingController _channels;
  late final TextEditingController _convs;
  late final TextEditingController _tokens;

  @override
  void initState() {
    super.initState();
    _agents = TextEditingController(
        text: widget.initial.maxAgentsOverride?.toString() ?? '');
    _channels = TextEditingController(
        text: widget.initial.maxChannelsOverride?.toString() ?? '');
    _convs = TextEditingController(
        text:
            widget.initial.maxConversationsPerMonthOverride?.toString() ?? '');
    _tokens = TextEditingController(
        text: widget.initial.maxAiTokensPerMonthOverride?.toString() ?? '');
  }

  @override
  void dispose() {
    _agents.dispose();
    _channels.dispose();
    _convs.dispose();
    _tokens.dispose();
    super.dispose();
  }

  int? _parse(String v) {
    final s = v.trim();
    if (s.isEmpty) return null;
    return int.tryParse(s);
  }

  Widget _field(
    TextEditingController controller,
    String label,
    AppColorsExtension colors,
  ) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: TextField(
        controller: controller,
        keyboardType: TextInputType.number,
        style: TextStyle(color: colors.textPrimary),
        decoration: InputDecoration(
          labelText: label,
          labelStyle: TextStyle(color: colors.textMuted),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return AlertDialog(
      backgroundColor: colors.backgroundSecondary,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      title: Text(t.owner.edit_overrides_cta,
          style: TextStyle(color: colors.textPrimary, fontSize: 16)),
      content: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            _field(_agents, t.owner.cap_max_agents, colors),
            _field(_channels, t.owner.cap_max_channels, colors),
            _field(_convs, t.owner.cap_max_conversations, colors),
            _field(_tokens, t.owner.cap_max_tokens, colors),
            Text(
              t.owner.overrides_clear_hint,
              style: TextStyle(fontSize: 11, color: colors.textMuted),
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context),
          child:
              Text(t.common.cancel, style: TextStyle(color: colors.textMuted)),
        ),
        ElevatedButton(
          style: ElevatedButton.styleFrom(
            backgroundColor: colors.bluePrimary,
            foregroundColor: Colors.white,
          ),
          onPressed: () => Navigator.pop(context, {
            'max_agents': _parse(_agents.text),
            'max_channels': _parse(_channels.text),
            'max_conversations_per_month': _parse(_convs.text),
            'max_ai_tokens_per_month': _parse(_tokens.text),
          }),
          child: Text(t.common.save),
        ),
      ],
    );
  }
}

/// Result of the seller-link dialog. [unlink] wins over the other
/// fields — the caller sends an empty seller_email to the API.
class _SellerResult {
  final String email;
  final String leadSource;
  final bool unlink;

  _SellerResult({this.email = '', this.leadSource = '', this.unlink = false});
}

/// Links / reallocates / unlinks the seller of a customer account
/// (vendedores v3, US-002): seller email + lead-source track picker.
class _SellerDialog extends StatefulWidget {
  final String currentEmail;
  final String? currentLeadSource;

  const _SellerDialog({required this.currentEmail, this.currentLeadSource});

  @override
  State<_SellerDialog> createState() => _SellerDialogState();
}

class _SellerDialogState extends State<_SellerDialog> {
  late final TextEditingController _email;
  late String _leadSource;

  @override
  void initState() {
    super.initState();
    _email = TextEditingController(text: widget.currentEmail);
    _leadSource = widget.currentLeadSource ?? 'inbound';
  }

  @override
  void dispose() {
    _email.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return AlertDialog(
      backgroundColor: colors.backgroundSecondary,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      title: Text(t.owner.seller_dialog_title,
          style: TextStyle(color: colors.textPrimary, fontSize: 16)),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          TextField(
            controller: _email,
            keyboardType: TextInputType.emailAddress,
            style: TextStyle(color: colors.textPrimary),
            decoration: InputDecoration(
              labelText: t.owner.seller_email_hint,
              labelStyle: TextStyle(color: colors.textMuted),
            ),
          ),
          const SizedBox(height: 12),
          DropdownButtonFormField<String>(
            initialValue: _leadSource,
            dropdownColor: colors.backgroundSecondary,
            style: TextStyle(color: colors.textPrimary, fontSize: 14),
            decoration: InputDecoration(
              labelText: t.owner.seller_source_label,
              labelStyle: TextStyle(color: colors.textMuted),
            ),
            items: [
              DropdownMenuItem(
                value: 'seller_ref',
                child: Text(t.owner.seller_source_seller_ref),
              ),
              DropdownMenuItem(
                value: 'inbound',
                child: Text(t.owner.seller_source_inbound),
              ),
            ],
            onChanged: (v) => setState(() => _leadSource = v ?? 'inbound'),
          ),
        ],
      ),
      actions: [
        if (widget.currentEmail.isNotEmpty)
          TextButton(
            onPressed: () =>
                Navigator.pop(context, _SellerResult(unlink: true)),
            child: Text(t.owner.seller_unlink_cta,
                style: TextStyle(color: colors.redAlert)),
          ),
        TextButton(
          onPressed: () => Navigator.pop(context),
          child:
              Text(t.common.cancel, style: TextStyle(color: colors.textMuted)),
        ),
        ElevatedButton(
          style: ElevatedButton.styleFrom(
            backgroundColor: colors.bluePrimary,
            foregroundColor: Colors.white,
          ),
          onPressed: () {
            final email = _email.text.trim();
            if (email.isEmpty) return;
            Navigator.pop(
                context, _SellerResult(email: email, leadSource: _leadSource));
          },
          child: Text(t.owner.seller_save_cta),
        ),
      ],
    );
  }
}

/// Result of the apply-contract dialog: the chosen contract plan + the
/// per-account limits + the trial-bypass / our-key toggles.
class _ApplyContractResult {
  final String planId;
  final bool trialBypass;
  final bool platformKeyAllowed;
  final Map<String, int?> overrides;

  _ApplyContractResult({
    required this.planId,
    required this.trialBypass,
    required this.platformKeyAllowed,
    required this.overrides,
  });
}

/// Puts an existing account onto a contract tier (Enterprise / Partner):
/// pick the plan, set per-account limits, optionally bypass trial and let
/// it run on OUR AI key (billed at the plan's higher multipliers) instead
/// of forcing BYOK.
class _ApplyContractDialog extends ConsumerStatefulWidget {
  final OwnerAccountSummary initial;

  const _ApplyContractDialog({required this.initial});

  @override
  ConsumerState<_ApplyContractDialog> createState() =>
      _ApplyContractDialogState();
}

class _ApplyContractDialogState extends ConsumerState<_ApplyContractDialog> {
  late final TextEditingController _agents;
  late final TextEditingController _channels;
  late final TextEditingController _convs;
  late final TextEditingController _tokens;
  List<PlanModel>? _plans;
  String? _planId;
  bool _trialBypass = false;
  bool _platformKey = false;

  @override
  void initState() {
    super.initState();
    final i = widget.initial;
    _agents =
        TextEditingController(text: i.maxAgentsOverride?.toString() ?? '');
    _channels =
        TextEditingController(text: i.maxChannelsOverride?.toString() ?? '');
    _convs = TextEditingController(
        text: i.maxConversationsPerMonthOverride?.toString() ?? '');
    _tokens = TextEditingController(
        text: i.maxAiTokensPerMonthOverride?.toString() ?? '');
    _planId = i.planId;
    _trialBypass = i.trialBypass;
    _loadPlans();
  }

  Future<void> _loadPlans() async {
    try {
      // /owner/plans is the UNFILTERED catalogue. The public /plans hid the
      // SDR contract tiers (Vendedor/Equipe/Escala), so filtering isContractPlan
      // over it left only Enterprise assignable. Only contract tiers are
      // assignable here — self-serve plans go through Stripe checkout.
      final res =
          await ref.read(dioProvider).get<List<dynamic>>('/owner/plans');
      if (!mounted) return;
      final contract = (res.data ?? const <dynamic>[])
          .whereType<Map<String, dynamic>>()
          .map(PlanModel.fromJson)
          .where((p) => p.isContractPlan)
          .toList();
      setState(() => _plans = contract);
    } catch (_) {
      if (mounted) setState(() => _plans = const []);
    }
  }

  @override
  void dispose() {
    _agents.dispose();
    _channels.dispose();
    _convs.dispose();
    _tokens.dispose();
    super.dispose();
  }

  int? _parse(String v) {
    final s = v.trim();
    return s.isEmpty ? null : int.tryParse(s);
  }

  Widget _field(
      TextEditingController c, String label, AppColorsExtension colors) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: TextField(
        controller: c,
        keyboardType: TextInputType.number,
        style: TextStyle(color: colors.textPrimary),
        decoration: InputDecoration(
          labelText: label,
          labelStyle: TextStyle(color: colors.textMuted),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final plans = _plans;
    return AlertDialog(
      backgroundColor: colors.backgroundSecondary,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      title: Text(t.owner.apply_plan_cta,
          style: TextStyle(color: colors.textPrimary, fontSize: 16)),
      content: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (plans == null)
              const Padding(
                padding: EdgeInsets.all(20),
                child: Center(child: CircularProgressIndicator()),
              )
            else ...[
              DropdownButtonFormField<String>(
                initialValue:
                    plans.any((p) => p.id == _planId) ? _planId : null,
                isExpanded: true,
                dropdownColor: colors.backgroundSecondary,
                style: TextStyle(color: colors.textPrimary),
                items: plans
                    .map((p) => DropdownMenuItem(
                        value: p.id,
                        child: Text('${p.name} · '
                            '${p.price <= 0 ? t.billing.plan_contract : p.formattedPrice}')))
                    .toList(),
                onChanged: (v) => setState(() => _planId = v),
                decoration: InputDecoration(
                  labelText: t.owner.create_plan,
                  labelStyle: TextStyle(color: colors.textMuted),
                ),
              ),
              const SizedBox(height: 16),
              _field(_agents, t.owner.cap_max_agents, colors),
              _field(_channels, t.owner.cap_max_channels, colors),
              _field(_convs, t.owner.cap_max_conversations, colors),
              _field(_tokens, t.owner.cap_max_tokens, colors),
              SwitchListTile(
                contentPadding: EdgeInsets.zero,
                value: _trialBypass,
                onChanged: (v) => setState(() => _trialBypass = v),
                title: Text(t.owner.create_trial_bypass,
                    style: TextStyle(color: colors.textPrimary, fontSize: 13)),
              ),
              SwitchListTile(
                contentPadding: EdgeInsets.zero,
                value: _platformKey,
                onChanged: (v) => setState(() => _platformKey = v),
                title: Text(t.owner.use_our_key_title,
                    style: TextStyle(color: colors.textPrimary, fontSize: 13)),
                subtitle: Text(t.owner.use_our_key_hint,
                    style: TextStyle(color: colors.textMuted, fontSize: 11)),
              ),
            ],
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context),
          child:
              Text(t.common.cancel, style: TextStyle(color: colors.textMuted)),
        ),
        ElevatedButton(
          style: ElevatedButton.styleFrom(
            backgroundColor: colors.bluePrimary,
            foregroundColor: Colors.white,
          ),
          onPressed: _planId == null
              ? null
              : () => Navigator.pop(
                    context,
                    _ApplyContractResult(
                      planId: _planId!,
                      trialBypass: _trialBypass,
                      platformKeyAllowed: _platformKey,
                      overrides: {
                        'max_agents': _parse(_agents.text),
                        'max_channels': _parse(_channels.text),
                        'max_conversations_per_month': _parse(_convs.text),
                        'max_ai_tokens_per_month': _parse(_tokens.text),
                      },
                    ),
                  ),
          child: Text(t.common.save),
        ),
      ],
    );
  }
}

/// Read-only triage list of recent conversations. Tapping a row enters
/// the account via impersonation so the operator can read the full
/// thread on the regular dashboard.
class _ConversationsSection extends ConsumerWidget {
  final String accountId;
  final AppTypographyExtension typography;
  final AppColorsExtension colors;
  final VoidCallback onTapRow;

  const _ConversationsSection({
    required this.accountId,
    required this.typography,
    required this.colors,
    required this.onTapRow,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final async = ref.watch(ownerConversationsProvider(accountId));
    return GlassCard(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              t.owner.conversations_label,
              style: typography.body.copyWith(
                color: colors.textPrimary,
                fontSize: 15,
                fontWeight: FontWeight.w700,
              ),
            ),
            const SizedBox(height: 12),
            async.when(
              loading: () => const Padding(
                padding: EdgeInsets.symmetric(vertical: 12),
                child: LinearProgressIndicator(minHeight: 2),
              ),
              error: (e, _) => Text(
                parseApiError(e),
                style: typography.body
                    .copyWith(color: colors.redAlert, fontSize: 12),
              ),
              data: (resp) {
                if (resp.items.isEmpty) {
                  return Text(
                    t.owner.conversations_empty,
                    style: typography.body
                        .copyWith(color: colors.textSecondary, fontSize: 12),
                  );
                }
                return Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      t.owner.conversations_tap_hint,
                      style: typography.body.copyWith(
                        color: colors.textMuted,
                        fontSize: 11,
                      ),
                    ),
                    const SizedBox(height: 6),
                    for (final c in resp.items.take(20))
                      InkWell(
                        borderRadius: BorderRadius.circular(8),
                        onTap: onTapRow,
                        child: Padding(
                          padding: const EdgeInsets.symmetric(vertical: 6),
                          child: Row(
                            children: [
                              Icon(Icons.chat_bubble_outline,
                                  size: 14, color: colors.textSecondary),
                              const SizedBox(width: 8),
                              Expanded(
                                child: Text(
                                  c.contactName,
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: typography.body.copyWith(
                                    color: colors.textPrimary,
                                    fontSize: 12,
                                  ),
                                ),
                              ),
                              Text(
                                '${formatCompactNumber(c.messageCount)} msgs',
                                style: typography.body.copyWith(
                                  color: colors.textSecondary,
                                  fontSize: 11,
                                ),
                              ),
                              const SizedBox(width: 6),
                              _ConvStatusDot(status: c.status, colors: colors),
                            ],
                          ),
                        ),
                      ),
                  ],
                );
              },
            ),
          ],
        ),
      ),
    );
  }
}

class _ConvStatusDot extends StatelessWidget {
  final String status;
  final AppColorsExtension colors;

  const _ConvStatusDot({required this.status, required this.colors});

  @override
  Widget build(BuildContext context) {
    final color = switch (status) {
      'open' => colors.cyanGlow,
      'pending' => colors.orangeAlert,
      'resolved' || 'closed' => colors.bluePrimary,
      _ => colors.textMuted,
    };
    return Container(
      width: 8,
      height: 8,
      decoration: BoxDecoration(color: color, shape: BoxShape.circle),
    );
  }
}

/// Compact 30-day token-usage bar chart. Manual paint via Containers to
/// avoid pulling in fl_chart for one widget. Bar height is proportional
/// to the day's tokens against the period max; 0-token days render a
/// thin baseline so the rhythm of activity is visible.
class _UsageTrendChart extends ConsumerWidget {
  final String accountId;
  final AppTypographyExtension typography;
  final AppColorsExtension colors;

  const _UsageTrendChart({
    required this.accountId,
    required this.typography,
    required this.colors,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final async = ref.watch(ownerUsageTrendProvider(accountId));
    return GlassCard(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              t.owner.usage_trend_label,
              style: typography.body.copyWith(
                color: colors.textPrimary,
                fontSize: 15,
                fontWeight: FontWeight.w700,
              ),
            ),
            const SizedBox(height: 4),
            Text(
              t.owner.usage_trend_desc,
              style: typography.body.copyWith(
                color: colors.textSecondary,
                fontSize: 11,
              ),
            ),
            const SizedBox(height: 12),
            async.when(
              loading: () => const Padding(
                padding: EdgeInsets.symmetric(vertical: 12),
                child: LinearProgressIndicator(minHeight: 2),
              ),
              error: (e, _) => Text(
                parseApiError(e),
                style: typography.body
                    .copyWith(color: colors.redAlert, fontSize: 12),
              ),
              data: (resp) {
                if (resp.items.isEmpty) {
                  return Text(
                    t.owner.usage_trend_empty,
                    style: typography.body.copyWith(
                      color: colors.textSecondary,
                      fontSize: 12,
                    ),
                  );
                }
                final maxTokens = resp.items
                    .map((d) => d.tokens)
                    .fold<int>(0, (a, b) => a > b ? a : b);
                final totalTokens =
                    resp.items.fold<int>(0, (a, d) => a + d.tokens);
                return Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    SizedBox(
                      height: 80,
                      child: Row(
                        crossAxisAlignment: CrossAxisAlignment.end,
                        children: [
                          for (final d in resp.items)
                            Expanded(
                              child: Tooltip(
                                message:
                                    '${d.day.toIso8601String().substring(0, 10)} · ${formatCompactNumber(d.tokens)} tok',
                                child: Padding(
                                  padding:
                                      const EdgeInsets.symmetric(horizontal: 1),
                                  child: Container(
                                    height: maxTokens == 0
                                        ? 2
                                        : (d.tokens / maxTokens) * 78 + 2,
                                    decoration: BoxDecoration(
                                      color: d.tokens == 0
                                          ? colors.borderMedium
                                          : colors.bluePrimary
                                              .withValues(alpha: 0.7),
                                      borderRadius: BorderRadius.circular(2),
                                    ),
                                  ),
                                ),
                              ),
                            ),
                        ],
                      ),
                    ),
                    const SizedBox(height: 8),
                    Text(
                      t.owner.usage_trend_total(
                          tokens: formatCompactNumber(totalTokens)),
                      style: typography.body.copyWith(
                        color: colors.textSecondary,
                        fontSize: 11,
                      ),
                    ),
                  ],
                );
              },
            ),
          ],
        ),
      ),
    );
  }
}

class _InvoicesSection extends ConsumerWidget {
  final String accountId;
  final AppTypographyExtension typography;
  final AppColorsExtension colors;

  const _InvoicesSection({
    required this.accountId,
    required this.typography,
    required this.colors,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final async = ref.watch(ownerInvoicesProvider(accountId));
    return GlassCard(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              t.owner.invoices_label,
              style: typography.body.copyWith(
                color: colors.textPrimary,
                fontSize: 15,
                fontWeight: FontWeight.w700,
              ),
            ),
            const SizedBox(height: 4),
            Text(
              t.owner.invoices_desc,
              style: typography.body.copyWith(
                color: colors.textSecondary,
                fontSize: 11,
              ),
            ),
            const SizedBox(height: 12),
            async.when(
              loading: () => const Padding(
                padding: EdgeInsets.symmetric(vertical: 12),
                child: LinearProgressIndicator(minHeight: 2),
              ),
              error: (e, _) => Text(
                parseApiError(e),
                style: typography.body
                    .copyWith(color: colors.redAlert, fontSize: 12),
              ),
              data: (resp) {
                if (resp.items.isEmpty) {
                  return Text(
                    t.owner.invoices_empty,
                    style: typography.body.copyWith(
                      color: colors.textSecondary,
                      fontSize: 12,
                    ),
                  );
                }
                return Column(
                  children: [
                    for (final inv in resp.items)
                      _InvoiceRow(
                        invoice: inv,
                        typography: typography,
                        colors: colors,
                      ),
                  ],
                );
              },
            ),
          ],
        ),
      ),
    );
  }
}

class _InvoiceRow extends StatelessWidget {
  final OwnerInvoice invoice;
  final AppTypographyExtension typography;
  final AppColorsExtension colors;

  const _InvoiceRow({
    required this.invoice,
    required this.typography,
    required this.colors,
  });

  String _formatAmount() {
    final cents =
        invoice.amountDue > 0 ? invoice.amountDue : invoice.amountPaid;
    final value = cents / 100.0;
    final cur = invoice.currency.toUpperCase();
    return '${value.toStringAsFixed(2)} $cur';
  }

  String _statusLabel() {
    switch (invoice.status) {
      case 'paid':
        return t.owner.invoice_status_paid;
      case 'open':
        return t.owner.invoice_status_open;
      case 'draft':
        return t.owner.invoice_status_draft;
      case 'uncollectible':
        return t.owner.invoice_status_uncollectible;
      case 'void':
        return t.owner.invoice_status_void;
      default:
        return invoice.status;
    }
  }

  Color _statusColor() {
    switch (invoice.status) {
      case 'paid':
        return colors.limeGlow;
      case 'open':
        return colors.bluePrimary;
      case 'uncollectible':
      case 'void':
        return colors.redAlert;
      default:
        return colors.textSecondary;
    }
  }

  Future<void> _open(String url) async {
    final uri = Uri.tryParse(url);
    if (uri != null) {
      await launchUrl(uri, mode: LaunchMode.externalApplication);
    }
  }

  @override
  Widget build(BuildContext context) {
    final dateStr = invoice.createdAt.toIso8601String().substring(0, 10);
    final number = invoice.number.isEmpty ? invoice.id : invoice.number;
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  number,
                  style: typography.body.copyWith(
                    color: colors.textPrimary,
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                  ),
                ),
                Text(
                  dateStr,
                  style: typography.body.copyWith(
                    color: colors.textSecondary,
                    fontSize: 11,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          StatusBadge(label: _statusLabel(), color: _statusColor()),
          const SizedBox(width: 12),
          Text(
            _formatAmount(),
            style: typography.body.copyWith(
              color: colors.textPrimary,
              fontSize: 13,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(width: 8),
          if (invoice.hostedUrl != null && invoice.hostedUrl!.isNotEmpty)
            IconButton(
              icon: const Icon(Icons.open_in_new, size: 16),
              tooltip: t.owner.invoice_open_hosted,
              onPressed: () => _open(invoice.hostedUrl!),
            ),
          if (invoice.invoicePdf != null && invoice.invoicePdf!.isNotEmpty)
            IconButton(
              icon: const Icon(Icons.picture_as_pdf, size: 16),
              tooltip: t.owner.invoice_open_pdf,
              onPressed: () => _open(invoice.invoicePdf!),
            ),
        ],
      ),
    );
  }
}
