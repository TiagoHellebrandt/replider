/// Web implementation of [PwaInstallService] — proxies to the
/// `window.repliderPwa` shim defined in web/push-client.js. All JS
/// interop ceremony stays in this file so the rest of the app can
/// import the abstract API without dart:js_util.
library;

import 'dart:js_interop';

import 'pwa_install_service.dart';

PwaInstallService createPwaInstallService() => _WebPwaInstallService();

@JS('repliderPwa')
external _RepliderPwaJs? get _repliderPwa;

@JS()
@staticInterop
class _RepliderPwaJs {}

extension _RepliderPwaJsExt on _RepliderPwaJs {
  external bool isStandalone();
  external String os();
  external bool isMobile();
  external bool iosPushAvailable();
  external bool canPromptInstall();
  external JSPromise<JSAny?> promptInstall();
}

class _WebPwaInstallService implements PwaInstallService {
  @override
  bool get isStandalone {
    final shim = _repliderPwa;
    if (shim == null) return false;
    try {
      return shim.isStandalone();
    } catch (_) {
      // shim was overwritten or method threw — fail closed (we'd
      // rather show the install banner once too often than accidentally
      // hide it after the user installed the app).
      return false;
    }
  }

  @override
  bool get isMobile {
    final shim = _repliderPwa;
    if (shim == null) return false;
    try {
      return shim.isMobile();
    } catch (_) {
      return false;
    }
  }

  @override
  PwaOs get os {
    final shim = _repliderPwa;
    if (shim == null) return PwaOs.other;
    try {
      switch (shim.os()) {
        case 'ios':
          return PwaOs.ios;
        case 'android':
          return PwaOs.android;
        default:
          return PwaOs.other;
      }
    } catch (_) {
      return PwaOs.other;
    }
  }

  @override
  bool get iosPushAvailable {
    final shim = _repliderPwa;
    if (shim == null) return false;
    try {
      return shim.iosPushAvailable();
    } catch (_) {
      return false;
    }
  }

  @override
  PwaInstallState currentState() {
    if (isStandalone) return PwaInstallState.installed;
    if (!isMobile) return PwaInstallState.notInstallable;
    final shim = _repliderPwa;
    final canPrompt = shim != null && shim.canPromptInstall();
    if (canPrompt) return PwaInstallState.androidPromptable;
    if (os == PwaOs.ios) return PwaInstallState.iosManual;
    // Android without a deferred prompt yet — Chrome may still fire
    // beforeinstallprompt later, but right now we have nothing actionable
    // to surface. Treat the same as not-installable so the banner
    // doesn't render an empty CTA.
    return PwaInstallState.notInstallable;
  }

  @override
  Future<String?> promptInstall() async {
    final shim = _repliderPwa;
    if (shim == null) return null;
    try {
      final raw = await shim.promptInstall().toDart;
      return (raw?.dartify() as String?);
    } catch (_) {
      return null;
    }
  }
}
