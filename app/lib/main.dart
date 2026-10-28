import 'dart:async';
import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_web_plugins/url_strategy.dart';
import 'package:app/core/utils/account_currency.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/core/notifications/push_click_router.dart';
import 'package:app/core/router/app_router.dart';
import 'package:app/core/i18n/locale_bootstrap.dart';
import 'package:app/core/analytics/utm_bootstrap.dart';
import 'package:app/core/observability/crash_reporting.dart';
import 'package:app/features/owner/presentation/widgets/impersonation_banner.dart';

Future<void> main() async {
  // Run the whole app inside the Sentry zone so uncaught async/framework
  // errors are reported (US-074). No-op when SENTRY_DSN isn't set at build
  // time, so local/dev/test behaviour is unchanged.
  await CrashReporting.run(_bootstrap);
}

Future<void> _bootstrap() async {
  WidgetsFlutterBinding.ensureInitialized();

  // Path-based URLs on web (no '#'), so external deep links carry the real
  // path — e.g. the password-reset email's /reset-password?token=… is read by
  // GoRouter instead of being dropped (hash routing only sees the fragment,
  // which sent every deep link to '/' → the login screen). nginx already
  // serves index.html for any path (try_files), so hard refreshes work.
  if (kIsWeb) usePathUrlStrategy();

  // Capture utm_* from the landing URL before GoRouter rewrites it, so the
  // register flow can forward acquisition attribution to the API on signup.
  UtmBootstrap.capture();

  // Resolve locale: ?lang= query param > stored preference > device locale.
  // Async because storage read is async; safe to await before runApp.
  await LocaleBootstrap.apply();

  // Warm the money-currency cache WITHOUT awaiting it: a cold load used to
  // flash R$ at a US customer until /me answered, and a bounded await before
  // runApp is still an await — a blank first frame is the worse failure.
  unawaited(AccountCurrencyCache.warmUp());

  // Catch framework errors so a single malformed widget (e.g. a
  // message bubble with bad JSON) doesn't cascade into a red screen
  // covering the whole dashboard. In release we keep it silent; in
  // debug we still want the console dump. Also forward to Sentry when
  // enabled so the crash reaches the dashboard.
  FlutterError.onError = (details) {
    FlutterError.presentError(details);
    debugPrint('FlutterError: ${details.exceptionAsString()}');
    CrashReporting.capture(details.exception, details.stack);
  };

  // Non-widget async errors (Dio, parsing, etc.) that escape the
  // zone. Returning true marks them handled.
  WidgetsBinding.instance.platformDispatcher.onError = (error, stack) {
    debugPrint('PlatformDispatcher error: $error');
    CrashReporting.capture(error, stack);
    return true;
  };

  // Inline fallback for any widget whose build throws — shows a
  // compact muted placeholder instead of the default red box.
  ErrorWidget.builder = (details) {
    return const _InlineErrorFallback();
  };

  runApp(
    ProviderScope(
      child: TranslationProvider(
        child: const RepliderApp(),
      ),
    ),
  );
}

/// Compact muted placeholder used in place of the default red error
/// box when a widget build throws. Keeps one broken bubble from
/// poisoning the whole conversation view.
class _InlineErrorFallback extends StatelessWidget {
  const _InlineErrorFallback();

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(8),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.error_outline, size: 14, color: Colors.grey),
          const SizedBox(width: 6),
          Text(
            t.common.unreadable_content,
            style: const TextStyle(color: Colors.grey, fontSize: 12),
          ),
        ],
      ),
    );
  }
}

class RepliderApp extends ConsumerWidget {
  const RepliderApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final router = ref.watch(appRouterProvider);

    // Keeps the SW notification-click → GoRouter bridge alive so a push
    // click deep-links into the app (US-006). No-op off web.
    ref.watch(pushClickRouterProvider);

    // Records the account's currency for the next cold load (see
    // AccountCurrencyCache). Watched here for the same reason as the router
    // bridge above: it must outlive whatever screen is on top.
    ref.watch(accountCurrencyPersistenceProvider);

    return MaterialApp.router(
      title: 'Replider',
      debugShowCheckedModeBanner: false,
      routerConfig: router,
      theme: AppTheme.darkTheme,
      // Slang delegates
      locale: TranslationProvider.of(context).flutterLocale,
      supportedLocales: AppLocaleUtils.supportedLocales,
      localizationsDelegates: GlobalMaterialLocalizations.delegates,
      // Wrap every screen with the impersonation banner. The widget
      // returns SizedBox.shrink() when no session is active so the
      // overhead is one provider read per build.
      builder: (context, child) => Column(
        children: [
          const ImpersonationBanner(),
          Expanded(child: child ?? const SizedBox.shrink()),
        ],
      ),
    );
  }
}
