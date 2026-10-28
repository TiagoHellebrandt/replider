import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:app/design_system/components/custom_text_field.dart';
import 'package:app/design_system/components/primary_pill_button.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/auth/data/datasources/public_config_provider.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/auth/presentation/controllers/mfa_login_controller.dart';
import 'package:app/features/auth/presentation/widgets/google_sign_in_button.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/core/auth/session_expiry.dart';
import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/validators/email_validator.dart';

class LoginScreen extends ConsumerStatefulWidget {
  const LoginScreen({super.key});

  @override
  ConsumerState<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends ConsumerState<LoginScreen> {
  final _formKey = GlobalKey<FormState>();
  final _emailController = TextEditingController();
  final _passwordController = TextEditingController();
  final _mfaCodeController = TextEditingController();

  /// The last login failure, rendered INLINE under the fields. The snackbar
  /// alone dismissed itself in four seconds — on a slow network the person
  /// came back to a silent form and retyped a password that was never the
  /// problem.
  String? _authError;

  void _onInputChanged(String _) {
    if (_authError != null) _authError = null;
    setState(() {}); // Trigger rebuild
  }

  void _submit() {
    if (_formKey.currentState!.validate()) {
      ref.read(authControllerProvider.notifier).login(
            _emailController.text,
            _passwordController.text,
          );
    }
  }

  void _submitMfaCode() {
    final code = _mfaCodeController.text.trim();
    if (code.isEmpty) return;
    ref.read(authControllerProvider.notifier).verifyMfaLogin(code);
  }

  void _cancelMfa() {
    _mfaCodeController.clear();
    ref.read(mfaLoginControllerProvider.notifier).clear();
  }

  @override
  void dispose() {
    _emailController.dispose();
    _passwordController.dispose();
    _mfaCodeController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    // We try to match the dark aesthetic of the new reference image exactly.
    // The reference uses a dark gray background very close to #1C1C1E.
    // We will use #1C1C1E to match, or rely on backgroundPrimary which is #0A0A0A.
    // We will tint it slightly to #1C1C1E since the reference looks soft dark.
    final backgroundColor =
        const Color(0xFF1E1E1E); // Soft dark gray matching reference
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    // Keep the reactive dependency on auth state (rebuild on login/logout)
    // without binding an unused local — error/navigation is handled by the
    // ref.listen below.
    ref.watch(authControllerProvider);

    // Listen to changes for error handling and navigation
    ref.listen<AsyncValue<UserModel?>>(authControllerProvider,
        (previous, next) {
      if (next is AsyncError) {
        final message = parseApiError(next.error);
        setState(() => _authError = message);
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
      } else if (next is AsyncData && next.value != null) {
        final user = next.value!;
        // The notice has done its job the moment the person signs back in;
        // leaving it raised would announce an expiry on the next deliberate
        // sign-out.
        ref.read(sessionExpiryProvider.notifier).clear();
        if (user.isNew) {
          context.go('/onboarding');
          return;
        }
        // Back to wherever they were thrown out of. `/start` is the fallback —
        // NOT `/home`, which is the conversation inbox and was sending every
        // returning user to a screen the router itself never picks.
        final from = sanitizeReturnPath(
          Uri.decodeComponent(
              GoRouterState.of(context).uri.queryParameters['from'] ?? ''),
        );
        context.go(from ?? '/start');
      }
    });

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
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      if (ref.watch(sessionExpiryProvider))
                        const _SessionExpiredNotice(),
                      ref.watch(mfaLoginControllerProvider) != null
                          ? _buildMfaForm(constraints.maxWidth > 800)
                          : _buildLoginForm(constraints.maxWidth > 800),
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

  Widget _buildLoginForm(bool isWide) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final isLoading = ref.watch(authControllerProvider).isLoading;
    final bool isFormFilled =
        _emailController.text.isNotEmpty && _passwordController.text.isNotEmpty;
    // Reduce heights if mobile to guarantee a better fit
    final double spacingHuge = isWide ? 64 : 32;
    final double spacingLarge = isWide ? 48 : 32;
    final double spacingMedium = isWide ? 32 : 24;

    return Form(
      key: _formKey,
      // Same autofill contract as the register screen: hints keep password
      // managers from guessing (and overlaying) the fields.
      child: AutofillGroup(
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
            SizedBox(height: spacingHuge),

            // Title
            Text(
              t.auth.login_title,
              style: typography.display.copyWith(
                  fontSize: 28,
                  color: Colors.white,
                  fontWeight: FontWeight.w600),
              textAlign: TextAlign.center,
            ),
            SizedBox(height: spacingLarge),

            // Inputs
            CustomTextField(
              key: const Key('email_input'),
              hint: t.auth.email_placeholder,
              prefixIcon: Icons.mail_outline,
              autofillHints: const [
                AutofillHints.username,
                AutofillHints.email
              ],
              controller: _emailController,
              keyboardType: TextInputType.emailAddress,
              autofocus: true,
              onChanged: _onInputChanged,
              validator: (v) => isValidEmail(v) ? null : t.auth.invalid_email,
            ),
            const SizedBox(height: 16),
            CustomTextField(
              key: const Key('password_input'),
              hint: t.auth.password_placeholder,
              prefixIcon: Icons.lock_outline,
              isPassword: true,
              autofillHints: const [AutofillHints.password],
              controller: _passwordController,
              onChanged: _onInputChanged,
              onSubmitted: (_) => _submit(),
              validator: (v) =>
                  v != null && v.length >= 8 ? null : t.auth.password_too_short,
            ),
            const SizedBox(height: 16),

            if (_authError != null) ...[
              Text(
                _authError!,
                key: const Key('login_inline_error'),
                style: typography.body
                    .copyWith(color: colors.redAlert, fontSize: 13),
              ),
              const SizedBox(height: 12),
            ],

            // Forgot password. The session is always persisted (the
            // AuthInterceptor depends on the stored token), so there is no
            // "remember me" toggle — a logged-in session would be unusable
            // if its token weren't written to storage.
            Align(
              alignment: Alignment.centerRight,
              child: TextButton(
                onPressed: () => context.go('/forgot-password'),
                style: TextButton.styleFrom(
                  padding: EdgeInsets.zero,
                  minimumSize: Size.zero,
                  tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                ),
                child: Text(
                  t.auth.forgot_password,
                  style: typography.body
                      .copyWith(color: colors.textSecondary, fontSize: 13),
                ),
              ),
            ),
            SizedBox(height: spacingMedium),

            // Login Button
            PrimaryPillButton(
              text: t.auth.login_button,
              isLoading: isLoading,
              onPressed: isFormFilled && !isLoading ? _submit : null,
            ),
            SizedBox(height: spacingMedium),

            // Divider + Social Buttons (only shown if at least one social
            // provider is available — see `isGoogleSignInAvailable`).
            if (isGoogleSignInAvailable) ...[
              Row(
                children: [
                  Expanded(child: Divider(color: colors.borderMedium)),
                  Padding(
                    padding: const EdgeInsets.symmetric(horizontal: 16),
                    child: Text(
                      t.auth.or_login_with,
                      style: typography.body
                          .copyWith(color: colors.textSecondary, fontSize: 14),
                    ),
                  ),
                  Expanded(child: Divider(color: colors.borderMedium)),
                ],
              ),
              SizedBox(height: spacingMedium),
              const Row(
                children: [
                  Expanded(
                    child: GoogleSignInButton(),
                  ),
                  // Meta/Facebook OAuth button hidden until SDK integration is complete
                ],
              ),
              SizedBox(height: spacingLarge),
            ],

            // Bottom Prompt. Hidden while signup is paused (US-F001) —
            // pointing at /register would only land the user on the
            // "em breve" screen. FAIL-CLOSED: while the flag is loading or
            // errored the link stays hidden. In production the door is
            // closed, so failing open made the dead-end path ("Criar conta"
            // → "Cadastro em breve") the most likely first experience.
            if (ref
                    .watch(publicConfigProvider)
                    .asData
                    ?.value
                    .registrationEnabled ??
                false)
              Wrap(
                alignment: WrapAlignment.center,
                crossAxisAlignment: WrapCrossAlignment.center,
                children: [
                  Text(
                    t.auth.dont_have_account,
                    style: typography.body
                        .copyWith(color: colors.textSecondary, fontSize: 14),
                  ),
                  TextButton(
                    onPressed: () => context.go('/register'),
                    style: TextButton.styleFrom(
                      padding: const EdgeInsets.only(left: 4),
                      minimumSize: Size.zero,
                      tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                    ),
                    child: Text(
                      t.auth.create_account,
                      style: typography.body.copyWith(
                          color: Colors.white,
                          fontSize: 14,
                          fontWeight: FontWeight.w500),
                    ),
                  ),
                ],
              ),
          ],
        ),
      ),
    );
  }

  Widget _buildMfaForm(bool isWide) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final isLoading = ref.watch(authControllerProvider).isLoading;
    final double spacingHuge = isWide ? 64 : 32;
    final double spacingLarge = isWide ? 48 : 32;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
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
                  fontSize: 20,
                  color: Colors.white,
                  fontWeight: FontWeight.w600),
            ),
          ],
        ),
        SizedBox(height: spacingHuge),
        Text(
          t.auth.mfa_title,
          style: typography.display.copyWith(
              fontSize: 24, color: Colors.white, fontWeight: FontWeight.w600),
          textAlign: TextAlign.center,
        ),
        const SizedBox(height: 12),
        Text(
          t.auth.mfa_subtitle,
          style: typography.body
              .copyWith(color: colors.textSecondary, fontSize: 14),
          textAlign: TextAlign.center,
        ),
        SizedBox(height: spacingLarge),
        CustomTextField(
          key: const Key('mfa_code_input'),
          hint: t.auth.mfa_code_hint,
          prefixIcon: Icons.shield_outlined,
          controller: _mfaCodeController,
          // The button below follows this field.
          onChanged: (_) => setState(() {}),
          keyboardType: TextInputType.number,
          autofocus: true,
          onSubmitted: (_) => _submitMfaCode(),
        ),
        const SizedBox(height: 24),
        PrimaryPillButton(
          text: t.auth.mfa_verify_button,
          isLoading: isLoading,
          onPressed: isLoading || _mfaCodeController.text.trim().isEmpty
              ? null
              : _submitMfaCode,
        ),
        const SizedBox(height: 16),
        TextButton(
          onPressed: isLoading ? null : _cancelMfa,
          child: Text(
            t.auth.mfa_cancel,
            style: typography.body
                .copyWith(color: colors.textSecondary, fontSize: 13),
          ),
        ),
      ],
    );
  }

  // To match the exact background color needed:
  final Color backgroundColor =
      const Color(0xFF222428); // Extracted soft dark gray from reference
}

/// Says why the login screen showed up on its own.
///
/// Sessions last 24h and there is no refresh, so a person in the middle of a
/// working day gets dropped here without touching anything. Unannounced, that
/// reads as "the app broke" or "someone kicked me out" — and the next step is a
/// call to support. One sentence turns a scare into an inconvenience.
class _SessionExpiredNotice extends StatelessWidget {
  const _SessionExpiredNotice();

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    return Container(
      key: const Key('session_expired_notice'),
      width: double.infinity,
      margin: const EdgeInsets.only(bottom: 20),
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      decoration: BoxDecoration(
        color: colors.orangeAlert.withValues(alpha: 0.14),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: colors.orangeAlert.withValues(alpha: 0.45)),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(Icons.schedule, size: 18, color: colors.orangeAlert),
          const SizedBox(width: 10),
          Expanded(
            child: Text(
              t.auth.session_expired_notice,
              style: TextStyle(color: colors.textSecondary, fontSize: 13),
            ),
          ),
        ],
      ),
    );
  }
}
