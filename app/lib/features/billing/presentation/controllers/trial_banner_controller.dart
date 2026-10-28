import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'trial_banner_controller.g.dart';

/// Tracks whether the trial countdown banner was dismissed for the current
/// calendar day. Persisted locally via [FlutterSecureStorage] — the app's
/// KV-pref mechanism (see locale_bootstrap.dart; the Drift db carries no
/// tables) — so a dismissal survives reloads but the banner re-surfaces the
/// next day, keeping the upgrade nudge alive without nagging within a day.
@Riverpod(keepAlive: true)
class TrialBannerDismissal extends _$TrialBannerDismissal {
  static const _storageKey = 'trial_banner_dismissed_date';
  static const _storage = FlutterSecureStorage();

  @override
  Future<bool> build() async {
    try {
      final stored = await _storage.read(key: _storageKey);
      return stored == _today();
    } catch (_) {
      // Storage failure is non-fatal — default to showing the banner.
      return false;
    }
  }

  /// Hides the banner for the rest of today and persists the choice.
  Future<void> dismissForToday() async {
    state = const AsyncValue.data(true);
    try {
      await _storage.write(key: _storageKey, value: _today());
    } catch (_) {
      // Non-fatal: in-memory state already hides it for this session.
    }
  }

  // Date-only key (yyyy-MM-dd) so the dismissal resets at local midnight.
  static String _today() {
    final now = DateTime.now();
    final m = now.month.toString().padLeft(2, '0');
    final d = now.day.toString().padLeft(2, '0');
    return '${now.year}-$m-$d';
  }
}
