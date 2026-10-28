/// Detects whether the app is running as an installed PWA and exposes
/// platform-aware install actions. Wraps `window.repliderPwa` (defined
/// in web/push-client.js); resolves to a stub on every non-web target.
///
/// Conditional imports keep dart:js_util out of mobile builds. Mobile
/// always reports `notInstallable` because the mobile build IS the
/// native shell — Replider doesn't ship a Flutter Android/iOS app, the
/// PWA is the mobile experience.
library;

import 'pwa_install_service_stub.dart'
    if (dart.library.js_interop) 'pwa_install_service_web.dart' as impl;

/// Operating systems we differentiate for install-flow UX. Non-web
/// builds always report `other`.
enum PwaOs { ios, android, other }

/// User-facing install state. Drives the install banner: we show it on
/// mobile when the app isn't installed and the platform supports
/// installation (Android Chrome with deferred prompt, iOS Safari with
/// Add-to-Home-Screen instructions).
enum PwaInstallState {
  /// Not running on web, or platform can't install — hide the banner.
  notInstallable,

  /// Installable on Android via the native deferred prompt.
  androidPromptable,

  /// Installable on iOS via manual Add-to-Home-Screen — show
  /// instructions because iOS Safari intentionally never fires the
  /// `beforeinstallprompt` event.
  iosManual,

  /// Already installed (display-mode: standalone OR navigator.standalone).
  installed,
}

abstract class PwaInstallService {
  factory PwaInstallService() => impl.createPwaInstallService();

  /// True when the app is running as an installed PWA (Android home-
  /// screen launch OR iOS standalone).
  bool get isStandalone;

  /// True when the user agent is iOS or Android. Drives whether we
  /// should bother surfacing an install banner at all.
  bool get isMobile;

  PwaOs get os;

  /// Only true on iOS PWA: tells the push controller whether the OS
  /// will actually deliver Web Push. iOS Safari exposes Notification
  /// + PushManager inside an installed PWA only (16.4+).
  bool get iosPushAvailable;

  /// One-shot snapshot of the install state. Designed to be cheap so
  /// callers can poll on mount without a stream wrapper.
  PwaInstallState currentState();

  /// Invokes the native Android install dialog. Returns 'accepted' /
  /// 'dismissed' or null when no deferred prompt is available (iOS
  /// Safari, already installed, prompt consumed). Safe to call when
  /// [currentState] is not `androidPromptable` — it just no-ops.
  Future<String?> promptInstall();
}
