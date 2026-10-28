import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'package:app/features/auth/presentation/controllers/auth_controller.dart';

part 'inbox_assignment_filter_controller.g.dart';

/// Inbox assignment scope (US-017, three buckets since US-020):
/// [mine] = conversations assigned to the signed-in user (`?assigned=<uid>`),
/// [unassigned] = conversations with no assignee (`?assigned=none`) — so a
/// thread nobody picked up yet never disappears from an attendant's view,
/// [all] = every conversation.
enum InboxAssignmentScope { mine, unassigned, all }

/// Resolves the current scope. Defaults by role, gated to the platform
/// account (US-020): only a platform-account attendant lands on "Minhas" —
/// customer-account teams keep the historical full view ("Todas") unless
/// the user explicitly picks otherwise. An explicit choice is persisted
/// per user via [FlutterSecureStorage] (the app's KV-pref mechanism; see
/// locale_bootstrap.dart) so it survives reloads and never leaks between
/// users sharing a browser.
@Riverpod(keepAlive: true)
class InboxAssignmentFilter extends _$InboxAssignmentFilter {
  static const _storageKeyPrefix = 'inbox_assignment_filter_';
  static const _storage = FlutterSecureStorage();

  @override
  Future<InboxAssignmentScope> build() async {
    // Re-resolves on login/logout so the default and the stored choice
    // always follow the CURRENT user.
    final user = await ref.watch(authControllerProvider.future);
    if (user == null) return InboxAssignmentScope.all;
    try {
      final stored = await _storage.read(key: '$_storageKeyPrefix${user.id}');
      switch (stored) {
        case 'mine':
          return InboxAssignmentScope.mine;
        case 'unassigned':
          return InboxAssignmentScope.unassigned;
        case 'all':
          return InboxAssignmentScope.all;
      }
    } catch (_) {
      // Storage failure is non-fatal — fall through to the role default.
    }
    if (!user.isAdmin && user.isPlatformAccount) {
      return InboxAssignmentScope.mine;
    }
    return InboxAssignmentScope.all;
  }

  /// Applies and persists an explicit toggle choice.
  Future<void> set(InboxAssignmentScope scope) async {
    state = AsyncValue.data(scope);
    final user = ref.read(authControllerProvider).asData?.value;
    if (user == null) return;
    try {
      await _storage.write(
        key: '$_storageKeyPrefix${user.id}',
        value: scope.name,
      );
    } catch (_) {
      // Non-fatal: in-memory state already applies for this session.
    }
  }
}
