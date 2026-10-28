import 'package:flutter/material.dart';

/// Consistent snackbar styling for save / mutation outcomes across
/// Settings. Reds on failure, greens on success, floating behavior
/// everywhere. Previously every screen had its own `Colors.red.shade700`
/// / `Colors.green.shade700` pair inlined — this keeps them in one
/// place so a future theme change propagates automatically.
class SaveFeedback {
  SaveFeedback._();

  /// Shows a green floating snackbar with [message]. Useful after a
  /// successful mutation (save, rename, reconnect, etc.).
  static void showSuccess(BuildContext context, String message) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      content: Text(message),
      backgroundColor: Colors.green.shade700,
      behavior: SnackBarBehavior.floating,
    ));
  }

  /// Shows a red floating snackbar with [message]. Useful for
  /// user-actionable errors (validation, network). For silent infra
  /// errors prefer logging + a retry CTA instead.
  static void showError(BuildContext context, String message) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(
      content: Text(message),
      backgroundColor: Colors.red.shade700,
      behavior: SnackBarBehavior.floating,
    ));
  }

  /// Convenience: if [error] is null show success with [onSuccess]
  /// message, otherwise show the error string. Matches the common
  /// `Future<String?>` return-contract the controllers use.
  static void showResult(BuildContext context, String? error,
      {required String onSuccess}) {
    if (error == null) {
      showSuccess(context, onSuccess);
    } else {
      showError(context, error);
    }
  }
}
