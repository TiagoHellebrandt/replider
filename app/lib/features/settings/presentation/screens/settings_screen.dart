import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:app/core/auth/account_role.dart';
import 'package:app/core/router/back_nav.dart';
import 'package:app/features/billing/domain/models/effective_plan_limits_model.dart';
import 'package:app/features/billing/domain/models/usage_model.dart';
import 'package:app/features/settings/presentation/controllers/channels_controller.dart';
import 'package:app/features/owner/presentation/controllers/owner_controller.dart';
import 'package:app/features/settings/presentation/widgets/api_keys_settings_card.dart';
import 'package:app/features/settings/presentation/widgets/notifications_settings_card.dart';
import 'package:app/features/settings/presentation/widgets/settings_tile.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/billing/presentation/widgets/billing_pill.dart';
import 'package:app/features/billing/presentation/widgets/talk_to_us_card.dart';
import 'package:app/features/billing/presentation/widgets/upgrade_plan_modal.dart';
import 'package:app/design_system/components/settings_section_card.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/i18n/strings.g.dart';

/// Whether the plan actually has a ceiling to measure usage against.
///
/// A contract plan stores 0 in the limit columns and the API resolves that as
/// "no ceiling"; an enterprise ceiling can also arrive as the int32 sentinel.
/// Dividing by either printed "Tokens: 0% | Conversas: 0%" on an account that
/// had spent 5.5K tokens across 13 conversations — the reading a customer is
/// most likely to check, and the one most likely to be wrong. With no
/// denominator there is no share to state.
bool planUsageHasCeiling(UsageModel usage) {
  bool bounded(int limit) =>
      limit > 0 && !EffectivePlanLimitsModel.isUnlimited(limit);
  return bounded(usage.maxAiTokensPerMonth) &&
      bounded(usage.maxConversationsPerMonth);
}

// One-shot session guard for the Google Calendar OAuth redirect
// snackbar (`/settings?gcal=connected|error|denied`) — the query param
// survives rebuilds, the toast must not.
bool _gcalResultShown = false;

@visibleForTesting
void resetGcalResultForTest() => _gcalResultShown = false;

class SettingsScreen extends ConsumerWidget {
  const SettingsScreen({super.key});

