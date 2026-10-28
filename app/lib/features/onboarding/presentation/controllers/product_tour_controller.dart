import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'product_tour_controller.g.dart';

/// Tracks whether the one-shot in-app product tour has already been shown
/// (US-183). Persisted locally via [FlutterSecureStorage] — the app's KV-pref
/// mechanism, same as the banner/NPS dismissals (the Drift db carries no
/// tables) — so the walkthrough surfaces exactly once per account/device and
/// the choice survives reloads.
///
/// `build()` returns `true` when the tour was already completed or skipped
/// (i.e. it should stay hidden). Unlike the NPS prompt this is a lifetime
/// one-shot, not period-based, so [complete] simply stamps a sticky flag.
@Riverpod(keepAlive: true)
class ProductTourController extends _$ProductTourController {
  static const _storageKey = 'product_tour_completed';
  static const _storage = FlutterSecureStorage();

  @override
  Future<bool> build() async {
    try {
      final stored = await _storage.read(key: _storageKey);
      return stored == 'true';
    } catch (_) {
      // Storage failure is non-fatal — default to NOT showing the tour so a
      // flaky keystore can't replay the walkthrough on every dashboard paint.
      return true;
    }
  }

  /// Marks the tour as finished (whether the user stepped through it or hit
  /// "skip"). Updates in-memory state immediately so the launcher stops
  /// re-triggering, then persists best-effort.
  Future<void> complete() async {
    state = const AsyncValue.data(true);
    try {
      await _storage.write(key: _storageKey, value: 'true');
    } catch (_) {
      // Non-fatal: in-memory state already hides it for this session.
    }
  }
}
