import 'package:flutter/material.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/i18n/strings.g.dart';

/// Friction-by-typing confirmation dialog used for any destructive
/// workspace-scoped action. Enforces that the user types [confirmString]
/// verbatim before the destructive button is enabled, so misclicks don't
/// nuke a workspace / calendar / channel.
///
/// Returns `true` if the user confirmed and the handler completed
/// successfully, `false` otherwise. The [onConfirm] callback returns a
/// nullable error string matching the controller convention; non-null
/// keeps the dialog open and shows the message inline so the user can
/// retry without re-typing the confirmation.
class DangerConfirmDialog extends StatefulWidget {
  final String title;
  final String body;
  final String confirmString;
  final String confirmHint;
  final String confirmButton;
  final Future<String?> Function() onConfirm;

  const DangerConfirmDialog({
    super.key,
    required this.title,
    required this.body,
    required this.confirmString,
    required this.confirmHint,
    required this.confirmButton,
    required this.onConfirm,
  });

  static Future<bool> show(
    BuildContext context, {
    required String title,
    required String body,
    required String confirmString,
    required String confirmHint,
    required String confirmButton,
    required Future<String?> Function() onConfirm,
  }) async {
    final result = await showDialog<bool>(
      context: context,
      barrierDismissible: true,
      builder: (_) => DangerConfirmDialog(
        title: title,
        body: body,
        confirmString: confirmString,
        confirmHint: confirmHint,
        confirmButton: confirmButton,
        onConfirm: onConfirm,
      ),
    );
    return result ?? false;
  }

  @override
  State<DangerConfirmDialog> createState() => _DangerConfirmDialogState();
}

class _DangerConfirmDialogState extends State<DangerConfirmDialog> {
  final _ctrl = TextEditingController();
  bool _busy = false;
  String? _inlineError;

  @override
  void initState() {
    super.initState();
    _ctrl.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  bool get _matches => _ctrl.text.trim() == widget.confirmString.trim();

  Future<void> _confirm() async {
    setState(() {
      _busy = true;
      _inlineError = null;
    });
    final err = await widget.onConfirm();
    if (!mounted) return;
    if (err == null) {
      Navigator.of(context).pop(true);
    } else {
      setState(() {
        _busy = false;
        _inlineError = err;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    return AlertDialog(
      backgroundColor: colors.backgroundSecondary,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      title: Text(widget.title, style: TextStyle(color: colors.textPrimary)),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(widget.body,
              style: TextStyle(color: colors.textSecondary, fontSize: 14)),
          const SizedBox(height: 16),
          TextField(
            controller: _ctrl,
            autofocus: true,
            style: TextStyle(color: colors.textPrimary),
            decoration: InputDecoration(
              hintText: widget.confirmHint,
              hintStyle: TextStyle(color: colors.textDisabled, fontSize: 13),
            ),
          ),
          if (_inlineError != null) ...[
            const SizedBox(height: 10),
            Text(
              _inlineError!,
              style: TextStyle(color: colors.redAlert, fontSize: 12),
            ),
          ],
        ],
      ),
      actions: [
        TextButton(
          onPressed: _busy ? null : () => Navigator.of(context).pop(false),
          child:
              Text(t.common.cancel, style: TextStyle(color: colors.textMuted)),
        ),
        ElevatedButton(
          onPressed: (_matches && !_busy) ? _confirm : null,
          style: ElevatedButton.styleFrom(
            backgroundColor: colors.redAlert,
            foregroundColor: Colors.white,
          ),
          child: _busy
              ? const SizedBox(
                  width: 14,
                  height: 14,
                  child: CircularProgressIndicator(
                      strokeWidth: 2, color: Colors.white),
                )
              : Text(widget.confirmButton),
        ),
      ],
    );
  }
}
