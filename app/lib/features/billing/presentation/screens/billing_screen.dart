import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/billing/domain/models/effective_plan_limits_model.dart';
import 'package:app/features/billing/domain/models/usage_model.dart';
import 'package:app/features/billing/presentation/controllers/billing_controller.dart';
import 'package:app/features/billing/presentation/widgets/ai_cost_card.dart';
import 'package:app/features/billing/presentation/widgets/talk_to_us_card.dart';
import 'package:app/features/feedback/presentation/widgets/cancellation_survey_modal.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:app/core/error/api_error_parser.dart';

class BillingScreen extends ConsumerStatefulWidget {
  const BillingScreen({super.key});

  @override
  ConsumerState<BillingScreen> createState() => _BillingScreenState();
}

class _BillingScreenState extends ConsumerState<BillingScreen> {
  // Anchor for the "Ver planos" CTA on the trial card. Using a
  // GlobalKey lets us scroll precisely to the plans section via
  // Scrollable.ensureVisible instead of a hardcoded offset that
  // breaks when the layout above changes size.

  @override
  void initState() {
    super.initState();
    // Stripe success_url points to /billing?status=success — after a
    // checkout completes the user lands here with stale cached usage,
    // plans, and effective limits. Invalidate the user-scoped providers
    // so the new subscription state shows up without a manual refresh.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      // Wrapped in try/catch because widget tests pump BillingScreen
      // inside a bare MaterialApp without a GoRouter — GoRouterState.of
      // throws there. Production always has the router in scope.
      try {
        final state = GoRouterState.of(context);
        if (state.uri.queryParameters['status'] == 'success') {
          ref.invalidate(accountUsageProvider);
          ref.invalidate(plansListProvider);
          ref.invalidate(accountPlanLimitsProvider);
        }
        // US-180: the Stripe portal cancel flow returns here with
        // cancel_survey=1 — surface the churn-reason survey once.
        maybeShowCancellationSurvey(
          context,
          trigger: state.uri.queryParameters['cancel_survey'] == '1',
        );
      } catch (_) {}
    });
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final usageAsync = ref.watch(accountUsageProvider);
    final planLimitsAsync = ref.watch(accountPlanLimitsProvider);

    return Scaffold(
      backgroundColor: colors.backgroundPrimary,
      appBar: AppBar(
        backgroundColor: colors.backgroundSecondary,
        elevation: 0,
        leading: IconButton(
          tooltip: t.billing.back_tooltip,
          icon: Icon(Icons.arrow_back, color: colors.textSecondary),
          onPressed: () => context.go('/settings'),
        ),
        title: Text(
          t.billing.title,
          style: TextStyle(
            fontSize: 20,
            fontWeight: FontWeight.w700,
            color: colors.textPrimary,
          ),
        ),
      ),
      body: RefreshIndicator(
        onRefresh: () async {
          ref.invalidate(accountUsageProvider);
          ref.invalidate(accountPlanLimitsProvider);
          ref.invalidate(contactSalesProvider);
        },
        color: colors.bluePrimary,
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            // The trial card that used to sit here compared the trial against
            // hardcoded numbers from a retired public tier ('200+', '100k+')
            // and opened a self-service upgrade modal — on a screen that
            // swears there is no price list. Sales-led has one door: the
            // TalkToUsCard below.

            // -- Manage payment (Stripe portal) — self-serve subscribers
            // only. A contract account's billing is off-platform (the
            // effective-limits card below says so), so offering it the
            // Stripe portal and a "cancel subscription" link pointed at a
            // portal that is empty or absent.
            usageAsync.whenOrNull(
                  data: (usage) {
                    if (usage.isTrialing) return null;
                    if (planLimitsAsync.asData?.value.isContractPlan ?? false) {
                      return null;
                    }
                    return Padding(
                      padding: const EdgeInsets.only(bottom: 24),
                      child: _buildManagePaymentCard(context, ref, colors),
                    );
                  },
                ) ??
                const SizedBox.shrink(),

            // -- Effective Plan Limits (only when overrides differ from
            // catalogue, i.e. contract accounts). Self-serve accounts see
            // the same numbers in the plan card list, so duplicating here
            // would be noise.
            planLimitsAsync.whenOrNull(
                  data: (limits) {
                    if (!limits.isContractPlan) return null;
                    return Padding(
                      padding: const EdgeInsets.only(bottom: 24),
                      child: _buildEffectiveLimitsCard(context, colors, limits),
                    );
                  },
                ) ??
                const SizedBox.shrink(),

            // -- Usage Section --
            _buildSectionHeader(colors, t.billing.section_usage),
            usageAsync.when(
              data: (usage) => _buildUsageCard(
                colors,
                usage,
                // The caps come from another provider; without them the month
                // block shows consumption with no denominator, which is honest.
                planLimitsAsync.asData?.value,
              ),
              loading: () => _buildCard(
                colors,
                child: const Padding(
                  padding: EdgeInsets.all(24),
                  child: Center(child: CircularProgressIndicator()),
                ),
              ),
              error: (err, _) => _buildErrorCard(
                colors,
                message: t.billing.load_usage_failed,
                detail: parseApiError(err),
                onRetry: () => ref.invalidate(accountUsageProvider),
              ),
            ),

            const SizedBox(height: 24),

            // -- Talk to us -- what replaced the plan catalogue and the
            // credit packs (sales-led, 2026-08-19). A hospital-equipment
            // distributor books a call and hears a number that fits her; the
            // contract is applied from the owner panel.
            const TalkToUsCard(),
            const SizedBox(height: 16),
            // BYOK: o custo de IA é do cliente, então o número fica na cara
            // — gasto do ciclo, ritmo mensal e o teto que ELE define.
            const AICostCard(),
          ],
        ),
      ),
    );
  }

  // -- Helpers --

  Widget _buildSectionHeader(AppColorsExtension colors, String title) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Text(
        title,
        style: TextStyle(
          fontSize: 13,
          fontWeight: FontWeight.w700,
          color: colors.textMuted,
          letterSpacing: 1.0,
        ),
      ),
    );
  }

  Widget _buildCard(AppColorsExtension colors, {required Widget child}) {
    return Container(
      decoration: BoxDecoration(
        color: colors.backgroundSecondary,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: colors.borderLight),
      ),
      child: child,
    );
  }

  // -- Effective Plan Limits Card (contract accounts only) --

  Widget _buildEffectiveLimitsCard(
    BuildContext context,
    AppColorsExtension colors,
    EffectivePlanLimitsModel limits,
  ) {
    final auth = ref.watch(authControllerProvider).asData?.value;
    final hasOpenAI = (auth?.openAIKeyMasked ?? '').isNotEmpty;
    final hasGemini = (auth?.geminiKeyMasked ?? '').isNotEmpty;
    final byokConfigured = hasOpenAI || hasGemini;

    /// A frase contada, sempre — ou nada.
    ///
    /// There is no "unlimited" version any more. Promising an infinite cap on
    /// screen is a promise the infrastructure does not sign: disk runs out,
    /// tokens cost money, and the customer reads it as part of what they bought.
    /// The server now answers with a real number even in the unnegotiated case;
    /// if the "no cap" sentinel still arrives (an older server), the line
    /// disappears — we do not announce what we cannot assert.
    String? line(int v, String Function({required String count}) counted) =>
        EffectivePlanLimitsModel.isUnlimited(v)
            ? null
            : counted(count: _formatNumber(v));

    return Container(
      decoration: BoxDecoration(
        color: colors.bluePrimary.withValues(alpha: 0.06),
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: colors.bluePrimary.withValues(alpha: 0.25)),
      ),
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.handshake_outlined,
                  color: colors.bluePrimary, size: 20),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  t.billing.effective_limits_title(plan: limits.planName),
                  style: TextStyle(
                    color: colors.textPrimary,
                    fontSize: 15,
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 4),
          Text(
            t.billing.effective_limits_subtitle,
            style: TextStyle(color: colors.textMuted, fontSize: 12),
          ),
          const SizedBox(height: 12),
          // Each row picks between a counted phrase and a written-out one.
          // Feeding "Ilimitado" into the counted template produced
          // "Ilimitado MB de armazenamento" and "Ilimitado tokens IA/mês" —
          // the label landing where a number belongs, on the screen where the
          // customer checks what they bought.
          // The order is the order things are bought in. The product sells
          // prospecting: leads, company research and drafts come before agents
          // and channels, which are infrastructure. `0` means "this plan does
          // not sell this" — the row disappears rather than announcing a zero.
          if (limits.maxLeadsPerMonth != 0)
            _limitRow(colors,
                line(limits.maxLeadsPerMonth, t.billing.plan_feature_leads)),
          if (limits.maxEnrichmentsPerMonth != 0)
            _limitRow(
                colors,
                line(limits.maxEnrichmentsPerMonth,
                    t.billing.plan_feature_enrichments)),
          if (limits.maxAiDraftsPerMonth != 0)
            _limitRow(
                colors,
                line(
                    limits.maxAiDraftsPerMonth, t.billing.plan_feature_drafts)),
          if (limits.maxFunnels != 0)
            _limitRow(colors,
                line(limits.maxFunnels, t.billing.plan_feature_funnels)),
          if (limits.maxEmailMailboxes != 0)
            _limitRow(
                colors,
                line(limits.maxEmailMailboxes,
                    t.billing.plan_feature_mailboxes)),
          _limitRow(
              colors, line(limits.maxAgents, t.billing.plan_feature_agents)),
          _limitRow(colors,
              line(limits.maxChannels, t.billing.plan_feature_channels)),
          _limitRow(
              colors, line(limits.maxSeats, t.billing.plan_feature_seats)),
          _limitRow(
              colors,
              line(limits.maxConversationsPerMonth,
                  t.billing.plan_feature_conversations)),
          if (!limits.unlimitedTokens)
            _limitRow(
              colors,
              t.billing.plan_feature_tokens(
                  tokens: _formatTokens(limits.maxAiTokensPerMonth)),
            ),
          if (!EffectivePlanLimitsModel.isUnlimited(limits.maxStorageMb))
            _limitRow(colors, _formatStorage(limits.maxStorageMb)),
          if (limits.maxBroadcastMessagesPerMonth != 0)
            _limitRow(
                colors,
                line(limits.maxBroadcastMessagesPerMonth,
                    t.billing.plan_feature_broadcast)),
          if (limits.maxRecipientsPerCampaign != 0)
            _limitRow(
                colors,
                // Through the helper, like all its neighbours: interpolating
                // "Unlimited" in place of the number produced "Unlimited
                // recipients per campaign", and the sentence written for that
                // case was never used.
                line(limits.maxRecipientsPerCampaign,
                    t.billing.plan_feature_recipients)),
          const SizedBox(height: 12),
          Divider(color: colors.borderLight, height: 1),
          const SizedBox(height: 12),
          _buildByokRow(context, colors,
              configured: byokConfigured,
              hasOpenAI: hasOpenAI,
              hasGemini: hasGemini),
        ],
      ),
    );
  }

  Widget _buildByokRow(
    BuildContext context,
    AppColorsExtension colors, {
    required bool configured,
    required bool hasOpenAI,
    required bool hasGemini,
  }) {
    final color = configured ? colors.bluePrimary : colors.orangeAlert;
    final providers = <String>[
      if (hasOpenAI) 'OpenAI',
      if (hasGemini) 'Gemini',
    ];
    final providerLabel = providers.isEmpty ? '—' : providers.join(' + ');
    final statusLine = configured
        ? t.billing.byok_configured(providers: providerLabel)
        : t.billing.byok_pending;
    return Row(
      children: [
        Icon(
          configured ? Icons.vpn_key_outlined : Icons.warning_amber_outlined,
          color: color,
          size: 18,
        ),
        const SizedBox(width: 8),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                t.billing.byok_label,
                style: TextStyle(
                    color: colors.textPrimary,
                    fontSize: 13,
                    fontWeight: FontWeight.w600),
              ),
              const SizedBox(height: 2),
              Text(
                statusLine,
                style: TextStyle(color: color, fontSize: 12),
              ),
            ],
          ),
        ),
        TextButton(
          onPressed: () => context.go('/settings'),
          child: Text(t.billing.byok_manage_cta),
        ),
      ],
    );
  }

  /// One limit row. `null` becomes no row: when the number cannot be asserted,
  /// the right thing is to announce nothing — and never "unlimited".
  Widget _limitRow(AppColorsExtension colors, String? text) {
    if (text == null) return const SizedBox.shrink();
    return Padding(
      padding: const EdgeInsets.only(bottom: 6),
      child: Row(
        children: [
          Icon(Icons.check_circle_outline, color: colors.bluePrimary, size: 14),
          const SizedBox(width: 8),
          Expanded(
            child: Text(text,
                style: TextStyle(color: colors.textSecondary, fontSize: 13)),
          ),
        ],
      ),
    );
  }

  Widget _buildManagePaymentCard(
    BuildContext context,
    WidgetRef ref,
    AppColorsExtension colors,
  ) {
    return _buildCard(
      colors,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          ListTile(
            leading: Container(
              width: 40,
              height: 40,
              decoration: BoxDecoration(
                color: colors.bluePrimary.withValues(alpha: 0.15),
                borderRadius: BorderRadius.circular(10),
              ),
              child: Icon(Icons.account_balance_wallet_outlined,
                  color: colors.bluePrimary, size: 22),
            ),
            title: Text(
              t.billing.manage_payment_title,
              style: TextStyle(
                  color: colors.textPrimary, fontWeight: FontWeight.w600),
            ),
            subtitle: Text(
              t.billing.manage_payment_subtitle,
              style: TextStyle(color: colors.textMuted, fontSize: 13),
            ),
            trailing:
                Icon(Icons.open_in_new, color: colors.textMuted, size: 18),
            onTap: () async {
              // The URL-launcher call happens inside the controller; the
              // return is an error string or null on success.
              final err = await ref
                  .read(billingActionsProvider.notifier)
                  .openBillingPortal();
              if (context.mounted && err != null) {
                ScaffoldMessenger.of(context).showSnackBar(SnackBar(
                  content: Text(err),
                  backgroundColor: colors.redAlert,
                ));
              }
            },
          ),
          // US-180: dedicated cancel entry point. Opens the Stripe portal
          // cancel flow with a return URL that triggers the churn-reason
          // survey when the user comes back.
          Align(
            alignment: Alignment.centerLeft,
            child: TextButton(
              onPressed: () async {
                final err = await ref
                    .read(billingActionsProvider.notifier)
                    .openBillingPortalForCancellation();
                if (context.mounted && err != null) {
                  ScaffoldMessenger.of(context).showSnackBar(SnackBar(
                    content: Text(err),
                    backgroundColor: colors.redAlert,
                  ));
                }
              },
              child: Text(
                t.billing.cancel_subscription,
                style: TextStyle(
                    color: colors.textMuted,
                    fontSize: 13,
                    fontWeight: FontWeight.w600),
              ),
            ),
          ),
        ],
      ),
    );
  }

  // -- Usage Card (detailed) --

  Color _usageColor(AppColorsExtension colors, double percent) {
    if (percent >= 0.8) return colors.redAlert;
    if (percent >= 0.6) return colors.orangeAlert;
    return colors.bluePrimary;
  }

  /// One row per unit the plan sells: how much was used and, when a cap exists,
  /// out of how much. With no known cap it says only the amount — inventing a
  /// denominator is worse than having none.
  Widget _buildSdrUsage(
    AppColorsExtension colors,
    SdrUsageModel sdr,
    EffectivePlanLimitsModel? limits,
  ) {
    String value(int used, int? limit) {
      if (limit == null ||
          limit == 0 ||
          EffectivePlanLimitsModel.isUnlimited(limit)) {
        return t.billing.usage_sdr_no_limit(used: _formatNumber(used));
      }
      return t.billing.usage_sdr_of_limit(
          used: _formatNumber(used), limit: _formatNumber(limit));
    }

    Widget row(String label, String value) => Padding(
          padding: const EdgeInsets.symmetric(vertical: 4),
          child: Row(
            children: [
              Expanded(
                child: Text(label,
                    style:
                        TextStyle(color: colors.textSecondary, fontSize: 13)),
              ),
              Text(value,
                  style: TextStyle(
                      color: colors.textPrimary,
                      fontSize: 13,
                      fontWeight: FontWeight.w600)),
            ],
          ),
        );

    return Column(
      key: const Key('billing_sdr_usage'),
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(t.billing.usage_sdr_title,
            style: TextStyle(
                color: colors.textMuted,
                fontSize: 11,
                fontWeight: FontWeight.w700,
                letterSpacing: 0.8)),
        const SizedBox(height: 8),
        row(t.billing.usage_sdr_leads,
            value(sdr.leads, limits?.maxLeadsPerMonth)),
        row(t.billing.usage_sdr_enrichments,
            value(sdr.enrichments, limits?.maxEnrichmentsPerMonth)),
        row(t.billing.usage_sdr_approaches, value(sdr.approachesSent, null)),
        row(t.billing.usage_sdr_meetings, value(sdr.meetings, null)),
      ],
    );
  }

  Widget _buildUsageCard(AppColorsExtension colors, UsageModel usage,
      [EffectivePlanLimitsModel? limits]) {
    final overallPercent = usage.tokensUsageFraction;
    final effectiveLimit = usage.computedEffectiveLimit;

    // Compute input/output percentages relative to effective limit
    final inputPercent =
        effectiveLimit > 0 ? usage.aiInputTokensUsed / effectiveLimit : 0.0;
    final outputPercent =
        effectiveLimit > 0 ? usage.aiOutputTokensUsed / effectiveLimit : 0.0;

    return _buildCard(
      colors,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // The month in the units the person buys, before the token
            // breakdown. Tokens are our cost; nobody buys tokens. Absent when
            // the server could not count them — the screen stays quiet rather
            // than showing a zero that would read as "I used nothing".
            if (usage.sdr != null) ...[
              _buildSdrUsage(colors, usage.sdr!, limits),
              const SizedBox(height: 16),
              Divider(color: colors.borderLight, height: 1),
              const SizedBox(height: 16),
            ],

            // Overall token usage
            _buildUsageRow(
              colors,
              icon: Icons.auto_awesome_outlined,
              label: t.billing.usage_ai_tokens,
              current: usage.aiTokensUsed,
              max: effectiveLimit,
              percent: overallPercent,
            ),

            if (usage.addonTokens > 0) ...[
              const SizedBox(height: 4),
              Padding(
                padding: const EdgeInsets.only(left: 24),
                child: Text(
                  t.billing.usage_includes_addon_tokens(
                      count: _formatNumber(usage.addonTokens)),
                  style: TextStyle(
                    color: colors.textMuted,
                    fontSize: 11,
                    fontStyle: FontStyle.italic,
                  ),
                ),
              ),
            ],

            const SizedBox(height: 16),

            // Input tokens breakdown
            _buildMiniUsageRow(
              colors,
              label: t.billing.usage_input,
              current: usage.aiInputTokensUsed,
              percent: inputPercent,
              color: colors.bluePrimary,
            ),
            const SizedBox(height: 10),

            // Output tokens breakdown
            _buildMiniUsageRow(
              colors,
              label: t.billing.usage_output,
              current: usage.aiOutputTokensUsed,
              percent: outputPercent,
              color: colors.cyanGlow,
            ),

            const SizedBox(height: 16),
            Divider(color: colors.borderLight, height: 1),
            const SizedBox(height: 16),

            // Conversations usage
            _buildUsageRow(
              colors,
              icon: Icons.chat_bubble_outline,
              label: t.billing.usage_conversations,
              current: usage.conversationsCount,
              max: usage.maxConversationsPerMonth,
              percent: usage.conversationsUsageFraction,
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildUsageRow(
    AppColorsExtension colors, {
    required IconData icon,
    required String label,
    required int current,
    required int max,
    required double percent,
  }) {
    final clampedPercent = percent.clamp(0.0, 1.0);
    final barColor = _usageColor(colors, clampedPercent);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Icon(icon, size: 16, color: colors.textMuted),
            const SizedBox(width: 8),
            Text(
              label,
              style: TextStyle(
                color: colors.textSecondary,
                fontSize: 13,
                fontWeight: FontWeight.w600,
              ),
            ),
            const Spacer(),
            Text(
              // A ceiling that is really "no ceiling" reads as 2147.5M — the
              // int32 sentinel printed as a quantity, beside a real number,
              // on the billing screen.
              // With no assertable cap it shows only what was consumed. Writing
              // "/ Unlimited" promises what the infrastructure does not sign,
              // and the customer reads the promise as part of what they bought.
              EffectivePlanLimitsModel.isUnlimited(max)
                  ? _formatNumber(current)
                  : '${_formatNumber(current)} / ${_formatNumber(max)}',
              style: TextStyle(
                color: clampedPercent >= 0.8 ? barColor : colors.textMuted,
                fontSize: 12,
                fontWeight: FontWeight.w600,
              ),
            ),
          ],
        ),
        const SizedBox(height: 6),
        ClipRRect(
          borderRadius: BorderRadius.circular(4),
          child: LinearProgressIndicator(
            value: clampedPercent,
            backgroundColor: colors.glassMedium,
            valueColor: AlwaysStoppedAnimation(barColor),
            minHeight: 6,
          ),
        ),
        const SizedBox(height: 2),
        Align(
          alignment: Alignment.centerRight,
          child: Text(
            '${(clampedPercent * 100).toStringAsFixed(0)}%',
            style: TextStyle(
              color: clampedPercent >= 0.8 ? barColor : colors.textMuted,
              fontSize: 11,
              fontWeight: FontWeight.w600,
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildMiniUsageRow(
    AppColorsExtension colors, {
    required String label,
    required int current,
    required double percent,
    required Color color,
  }) {
    final clampedPercent = percent.clamp(0.0, 1.0);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            const SizedBox(width: 24),
            Text(
              label,
              style: TextStyle(
                color: colors.textMuted,
                fontSize: 12,
              ),
            ),
            const Spacer(),
            Text(
              _formatNumber(current),
              style: TextStyle(
                color: colors.textMuted,
                fontSize: 11,
                fontWeight: FontWeight.w600,
              ),
            ),
          ],
        ),
        const SizedBox(height: 3),
        Padding(
          padding: const EdgeInsets.only(left: 24),
          child: ClipRRect(
            borderRadius: BorderRadius.circular(3),
            child: LinearProgressIndicator(
              value: clampedPercent,
              backgroundColor: colors.glassMedium,
              valueColor: AlwaysStoppedAnimation(color),
              minHeight: 4,
            ),
          ),
        ),
      ],
    );
  }

  // -- Plan Card --

  // -- Error Card --

  Widget _buildErrorCard(
    AppColorsExtension colors, {
    required String message,
    required String detail,
    required VoidCallback onRetry,
  }) {
    return _buildCard(
      colors,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          children: [
            Icon(Icons.error_outline, color: colors.redAlert, size: 36),
            const SizedBox(height: 8),
            Text(
              message,
              style: TextStyle(color: colors.textSecondary),
            ),
            const SizedBox(height: 4),
            Text(
              detail,
              style: TextStyle(color: colors.textMuted, fontSize: 11),
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
            const SizedBox(height: 12),
            TextButton.icon(
              onPressed: onRetry,
              icon: const Icon(Icons.refresh, size: 16),
              label: Text(t.common.retry),
            ),
          ],
        ),
      ),
    );
  }

  // -- Actions --

  // -- Formatting --

  /// Storage in the unit people actually say out loud.
  ///
  /// The platform ceiling is 512000 MB, which the generic formatter wrote as
  /// "512k MB" — correct and unreadable. Nobody buys half a million megabytes;
  /// they buy 500 GB.
  String _formatStorage(int mb) {
    if (mb >= 1024) {
      final gb = mb / 1024;
      return t.billing.plan_feature_storage_gb(
          gb: gb.toStringAsFixed(gb.truncateToDouble() == gb ? 0 : 1));
    }
    return t.billing.plan_feature_storage(mb: _formatNumber(mb));
  }

  /// Tokens acima de um bilhão. "1000M" é o mesmo problema de "512k MB".
  String _formatTokens(int tokens) {
    if (tokens >= 1000000000) {
      final b = tokens / 1000000000;
      return '${b.toStringAsFixed(b.truncateToDouble() == b ? 0 : 1)}B';
    }
    return _formatNumber(tokens);
  }

  String _formatNumber(int value) {
    if (value >= 1000000) {
      final m = value / 1000000;
      return '${m.toStringAsFixed(m.truncateToDouble() == m ? 0 : 1)}M';
    }
    if (value >= 1000) {
      final k = value / 1000;
      return '${k.toStringAsFixed(k.truncateToDouble() == k ? 0 : 1)}k';
    }
    return value.toString();
  }
}
