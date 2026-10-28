import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:app/design_system/components/custom_text_field.dart';
import 'package:app/design_system/components/primary_pill_button.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/validators/email_validator.dart';

class ForgotPasswordScreen extends ConsumerStatefulWidget {
  const ForgotPasswordScreen({super.key});

  @override
  ConsumerState<ForgotPasswordScreen> createState() =>
      _ForgotPasswordScreenState();
}

class _ForgotPasswordScreenState extends ConsumerState<ForgotPasswordScreen> {
  static const _resendCooldownSeconds = 60;

  final _formKey = GlobalKey<FormState>();
  final _emailController = TextEditingController();
  bool _successMessage = false;
  // True while a send is in flight (prevents a double-fire from a fast
  // double-tap); _cooldown counts down the post-send window during which the
  // button stays disabled so repeated clicks can't spam multiple emails.
  bool _submitting = false;
  int _cooldown = 0;
  Timer? _cooldownTimer;

  bool get _canSubmit =>
      _emailController.text.isNotEmpty && !_submitting && _cooldown == 0;

  void _onInputChanged(String _) {
    setState(() {}); // Trigger rebuild
  }

  void _startCooldown() {
    _cooldownTimer?.cancel();
    setState(() => _cooldown = _resendCooldownSeconds);
    _cooldownTimer = Timer.periodic(const Duration(seconds: 1), (timer) {
      if (!mounted) {
        timer.cancel();
        return;
      }
      setState(() => _cooldown--);
      if (_cooldown <= 0) timer.cancel();
    });
  }

  void _submit() {
    // Guard against re-entry (in-flight or still cooling down).
    if (!_canSubmit) return;
    if (!_formKey.currentState!.validate()) return;

    setState(() => _submitting = true);
    ref
        .read(authControllerProvider.notifier)
        .forgotPassword(_emailController.text)
        .then((_) {
      if (!mounted) return;
      setState(() {
        _submitting = false;
        _successMessage = true;
      });
      // Only arm the cooldown on a successful send.
      _startCooldown();
    }).catchError((Object error) {
      if (!mounted) return;
      // On failure, re-enable immediately so the user can retry.
      setState(() => _submitting = false);
      final colors = Theme.of(context).extension<AppColorsExtension>()!;
      final message = parseApiError(error);
      ScaffoldMessenger.of(context).clearSnackBars();
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(message),
          backgroundColor: colors.redAlert,
          behavior: SnackBarBehavior.floating,
          margin: const EdgeInsets.all(16),
          shape:
              RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
        ),
      );
    });
  }

  @override
  void dispose() {
    _cooldownTimer?.cancel();
    _emailController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final backgroundColor = const Color(0xFF1E1E1E);
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    return Scaffold(
      backgroundColor: backgroundColor,
      body: SafeArea(
        child: LayoutBuilder(
          builder: (context, constraints) {
            return SingleChildScrollView(
              child: Container(
                padding: const EdgeInsets.symmetric(
                    horizontal: 24.0, vertical: 16.0),
                constraints: BoxConstraints(minHeight: constraints.maxHeight),
                alignment: Alignment.center,
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 400),
                  child: _buildForgotForm(colors, typography),
                ),
              ),
            );
          },
        ),
      ),
    );
  }

  Widget _buildForgotForm(
      AppColorsExtension colors, AppTypographyExtension typography) {
    return Form(
      key: _formKey,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // Top Bar (Logo)
          Row(
            mainAxisAlignment: MainAxisAlignment.center,
            mainAxisSize: MainAxisSize.min,
            children: [
              Image.asset(
                'assets/logo_white.webp',
                height: 24,
                errorBuilder: (context, error, stackTrace) => const Icon(
                    Icons.layers,
                    size: 24,
                    color: Color(0xFF4A89F3)),
              ),
              const SizedBox(width: 8),
              Text(
                'Replider',
                style: typography.display.copyWith(
                    fontSize: 20,
                    color: Colors.white,
                    fontWeight: FontWeight.w600),
              ),
            ],
          ),
          const SizedBox(height: 32),

          // Title
          Text(
            t.auth.forgot_title,
            style: typography.display.copyWith(
                fontSize: 28, color: Colors.white, fontWeight: FontWeight.w600),
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: 8),
          Text(
            t.auth.forgot_subtitle,
            style: typography.body
                .copyWith(color: colors.textSecondary, fontSize: 14),
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: 32),

          if (_successMessage)
            Container(
              padding: const EdgeInsets.all(16),
              margin: const EdgeInsets.only(bottom: 24),
              decoration: BoxDecoration(
                color: const Color(0xFF34A853).withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(8),
                border: Border.all(
                    color: const Color(0xFF34A853).withValues(alpha: 0.5)),
              ),
              child: Text(
                // The honest phrasing: the backend answers 200 for unknown
                // accounts on purpose (no enumeration), so "sent with
                // success!" asserted a send that may not have happened.
                t.auth.forgot_success,
                style: typography.body.copyWith(
                    color: const Color(0xFF34A853),
                    fontWeight: FontWeight.w500),
                textAlign: TextAlign.center,
              ),
            ),

          // Inputs
          CustomTextField(
            key: const Key('email_input'),
            hint: t.auth.email_placeholder,
            prefixIcon: Icons.mail_outline,
            controller: _emailController,
            keyboardType: TextInputType.emailAddress,
            autofocus: true,
            onChanged: _onInputChanged,
            onSubmitted: (_) {
              if (_canSubmit) _submit();
            },
            validator: (v) => isValidEmail(v) ? null : t.auth.invalid_email,
          ),
          const SizedBox(height: 32),

          // Submit Button — disabled while sending and during the 60s cooldown
          // so repeated taps can't fire multiple recovery emails.
          PrimaryPillButton(
            key: const Key('submit_button'),
            text: _cooldown > 0
                ? t.auth.forgot_resend_in(seconds: _cooldown)
                : t.auth.forgot_send_link,
            isLoading: _submitting,
            onPressed: _canSubmit ? _submit : null,
          ),
          const SizedBox(height: 32),

          // Bottom Prompt
          TextButton(
            onPressed: () => context.go('/'),
            style: TextButton.styleFrom(
              padding: EdgeInsets.zero,
              minimumSize: Size.zero,
              tapTargetSize: MaterialTapTargetSize.shrinkWrap,
            ),
            child: Text(
              t.auth.back_to_login,
              style: typography.body.copyWith(
                  color: Colors.white,
                  fontSize: 14,
                  fontWeight: FontWeight.w500),
            ),
          ),
        ],
      ),
    );
  }
}
