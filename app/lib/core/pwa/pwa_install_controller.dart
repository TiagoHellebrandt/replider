/// Riverpod state for the PWA install banner. Polls
/// `window.repliderPwa.canPromptInstall()` once a second so the UI
/// reacts to a late `beforeinstallprompt` event without complicated
/// JS-bridge subscriptions.
///
/// Uses Riverpod 3's plain Notifier (no codegen) so this lands without
/// running build_runner.
library;

import 'dart:async';

import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'pwa_install_service.dart';

class PwaInstallController extends Notifier<PwaInstallState> {
  PwaInstallService? _serviceInstance;
  Timer? _poll;

  PwaInstallService get service {
    final cached = _serviceInstance;
    if (cached != null) return cached;
    final svc = ref.read(pwaInstallServiceProvider);
    _serviceInstance = svc;
    return svc;
  }

  @override
  PwaInstallState build() {
    if (!kIsWeb) {
      return PwaInstallState.notInstallable;
    }
    final initial = service.currentState();
    // Chromium fires beforeinstallprompt asynchronously during/after
    // first paint. Poll briefly so we flip to androidPromptable as soon
    // as the deferred prompt is captured. 1s × 30 = 30s window — Chrome
    // either fired by then or won't this session.
    var ticks = 0;
    _poll?.cancel();
    _poll = Timer.periodic(const Duration(seconds: 1), (t) {
      ticks++;
      final next = service.currentState();
      if (next != state) state = next;
      // Stop polling once installed (the appinstalled event is the
      // terminal state) or after 30 ticks if nothing changed.
      if (state == PwaInstallState.installed || ticks >= 30) {
        t.cancel();
        _poll = null;
      }
    });
    ref.onDispose(() {
      _poll?.cancel();
      _poll = null;
    });
    return initial;
  }

  /// Re-checks state on demand. Called after [promptInstall] resolves
  /// and from the dashboard after a navigation event so a freshly-
  /// installed app flips the banner off without a full reload.
  void refresh() {
    if (!kIsWeb) return;
    state = service.currentState();
  }

  /// Triggers the native Android install dialog and refreshes state
  /// based on the user's choice. Returns true when the user accepted
  /// (so callers can show a "thanks" snackbar or unlock the iOS push
  /// flow).
  Future<bool> promptInstall() async {
    final outcome = await service.promptInstall();
    refresh();
    return outcome == 'accepted';
  }
}

final pwaInstallServiceProvider = Provider<PwaInstallService>((ref) {
  return PwaInstallService();
});

final pwaInstallControllerProvider =
    NotifierProvider<PwaInstallController, PwaInstallState>(
  PwaInstallController.new,
);
