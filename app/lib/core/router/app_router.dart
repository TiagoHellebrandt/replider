import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import 'package:app/core/auth/account_role.dart';
import 'package:app/core/auth/session_expiry.dart';
import 'package:app/core/router/app_shell.dart';
import 'package:app/core/router/not_found_screen.dart';
import 'package:app/features/sales/presentation/screens/funnel_branch_screen.dart';
import 'package:app/features/home/presentation/screens/start_screen.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/auth/presentation/screens/login_screen.dart';
import 'package:app/features/auth/presentation/screens/register_screen.dart';
import 'package:app/features/auth/presentation/screens/forgot_password_screen.dart';
import 'package:app/features/auth/presentation/screens/reset_password_screen.dart';
import 'package:app/features/auth/presentation/screens/email_verification_screen.dart';
import 'package:app/features/auth/presentation/screens/email_verify_token_screen.dart';
import 'package:app/features/dashboard/presentation/screens/dashboard_screen.dart';
import 'package:app/features/settings/presentation/screens/brand_kit_screen.dart';
import 'package:app/features/sales/presentation/screens/portfolio_customer_screen.dart';
import 'package:app/features/sales/presentation/screens/proposal_screen.dart';
import 'package:app/features/sales/presentation/screens/relationship_screen.dart';
import 'package:app/features/settings/presentation/screens/email_suppressions_screen.dart';
import 'package:app/features/settings/presentation/screens/settings_screen.dart';
import 'package:app/features/settings/presentation/screens/account_screen.dart';
import 'package:app/features/settings/presentation/screens/profile_screen.dart';
import 'package:app/features/settings/presentation/screens/channels_screen.dart';
import 'package:app/features/settings/presentation/screens/claude_integration_screen.dart';
import 'package:app/features/settings/presentation/screens/whatsapp_settings_screen.dart';
import 'package:app/features/lgpd/presentation/screens/compliance_screen.dart';
import 'package:app/features/owner/presentation/screens/owner_account_detail_screen.dart';
import 'package:app/features/owner/presentation/screens/owner_accounts_screen.dart';
import 'package:app/features/support/presentation/screens/support_screen.dart';
import 'package:app/features/owner/presentation/screens/owner_create_contract_screen.dart';
import 'package:app/features/owner/presentation/screens/owner_provider_costs_screen.dart';
import 'package:app/features/billing/presentation/screens/billing_screen.dart';
import 'package:app/features/onboarding/presentation/screens/onboarding_screen.dart';
import 'package:app/features/settings/presentation/screens/team_screen.dart';
import 'package:app/features/sales/presentation/screens/products_screen.dart';
import 'package:app/features/sales/presentation/screens/product_form_screen.dart';
import 'package:app/features/sales/presentation/screens/customer_import_screen.dart';
import 'package:app/features/sales/presentation/screens/sales_import_screen.dart';
import 'package:app/features/sales/presentation/screens/product_import_screen.dart';
import 'package:app/features/sales/presentation/screens/funnels_screen.dart';
import 'package:app/features/sales/presentation/screens/funnel_config_screen.dart';
import 'package:app/features/copilot/presentation/screens/copilot_screen.dart';
import 'package:app/features/sales/presentation/screens/prospect_branch_screen.dart';

part 'app_router.g.dart';

/// Returns the canonical, slash-stripped location (query preserved) when [uri]
/// has a redundant trailing slash on a non-root path, else null (no redirect).
/// Keeps deep links like `/reset-password/?token=…` from falling through to the
/// login screen because the slashed path isn't in the route table / allowlist.
String? canonicalTrailingSlashRedirect(Uri uri) {
  final path = uri.path;
  if (path.length > 1 && path.endsWith('/')) {
    final stripped = path.substring(0, path.length - 1);
    return uri.query.isEmpty ? stripped : '$stripped?${uri.query}';
  }
  return null;
}

const _publicRoutes = {
  '/',
  '/register',
  '/forgot-password',
  '/reset-password',
  '/verify-email'
};

/// Where a fresh login lands, by role.
///
/// `/start` (the command center) requires the vendedor role server-side and
/// answers 403 to an attendant — so sending every login there put every
/// attendant on a RetryableError as their first screen of the day. The
/// attendant's home is the inbox.
String landingFor(String? role) =>
    AccountRole.from(role) == AccountRole.attendant ? '/home' : '/start';