  void _maybeShowGcalResult(BuildContext context) {
    if (_gcalResultShown) return;
    // Widget tests may pump this screen without a GoRouter in scope.
    String? result;
    try {
      result = GoRouterState.of(context).uri.queryParameters['gcal'];
    } catch (_) {
      return;
    }
    if (result == null) return;
    _gcalResultShown = true;
    final message = switch (result) {
      'connected' => t.settings.gcal_connected_snackbar,
      'denied' => t.settings.gcal_denied_snackbar,
      _ => t.settings.gcal_error_snackbar,
    };
    final isSuccess = result == 'connected';
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!context.mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(message),
          backgroundColor:
              isSuccess ? Colors.green.shade700 : Colors.red.shade700,
          behavior: SnackBarBehavior.floating,
        ),
      );
    });
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    _maybeShowGcalResult(context);
    final authState = ref.watch(authControllerProvider);
    // Read-only mode: trial expired or subscription past_due/canceled.
    // The user can still browse this screen (products, channels, brand),
    // but every paid edit endpoint 402s. We surface that up front with a
    // banner so they don't go fill out a form for nothing.
    final paidActive = authState.asData?.value?.isPaidActive ?? true;
    // Attendants get a 403 FORBIDDEN on every management endpoint
    // (channels/team/billing/brand are RequireRole("admin") server-side).
    // Hide those sections so they never tap into a dead end — the API stays
    // the source of truth, this is UX only. They keep: workspace, profile,
    // canned responses
    // (admin+attendant), logout.
    final isAdmin = authState.asData?.value?.isAdmin ?? false;
    // GET /brand-kit is RequireRole("admin", "attendant"): gestor and above
    // satisfy attendant, a vendedor does not — for her the tile was a 403.
    final canSeeBrand = AccountRole.from(authState.asData?.value?.role)
        .satisfies(AccountRole.attendant);

    return Scaffold(
      backgroundColor: colors.backgroundPrimary,
      appBar: AppBar(
        backgroundColor: colors.backgroundSecondary,
        elevation: 0,
        leading: IconButton(
          tooltip: t.settings.back_tooltip,
          icon: Icon(Icons.arrow_back, color: colors.textSecondary),
          onPressed: () => appBack(context),
        ),
        title: Text(
          t.settings.title,
          style: TextStyle(
            fontSize: 20,
            fontWeight: FontWeight.w700,
            color: colors.textPrimary,
          ),
        ),
        actions: const [
          // Billing pill in Settings too — the user's most common "I want
          // to pay" journey starts on the dashboard but Settings is the
          // second-most-clicked screen, so it gets the same affordance
          // to keep the upgrade path visible regardless of entry point.
          Padding(
            padding: EdgeInsets.only(right: 12),
            child: BillingPill(),
          ),
        ],
      ),
      body: SettingsPageBody(
        children: [
          if (!paidActive) ...[
            Container(
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: colors.bluePrimary.withValues(alpha: 0.10),
                borderRadius: BorderRadius.circular(8),
                border: Border.all(
                    color: colors.bluePrimary.withValues(alpha: 0.4)),
              ),
              child: Row(
                children: [
                  Icon(Icons.lock_outline, color: colors.bluePrimary, size: 18),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      t.settings.plan_expired_banner,
                      style: TextStyle(color: colors.textPrimary, fontSize: 13),
                    ),
                  ),
                  TextButton(
                    onPressed: () => showUpgradePlanModal(context),
                    child: Text(
                      t.billing.talk_book_cta,
                      style: TextStyle(
                          color: colors.bluePrimary,
                          fontWeight: FontWeight.w700),
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 16),
          ],

          // WHO WE ARE. The company name is what the agent introduces itself
          // with, and the brand screen holds what the business may never claim —
          // in healthcare a regulatory rule, not a style preference.
          SettingsSectionLabel(t.settings.section_company),
          SettingsGroup(children: [
            if (isAdmin)
              SettingsTile(
                icon: Icons.business,
                title: t.settings.company_section_title,
                subtitle: t.settings.account_subtitle,
                onTap: () => context.go('/settings/account'),
              ),
            // Read-only below admin. Meant for every role — the forbidden-
            // claims list is what a vendedora is held to — but the server
            // gates the read at attendant, so below that the tile would only
            // open a 403. Hidden until the API admits her.
            if (canSeeBrand)
              SettingsTile(
                key: const Key('settings_brand_tile'),
                icon: Icons.verified_outlined,
                title: t.settings.brand_tile,
                subtitle: t.settings.brand_tile_sub,
                onTap: () => context.go('/settings/brand'),
              ),
          ]),
          const SizedBox(height: 24),

          // The person, not the account: name, email, password, and the
          // per-device notification toggle.
          SettingsSectionLabel(t.profile.title),
          SettingsGroup(children: [
            authState.when(
              data: (user) {
                if (user == null) return null;
                return ListTile(
                  onTap: () => context.go('/settings/profile'),
                  leading: CircleAvatar(
                    backgroundColor: colors.bluePrimary,
                    child: Text(
                      user.name.isNotEmpty ? user.name[0].toUpperCase() : '?',
                      style: const TextStyle(
                        color: Colors.white,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                  ),
                  title: Text(
                    user.name,
                    style: TextStyle(
                      color: colors.textPrimary,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  subtitle: Text(
                    user.email,
                    style: TextStyle(color: colors.textMuted, fontSize: 13),
                  ),
                  trailing: Icon(Icons.edit_outlined,
                      size: 18, color: colors.textMuted),
                );
              },
              loading: () => const Center(child: CircularProgressIndicator()),
              // Disappearing took the whole profile row with it: anyone
              // looking for "My Profile" would conclude the option does not
              // exist. A failed read is not an absent feature.
              error: (_, __) => ListTile(
                key: const Key('settings_profile_error'),
                onTap: () => ref.invalidate(authControllerProvider),
                leading: Icon(Icons.refresh, color: colors.textMuted),
                title: Text(
                  t.common.load_failed_tap,
                  style: TextStyle(color: colors.textMuted, fontSize: 13),
                ),
              ),
            ),
            if (kIsWeb) const NotificationsSettingsCard(),
          ]),

          // BYOK API-keys card. Renders only for contract plans
          // (Enterprise / Partner); the widget itself returns
          // SizedBox.shrink for self-serve accounts. Admin-only: the
          // save endpoint is PATCH /account (RequireRole admin).
          if (isAdmin) ...[
            const SizedBox(height: 16),
            const ApiKeysSettingsCard(),
          ],

          // HOW MAIL LEAVES. The first pillar of the product lives in this
          // group: the mailbox she already has, proved, then the texts that go
          // out through it, who must never receive them, and how much goes
          // without her reading it. They were in four different sections.
          if (isAdmin) ...[
            const SizedBox(height: 24),
            SettingsSectionLabel(t.settings.section_email),
            SettingsGroup(children: [
              SettingsTile(
                icon: Icons.forum_outlined,
                iconColor: colors.whatsappGreen,
                title: t.settings.channels_title,
                subtitle: t.settings.channels_subtitle,
                onTap: () => context.go('/settings/channels'),
                trailing: _channelsTrailing(ref, colors),
              ),
              SettingsTile(
                key: const Key('settings_suppressions_tile'),
                icon: Icons.mark_email_unread_outlined,
                iconColor: colors.redAlert,
                title: t.settings.suppressions_tile,
                subtitle: t.settings.suppressions_tile_sub,
                onTap: () => context.go('/settings/email-suppressions'),
              ),
            ]),
            const SizedBox(height: 24),

            // WHAT WE SELL. It was unreachable from Settings: products only
            // from the account menu. (The funnel list moved under "Avançado"
            // at the bottom on 2026-09-01 — see there.)
            SettingsSectionLabel(t.settings.section_catalog),
            SettingsGroup(children: [
              SettingsTile(
                key: const Key('settings_products_tile'),
                icon: Icons.inventory_2_outlined,
                iconColor: colors.orangeAlert,
                title: t.sales.products.title,
                subtitle: t.sales.products.subtitle,
                onTap: () => context.go('/sales/products'),
              ),
              // The history behind the catalogue: who bought what. It feeds
              // the Carteira's reorder dates and the cross-sell suggestions,
              // so the door lives next to the products it references.
              SettingsTile(
                key: const Key('settings_sales_import_tile'),
                icon: Icons.receipt_long_outlined,
                iconColor: colors.whatsappGreen,
                title: t.sales_import.title,
                subtitle: t.sales_import.settings_subtitle,
                onTap: () => context.go('/sales/import'),
              ),
            ]),
            const SizedBox(height: 24),

            // WHO SELLS.
            SettingsSectionLabel(t.settings.team),
            SettingsGroup(children: [
              SettingsTile(
                icon: Icons.group_outlined,
                title: t.settings.manage_team,
                subtitle: t.settings.manage_team_subtitle,
                onTap: () => context.go('/settings/team'),
              ),
            ]),
            const SizedBox(height: 24),
          ],

          // The automated-support product, the contracts, the commissions and
          // the outbound webhooks left this hub on 2026-08-18: none of them is
          // on the path from "here are my products and my customers" to "an
          // offer went out with my approval", and every extra row here was a
          // row the person had to learn to skip. The screens live on in the
          // archive branch, not behind a door.

          // PLUMBING. Nobody comes to Settings looking for this; they come
          // once, wire it up, and never return.
          if (isAdmin) ...[
            SettingsSectionLabel(t.settings.section_integrations),
            SettingsGroup(children: [
              SettingsTile(
                icon: Icons.auto_awesome_outlined,
                title: t.settings.claude_title,
                subtitle: t.settings.claude_subtitle,
                onTap: () => context.go('/settings/claude'),
              ),
            ]),
            const SizedBox(height: 24),

            // Sales-led (2026-08-19): no "Cobrança" door. The usage lives
            // on the dashboard strip; contracting or growing is a
            // conversation, offered right here under its own heading.
            SettingsSectionLabel(t.settings.section_contract),
            const TalkToUsCard(compact: true),
            const SizedBox(height: 12),
            // The AI-cost / BYOK-budget screen. Without this tile /billing
            // had NO entry in the whole UI — the customer's only spend
            // brake was reachable by pasted URL or by hitting a paywall.
            SettingsGroup(children: [
              SettingsTile(
                icon: Icons.query_stats_rounded,
                iconColor: colors.cyanGlow,
                title: t.settings.usage_costs_title,
                subtitle: t.settings.usage_costs_subtitle,
                onTap: () => context.go('/billing'),
              ),
            ]),
            const SizedBox(height: 24),

            SettingsSectionLabel(t.settings.section_compliance),
            SettingsGroup(children: [
              SettingsTile(
                icon: Icons.gavel_rounded,
                iconColor: colors.redAlert,
                title: t.settings.compliance_title,
                subtitle: t.settings.compliance_subtitle,
                onTap: () => context.go('/settings/compliance'),
              ),
            ]),
            const SizedBox(height: 24),

            // ADVANCED — collapsed by default. The funnel configuration and
            // the relationship dates are real, kept and routable, but they
            // are not on the path from "my products and my customers" to
            // "an offer went out with my approval": the default funnel is
            // created for every account and the dates are reachable from the
            // portfolio. A person who wants them opens this; everyone else
            // never has to learn what a funnel is.
            _AdvancedSection(colors: colors),
            const SizedBox(height: 24),
          ],

          // Owner panel — only rendered when the probe to /v1/owner
          // succeeds (the API echoes the SUPER_ADMIN_EMAILS allowlist
          // via 200/403 on the listing endpoint). Non-allowlisted
          // operators never see this entry, so the route is effectively
          // hidden from regular customers.
          ...ref.watch(ownerIsSuperAdminProvider).maybeWhen(
                data: (allowed) => allowed
                    ? [
                        SettingsSectionLabel(t.owner.menu_label),
                        SettingsGroup(children: [
                          SettingsTile(
                            icon: Icons.shield_outlined,
                            title: t.owner.accounts_title,
                            onTap: () => context.go('/owner/accounts'),
                          ),
                        ]),
                        const SizedBox(height: 24),
                      ]
                    : <Widget>[],
                orElse: () => <Widget>[],
              ),

          // Logout at the bottom of the settings page. Mobile users
          // don't have the app-bar UserAccountChip visible on every
          // screen, so exposing a dedicated button here is the only
          // self-serve way to end the session without clearing cookies.
          //
          // Works during an active impersonation session too: logout() is
          // a purely local storage wipe (there is no backend token-revoke
          // call), and it clears the impersonation slot BEFORE the regular
          // user, so a full logout from inside an impersonated session
          // safely ends both. The banner's "End" button is the lighter
          // exit (back to the operator); this one signs out entirely.
          SettingsGroup(children: [
            SettingsTile(
              icon: Icons.logout,
              iconColor: colors.redAlert,
              danger: true,
              title: t.settings.logout_title,
              subtitle: t.settings.logout_subtitle,
              onTap: () async {
                final confirmed = await showDialog<bool>(
                  context: context,
                  builder: (ctx) => AlertDialog(
                    title: Text(t.settings.logout_confirm_title),
                    content: Text(t.settings.logout_confirm_body),
                    actions: [
                      TextButton(
                        onPressed: () => Navigator.of(ctx).pop(false),
                        child: Text(t.common.cancel),
                      ),
                      TextButton(
                        onPressed: () => Navigator.of(ctx).pop(true),
                        child: Text(t.common.logout,
                            style: TextStyle(color: colors.redAlert)),
                      ),
                    ],
                  ),
                );
                if (confirmed != true) return;
                await ref.read(authControllerProvider.notifier).logout();
                if (context.mounted) context.go('/');
              },
            ),
          ]),
          const SizedBox(height: 32),
        ],
      ),
    );
  }

  /// Trailing slot for the "Canais de Comunicação" tile. Renders a
  /// red "N offline" badge ahead of the chevron when at least one
  /// channel is paused; otherwise just the chevron. Reuses the
  /// existing `channelsListProvider` so we don't fire an extra request.
  Widget _channelsTrailing(WidgetRef ref, AppColorsExtension colors) {
    final chevron =
        Icon(Icons.chevron_right, color: colors.textMuted, size: 20);
    final async = ref.watch(channelsListProvider);
    final list = async.asData?.value;
    if (list == null) return chevron;
    final offline = list.where((c) => !c.isActive).length;
    if (offline == 0) return chevron;
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
          decoration: BoxDecoration(
            color: colors.redAlert.withValues(alpha: 0.15),
            borderRadius: BorderRadius.circular(4),
          ),
          child: Text(
            t.settings.channels_count_offline(count: offline.toString()),
            style: TextStyle(
              color: colors.redAlert,
              fontSize: 10,
              fontWeight: FontWeight.w700,
            ),
          ),
        ),
        const SizedBox(width: 6),
        chevron,
      ],
    );
  }
}

