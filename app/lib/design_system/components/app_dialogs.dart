import 'package:flutter/material.dart';
import 'package:pointer_interceptor/pointer_interceptor.dart';

import 'package:app/design_system/theme_extensions.dart';
import 'package:app/i18n/strings.g.dart';

/// Centralized confirmation dialog so destructive actions across the
/// app look and behave the same: same button order, same destructive
/// color, same cancel copy. Returns true only when the user explicitly
/// confirms — dismissing by tap-outside resolves to false.
///
/// Usage:
///
///   final ok = await showConfirmDialog(
///     context,
///     title: 'Excluir agente?',
///     message: 'This cannot be undone.',
///     confirmText: 'Excluir',
///     destructive: true,
///   );
///   if (ok) { ... }
Future<bool> showConfirmDialog(
  BuildContext context, {
  required String title,
  required String message,
  String? confirmText,
  String? cancelText,
  bool destructive = false,
}) async {
  final colors = Theme.of(context).extension<AppColorsExtension>()!;
  final resolvedConfirm = confirmText ?? t.common.confirm;
  final resolvedCancel = cancelText ?? t.common.cancel;
  final result = await showDialog<bool>(
    context: context,
    // PointerInterceptor: on web, an HtmlElementView (the email reader's
    // iframe) is a real DOM element ABOVE Flutter's canvas — a dialog drawn
    // over it is visible but its clicks land in the iframe, leaving every
    // button dead. The interceptor puts an invisible DOM layer under the
    // dialog so its buttons actually receive the pointer. No-op elsewhere.
    builder: (ctx) => PointerInterceptor(
      child: AlertDialog(
        backgroundColor: colors.backgroundSecondary,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
        // Scrollable, and selectable, because this dialog carries the outbound
        // message a human is about to send to a real person. A cold approach
        // runs 400-800 characters; without this the AlertDialog clips it on a
        // phone, and the person approves a send having read half of it —
        // which is precisely the review this product promises.
        scrollable: true,
        title: Text(title,
            style: TextStyle(color: colors.textPrimary, fontSize: 16)),
        content: SelectableText(
          message,
          style:
              TextStyle(color: colors.textSecondary, fontSize: 14, height: 1.4),
        ),
        actions: [
          // An EMPTY cancelText means there is no second option: some dialogs
          // exist only to explain something the user cannot act on (a file the
          // scanner flagged), and a cancel button there implies a choice that
          // does not exist. Null still means "use the default label".
          if (resolvedCancel.isNotEmpty)
            TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: Text(resolvedCancel,
                  style: TextStyle(color: colors.textMuted)),
            ),
          ElevatedButton(
            onPressed: () => Navigator.pop(ctx, true),
            style: ElevatedButton.styleFrom(
              backgroundColor:
                  destructive ? colors.redAlert : colors.bluePrimary,
              foregroundColor: Colors.white,
            ),
            child: Text(resolvedConfirm),
          ),
        ],
      ),
    ),
  );
  return result == true;
}

/// Inline error snackbar with consistent styling. Wraps
/// `ScaffoldMessenger.showSnackBar` so every error shown to the user
/// looks identical — red background, 4s, floating above FAB/nav.
void showErrorSnackbar(BuildContext context, String message,
    {SnackBarAction? action}) {
  final colors = Theme.of(context).extension<AppColorsExtension>()!;
  ScaffoldMessenger.of(context).showSnackBar(
    SnackBar(
      content: Text(message),
      backgroundColor: colors.redAlert,
      behavior: SnackBarBehavior.floating,
      duration: const Duration(seconds: 4),
      action: action,
    ),
  );
}

/// Success snackbar counterpart. Kept shorter (2.2s) — users don't
/// need to read it, it's just the "it worked" acknowledgement.
void showSuccessSnackbar(BuildContext context, String message) {
  final colors = Theme.of(context).extension<AppColorsExtension>()!;
  ScaffoldMessenger.of(context).showSnackBar(
    SnackBar(
      content: Text(message),
      backgroundColor: colors.whatsappGreen,
      behavior: SnackBarBehavior.floating,
      duration: const Duration(milliseconds: 2200),
    ),
  );
}

/// Single-field input dialog styled identically to [showConfirmDialog]
/// (same surface, shape, button order). Returns the trimmed text on
/// confirm, or null on cancel / tap-outside. Reuse for the "type a
/// reason / amount / N days" prompts so they look the same everywhere
/// instead of each screen hand-rolling an AlertDialog + raw TextField.
Future<String?> showInputDialog(
  BuildContext context, {
  required String title,
  String? hint,
  String? initialValue,
  String? confirmText,
  TextInputType keyboardType = TextInputType.text,
}) {
  return showDialog<String>(
    context: context,
    // Same iframe-swallows-clicks protection as showConfirmDialog above.
    builder: (_) => PointerInterceptor(
      child: _InputDialog(
        title: title,
        hint: hint,
        initialValue: initialValue,
        confirmText: confirmText,
        keyboardType: keyboardType,
      ),
    ),
  );
}

class _InputDialog extends StatefulWidget {
  final String title;
  final String? hint;
  final String? initialValue;
  final String? confirmText;
  final TextInputType keyboardType;

  const _InputDialog({
    required this.title,
    required this.hint,
    required this.initialValue,
    required this.confirmText,
    required this.keyboardType,
  });

  @override
  State<_InputDialog> createState() => _InputDialogState();
}

class _InputDialogState extends State<_InputDialog> {
  late final TextEditingController _controller =
      TextEditingController(text: widget.initialValue ?? '');

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return AlertDialog(
      backgroundColor: colors.backgroundSecondary,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      title: Text(widget.title,
          style: TextStyle(color: colors.textPrimary, fontSize: 16)),
      content: TextField(
        controller: _controller,
        autofocus: true,
        keyboardType: widget.keyboardType,
        style: TextStyle(color: colors.textPrimary),
        onSubmitted: (v) => Navigator.pop(context, v.trim()),
        decoration: InputDecoration(
          hintText: widget.hint,
          hintStyle: TextStyle(color: colors.textMuted),
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context),
          child:
              Text(t.common.cancel, style: TextStyle(color: colors.textMuted)),
        ),
        ElevatedButton(
          onPressed: () => Navigator.pop(context, _controller.text.trim()),
          style: ElevatedButton.styleFrom(
            backgroundColor: colors.bluePrimary,
            foregroundColor: Colors.white,
          ),
          child: Text(widget.confirmText ?? t.common.confirm),
        ),
      ],
    );
  }
}
