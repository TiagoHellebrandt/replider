import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'support_push_prompt.g.dart';

/// Whether the contextual push opt-in prompt inside the support widget has
/// already been handled (enabled OR declined) — canal-interno US-006. Once
/// true the banner never comes back: the user can still flip push from
/// Settings, and a browser-level denial is independently reflected by
/// [PushSubscriptionStatus.denied], which also hides the prompt.
///
/// Persisted via [FlutterSecureStorage], the app's KV-pref mechanism (same
/// pattern as OnboardingChecklistDismissal — the Drift db carries no
/// tables). Unlike the checklist there is no expiry window: a nag that
/// returns is worse than a Settings toggle the user seeks out.
@Riverpod(keepAlive: true)
class SupportPushPromptDismissal extends _$SupportPushPromptDismissal {
  static const _storageKey = 'support_push_prompt_dismissed';

  static const _storage = FlutterSecureStorage();

  @override
  Future<bool> build() async {
    try {
      return await _storage.read(key: _storageKey) != null;
    } catch (_) {
      // Storage failure — err on NOT nagging (prompt is a nicety; push
      // stays reachable from Settings).
      return true;
    }
  }

  /// Marks the prompt as handled forever, regardless of the outcome
  /// (enabled, declined, or permission denied mid-flow).
  Future<void> dismiss() async {
    state = const AsyncValue.data(true);
    try {
      await _storage.write(key: _storageKey, value: '1');
    } catch (_) {
      // Non-fatal: in-memory state already hides it for this session.
    }
  }
}
