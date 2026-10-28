import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/auth/data/datasources/auth_remote_datasource.dart';
import 'package:app/features/auth/domain/models/auth_dto.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/i18n/strings.g.dart';

/// The in-app "confirm your e-mail" nudge, with resend + cooldown.
///
/// Lived only on /home — which onboarding never lands on (the wizard exits
/// to /prospect, the landing screen is /start), so an owner whose signup
/// mail hit spam had NO reachable way to re-trigger it. It now mounts in
/// the AppShell, above whichever branch is showing.
class EmailVerificationBanner extends ConsumerStatefulWidget {
  const EmailVerificationBanner();

  @override
  ConsumerState<EmailVerificationBanner> createState() =>
      EmailVerificationBannerState();
}

class EmailVerificationBannerState
    extends ConsumerState<EmailVerificationBanner> {
  bool _resending = false;
  int _cooldownSecs = 0;
  Timer? _cooldownTimer;

  @override
  void dispose() {
    _cooldownTimer?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final email = ref.watch(authControllerProvider).asData?.value?.email ?? '';
    final disabled = email.isEmpty || _resending || _cooldownSecs > 0;

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
      color: colors.orangeAlert,
      child: Row(
        children: [
          const Icon(Icons.mark_email_unread_outlined,
              color: Colors.white, size: 18),
          const SizedBox(width: 10),
          Expanded(
            child: Text(
              t.email_verification.banner_message,
              style: const TextStyle(
                color: Colors.white,
                fontSize: 13,
                fontWeight: FontWeight.w500,
              ),
            ),
          ),
          const SizedBox(width: 8),
          SizedBox(
            height: 30,
            child: ElevatedButton(
              onPressed: disabled ? null : _resend,
              style: ElevatedButton.styleFrom(
                backgroundColor: Colors.white,
                foregroundColor: colors.orangeAlert,
                disabledBackgroundColor: Colors.white70,
                padding: const EdgeInsets.symmetric(horizontal: 12),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(6),
                ),
              ),
              child: _resending
                  ? const SizedBox(
                      width: 14,
                      height: 14,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : Text(
                      _cooldownSecs > 0
                          ? '${t.email_verification.resend} (${_cooldownSecs}s)'
                          : t.email_verification.resend,
                      style: const TextStyle(
                          fontWeight: FontWeight.w700, fontSize: 12),
                    ),
            ),
          ),
        ],
      ),
    );
  }

  Future<void> _resend() async {
    final email = ref.read(authControllerProvider).asData?.value?.email ?? '';
    if (email.isEmpty) return;
    setState(() => _resending = true);
    try {
      final ds = AuthRemoteDataSource(ref.read(dioProvider));
      await ds.resendVerification(ForgotPasswordDTO(email: email));
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
