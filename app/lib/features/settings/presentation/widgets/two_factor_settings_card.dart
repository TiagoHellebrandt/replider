import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:qr_flutter/qr_flutter.dart';
import 'package:app/core/error/api_error_parser.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/design_system/components/glass_card.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/settings/domain/models/two_factor_models.dart';
import 'package:app/features/settings/presentation/controllers/two_factor_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// Renders the 2FA panel inside the Profile screen. The enrolled state
/// comes from the cached /me user (instant, no spinner). Two states:
///   1. Disabled — "Set up" CTA → setup() → in-card wizard with QR +
///      6-digit code field → verify() → recovery-codes display →
///      "Done" returns to enabled card.
///   2. Enabled — "Disable" with proof-code prompt.
class TwoFactorSettingsCard extends ConsumerStatefulWidget {
  const TwoFactorSettingsCard({super.key});

  @override
  ConsumerState<TwoFactorSettingsCard> createState() =>
      _TwoFactorSettingsCardState();
}

class _TwoFactorSettingsCardState extends ConsumerState<TwoFactorSettingsCard> {
  TwoFactorSetupResult? _setupInProgress;
  List<String>? _recoveryCodesShown;
  final TextEditingController _codeController = TextEditingController();
  bool _busy = false;
  String? _error;

  // Wins over the cached /me totp_enabled right after an enroll/disable so
  // the card flips instantly, before the background refreshMe() updates
  // the cached user. Null = defer to the cached value.
  bool? _enabledOverride;

  // A 6-digit TOTP code, or an 8-char recovery code (XXXX-XXXX, the
  // server's Crockford alphabet). Used to gate the action buttons so a
  // half-typed or garbage value can't be submitted.
  static final RegExp _totpCode = RegExp(r'^\d{6}$');
  static final RegExp _recoveryCode = RegExp(r'^[0-9A-Z]{4}-?[0-9A-Z]{4}$');

  // Enrollment confirmation only accepts the 6-digit authenticator code.
  bool get _verifyCodeValid => _totpCode.hasMatch(_codeController.text.trim());

  // Disable accepts the TOTP code OR a recovery code.
  bool get _disableCodeValid {
    final v = _codeController.text.trim().toUpperCase();
    return _totpCode.hasMatch(v) || _recoveryCode.hasMatch(v);
  }

  // Upper-cases recovery-code input as the user types so the field only
  // ever holds the server's alphabet.
  static final TextInputFormatter _upperCase = TextInputFormatter.withFunction(
    (oldValue, newValue) =>
        newValue.copyWith(text: newValue.text.toUpperCase()),
  );

  @override
  void dispose() {
    _codeController.dispose();
    super.dispose();
  }

