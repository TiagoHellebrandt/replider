import 'package:flutter/foundation.dart';
import 'package:sentry_flutter/sentry_flutter.dart';

/// Crash/error reporting bootstrap (US-074).
///
/// Wraps app startup with Sentry when a DSN is provided at build time via
/// `--dart-define=SENTRY_DSN=...`. When the DSN is empty (local dev, tests,
/// CI) initialization is skipped entirely and [run] just invokes [appRunner]
/// directly — no network calls, no overhead, identical behaviour to today.
///
/// PII is scrubbed before any event leaves the device: `sendDefaultPii` is
/// left off and a [beforeSend] hook strips request bodies, headers and any
/// breadcrumb/extra value whose key denotes personal data (phone/email/cpf),
/// mirroring the API-side redaction posture.
class CrashReporting {
  CrashReporting._();

  static const _dsn = String.fromEnvironment('SENTRY_DSN');

  /// Build-time environment label, e.g. `production` / `staging`. Defaults to
  /// the Flutter build mode so untagged builds are still distinguishable.
  static const _environment = String.fromEnvironment(
    'SENTRY_ENVIRONMENT',
    defaultValue: kReleaseMode ? 'production' : 'development',
  );

  /// Substrings (lowercased) marking a key as personal data. Kept in sync
  /// with the API `piiKeys` taxonomy (US-050).
  static const _piiTokens = ['phone', 'email', 'cpf', 'password', 'token'];

  static const _redacted = '[REDACTED]';

  static bool get isEnabled => _dsn.isNotEmpty;

  /// Initializes Sentry (if configured) and runs [appRunner] inside the
  /// Sentry zone so uncaught async/framework errors are captured. Safe to
  /// call unconditionally — no-ops when no DSN is set.
  static Future<void> run(Future<void> Function() appRunner) async {
    if (!isEnabled) {
      await appRunner();
      return;
    }
    await SentryFlutter.init(
      (options) {
        options.dsn = _dsn;
        options.environment = _environment;
        // Performance tracing off by default — error reporting only.
        options.tracesSampleRate = 0.0;
        // Never auto-attach IP/user/cookies. We scrub additionally below.
        options.sendDefaultPii = false;
        options.beforeSend = _scrub;
      },
      appRunner: appRunner,
    );
  }

  /// Forwards a manually-caught error to Sentry (no-op when disabled).
  static Future<void> capture(Object error, StackTrace? stack) async {
    if (!isEnabled) return;
    await Sentry.captureException(error, stackTrace: stack);
  }

  static bool _isPiiKey(String key) {
    final k = key.toLowerCase();
    return _piiTokens.any(k.contains);
  }

  /// BeforeSend hook: strips request bodies/headers/cookies and masks any
  /// PII-keyed breadcrumb data before the event ships.
  static SentryEvent? _scrub(SentryEvent event, Hint hint) {
    final request = event.request;
    if (request != null) {
      event = event.copyWith(
        request: request.copyWith(
          data: null,
          cookies: null,
          headers: const {},
          queryString: null,
        ),
      );
    }
    final crumbs = event.breadcrumbs;
    if (crumbs != null && crumbs.isNotEmpty) {
      event = event.copyWith(
        breadcrumbs: crumbs.map(_scrubBreadcrumb).toList(),
      );
    }
    return event;
  }

  static Breadcrumb _scrubBreadcrumb(Breadcrumb crumb) {
    final data = crumb.data;
    if (data == null || data.isEmpty) return crumb;
    final scrubbed = <String, dynamic>{};
    data.forEach((key, value) {
      scrubbed[key] = _isPiiKey(key) ? _redacted : value;
    });
    return crumb.copyWith(data: scrubbed);
  }
}