/// Shared navigator key so interceptors living outside the widget tree
/// (dio, websocket, …) can still push routes. GoRouter picks this up
/// via its `navigatorKey` argument below.
final rootNavigatorKey = GlobalKey<NavigatorState>();

@Riverpod(keepAlive: true)
GoRouter appRouter(Ref ref) {
  // Intentionally NOT ref.watch'ing the auth state here. Watching it
  // would rebuild this provider on every auth mutation (e.g. the
  // updateUserInState call after completeOnboarding), returning a new
  // GoRouter instance — MaterialApp.router then swaps router mid-
  // navigation and the context.go('/home') triggered alongside the
  // state change is silently discarded. Instead the redirect reads
  // the current auth state via ref.read at evaluation time, and
  // _GoRouterAuthNotifier triggers re-evaluation via refreshListenable
  // when auth state flips. keepAlive ensures the single instance
  // lives for the whole app session.
  return GoRouter(
    initialLocation: '/',
    navigatorKey: rootNavigatorKey,
    refreshListenable: _GoRouterAuthNotifier(ref),
    // Without this, an unmatched URL renders GoRouter's own debug page —
    // customers were being shown `GoException: no routes for location: …` in
    // English.
    errorBuilder: (context, state) =>
        NotFoundScreen(location: state.uri.toString()),
    redirect: (context, state) {
      // Normalize a trailing slash (e.g. /reset-password/?token=… from a
      // pasted/forwarded link) to the canonical path, preserving the query.
      // Without this the slashed path matches neither the route table nor the
      // public-route allowlist and falls through to '/', dumping the user on
      // login instead of the deep-linked screen. Runs first; the stripped path
      // has no trailing slash so it can't loop.
      final canonical = canonicalTrailingSlashRedirect(state.uri);
      if (canonical != null) return canonical;

      final authState = ref.read(authControllerProvider);

      // Cold-load deep links land here while authState is still in
      // `loading` (getCurrentUser is async). Returning null keeps the
      // requested URL on screen — refreshListenable re-runs this redirect
      // once auth resolves, by which point the rules below evaluate
      // against real user data instead of treating the user as logged out.
      if (authState.isLoading) return null;

      final user = authState.whenOrNull(data: (user) => user);
      final isLoggedIn = user != null;
      final isPublicRoute = _publicRoutes.contains(state.matchedLocation);

      // Being bounced to login must not cost the person their place. The
      // requested location rides along as `?from=` so that, once they sign in
      // again, they land back on the screen they were using instead of on the
      // dashboard wondering where their work went. Carried in the URL rather
      // than in a provider so it survives the reload that a web user reaches
      // for when the app surprises them.
      if (!isLoggedIn && !isPublicRoute) {
        final from = state.uri.toString();
        return '/?from=${Uri.encodeComponent(from)}';
      }
      if (isLoggedIn && isPublicRoute) {
        // A signed-in browser clicking the emailed verification or
        // password-reset link must still CONSUME the token. Bouncing these
        // to the landing screen left the e-mail unverified and the password
        // unchanged, with no message — the link just "didn't work".
        final hasToken = (state.uri.queryParameters['token'] ?? '').isNotEmpty;
        if (hasToken &&
            (state.matchedLocation == '/verify-email' ||
                state.matchedLocation == '/reset-password')) {
          return null;
        }
        // The post-signup "check your inbox" screen (?email=, no token).
        // Registration auto-logs the user in, so without this escape the
        // redirect swallowed the screen — the resend button and the nudge
        // were dead code for every new account.
        if (state.matchedLocation == '/verify-email' &&
            (state.uri.queryParameters['email'] ?? '').isNotEmpty) {
          return null;
        }
        if (user.isNew) return '/onboarding';
        return sanitizeReturnPath(
              Uri.decodeComponent(state.uri.queryParameters['from'] ?? ''),
            ) ??
            landingFor(user.role);
      }
      // A brand-new account belongs in onboarding no matter which URL it
      // arrives on. Guarding only /home let an F5 on /start — the actual
      // landing screen — skip the wizard entirely.
      //
      // Exception: the Gmail OAuth callback redirects the browser to
      // /settings/channels?gmail=… — bouncing that to /onboarding swallowed
      // the success/failure message and left the person guessing whether
      // the mailbox connected. The channels screen shows the toast; the
      // wizard's own card re-checks on "Já conectei".
      //
      // Same for /billing: a trial that expired mid-wizard gets a 402 from
      // the very calls the wizard makes, the billing interceptor sends the
      // person to /billing to pay, and this guard used to send them straight
      // back to /onboarding — a ping-pong with no way to pay.
      if (isLoggedIn && user.isNew && state.matchedLocation != '/onboarding') {
        final gmailCallback = state.matchedLocation == '/settings/channels' &&
            (state.uri.queryParameters['gmail'] ?? '').isNotEmpty;
        final billing = state.matchedLocation.startsWith('/billing');
        if (!gmailCallback && !billing) return '/onboarding';
      }
      // And the inverse: a finished account re-opening /onboarding by URL
      // has nothing to do there.
      if (isLoggedIn && !user.isNew && state.matchedLocation == '/onboarding') {
        return landingFor(user.role);
      }
      // Team management is admin-only on the server; a deep link from a
      // non-admin lands on the landing screen instead of a 403 error page.
      if (isLoggedIn &&
          state.matchedLocation == '/settings/team' &&
          !roleIsAdmin(user.role)) {
        return landingFor(user.role);
      }
      return null;
    },
    routes: [
      GoRoute(
        path: '/',
        builder: (context, state) => const LoginScreen(),
      ),
      GoRoute(
        path: '/register',
        builder: (context, state) => RegisterScreen(
          couponCode: state.uri.queryParameters['coupon'],
          inviteToken: state.uri.queryParameters['invite'],
        ),
      ),
      GoRoute(
        path: '/forgot-password',
        builder: (context, state) => const ForgotPasswordScreen(),
      ),
      GoRoute(
        path: '/reset-password',
        builder: (context, state) => ResetPasswordScreen(
          token: state.uri.queryParameters['token'] ?? '',
        ),
      ),
      GoRoute(
        path: '/verify-email',
        builder: (context, state) {
          // Two modes share this route:
          //  - ?token=...  → the emailed verification link; consume it.
          //  - ?email=...  → post-register "check your inbox" screen.
          final token = state.uri.queryParameters['token'] ?? '';
          if (token.isNotEmpty) {
            return EmailVerifyTokenScreen(token: token);
          }
          final email = state.uri.queryParameters['email'] ?? '';
          // Basic validation - don't crash on bad input
          final validEmail =
              RegExp(r'^[^@]+@[^@]+\.[^@]+$').hasMatch(email) ? email : '';
          return EmailVerificationScreen(email: validEmail);
        },
      ),
      GoRoute(
        path: '/onboarding',
        builder: (context, state) => const OnboardingScreen(),
      ),
      // US-M016: the persistent shell. Four branches, each with its own
      // Navigator, so switching tabs preserves scroll position and stack.
      //
      // `/home` deliberately stays the CONVERSATIONS branch: ~20 call sites
      // (composer prefill, push deep links, seller co-pilot) already
      // `go('/home')` expecting the inbox, and silently retargeting them at
      // a new landing screen would break every one of those flows. The new
      // landing screen is `/start`, and post-login lands there.
      StatefulShellRoute.indexedStack(
        builder: (context, state, navigationShell) =>
            AppShell(navigationShell: navigationShell),
        branches: [
          StatefulShellBranch(
            routes: [
              GoRoute(
                path: '/start',
                builder: (context, state) => const StartScreen(),
              ),
            ],
          ),
          StatefulShellBranch(
            routes: [
              GoRoute(
                path: '/funnel',
                // The Kanban IS the funnel destination (US-M017/M018); the
                // funnel LIST stays at /sales/funnels for configuration.
                // ?lead= deep-links straight to one lead's detail — the
                // inbox chip's destination (US-M024).
                builder: (context, state) => FunnelBranchScreen(
                  openLeadId: state.uri.queryParameters['lead'],
                ),
              ),
            ],
          ),
          StatefulShellBranch(
            routes: [
              GoRoute(
                path: '/home',
                // `?conversation=<id>` pre-selects a thread: the push
                // notification's click_url (see push_dispatch.go) and any
                // shared link land on the conversation, not just the inbox.
                builder: (context, state) => DashboardScreen(
                  initialConversationId:
                      state.uri.queryParameters['conversation'],
                ),
              ),
            ],
          ),
          StatefulShellBranch(
            routes: [
              GoRoute(
                path: '/prospect',
                // Hunting and approving are one sitting (US-M021); ?tab=queue
                // deep-links to the backlog.
                builder: (context, state) => ProspectBranchScreen(
                  initialTab: state.uri.queryParameters['tab'],
                ),
              ),
            ],
          ),
        ],
      ),
      // The in-app copilot left the shell and the account menu on 2026-09-01:
      // the MCP connector does the same job from Claude, and a chat is not a
      // place. The route stays reachable BY URL ONLY for 30 days (until
      // 2026-10-01) so a bookmark still answers; then delete it with
      // `copilot_screen.dart`. `?prompt=` seeds the composer.
      GoRoute(
        path: '/copilot',
        builder: (context, state) => CopilotScreen(
          initialPrompt: state.uri.queryParameters['prompt'],
        ),
      ),
      GoRoute(
        path: '/settings',
        builder: (context, state) => const SettingsScreen(),
      ),
      GoRoute(
        path: '/settings/account',
        builder: (context, state) => const AccountScreen(),
      ),
      GoRoute(
        path: '/settings/email-suppressions',
        builder: (context, state) => const EmailSuppressionsScreen(),
      ),
      GoRoute(
        path: '/settings/brand',
        builder: (context, state) => const BrandKitScreen(),
      ),
      GoRoute(
        path: '/settings/profile',
        builder: (context, state) => const ProfileScreen(),
      ),
      GoRoute(
        path: '/settings/channels',
        builder: (context, state) => const ChannelsScreen(),
      ),
      GoRoute(
        path: '/settings/channels/:id/whatsapp',
        builder: (context, state) => WhatsAppSettingsScreen(
          channelId: state.pathParameters['id']!,
        ),
      ),
      GoRoute(
        path: '/settings/claude',
        builder: (_, __) => const ClaudeIntegrationScreen(),
      ),
      GoRoute(
        path: '/settings/compliance',
        builder: (_, __) => const ComplianceScreen(),
      ),
      GoRoute(
        path: '/settings/team',
        builder: (context, state) => const TeamScreen(),
      ),
      // The customer list lives in Clientes › Carteira since 2026-09-01;
      // `/contacts` was a fourth list of the same people. Redirect kept for
      // 30 days (until 2026-10-01) for saved links, like the `/sales/*` ones.
      GoRoute(
        path: '/contacts',
        redirect: (_, __) => '/funnel?view=portfolio',
      ),
      // Kept next to the list it fills for the same reason the server keeps
      // them together: this is the only way a customer base gets filled
      // without waiting for each customer to write first.
      GoRoute(
        path: '/contacts/import',
        builder: (context, state) => const CustomerImportScreen(),
      ),
      // Internal CS thread with the Replider platform (canal-interno
      // US-005) — every customer role gets their own private thread.
      GoRoute(
        path: '/support',
        // `?draft=` seeds the composer. Used by screens that can describe
        // the problem better than the user can — the DNS card knows the
        // domain, the detected provider and exactly which records are
        // missing, and making someone retype that is how a support thread
        // starts with half the facts.
        builder: (context, state) =>
            SupportScreen(draft: state.uri.queryParameters['draft']),
      ),
      // `/sales` used to be a hub listing the module's sub-sections. The shell
      // replaced it — funnel, prospecting and approvals are branches now — so
      // the hub only survived as the back-button target of the screens below
      // it, which meant "back" landed on a menu nobody navigates to on purpose.
      //
      // The route stays as a redirect because it is the one sales URL a person
      // may have bookmarked, and because a redirect costs a line while a dead
      // screen costs a maintained one.
      GoRoute(
        path: '/sales',
        redirect: (_, __) => '/funnel',
      ),
      // The sales history the account already has — the fuel the Carteira's
      // reorder dates and the Radar's cross-sell read ("vendas como fonte
      // única"). Admin-only on the server, like /contacts/import.
      GoRoute(
        path: '/sales/import',
        builder: (context, state) => const SalesImportScreen(),
      ),
      GoRoute(
        path: '/sales/products',
        builder: (context, state) => const ProductsScreen(),
      ),
      // Registered BEFORE '/sales/products/:id' so "import" is a route and not a
      // product id — same reason the server registers its own that way.
      GoRoute(
        path: '/sales/products/import',
        builder: (context, state) => const ProductImportScreen(),
      ),
      GoRoute(
        path: '/sales/products/new',
        builder: (context, state) => const ProductFormScreen(),
      ),
      GoRoute(
        path: '/sales/products/:id',
        builder: (context, state) =>
            ProductFormScreen(productId: state.pathParameters['id']),
      ),
      GoRoute(
        path: '/sales/funnels',
        builder: (context, state) => const FunnelsScreen(),
      ),
      GoRoute(
        path: '/sales/funnels/new',
        builder: (context, state) => const FunnelConfigScreen(),
      ),
      GoRoute(
        path: '/sales/funnels/:id',
        builder: (context, state) =>
            FunnelConfigScreen(funnelId: state.pathParameters['id']),
      ),
      // Public buying signals (§2.1) — the one prospecting source that
      // answers "who SAID they intend to buy, and by when".
      // The portfolio also lives as a toggle inside the funnel shell; these
      // are the deep links to it.
      // A redirect into the Clientes destination's portfolio view. It used to
      // render `PortfolioBody` in a Scaffold of its own, outside the shell —
      // the same list, in a second place, with no rail and no bottom bar.
      GoRoute(
        path: '/sales/portfolio',
        redirect: (_, __) => '/funnel?view=portfolio',
      ),
      // Same as /sales/portfolio: a view of Clientes now. The `?channel=` this
      // route used to take is gone on purpose — the view resolves the account's
      // email channel itself, because "which channel?" is not a question
      // somebody reviewing their own customers can answer.
      GoRoute(
        path: '/sales/my-customers',
        redirect: (_, __) => '/funnel?view=inbox',
      ),
      GoRoute(
        path: '/sales/portfolio/:id',
        builder: (context, state) =>
            PortfolioCustomerScreen(contactId: state.pathParameters['id']!),
      ),
      // Composing a priced offer (pillar 3). The lead rides as a query param
      // rather than in the path: the screen belongs to the OFFER, and a lead
      // with no proposal yet has no id to put there.
      GoRoute(
        path: '/sales/proposals/new',
        builder: (context, state) => ProposalScreen(
          leadId: state.uri.queryParameters['lead'] ?? '',
        ),
      ),
      GoRoute(
        // A redirect, not a screen: `SignalsBody` already renders as a tab of
        // the Prospecção destination, so the wrapper was a second copy of the
        // same work OUTSIDE the shell — no rail, no bottom bar, and a back
        // button that had to be invented for it. The address stays because
        // somebody may have it saved.
        path: '/sales/signals',
        redirect: (_, __) => '/prospect?tab=signals',
      ),
      GoRoute(
        path: '/sales/relationship',
        builder: (context, state) => const RelationshipScreen(),
      ),
      GoRoute(
        // Same as /sales/signals: `ProspectingBody` is the first tab of the
        // Prospecção destination.
        path: '/sales/prospecting',
        redirect: (_, __) => '/prospect',
      ),
      // Kept as a stable address for the approval backlog; it lives inside
      // the Prospecção destination (US-M021), so send callers there rather
      // than rendering a second copy outside the shell.
      GoRoute(
        path: '/sales/approvals',
        redirect: (context, state) => '/prospect?tab=queue',
      ),
      GoRoute(
        path: '/billing',
        builder: (context, state) => const BillingScreen(),
      ),
      // /checkout is GONE, not hidden. Sales-led means no self-checkout:
      // the 485-line transparent-checkout screen sat routed with nothing
      // navigating to it, one pasted URL away from contradicting every
      // "no price list, talk to us" surface in the product.
      GoRoute(
        path: '/owner/accounts',
        builder: (context, state) => const OwnerAccountsScreen(),
      ),
      GoRoute(
        path: '/owner/accounts/new',
        builder: (context, state) => const OwnerCreateContractScreen(),
      ),
      GoRoute(
        path: '/owner/accounts/:id',
        builder: (context, state) =>
            OwnerAccountDetailScreen(accountId: state.pathParameters['id']!),
      ),
      GoRoute(
        path: '/owner/provider-costs',
        builder: (context, state) => const OwnerProviderCostsScreen(),
      ),
    ],
  );
}

/// Bridges Riverpod's auth state changes to GoRouter's refresh mechanism.
class _GoRouterAuthNotifier extends ChangeNotifier {
  _GoRouterAuthNotifier(this._ref) {
    _ref.listen(authControllerProvider, (_, __) => notifyListeners());
  }

  final Ref _ref;
}
