import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/owner/presentation/controllers/owner_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Format the remaining time until [expiresAt] for the impersonation
/// banner. Under 5min we switch to mm:ss precision so the operator
/// watches the session wind down — "expires in 1m" frozen for 59
/// seconds is alarming when the JWT actually has 3 seconds left.
@visibleForTesting
String formatBannerRemaining(DateTime expiresAt, {DateTime? now}) {
  final reference = now ?? DateTime.now();
  final diff = expiresAt.difference(reference);
  if (diff.isNegative) return '0:00';
  final totalSeconds = diff.inSeconds;
  if (totalSeconds < 300) {
    final m = totalSeconds ~/ 60;
    final s = totalSeconds % 60;
    return '$m:${s.toString().padLeft(2, '0')}';
  }
  final minutes = diff.inMinutes;
  if (minutes >= 60) {
    final h = minutes ~/ 60;
    final m = minutes % 60;
    return '${h}h${m > 0 ? '${m}m' : ''}';
  }
  return '${minutes}m';
}

/// Banner switches to a brighter red and 1s tick cadence under 60s left.
@visibleForTesting
bool isBannerUrgent(DateTime expiresAt, {DateTime? now}) =>
    expiresAt.difference(now ?? DateTime.now()).inSeconds <= 60;

/// Renders a thin red strip across the top of the dashboard whenever
/// an impersonation session is active. Stays out of the way otherwise
/// (returns SizedBox.shrink). Mounted from the home shell so the
/// operator sees it on every screen during the session.
///
/// Stateful so the countdown can tick without rebuilding the whole
/// provider tree. The session itself is read off Riverpod; only the
/// "X minutes left" string is local.
class ImpersonationBanner extends ConsumerStatefulWidget {
  const ImpersonationBanner({super.key});

  @override
  ConsumerState<ImpersonationBanner> createState() =>
      _ImpersonationBannerState();
}

class _ImpersonationBannerState extends ConsumerState<ImpersonationBanner> {
  Timer? _ticker;
  bool _fastTick = false;

  @override
  void initState() {
    super.initState();
    _scheduleTicker(fast: false);
  }

  void _scheduleTicker({required bool fast}) {
    _ticker?.cancel();
    _fastTick = fast;
    _ticker = Timer.periodic(
      Duration(seconds: fast ? 1 : 30),
      (_) {
        if (!mounted) return;
        setState(() {});
        // Re-evaluate cadence each tick — once we cross under 2min the
        // countdown needs second precision so the operator sees the
        // session winding down rather than a frozen "1m" until expiry.
        _maybeAdaptCadence();
      },
    );
  }

  void _maybeAdaptCadence() {
    final s = ref.read(impersonationControllerProvider).maybeWhen(
          data: (v) => v,
          orElse: () => null,
        );
    if (s == null) {
      if (_fastTick) _scheduleTicker(fast: false);
      return;
    }
    final secondsLeft = s.expiresAt.difference(DateTime.now()).inSeconds;
    final shouldFast = secondsLeft <= 120;
    if (shouldFast != _fastTick) _scheduleTicker(fast: shouldFast);
  }

  @override
  void dispose() {
    _ticker?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final session = ref.watch(impersonationControllerProvider);
    // Server-truth fallback: if /me reports the JWT is impersonate but
    // the local storage slot was wiped (devtools / new tab), still
    // render the banner so the operator can't miss the session.
    // Account name comes from the cached user when storage is empty.
    final auth = ref.watch(authControllerProvider);
    final serverImpersonating = auth.maybeWhen(
      data: (u) => u != null && u.isImpersonating,
      orElse: () => false,
    );
    return session.maybeWhen(
      data: (s) {
        if (s == null && !serverImpersonating) return const SizedBox.shrink();
        final String accountName = s?.accountName ??
            auth.maybeWhen<String?>(
              data: (u) => u?.email,
              orElse: () => null,
            ) ??
            '?';
        final urgent = s != null && isBannerUrgent(s.expiresAt);
        final countdown = s != null
            ? ' · ${t.owner.banner_expires_in(time: formatBannerRemaining(s.expiresAt))}'
            : '';
        return Material(
          color: urgent
              ? colors.redAlert
              : colors.redAlert.withValues(alpha: 0.92),
          child: SafeArea(
            bottom: false,
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
              child: Row(
                children: [
                  const Icon(Icons.warning_amber_rounded,
                      color: Colors.white, size: 18),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      '${t.owner.banner_acting_as(account: accountName)}$countdown',
                      style: const TextStyle(
                        color: Colors.white,
                        fontSize: 12,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),
                  TextButton(
                    onPressed: () async {
                      // Clears the impersonation slot AND drops the
                      // dashboard data caches loaded under the client's
                      // token, so /home refetches as the operator. Calling
                      // ImpersonationController.end() alone left the stale
                      // client data on screen (looked like it never exited).
                      await ref
                          .read(authControllerProvider.notifier)
                          .endImpersonation();
                      if (context.mounted) context.go('/home');
                    },
                    style: TextButton.styleFrom(foregroundColor: Colors.white),
                    child: Text(t.owner.banner_end_cta),
                  ),
                ],
              ),
            ),
          ),
        );
      },
      orElse: () => const SizedBox.shrink(),
    );
  }
}