/// The two tiles that only an advanced user needs, behind one closed door.
/// Collapsed by default (`initiallyExpanded: false`); the routes stay.
class _AdvancedSection extends StatelessWidget {
  const _AdvancedSection({required this.colors});
  final AppColorsExtension colors;

  @override
  Widget build(BuildContext context) {
    return Theme(
      // The ExpansionTile paints its own divider lines on expand; the
      // settings group already carries a border, so drop them.
      data: Theme.of(context).copyWith(dividerColor: Colors.transparent),
      child: ExpansionTile(
        key: const Key('settings_advanced_toggle'),
        tilePadding: EdgeInsets.zero,
        childrenPadding: EdgeInsets.zero,
        initiallyExpanded: false,
        iconColor: colors.textMuted,
        collapsedIconColor: colors.textMuted,
        // The same label the open sections wear, so the closed door reads
        // as one more section and not as a foreign widget.
        title: SettingsSectionLabel(t.settings.section_advanced),
        subtitle: Text(
          t.settings.section_advanced_sub,
          style: TextStyle(color: colors.textMuted, fontSize: 12),
        ),
        children: [
          SettingsGroup(children: [
            SettingsTile(
              key: const Key('settings_funnels_tile'),
              icon: Icons.filter_alt_outlined,
              title: t.sales.funnels.title,
              subtitle: t.settings.funnels_tile_sub,
              onTap: () => context.go('/sales/funnels'),
            ),
            SettingsTile(
              key: const Key('settings_relationship_tile'),
              icon: Icons.cake_outlined,
              title: t.relationship.title,
              subtitle: t.settings.relationship_tile_sub,
              onTap: () => context.go('/sales/relationship'),
            ),
          ]),
        ],
      ),
    );
  }
}
