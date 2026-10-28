import 'package:dio/dio.dart';
import 'package:flutter/widgets.dart';
import 'package:go_router/go_router.dart';

/// Watches every response for the two payment-required signals the API
/// returns when the account can't use paid endpoints:
///
///   - 402 + code `TRIAL_EXPIRED`          — free trial window is over
///   - 402 + code `SUBSCRIPTION_INACTIVE`  — Stripe marked it past_due/canceled
///
/// We push the user to `/billing` on a 402, then enter a 5s cooldown.
/// Within that window further 402s (a burst of background requests all
/// failing at once) do NOT re-navigate — that would yank the user back
/// every frame and trap them. After the cooldown elapses a fresh 402
/// redirects again, so the paywall re-triggers per real payment-required
/// event instead of only once per session. The previous single-shot
/// `_hasRedirectedOnce` flag let the user roam permanently after dismissing
/// billing once, which meant a still-expired account never saw the paywall
/// again. Subsequent 402s also surface their error to the caller, which the
/// per-feature UI turns into an inline "Plan expired — upgrade to continue"
/// banner with the upgrade CTA.
///
/// We also never redirect when the user is already inside a route that
/// is intentionally reachable on a dead trial (`/billing`, `/settings`,
/// `/profile`). Otherwise opening Settings would 402 on `/agents`,
/// redirect to /billing, and the user could never reach Profile or
/// Logout.
///
/// The navigator key is injected (not looked up via
/// `GoRouter.of(context)`) because this runs outside the widget tree.
class BillingRedirectInterceptor extends Interceptor {
  BillingRedirectInterceptor(this._rootNavigatorKey, {DateTime Function()? now})
      : _now = now ?? DateTime.now;

  final GlobalKey<NavigatorState> _rootNavigatorKey;
  final DateTime Function() _now;

  /// Cooldown between redirects. Absorbs the burst of simultaneous 402s
  /// from parallel background requests without re-navigating each one.
  static const Duration _cooldown = Duration(seconds: 5);

  DateTime? _lastRedirectAt;

  @override
  void onError(DioException err, ErrorInterceptorHandler handler) {
    final res = err.response;
    if (res != null &&
        res.statusCode == 402 &&
        res.data is Map<String, dynamic>) {
      final code = (res.data as Map<String, dynamic>)['code'] as String?;
      if (code == 'TRIAL_EXPIRED' || code == 'SUBSCRIPTION_INACTIVE') {
        _maybeRedirectToBilling();
      }
    }
    handler.next(err);
  }

  void _maybeRedirectToBilling() {
    final last = _lastRedirectAt;
    if (last != null && _now().difference(last) < _cooldown) return;

    final ctx = _rootNavigatorKey.currentContext;
    if (ctx == null) return;

    // Resolve the router once, here, while the context is alive. Re-looking
    // it up inside the post-frame callback risks "deactivated widget" if the
    // tree rebuilt in between.
    final router = GoRouter.of(ctx);

    // Skip if the user is already inside a route they may have
    // navigated to deliberately on a dead trial. No cooldown stamp here:
    // leaving these routes on a later 402 should redirect immediately.
    final currentLocation =
        router.routerDelegate.currentConfiguration.uri.toString();
    if (currentLocation.startsWith('/billing') ||
        currentLocation.startsWith('/settings') ||
        currentLocation.startsWith('/profile')) {
      return;
    }

    // `onError` runs in the Dio event-loop callback, never synchronously
    // inside a widget build, so navigating here is safe — no post-frame
    // hop needed (and a post-frame callback wouldn't fire reliably while
    // the tree is idle, since nothing else schedules a frame).
    router.go('/billing');
    _lastRedirectAt = _now();
  }
}