  Future<void> _startSetup() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final out = await ref.read(twoFactorActionsProvider.notifier).setup();
      setState(() => _setupInProgress = out);
    } catch (e) {
      setState(() => _error = parseApiError(e));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _verify() async {
    final code = _codeController.text.trim();
    if (code.isEmpty) return;
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final out =
          await ref.read(twoFactorActionsProvider.notifier).verify(code);
      // 2FA just flipped on — show it instantly, then refresh the cached
      // /me user in the background so other screens (and a card remount)
      // stay consistent.
      unawaited(ref.read(authControllerProvider.notifier).refreshMe());
      setState(() {
        _enabledOverride = true;
        _recoveryCodesShown = out.recoveryCodes;
        _setupInProgress = null;
        _codeController.clear();
      });
    } catch (e) {
      setState(() => _error = parseApiError(e));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _disable() async {
    final code = _codeController.text.trim();
    if (code.isEmpty) return;
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await ref.read(twoFactorActionsProvider.notifier).disable(code);
      // 2FA just flipped off — show it instantly, then refresh cached /me.
      unawaited(ref.read(authControllerProvider.notifier).refreshMe());
      setState(() {
        _enabledOverride = false;
        _codeController.clear();
      });
    } catch (e) {
      setState(() => _error = parseApiError(e));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    // Render instantly from the cached /me user (which now carries
    // totp_enabled) — no dedicated GET /me/2fa/status round-trip, so no
    // spinner. A local override wins right after an enroll/disable so the
    // card flips before the background refreshMe() lands.
    final enabled = _enabledOverride ??
        (ref.watch(authControllerProvider).asData?.value?.totpEnabled ?? false);

    return GlassCard(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(Icons.shield_outlined,
                    color: colors.bluePrimary, size: 20),
                const SizedBox(width: 8),
                Text(
                  t.profile.two_factor_label,
                  style: typography.body.copyWith(
                    color: colors.textPrimary,
                    fontSize: 15,
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 4),
            Text(
              t.profile.two_factor_desc,
              style: typography.body.copyWith(
                color: colors.textSecondary,
                fontSize: 12,
              ),
            ),
            const SizedBox(height: 12),
            _buildBody(colors, typography, enabled),
            if (_error != null) ...[
              const SizedBox(height: 8),
              Text(
                _error!,
                style: typography.body
                    .copyWith(color: colors.redAlert, fontSize: 12),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildBody(
    AppColorsExtension colors,
    AppTypographyExtension typography,
    bool enabled,
  ) {
    if (_recoveryCodesShown != null) {
      return _buildRecoveryDisplay(colors, typography);
    }
    if (_setupInProgress != null) {
      return _buildSetupWizard(colors, typography);
    }
    if (enabled) {
      return _buildEnabledCard(colors, typography);
    }
    return _buildDisabledCard(colors, typography);
  }

  Widget _buildDisabledCard(
    AppColorsExtension colors,
    AppTypographyExtension typography,
  ) {
    return Row(
      children: [
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
          decoration: BoxDecoration(
            color: colors.borderMedium.withValues(alpha: 0.3),
            borderRadius: BorderRadius.circular(8),
          ),
          child: Text(
            t.profile.two_factor_status_off,
            style: typography.body.copyWith(
              color: colors.textSecondary,
              fontSize: 11,
              fontWeight: FontWeight.w600,
            ),
          ),
        ),
        const Spacer(),
        ElevatedButton(
          onPressed: _busy ? null : _startSetup,
          style: ElevatedButton.styleFrom(
            backgroundColor: colors.bluePrimary,
            foregroundColor: Colors.white,
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
          ),
          child: Text(t.profile.two_factor_setup_cta),
        ),
      ],
    );
  }

  Widget _buildEnabledCard(
    AppColorsExtension colors,
    AppTypographyExtension typography,
  ) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
              decoration: BoxDecoration(
                color: colors.limeGlow.withValues(alpha: 0.18),
                borderRadius: BorderRadius.circular(8),
              ),
              child: Text(
                t.profile.two_factor_status_on,
                style: typography.body.copyWith(
                  color: colors.limeGlow,
                  fontSize: 11,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ),
          ],
        ),
        const SizedBox(height: 12),
        Text(
          t.profile.two_factor_disable_prompt,
          style: typography.body.copyWith(
            color: colors.textSecondary,
            fontSize: 12,
          ),
        ),
        const SizedBox(height: 8),
        TextField(
          controller: _codeController,
          enabled: !_busy,
          maxLength: 9,
          autocorrect: false,
          enableSuggestions: false,
          textCapitalization: TextCapitalization.characters,
          inputFormatters: [
            FilteringTextInputFormatter.allow(RegExp(r'[0-9A-Za-z-]')),
            _upperCase,
          ],
          onChanged: (_) => setState(() {}),
          decoration: InputDecoration(
            hintText: t.profile.two_factor_code_hint,
            counterText: '',
          ),
        ),
        const SizedBox(height: 8),
        Align(
          alignment: Alignment.centerRight,
          child: TextButton(
            onPressed: _busy || !_disableCodeValid ? null : _disable,
            style: TextButton.styleFrom(foregroundColor: colors.redAlert),
            child: Text(t.profile.two_factor_disable_cta),
          ),
        ),
      ],
    );
  }

  Widget _buildSetupWizard(
    AppColorsExtension colors,
    AppTypographyExtension typography,
  ) {
    final s = _setupInProgress!;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          t.profile.two_factor_step1,
          style: typography.body.copyWith(
              color: colors.textPrimary,
              fontSize: 13,
              fontWeight: FontWeight.w600),
        ),
        const SizedBox(height: 8),
        Center(
          child: Container(
            color: Colors.white,
            padding: const EdgeInsets.all(12),
            child: QrImageView(
              data: s.otpauthUrl,
              size: 180,
              backgroundColor: Colors.white,
            ),
          ),
        ),
        const SizedBox(height: 12),
        Text(
          t.profile.two_factor_manual_secret_label,
          style: typography.body
              .copyWith(color: colors.textSecondary, fontSize: 11),
        ),
        const SizedBox(height: 4),
        Row(
          children: [
            Expanded(
              child: SelectableText(
                s.secret,
                style: typography.body.copyWith(
                  color: colors.textPrimary,
                  fontSize: 13,
                  fontFamily: 'monospace',
                ),
              ),
            ),
            IconButton(
              icon: const Icon(Icons.copy, size: 16),
              tooltip: t.common.copy,
              onPressed: () => Clipboard.setData(ClipboardData(text: s.secret)),
            ),
          ],
        ),
        const SizedBox(height: 12),
        Text(
          t.profile.two_factor_step2,
          style: typography.body.copyWith(
              color: colors.textPrimary,
              fontSize: 13,
              fontWeight: FontWeight.w600),
        ),
        const SizedBox(height: 8),
        TextField(
          controller: _codeController,
          enabled: !_busy,
          maxLength: 6,
          keyboardType: TextInputType.number,
          inputFormatters: [FilteringTextInputFormatter.digitsOnly],
          onChanged: (_) => setState(() {}),
          decoration: InputDecoration(
            hintText: '000000',
            counterText: '',
          ),
        ),
        const SizedBox(height: 8),
        Row(
          children: [
            TextButton(
              onPressed:
                  _busy ? null : () => setState(() => _setupInProgress = null),
              child: Text(t.common.cancel),
            ),
            const Spacer(),
            ElevatedButton(
              onPressed: _busy || !_verifyCodeValid ? null : _verify,
              style: ElevatedButton.styleFrom(
                backgroundColor: colors.bluePrimary,
                foregroundColor: Colors.white,
              ),
              child: _busy
                  ? const SizedBox(
                      width: 18,
                      height: 18,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : Text(t.profile.two_factor_verify_cta),
            ),
          ],
        ),
      ],
    );
  }

  Widget _buildRecoveryDisplay(
    AppColorsExtension colors,
    AppTypographyExtension typography,
  ) {
    final codes = _recoveryCodesShown!;
    final blob = codes.join('\n');
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          t.profile.two_factor_recovery_title,
          style: typography.body.copyWith(
            color: colors.limeGlow,
            fontSize: 14,
            fontWeight: FontWeight.w700,
          ),
        ),
        const SizedBox(height: 4),
        Text(
          t.profile.two_factor_recovery_warning,
          style: typography.body.copyWith(
            color: colors.orangeAlert,
            fontSize: 12,
          ),
        ),
        const SizedBox(height: 12),
        Container(
          padding: const EdgeInsets.all(12),
          decoration: BoxDecoration(
            color: colors.borderMedium.withValues(alpha: 0.18),
            borderRadius: BorderRadius.circular(8),
            border: Border.all(color: colors.borderMedium),
          ),
          child: Wrap(
            spacing: 12,
            runSpacing: 8,
            children: [
              for (final c in codes)
                SelectableText(
                  c,
                  style: typography.body.copyWith(
                    color: colors.textPrimary,
                    fontSize: 13,
                    fontFamily: 'monospace',
                    fontWeight: FontWeight.w600,
                  ),
                ),
            ],
          ),
        ),
        const SizedBox(height: 12),
        Row(
          children: [
            OutlinedButton.icon(
              icon: const Icon(Icons.copy, size: 16),
              label: Text(t.profile.two_factor_recovery_copy_cta),
              onPressed: () => Clipboard.setData(ClipboardData(text: blob)),
            ),
            const Spacer(),
            ElevatedButton(
              onPressed: () => setState(() => _recoveryCodesShown = null),
              child: Text(t.profile.two_factor_recovery_done_cta),
            ),
          ],
        ),
      ],
    );
  }
}
