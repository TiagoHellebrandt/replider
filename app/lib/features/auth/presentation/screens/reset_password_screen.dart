import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:app/design_system/components/custom_text_field.dart';
import 'package:app/design_system/components/primary_pill_button.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:app/core/error/api_error_parser.dart';

/// Landing for the password-reset link emailed by the API
/// (`APP_BASE_URL/reset-password?token=...`). Reads the token from the query
/// param, lets the user set a new password, and calls the reset endpoint. On
/// success it routes back to login. Public route (no auth required).
class ResetPasswordScreen extends ConsumerStatefulWidget {
  final String token;

  const ResetPasswordScreen({super.key, required this.token});

  @override
  ConsumerState<ResetPasswordScreen> createState() =>
      _ResetPasswordScreenState();
}

class _ResetPasswordScreenState extends ConsumerState<ResetPasswordScreen> {
  final _formKey = GlobalKey<FormState>();
  final _passwordController = TextEditingController();
  final _confirmController = TextEditingController();
  bool _busy = false;
  bool _done = false;

  void _onInputChanged(String _) => setState(() {});

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() => _busy = true);
    try {
      await ref
          .read(authControllerProvider.notifier)
          .resetPassword(widget.token, _passwordController.text);
      if (!mounted) return;
      setState(() => _done = true);
    } catch (error) {
      if (!mounted) return;
      final colors = Theme.of(context).extension<AppColorsExtension>()!;
      ScaffoldMessenger.of(context).clearSnackBars();
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(parseApiError(error)),
          backgroundColor: colors.redAlert,
          behavior: SnackBarBehavior.floating,
          margin: const EdgeInsets.all(16),
          shape:
              RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
        ),
      );
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  void dispose() {
    _passwordController.dispose();
    _confirmController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    const backgroundColor = Color(0xFF1E1E1E);
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
                  child: widget.token.isEmpty
                      ? _invalidLink(colors, typography)
                      : _buildForm(colors, typography),
                ),
              ),
            );
          },
        ),
      ),
    );
  }

  Widget _logo(AppTypographyExtension typography) => Row(
        mainAxisAlignment: MainAxisAlignment.center,
        mainAxisSize: MainAxisSize.min,
        children: [
          Image.asset(
            'assets/logo_white.webp',
            height: 24,
            errorBuilder: (context, error, stackTrace) =>
                const Icon(Icons.layers, size: 24, color: Color(0xFF4A89F3)),
          ),
          const SizedBox(width: 8),
          Text(
            'Replider',
            style: typography.display.copyWith(
                fontSize: 20, color: Colors.white, fontWeight: FontWeight.w600),
          ),
        ],
      );

  // Token missing/blank — the link was malformed. Point the user back to the
  // forgot-password flow to request a fresh link.
  Widget _invalidLink(
      AppColorsExtension colors, AppTypographyExtension typography) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _logo(typography),
        const SizedBox(height: 32),
        Text(
          t.auth.reset_invalid_link,
          style: typography.body
              .copyWith(color: colors.textSecondary, fontSize: 14),
          textAlign: TextAlign.center,
        ),
        const SizedBox(height: 32),
        PrimaryPillButton(
          text: t.auth.forgot_send_link,
          onPressed: () => context.go('/forgot-password'),
        ),
      ],
    );
  }

  Widget _buildForm(
      AppColorsExtension colors, AppTypographyExtension typography) {
    final filled = _passwordController.text.isNotEmpty &&
        _confirmController.text.isNotEmpty;

    if (_done) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _logo(typography),
          const SizedBox(height: 32),
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
              t.auth.reset_success,
              style: typography.body.copyWith(
                  color: const Color(0xFF34A853), fontWeight: FontWeight.w500),
              textAlign: TextAlign.center,
            ),
          ),
          PrimaryPillButton(
            key: const Key('go_login_button'),
            text: t.auth.back_to_login,
            onPressed: () => context.go('/'),
          ),
        ],
      );
    }

    return Form(
      key: _formKey,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _logo(typography),
          const SizedBox(height: 32),
          Text(
            t.auth.reset_title,
            style: typography.display.copyWith(
                fontSize: 28, color: Colors.white, fontWeight: FontWeight.w600),
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: 8),
          Text(
            t.auth.reset_subtitle,
            style: typography.body
                .copyWith(color: colors.textSecondary, fontSize: 14),
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: 32),
          CustomTextField(
            key: const Key('password_input'),
            hint: t.auth.reset_new_password,
            prefixIcon: Icons.lock_outline,
            controller: _passwordController,
            isPassword: true,
            autofocus: true,
            onChanged: _onInputChanged,
            validator: (v) =>
                (v != null && v.length >= 8) ? null : t.auth.reset_min_length,
          ),
          const SizedBox(height: 16),
          CustomTextField(
            key: const Key('confirm_input'),
            hint: t.auth.reset_confirm_password,
            prefixIcon: Icons.lock_outline,
            controller: _confirmController,
            isPassword: true,
            onChanged: _onInputChanged,
            onSubmitted: (_) {
              if (filled && !_busy) _submit();
            },
            validator: (v) =>
                v == _passwordController.text ? null : t.auth.reset_mismatch,
          ),
          const SizedBox(height: 32),
          PrimaryPillButton(
            key: const Key('submit_button'),
            text: t.auth.reset_submit,
            // The spinner every other auth button already shows — a grey
            // button with no motion reads as broken during a slow request.
            isLoading: _busy,
            onPressed: (filled && !_busy) ? _submit : null,
          ),
          const SizedBox(height: 32),
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
