import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/owner/presentation/controllers/owner_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Step-up re-authentication for the owner panel.
///
/// The panel requires a 2FA proof fresher than 12h (`SuperAdminFreshness`
/// server-side). When it lapses, the server answers `OWNER_2FA_STALE` — and
/// before this existed the only cure was a full logout + login, which threw
/// away whatever the operator had half-typed (an invite e-mail, a contract's
/// override fields) and read as a dead end.
///
/// So the expiry is handled where it happens: ask for the six digits, refresh
/// the proof through `POST /me/2fa/reverify`, and RETRY the original call. The
/// security property is unchanged — a valid JWT alone still proves nothing, a
/// current authenticator (or recovery) code is still required, and the endpoint
/// is rate-limited like the login verify. Only the dead end is gone.

/// Returns true when the proof was refreshed.
Future<bool> showOwner2FAStepUp(BuildContext context, WidgetRef ref) async {
  final ok = await showDialog<bool>(
    context: context,
    barrierDismissible: false,
    builder: (_) => const _StepUpDialog(),
  );
  if (ok == true) {
    // The gate probe caches the stale verdict; it has to re-run or the screen
    // keeps rendering the "reconfirm" state behind the now-valid session.
    ref.invalidate(ownerAccessStatusProvider);
    ref.invalidate(ownerIsSuperAdminProvider);
    return true;
  }
  return false;
}

/// Runs [action]; on `OWNER_2FA_STALE` asks for the code and runs it ONCE more.
///
/// Retried exactly once on purpose: a second failure is either a wrong code or
/// a real refusal, and a loop would hide both. Every other error propagates
/// untouched so callers keep their own handling.
Future<T> withOwnerStepUp<T>(
  BuildContext context,
  WidgetRef ref,
  Future<T> Function() action,
) async {
  try {
    return await action();
  } on DioException catch (e) {
    if (!_isStale(e)) rethrow;
    if (!context.mounted) rethrow;
    final refreshed = await showOwner2FAStepUp(context, ref);
    if (!refreshed) rethrow;
    return await action();
  }
}

bool _isStale(DioException e) {
  final data = e.response?.data;
  if (data is! Map) return false;
  // RequireSuperAdmin emits a FLAT body: {"code": "...", "message": "..."}.
  // The nested read is only a tolerance for an enveloped responder.
  final flat = data['code'];
  if (flat is String) return flat == 'OWNER_2FA_STALE';
  final nested = data['error'];
  return nested is Map && nested['code'] == 'OWNER_2FA_STALE';
}

class _StepUpDialog extends ConsumerStatefulWidget {
  const _StepUpDialog();

  @override
  ConsumerState<_StepUpDialog> createState() => _StepUpDialogState();
}

class _StepUpDialogState extends ConsumerState<_StepUpDialog> {
  final _codeCtrl = TextEditingController();
  bool _busy = false;
  String? _error;

  @override
  void dispose() {
    _codeCtrl.dispose();
    super.dispose();
  }

  /// Enabled only with something typed: the Confirm button is gated on the
  /// same emptiness the submit would otherwise have bailed on silently.
  bool get _canSubmit => !_busy && _codeCtrl.text.trim().isNotEmpty;

  Future<void> _submit() async {
    final code = _codeCtrl.text.trim();
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await ref.read(dioProvider).post<dynamic>(
        '/me/2fa/reverify',
        data: <String, dynamic>{'code': code},
      );
      if (!mounted) return;
      Navigator.of(context).pop(true);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _busy = false;
        _error = parseApiError(e);
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    return AlertDialog(
      backgroundColor: colors.backgroundSecondary,
      title: Row(
        children: [
          Icon(Icons.shield_outlined, size: 20, color: colors.bluePrimary),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              t.owner.stepup_title,
              style: TextStyle(color: colors.textPrimary, fontSize: 18),
            ),
          ),
        ],
      ),
      content: SizedBox(
        width: 380,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              t.owner.stepup_body,
              style: TextStyle(color: colors.textSecondary, fontSize: 13),
            ),
            const SizedBox(height: 16),
            TextField(
              key: const Key('owner_stepup_code'),
              controller: _codeCtrl,
              autofocus: true,
              enabled: !_busy,
              keyboardType: TextInputType.text,
              textInputAction: TextInputAction.done,
              onChanged: (_) => setState(() {}),
              onSubmitted: (_) {
                if (_canSubmit) _submit();
              },
              // A TOTP code is digits, but a recovery code is alphanumeric —
              // filtering to digits would silently make the fallback
              // untypeable, which is the one path someone locked out needs.
              inputFormatters: [LengthLimitingTextInputFormatter(16)],
              style: TextStyle(color: colors.textPrimary, letterSpacing: 2),
              decoration: InputDecoration(labelText: t.owner.stepup_code),
            ),
            const SizedBox(height: 8),
            Text(
              t.owner.stepup_hint,
              style: TextStyle(color: colors.textMuted, fontSize: 12),
            ),
            if (_error != null) ...[
              const SizedBox(height: 12),
              Text(
                _error!,
                key: const Key('owner_stepup_error'),
                style: TextStyle(color: colors.redAlert, fontSize: 12),
              ),
            ],
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: _busy ? null : () => Navigator.of(context).pop(false),
          child: Text(t.common.cancel),
        ),
        FilledButton(
          key: const Key('owner_stepup_confirm'),
          onPressed: _canSubmit ? _submit : null,
          child: Text(t.owner.stepup_confirm),
        ),
      ],
    );
  }
}
