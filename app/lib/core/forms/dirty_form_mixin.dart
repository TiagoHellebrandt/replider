import 'package:flutter/material.dart';
import 'package:app/design_system/components/app_dialogs.dart';
import 'package:app/i18n/strings.g.dart';

/// Reusable "unsaved changes" guard for screens that edit a server-side
/// resource. Replicates the pattern that `agent_detail_screen.dart`
/// originally implemented inline:
///   1. Load the resource and snapshot the form values via [snapshotForm].
///   2. Wire field changes (`onChanged`, `setState` for non-text inputs)
///      to call [recomputeDirty].
///   3. Wrap the screen with [buildDirtyGuard] so a back navigation prompts.
///   4. After a successful save, call [resetSnapshot] so the screen is
///      no longer dirty.
///
/// Why a mixin and not a widget: the dirty state lives on the screen's
/// own [State] object alongside `_isSaving` and other flags; a wrapper
/// would either need callbacks or complex inheritance. A mixin keeps
/// the call sites natural — same `setState` discipline as before.
mixin DirtyFormMixin<T extends StatefulWidget> on State<T> {
  Map<String, Object?> _initialSnapshot = const {};
  bool _isDirty = false;

  bool get isDirty => _isDirty;

  /// Override to return the comparable representation of the form's
  /// persisted state. Use simple types (String, int, bool, list of
  /// primitives) so equality is value-based and trivial to extend.
  Map<String, Object?> snapshotForm();

  /// Call once after the source data is loaded into controllers. Stores
  /// the baseline used to compute dirtiness; without this every change
  /// would compare against an empty map and report "dirty" instantly.
  void resetSnapshot() {
    _initialSnapshot = snapshotForm();
    if (_isDirty) {
      // Use markNeedsBuild rather than setState because callers may
      // invoke this from inside another setState (e.g. on save).
      _isDirty = false;
      if (mounted) (context as Element).markNeedsBuild();
    }
  }

  /// Recompute the dirty flag against the stored snapshot. Cheap —
  /// O(snapshot size) per change. Wire to TextField onChanged + every
  /// setState that touches a persisted field.
  void recomputeDirty() {
    final now = snapshotForm();
    bool dirty = false;
    for (final k in now.keys) {
      if (now[k] != _initialSnapshot[k]) {
        dirty = true;
        break;
      }
    }
    if (dirty != _isDirty) {
      setState(() => _isDirty = dirty);
    }
  }

  /// Confirmation dialog shown by [buildDirtyGuard]. Routed through
  /// [showConfirmDialog] so the visual matches every other destructive
  /// confirm in the app (red primary, "Continuar editando" cancel copy).
  Future<bool> confirmDiscard() {
    return showConfirmDialog(
      context,
      title: t.common.discard_changes_title,
      message: t.common.discard_changes_body,
      confirmText: t.common.discard,
      cancelText: t.common.keep_editing,
      destructive: true,
    );
  }

  /// Wraps [child] in a [PopScope] that intercepts back navigation
  /// when the form is dirty. Successful confirmation triggers
  /// [onDiscard] (typically `() => context.go(...)`); declining keeps
  /// the user on the screen.
  Widget buildDirtyGuard({
    required Widget child,
    required VoidCallback onDiscard,
  }) {
    return PopScope(
      canPop: !_isDirty,
      onPopInvokedWithResult: (didPop, _) async {
        if (didPop) return;
        final ok = await confirmDiscard();
        if (ok && mounted) onDiscard();
      },
      child: child,
    );
  }
}
