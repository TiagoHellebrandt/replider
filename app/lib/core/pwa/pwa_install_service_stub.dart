/// Mobile / desktop-Flutter stub for PwaInstallService. The Replider
/// "mobile app" IS the installed PWA — no Flutter native shell ships
/// today — so this stub always reports notInstallable. Kept as a small
/// no-op so feature code can reference PwaInstallService without
/// platform guards on every call site.
library;

import 'pwa_install_service.dart';

PwaInstallService createPwaInstallService() => _StubPwaInstallService();

class _StubPwaInstallService implements PwaInstallService {
  @override
  bool get isStandalone => false;

  @override
  bool get isMobile => false;

  @override
  PwaOs get os => PwaOs.other;

  @override
  bool get iosPushAvailable => false;

  @override
  PwaInstallState currentState() => PwaInstallState.notInstallable;

  @override
  Future<String?> promptInstall() async => null;
}
