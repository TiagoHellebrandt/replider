import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/components/primary_pill_button.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/auth/data/datasources/auth_remote_datasource.dart';
import 'package:app/features/auth/domain/models/auth_dto.dart';
import 'package:app/i18n/strings.g.dart';

/// Post-registration screen. User lands here after submitting the
/// register form and must click the link in their inbox to activate
/// the account. Needs to be a ConsumerStatefulWidget (not stateless)
/// because the resend button has three live states — idle / sending /
/// cooldown — and the cooldown persists for 45s to discourage spam
/// without needing a backend call to check last-sent.
class EmailVerificationScreen extends ConsumerStatefulWidget {
  final String email;

  const EmailVerificationScreen({super.key, required this.email});

  @override
  ConsumerState<EmailVerificationScreen> createState() =>
      _EmailVerificationScreenState();
}

class _EmailVerificationScreenState
    extends ConsumerState<EmailVerificationScreen> {
  bool _resending = false;
  // Seconds until the user can hit resend again. Counted down by a
  // periodic timer; we only reduce if the widget is still mounted.
  int _cooldownSecs = 0;
  Timer? _cooldownTimer;

  @override
  void dispose() {
    _cooldownTimer?.cancel();
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
                            errorBuilder: (context, error, stackTrace) =>
                                const Icon(Icons.layers,
                                    size: 24, color: Color(0xFF4A89F3)),
                          ),
                          const SizedBox(width: 8),
                          Text(
                            'Replider',
                            style: typography.display.copyWith(
                              fontSize: 20,
                              color: Colors.white,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 48),

                      // Email icon
                      Icon(
                        Icons.mark_email_read_outlined,
                        size: 64,
                        color: const Color(0xFF4A89F3),
                      ),
                      const SizedBox(height: 32),

                      // Title
                      Text(
                        t.email_verification.title,
                        style: typography.display.copyWith(
                          fontSize: 28,
                          color: Colors.white,
                          fontWeight: FontWeight.w600,
                        ),
                        textAlign: TextAlign.center,
                      ),
                      const SizedBox(height: 12),

                      // Subtitle with email
                      Text(
                        t.email_verification.subtitle,
                        style: typography.body.copyWith(
                          color: colors.textSecondary,
                          fontSize: 14,
                        ),
                        textAlign: TextAlign.center,
                      ),
                      const SizedBox(height: 8),
                      Text(
                        // A bad ?email= used to render an empty line under
                        // "Enviamos um link para" and a resend button that
                        // was disabled with no explanation.
                        widget.email.isEmpty
                            ? t.auth.verify_email_unknown
                            : widget.email,
                        key: const Key('verify_email_address'),
                        style: typography.body.copyWith(
                          color: Colors.white,
                          fontSize: 16,
                          fontWeight: FontWeight.w600,
                        ),
                        textAlign: TextAlign.center,
                      ),
                      const SizedBox(height: 24),

                      // Instruction
                      Container(
                        padding: const EdgeInsets.all(16),
                        decoration: BoxDecoration(
                          color: const Color(0xFF4A89F3).withValues(alpha: 0.1),
                          borderRadius: BorderRadius.circular(12),
                          border: Border.all(
                            color:
                                const Color(0xFF4A89F3).withValues(alpha: 0.3),
                          ),
                        ),
                        child: Text(
                          t.email_verification.instruction,
                          style: typography.body.copyWith(
                            color: colors.textSecondary,
                            fontSize: 14,
                          ),
                          textAlign: TextAlign.center,
                        ),
                      ),
                      const SizedBox(height: 32),

                      // Back to Login button
                      PrimaryPillButton(
                        text: t.email_verification.back_to_login,
                        onPressed: () => context.go('/'),
                      ),
                      const SizedBox(height: 24),

                      // Didn't receive + resend affordance. Stacked so
                      // the resend button is immediately below the hint
                      // and can't be missed.
                      Text(
                        t.email_verification.didnt_receive,
                        style: typography.body.copyWith(
                          color: colors.textSecondary,
                          fontSize: 13,
                        ),
                        textAlign: TextAlign.center,
                      ),
                      const SizedBox(height: 8),
                      TextButton.icon(
                        onPressed: (widget.email.isEmpty ||
                                _resending ||
                                _cooldownSecs > 0)
                            ? null
                            : _resend,
                        icon: _resending
                            ? const SizedBox(
                                width: 14,
                                height: 14,
                                child:
                                    CircularProgressIndicator(strokeWidth: 2),
                              )
                            : const Icon(Icons.refresh, size: 16),
                        label: Text(
                          _cooldownSecs > 0
                              ? '${t.email_verification.resend} (${_cooldownSecs}s)'
                              : t.email_verification.resend,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        t.email_verification.check_spam,
                        style: typography.body.copyWith(
                          color: colors.textMuted,
                          fontSize: 11,
                        ),
                        textAlign: TextAlign.center,
                      ),
                    ],
                  ),
                ),
              ),
            );
          },
        ),
      ),
    );
  }

  Future<void> _resend() async {
    if (widget.email.isEmpty) return;
    setState(() => _resending = true);
    try {
      final ds = AuthRemoteDataSource(ref.read(dioProvider));
      await ds.resendVerification(ForgotPasswordDTO(email: widget.email));
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(
        content: Text(t.email_verification.resent),
        behavior: SnackBarBehavior.floating,
      ));
      _startCooldown();
    } on DioException catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(
        content: Text(parseApiError(e)),
        behavior: SnackBarBehavior.floating,
      ));
    } catch (_) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(
        content: Text(t.email_verification.resend_failed),
        behavior: SnackBarBehavior.floating,
      ));
    } finally {
      if (mounted) setState(() => _resending = false);
    }
  }

  void _startCooldown() {
    // Client-side cool-down mirrors the server's 3-per-minute IP cap
    // but keeps the button clearly locked so the user doesn't spam.
    _cooldownTimer?.cancel();
    setState(() => _cooldownSecs = 45);
    _cooldownTimer = Timer.periodic(const Duration(seconds: 1), (timer) {
      if (!mounted) {
        timer.cancel();
        return;
      }
      setState(() => _cooldownSecs -= 1);
      if (_cooldownSecs <= 0) timer.cancel();
    });
  }
}
