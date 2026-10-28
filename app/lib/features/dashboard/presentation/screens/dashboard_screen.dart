import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:app/core/network/websocket_service.dart';
import 'package:app/core/pwa/pwa_install_banner.dart';
import 'package:app/core/notifications/notification_service.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/billing/presentation/controllers/billing_controller.dart';
import 'package:app/features/billing/presentation/controllers/trial_banner_controller.dart';
import 'package:app/features/billing/presentation/controllers/usage_warning_dismissal_controller.dart';
import 'package:app/features/billing/presentation/widgets/billing_pill.dart';
import 'package:app/features/billing/presentation/widgets/quota_wall_modal.dart';
import 'package:app/features/billing/presentation/widgets/upgrade_plan_modal.dart';
import 'package:app/features/dashboard/presentation/controllers/dashboard_controller.dart';
import 'package:app/features/dashboard/presentation/widgets/conversation_list_panel.dart';
import 'package:app/features/dashboard/presentation/widgets/conversation_detail_panel.dart';
import 'package:app/features/announcements/presentation/widgets/feature_announcement_modal.dart';
import 'package:app/features/feedback/presentation/widgets/nps_survey_modal.dart';
import 'package:app/features/onboarding/presentation/widgets/product_tour_modal.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/i18n/strings.g.dart';

class DashboardScreen extends ConsumerStatefulWidget {
  const DashboardScreen({super.key, this.initialConversationId});

  /// Thread to open on arrival — `/home?conversation=<id>`, the push
  /// notification's click_url. Null keeps whatever was selected.
  final String? initialConversationId;

  @override
  ConsumerState<DashboardScreen> createState() => _DashboardScreenState();
}

class _DashboardScreenState extends ConsumerState<DashboardScreen> {
  static const double _mobileBreakpoint = 600;
  // Below this the split view is cramped on tablets (e.g. iPad at 768px):
  // a full 380px sidebar leaves the conversation detail too narrow. We keep
  // the split but shrink the sidebar so the message thread gets more room.
  static const double _tabletBreakpoint = 900;
  static const double _sidebarWidth = 380;
  static const double _tabletSidebarWidth = 300;

  @override
  void initState() {
    super.initState();
    _applyDeepLink(widget.initialConversationId);
  }

  @override
  void didUpdateWidget(DashboardScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    // Same screen, new `?conversation=` (a second push click while the
    // inbox is already open) — GoRouter reuses the element.
    if (widget.initialConversationId != oldWidget.initialConversationId) {
      _applyDeepLink(widget.initialConversationId);
    }
  }

