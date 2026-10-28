import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'onboarding_checklist_dismissal_controller.g.dart';

/// Locally persisted twin of the server-side 24h checklist dismissal
/// (`POST /me/onboarding-checklist/dismiss`). The card consults BOTH this
/// local flag AND [OnboardingProgressModel.isDismissActive] so a dismissal
/// survives a page reload immediately, without waiting for the next
/// `/me/onboarding-progress` round-trip to echo `checklist_dismissed_at`
/// back (US-112).
///
/// Persisted via [FlutterSecureStorage] — the app's KV-pref mechanism (the
/// Drift db carries no tables; see app/CLAUDE.md). The stored value is an
/// ISO-8601 timestamp; [build] returns true while it is younger than 24h,
/// mirroring the server window so local and server expire together.
@Riverpod(keepAlive: true)
class OnboardingChecklistDismissal extends _$OnboardingChecklistDismissal {
  static const _storageKey = 'onboarding_checklist_dismissed_at';

  /// The dismissal window. Overridable in tests so the auto-return behaviour
  /// can be exercised in milliseconds instead of waiting 24h.
  @visibleForTesting
  static Duration window = const Duration(hours: 24);

  static const _storage = FlutterSecureStorage();

  Timer? _expiry;

  @override
  Future<bool> build() async {
    // This provider is keepAlive, so build() runs ONCE and the boolean is
    // cached for the whole session. Without an explicit timer the dismissed
    // card would never return in a long-lived tab even after the window
    // closes (US-115). Cancel any pending timer when the provider is torn
    // down to avoid mutating a disposed notifier.
    ref.onDispose(() => _expiry?.cancel());
    try {
      final stored = await _storage.read(key: _storageKey);
      if (stored == null) return false;
      final at = DateTime.tryParse(stored);
      if (at == null) return false;
      final elapsed = DateTime.now().difference(at);
      if (elapsed >= window) return false;
      // Still dismissed — schedule the card to reappear when the remaining
      // window elapses, without requiring a page reload.
      _scheduleReturn(window - elapsed);
      return true;
    } catch (_) {
      // Storage failure is non-fatal — default to showing the checklist.
      return false;
    }
  }

  /// Hides the checklist locally for the next [window] and persists the choice.
  Future<void> dismiss() async {
    state = const AsyncValue.data(true);
    _scheduleReturn(window);
    try {
      await _storage.write(
        key: _storageKey,
        value: DateTime.now().toIso8601String(),
      );
    } catch (_) {
      // Non-fatal: in-memory state already hides it for this session.
    }
  }

  /// Flips the dismissal back off once [remaining] elapses so the checklist
  /// returns automatically (gated until completion hits 100% by the card).
  void _scheduleReturn(Duration remaining) {
    _expiry?.cancel();
    _expiry = Timer(remaining, () {
      state = const AsyncValue.data(false);
    });
  }
}
