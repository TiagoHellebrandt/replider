import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'feature_announcement_controller.g.dart';

/// Tracks whether the "What's new" feature-announcement modal has already been
/// shown for the CURRENT release (US-184). Persisted locally via
/// [FlutterSecureStorage] — the app's KV-pref mechanism (the Drift db carries
/// no tables), same shape as the product-tour / NPS one-shots.
///
/// The release is identified by [currentRelease]: bumping that constant (and
/// the `feature_announcement.*` i18n copy) re-surfaces the modal exactly once
/// per upgrade. `build()` returns `true` when the modal should stay HIDDEN
/// (already seen this release, or this is a brand-new install — see below).
///
/// Brand-new installs (no stored release) have nothing to announce: they're
/// seeing every feature for the first time. We stamp the current release
/// silently and suppress the modal, so it only ever fires for RETURNING users
/// after a real upgrade — which also guarantees it never stacks on top of the
/// new-account product tour.
@Riverpod(keepAlive: true)
class FeatureAnnouncementController extends _$FeatureAnnouncementController {
  /// Bump on every release that ships user-facing changes worth announcing,
  /// alongside the `feature_announcement.item*` copy in the i18n JSONs.
  static const currentRelease = '2026-06';
  static const _storageKey = 'feature_announcement_seen_release';
  static const _storage = FlutterSecureStorage();

  @override
  Future<bool> build() async {
    try {
      final stored = await _storage.read(key: _storageKey);
      if (stored == null) {
        // First run on this device: nothing is "new" yet. Stamp the current
        // release so the modal only fires after a future upgrade, never to a
        // first-time user (best-effort; a write failure just means it may show
        // once on the next launch, which is harmless).
        await _storage.write(key: _storageKey, value: currentRelease);
        return true;
      }
      return stored == currentRelease;
    } catch (_) {
      // Storage failure is non-fatal — default to NOT showing so a flaky
      // keystore can't replay the announcement on every dashboard paint.
      return true;
    }
  }

  /// Marks the current release's announcement as seen. Updates in-memory state
  /// immediately so the launcher stops re-triggering, then persists best-effort.
  Future<void> markSeen() async {
    state = const AsyncValue.data(true);
    try {
      await _storage.write(key: _storageKey, value: currentRelease);
    } catch (_) {
      // Non-fatal: in-memory state already hides it for this session.
    }
  }
}