  void _applyDeepLink(String? conversationId) {
    if (conversationId == null || conversationId.isEmpty) return;
    // Post-frame: a provider must not be mutated during build/initState.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      ref.read(selectedConversationProvider.notifier).select(conversationId);
    });
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final selectedId = ref.watch(selectedConversationProvider);

    // Activate the global "new message in another tab" notifier. The
    // provider does the actual work via ref.listen; we just need to
    // make sure something keeps it watched while the dashboard is
    // alive — keepAlive carries it across non-dashboard screens.
    ref.watch(inboundNotificationListenerProvider);

    // Listen for usage alerts and show a snackbar for <100% (the 100%
    // state opens the QuotaWallModal instead — a snackbar is too
    // dismissable for a hard block that stops the bot replying).
    ref.listen<double?>(usageAlertProvider, (previous, next) {
      if (next == null || !context.mounted) return;
      if (next >= 100) {
        maybeShowQuotaWall(context, tokensUsagePercent: next);
      } else {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(t.dashboard.token_usage_warning_snackbar(
                percent: next.toStringAsFixed(0))),
            backgroundColor: colors.orangeAlert,
            behavior: SnackBarBehavior.floating,
            duration: const Duration(seconds: 5),
          ),
        );
      }
      // Clear the alert after handling so the same value doesn't
      // re-trigger on a rebuild.
      ref.read(usageAlertProvider.notifier).clear();
    });

    final authState = ref.watch(authControllerProvider);
    // Single subscription to accountUsageProvider — the trial banner and
    // the cold-start quota wall both read off this one watched value, so
    // we never open a second listen on the same provider (which would be
    // a redundant double-watch). The WS-pushed mid-session transitions
    // still come through usageAlertProvider above.
    final usageAsync = ref.watch(accountUsageProvider);

    // Cold-start wall: when the dashboard first renders with a
    // saturated /usage response (user was already at 100% when they
    // opened the app), the WS-driven usageAlert listen won't fire.
    // Derive the check off the watched usage value so the modal still
    // pops. Safe to call from build — maybeShowQuotaWall dedupes per
    // session and defers the dialog to a post-frame callback.
    final coldStartUsage = usageAsync.asData?.value;
    if (coldStartUsage != null) {
      maybeShowQuotaWall(context,
          tokensUsagePercent: coldStartUsage.tokensUsagePercent);
    }
    // Reconnection banner. We only surface the "offline" state, never
    // a persistent green "connected" one — the absence of the banner
    // IS the OK state. maybeWhen defaults to connected so the banner
    // doesn't flash during cold start before the first handshake.
    final wsOk = ref.watch(wsConnectionStatusProvider).maybeWhen(
          data: (v) => v,
          orElse: () => true,
        );

    // Quarterly in-app NPS prompt (US-178). Suppressed when the account is
    // at its hard token wall so we don't stack two dialogs — the quota wall
    // is a blocking decision moment and takes priority.
    final atQuotaWall = (coldStartUsage?.tokensUsagePercent ?? 0) >= 100;
    if (!atQuotaWall) {
      // One-shot post-onboarding product tour (US-183) takes priority over the
      // NPS nudge for brand-new accounts; once it's been shown (or skipped)
      // the launcher no-ops and the quarterly NPS prompt resumes.
      maybeShowProductTour(context, ref);
      // "What's new" announcement (US-184) for returning, fully-onboarded
      // accounts after a release. Its launcher self-suppresses for brand-new
      // accounts, so it never stacks on top of the product tour above.
      maybeShowFeatureAnnouncement(context, ref);
      maybeShowNpsPrompt(context, ref);
    }

    return Scaffold(
      backgroundColor: colors.backgroundPrimary,
      body: CallbackShortcuts(
        bindings: {
          // ESC closes the open conversation, mirroring WhatsApp Web.
          // No-op when nothing is selected so the key still reaches
          // dialogs/modals above us (which install their own shortcut
          // scope on a separate Route).
          const SingleActivator(LogicalKeyboardKey.escape): () {
            if (ref.read(selectedConversationProvider) != null) {
              ref.read(selectedConversationProvider.notifier).select(null);
            }
          },
        },
        child: Focus(
          autofocus: true,
          child: Stack(
            children: [
              SafeArea(
                child: Column(
                  children: [
                    // WS reconnection banner. Only shown while the socket is
                    // down — once reconnection succeeds it vanishes. Drawn
                    // above the trial banner so an offline trialing user sees
                    // the connectivity state first (can't upgrade if they're
                    // offline anyway).
                    if (!wsOk)
                      Container(
                        width: double.infinity,
                        padding: const EdgeInsets.symmetric(
                            horizontal: 16, vertical: 8),
                        color: colors.orangeAlert,
                        child: Row(
                          mainAxisAlignment: MainAxisAlignment.center,
                          children: [
                            const SizedBox(
                              width: 12,
                              height: 12,
                              child: CircularProgressIndicator(
                                strokeWidth: 1.6,
                                valueColor:
                                    AlwaysStoppedAnimation<Color>(Colors.white),
                              ),
                            ),
                            const SizedBox(width: 10),
                            Text(
                              t.dashboard.reconnecting,
                              style: const TextStyle(
                                  color: Colors.white,
                                  fontSize: 12,
                                  fontWeight: FontWeight.w600),
                            ),
                          ],
                        ),
                      ),

                    // PWA install prompt — only renders on mobile-web when the
                    // app isn't already installed. Sits ABOVE the trial banner
                    // so users see the install path before being asked to pay.
                    // Critical for iOS: web push only fires inside the
                    // installed PWA, so the dashboard stays useless for
                    // notifications until they go through Add-to-Home-Screen.
                    const PwaInstallBanner(),

                    // Trial banner
                    authState.whenOrNull(
                          data: (user) {
                            // Contract / trial-bypass accounts are never on
                            // trial — guard here too so the banner never flashes
                            // on a paid contract account while /me is cached.
                            if (user == null ||
                                !user.isTrialing ||
                                user.isContractPlan ||
                                user.trialBypass) {
                              return null;
                            }
                            return usageAsync.whenOrNull(
                              data: (usage) => _TrialBanner(
                                conversationsUsed: usage.conversationsCount,
                                conversationsMax:
                                    usage.maxConversationsPerMonth,
                                daysRemaining: user.trialDaysRemaining,
                              ),
                            );
                          },
                        ) ??
                        const SizedBox.shrink(),

                    // Usage warning banners. The 100% "exhausted" banner is a hard
                    // block (the bot stops replying) and is never dismissible. The
                    // soft 80% banner is dismissible per calendar day via
                    // [usageWarningDismissalProvider] so it nudges without nagging.
                    usageAsync.whenOrNull(
                          data: (usage) {
                            if (usage.tokensUsagePercent >= 100) {
                              return _UsageWarningBanner(
                                message: t.dashboard.tokens_exhausted_banner,
                                color: colors.redAlert,
                              );
                            } else if (usage.tokensUsagePercent >= 80) {
                              final dismissed = ref
                                      .watch(usageWarningDismissalProvider)
                                      .asData
                                      ?.value ??
                                  false;
                              if (dismissed) return null;
                              return _UsageWarningBanner(
                                message: t.dashboard.tokens_high_usage_banner(
                                    percent: usage.tokensUsagePercent
                                        .toStringAsFixed(0)),
                                color: colors.orangeAlert,
                                onDismiss: () => ref
                                    .read(
                                        usageWarningDismissalProvider.notifier)
                                    .dismissForToday(),
                              );
                            }
                            return null;
                          },
                        ) ??
                        const SizedBox.shrink(),

                    Expanded(
                      child: LayoutBuilder(
                        builder: (context, constraints) {
                          final isMobile =
                              constraints.maxWidth < _mobileBreakpoint;

                          if (isMobile) {
                            return _buildMobileLayout(ref, selectedId);
                          }
                          final isTablet =
                              constraints.maxWidth < _tabletBreakpoint;
                          return _buildDesktopLayout(
                            colors,
                            sidebarWidth:
                                isTablet ? _tabletSidebarWidth : _sidebarWidth,
                          );
                        },
                      ),
                    ),
                  ],
                ),
              ),
              // Billing pill floats over the content instead of living in a
              // reserved bottom strip. The ambient quota strip is gone with
              // the BYOK/token model, and the onboarding checklist lives on
              // Início ONLY (2026-08-27) — its panel slid over the open
              // conversation and its rocket sat on the composer; first-steps
              // guidance has no business inside a live thread.
              const Positioned(
                right: 12,
                bottom: 12,
                child: BillingPill(),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildMobileLayout(WidgetRef ref, String? selectedId) {
    // Mobile: full-screen list or full-screen detail
    if (selectedId != null) {
      return PopScope(
        canPop: false,
        onPopInvokedWithResult: (didPop, result) {
          if (!didPop) {
            ref.read(selectedConversationProvider.notifier).select(null);
          }
        },
        child: const ConversationDetailPanel(showBackButton: true),
      );
    }
    return const ConversationListPanel();
  }

  Widget _buildDesktopLayout(
    AppColorsExtension colors, {
    double sidebarWidth = _sidebarWidth,
  }) {
    return Row(
      children: [
        // Sidebar
        SizedBox(
          key: const Key('dashboard_sidebar'),
          width: sidebarWidth,
          child: Container(
            decoration: BoxDecoration(
              color: colors.backgroundSecondary,
              border: Border(
                right: BorderSide(color: colors.borderLight, width: 0.5),
              ),
            ),
            child: const ConversationListPanel(),
          ),
        ),
        // Detail
        const Expanded(
          child: ConversationDetailPanel(),
        ),
      ],
    );
  }
}

/// Dismissible banner shown when the user is on the trial plan.
///
/// Dismissal persists per calendar day via [trialBannerDismissalProvider]
/// (local pref) so it survives reloads but returns the next day. Urgent
/// banners (<= 3 days left) ignore the dismissal and drop the close button
/// so the deadline can't be hidden.
class _TrialBanner extends ConsumerWidget {
  final int conversationsUsed;
  final int conversationsMax;

  /// Days remaining in the free trial. When <= 3 the banner paints red
  /// and drops the dismiss button so the user can't lose it. Null means
  /// the backend didn't report a deadline (legacy account or non-trial),
  /// in which case we fall back to the conversations-based copy.
  final int? daysRemaining;

  const _TrialBanner({
    required this.conversationsUsed,
    required this.conversationsMax,
    this.daysRemaining,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final days = daysRemaining;
    final urgent = days != null && days <= 3;

    // Per-day dismissal only applies to the non-urgent banner; once the
    // trial is <= 3 days out we always show it.
    final dismissed = !urgent &&
        (ref.watch(trialBannerDismissalProvider).asData?.value ?? false);
    if (dismissed) return const SizedBox.shrink();

    final copy = _bannerCopy();
    final bgColor = urgent ? Colors.red.shade700 : Colors.amber.shade700;
    final btnFg = urgent ? Colors.red.shade800 : Colors.amber.shade800;

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
      color: bgColor,
      child: Row(
        children: [
          Icon(urgent ? Icons.warning_amber_rounded : Icons.info_outline,
              color: Colors.white, size: 18),
          const SizedBox(width: 10),
          Expanded(
            child: Text(
              copy,
              style: const TextStyle(
                color: Colors.white,
                fontSize: 13,
                fontWeight: FontWeight.w500,
              ),
            ),
          ),
          const SizedBox(width: 8),
          SizedBox(
            height: 30,
            child: ElevatedButton(
              // Opens the upgrade modal inline instead of routing off to
              // /billing. Saves the user 2-3 clicks + a page change;
              // the modal fires checkout directly into Stripe.
              onPressed: () => showUpgradePlanModal(context),
              style: ElevatedButton.styleFrom(
                backgroundColor: Colors.white,
                foregroundColor: btnFg,
                padding: const EdgeInsets.symmetric(horizontal: 12),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(6),
                ),
              ),
              child: Text(
                t.dashboard.upgrade_btn,
                style:
                    const TextStyle(fontWeight: FontWeight.w700, fontSize: 12),
              ),
            ),
          ),
          // Dismiss is hidden when urgent — the deadline is too close to
          // let the user lose the banner.
          if (!urgent) ...[
            const SizedBox(width: 4),
            IconButton(
              onPressed: () => ref
                  .read(trialBannerDismissalProvider.notifier)
                  .dismissForToday(),
              icon: const Icon(Icons.close, color: Colors.white, size: 16),
              padding: EdgeInsets.zero,
              constraints: const BoxConstraints(minWidth: 24, minHeight: 24),
              splashRadius: 14,
            ),
          ],
        ],
      ),
    );
  }

  // Banner copy. Prefers the days-remaining framing (biggest drive-to-
  // billing CTA) and only falls back to the conversations count for
  // legacy accounts that predate the trial_expires_at column.
  String _bannerCopy() {
    final days = daysRemaining;
    if (days != null) {
      if (days <= 0) {
        return t.dashboard.trial_ended;
      }
      if (days == 1) {
        return t.dashboard.trial_last_day;
      }
      return t.dashboard.trial_days_remaining(
        days: days.toString(),
        used: conversationsUsed.toString(),
        max: conversationsMax.toString(),
      );
    }
    return t.dashboard.trial_free_plan(
      used: conversationsUsed.toString(),
      max: conversationsMax.toString(),
    );
  }
}

/// "Verify your email" banner for a logged-in but unverified user.
///
/// Stateful because the Resend button has the same three live states as the
/// post-register [EmailVerificationScreen] — idle / sending / 45s cooldown —
/// mirroring the server's 3-per-minute resend cap so the user can't spam it.
/// Not dismissible: clicking the link in the email is what removes it, and
/// the underlying [UserModel.isEmailVerified] flips on the next /me.
/// Banner for usage warnings (high token usage or exhausted).
class _UsageWarningBanner extends StatelessWidget {
  final String message;
  final Color color;

  /// When non-null, a close button is rendered that calls this callback.
  /// Only the soft 80% banner passes one; the 100% hard block omits it.
  final VoidCallback? onDismiss;

  const _UsageWarningBanner({
    required this.message,
    required this.color,
    this.onDismiss,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
      color: color,
      child: Row(
        children: [
          const Icon(Icons.warning_amber_rounded,
              color: Colors.white, size: 18),
          const SizedBox(width: 10),
          Expanded(
            child: Text(
              message,
              style: const TextStyle(
                color: Colors.white,
                fontSize: 13,
                fontWeight: FontWeight.w500,
              ),
            ),
          ),
          const SizedBox(width: 8),
          SizedBox(
            height: 30,
            child: ElevatedButton(
              // Inline upgrade: skips the /billing route and fires the
              // same modal the trial banner + billing pill open.
              onPressed: () => showUpgradePlanModal(context),
              style: ElevatedButton.styleFrom(
                backgroundColor: Colors.white,
                foregroundColor: color,
                padding: const EdgeInsets.symmetric(horizontal: 12),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(6),
                ),
              ),
              child: Text(
                t.dashboard.upgrade_btn,
                style:
                    const TextStyle(fontWeight: FontWeight.w700, fontSize: 12),
              ),
            ),
          ),
          if (onDismiss != null) ...[
            const SizedBox(width: 4),
            IconButton(
              onPressed: onDismiss,
              icon: const Icon(Icons.close, color: Colors.white, size: 16),
              padding: EdgeInsets.zero,
              constraints: const BoxConstraints(minWidth: 24, minHeight: 24),
              splashRadius: 14,
              tooltip: t.common.close,
            ),
          ],
        ],
      ),
    );
  }
}
