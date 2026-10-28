import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/components/primary_pill_button.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/auth/data/datasources/auth_remote_datasource.dart';
import 'package:app/i18n/strings.g.dart';

/// Lands here from the verification email link
/// (app.replider.com/verify-email?token=...). Consumes the token by
/// calling GET /v1/auth/verify-email and shows the result. The bare API
/// endpoint used to be linked directly, which 404'd on the app host —
/// this screen keeps the whole flow on the branded app origin.
class EmailVerifyTokenScreen extends ConsumerStatefulWidget {
  final String token;

  const EmailVerifyTokenScreen({super.key, required this.token});

  @override
  ConsumerState<EmailVerifyTokenScreen> createState() =>
      _EmailVerifyTokenScreenState();
}

enum _Status { verifying, success, failure }

class _EmailVerifyTokenScreenState
    extends ConsumerState<EmailVerifyTokenScreen> {
  _Status _status = _Status.verifying;
  String _error = '';

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _verify());
  }

  Future<void> _verify() async {
    if (widget.token.isEmpty) {
      setState(() {
        _status = _Status.failure;
        _error = t.email_verification.verify_failed_subtitle;
      });
      return;
    }
    try {
      final ds = AuthRemoteDataSource(ref.read(dioProvider));
      await ds.verifyEmail(widget.token);
      if (!mounted) return;
      setState(() => _status = _Status.success);
    } on DioException catch (e) {
      if (!mounted) return;
      setState(() {
        _status = _Status.failure;
        _error = parseApiError(e);
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _status = _Status.failure;
        _error = t.email_verification.verify_failed_subtitle;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    return Scaffold(
      backgroundColor: const Color(0xFF1E1E1E),
      body: SafeArea(
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 400),
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 24),
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  _icon(colors),
                  const SizedBox(height: 24),
                  Text(
                    _title(),
                    textAlign: TextAlign.center,
                    style: typography.display.copyWith(
                      fontSize: 24,
                      color: Colors.white,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  if (_status != _Status.verifying) ...[
                    const SizedBox(height: 12),
                    Text(
                      _subtitle(),
                      textAlign: TextAlign.center,
                      style: typography.body.copyWith(
                        color: colors.textSecondary,
                        fontSize: 14,
                      ),
                    ),
                    const SizedBox(height: 32),
                    PrimaryPillButton(
                      text: t.email_verification.back_to_login,
                      onPressed: () => context.go('/'),
                    ),
                  ],
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _icon(AppColorsExtension colors) {
    switch (_status) {
      case _Status.verifying:
        return const SizedBox(
          height: 64,
          child: Center(
            child: SizedBox(
              width: 36,
              height: 36,
              child: CircularProgressIndicator(strokeWidth: 3),
            ),
          ),
        );
      case _Status.success:
        return Icon(Icons.check_circle_outline,
            size: 64, color: colors.whatsappGreen);
      case _Status.failure:
        return Icon(Icons.error_outline, size: 64, color: colors.redAlert);
    }
  }

  String _title() {
    switch (_status) {
      case _Status.verifying:
        return t.email_verification.verifying;
      case _Status.success:
        return t.email_verification.verified_title;
      case _Status.failure:
        return t.email_verification.verify_failed_title;
    }
  }

  String _subtitle() {
    if (_status == _Status.success) {
      return t.email_verification.verified_subtitle;
    }
    return _error.isNotEmpty
        ? _error
        : t.email_verification.verify_failed_subtitle;
  }
}
