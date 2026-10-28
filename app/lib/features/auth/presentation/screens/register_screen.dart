import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:go_router/go_router.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/components/custom_text_field.dart';
import 'package:app/design_system/components/primary_pill_button.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/auth/data/datasources/public_config_provider.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/auth/presentation/widgets/google_sign_in_button.dart';
import 'package:app/features/owner/presentation/controllers/coupon_controller.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/validators/email_validator.dart';

/// Storage key the post-register billing flow reads to find a
/// pending coupon. Cleared on successful checkout.
const kPendingCouponKey = 'pending_coupon';

class RegisterScreen extends ConsumerStatefulWidget {
  /// Public-facing coupon slug from `?coupon=…` on the marketing
  /// site / invite link. We render a banner so the user knows the
  /// discount will be applied at checkout, then hand it to the
  /// post-register billing flow via FlutterSecureStorage so the
  /// upgrade screen can forward it to /v1/payment/checkout.
  final String? couponCode;

  /// One-shot signup invite from `/register?invite=…` (sales-led flow). The
  /// screen probes it, locks the e-mail the invite is bound to, and sends
  /// the token with the signup — the server refuses everything else.
  final String? inviteToken;

  const RegisterScreen({super.key, this.couponCode, this.inviteToken});

  @override
  ConsumerState<RegisterScreen> createState() => _RegisterScreenState();
}

class _RegisterScreenState extends ConsumerState<RegisterScreen> {
  final _formKey = GlobalKey<FormState>();
  final _accountNameController = TextEditingController();
  final _nameController = TextEditingController();
  final _emailController = TextEditingController();
  final _passwordController = TextEditingController();
  final _confirmPasswordController = TextEditingController();
  bool _termsAccepted = false;

  /// Probe result: null while loading (or no invite), true/false afterwards.
  bool? _inviteValid;

  /// The probe itself failed (network, server down) — which says nothing
  /// about the INVITE. Rendering that as "this link is invalid" told a
  /// customer with a perfectly good invite and bad wifi that their link was
  /// dead, with no way to try again.
  bool _probeFailed = false;

  bool get _hasInvite =>
      widget.inviteToken != null && widget.inviteToken!.trim().isNotEmpty;

  Future<void> _probeInvite() async {
    final token = widget.inviteToken!.trim();
    if (mounted && _probeFailed) {
      setState(() => _probeFailed = false);
    }
    try {
      final res = await ref.read(dioProvider).get<Map<String, dynamic>>(
        '/invites/probe',
        queryParameters: <String, dynamic>{'token': token},
      );
      if (!mounted) return;
      final valid = res.data?['valid'] == true;
      setState(() {
        _inviteValid = valid;
        if (valid) {
          // The invite is bound to this e-mail server-side; locking the
          // field just spares the person a guaranteed refusal.
          _emailController.text = (res.data?['email'] as String?) ?? '';
        }
      });
    } catch (_) {
      // A probe that could not RUN is not a verdict about the invite.
      if (mounted) setState(() => _probeFailed = true);
    }
  }

  @override
  void initState() {
    super.initState();
    if (_hasInvite) {
      Future.microtask(_probeInvite);
    }
    final code = widget.couponCode?.trim();
    if (code != null && code.isNotEmpty) {
      // Stash for the post-register billing flow. Best-effort —
      // the upgrade screen reads this to forward to /payment/checkout
      // and clears it on success.
      const FlutterSecureStorage()
          .write(key: kPendingCouponKey, value: code.toUpperCase());
    }
  }

  void _onInputChanged(String _) {
    setState(() {}); // Trigger rebuild for button enablement
  }

  void _submit() {
    if (!_termsAccepted) {
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(
        content: Text(t.auth.terms_required),
        behavior: SnackBarBehavior.floating,
      ));
      return;
    }
    if (_formKey.currentState!.validate()) {
      ref.read(authControllerProvider.notifier).register(
            _accountNameController.text,
            _nameController.text,
            _emailController.text,
            _passwordController.text,
            termsAccepted: _termsAccepted,
            inviteToken: _hasInvite ? widget.inviteToken!.trim() : null,
          );
    }
  }

  @override
  void dispose() {
    _accountNameController.dispose();
    _nameController.dispose();
    _emailController.dispose();
    _passwordController.dispose();
    _confirmPasswordController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final backgroundColor =
        const Color(0xFF1E1E1E); // Soft dark gray matching reference
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;

    // Listen to changes for error handling and navigation
    ref.listen<AsyncValue<UserModel?>>(authControllerProvider,
        (previous, next) {
      if (next is AsyncError) {
        final message = parseApiError(next.error);
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
        final email = _emailController.text;
        context.go('/verify-email?email=${Uri.encodeComponent(email)}');
      }
    });

    return Scaffold(
      backgroundColor: backgroundColor,
      body: SafeArea(
        child: LayoutBuilder(
          builder: (context, constraints) {
            final isDesktop =
                constraints.maxWidth > 600; // Example breakpoint for desktop
            return SingleChildScrollView(
              child: Container(
                padding: const EdgeInsets.symmetric(
                    horizontal: 24.0, vertical: 16.0),
                constraints: BoxConstraints(minHeight: constraints.maxHeight),
                alignment: Alignment.center,
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 400),
                  // US-F001: signup can be closed server-side. While the
                  // flag is in flight we render a spinner rather than the
                  // form, so a paused deploy never flashes a door that
                  // isn't open. Any error → form (the API still refuses
                  // with 503 REGISTRATION_PAUSED).
                  child: ref.watch(publicConfigProvider).when(
                        loading: () => const Padding(
                          padding: EdgeInsets.symmetric(vertical: 64),
                          child: Center(child: CircularProgressIndicator()),
                        ),
                        error: (_, __) =>
                            _buildRegisterForm(colors, typography, isDesktop),
                        // A VALID invite is the permission that passes a
                        // closed door. A dead one gets a dead end — showing
                        // the form under an "invalid link" banner invites the
                        // person to fill five fields that can only be refused
                        // at the end.
                        data: (config) {
                          if (config.registrationEnabled) {
                            return _buildRegisterForm(
                                colors, typography, isDesktop);
                          }
                          if (!_hasInvite) {
                            return _buildRegistrationPaused(colors, typography);
                          }
                          if (_probeFailed) {
                            return _buildProbeFailed(colors, typography);
                          }
                          if (_inviteValid == null) {
                            // Probe still in flight — don't flash either
                            // outcome.
                            return const Padding(
                              padding: EdgeInsets.symmetric(vertical: 64),
                              child: Center(child: CircularProgressIndicator()),
                            );
                          }
                          return _inviteValid == true
                              ? _buildRegisterForm(
                                  colors, typography, isDesktop)
                              : _buildInviteInvalid(colors, typography);
                        },
                      ),
                ),
              ),
            );
          },
        ),
      ),
    );
  }

  /// "Em breve" state shown when the API reports registration is paused
  /// (US-F001). No form, no social buttons — the only exit is login, so a
  /// returning user is never stranded here.
  Widget _buildRegistrationPaused(
      AppColorsExtension colors, AppTypographyExtension typography) {
    return Column(
      key: const Key('registration_paused'),
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
        const SizedBox(height: 40),
        Icon(Icons.hourglass_top, size: 40, color: colors.bluePrimary),
        const SizedBox(height: 16),
        Text(
          t.auth.register_paused_title,
          style: typography.display.copyWith(
              fontSize: 24, color: Colors.white, fontWeight: FontWeight.w600),
          textAlign: TextAlign.center,
        ),
        const SizedBox(height: 12),
        Text(
          t.auth.register_paused_body,
          style: typography.body
              .copyWith(color: colors.textSecondary, fontSize: 14),
          textAlign: TextAlign.center,
        ),
        const SizedBox(height: 32),
        PrimaryPillButton(
          key: const Key('paused_go_to_login_button'),
          text: t.auth.sign_in,
          onPressed: () => context.go('/'),
        ),
      ],
    );
  }

  /// The probe could not run — network, not verdict. Retry, never a
  /// pronouncement about a link nobody managed to check.
  Widget _buildProbeFailed(
      AppColorsExtension colors, AppTypographyExtension typography) {
    return Column(
      key: const Key('invite_probe_failed'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const SizedBox(height: 40),
        Icon(Icons.wifi_off_outlined, size: 40, color: colors.orangeAlert),
        const SizedBox(height: 16),
        Text(
          t.auth.invite_probe_failed,
          style: typography.body
              .copyWith(color: colors.textSecondary, fontSize: 14),
          textAlign: TextAlign.center,
        ),
        const SizedBox(height: 24),
        PrimaryPillButton(
          key: const Key('invite_probe_retry'),
          text: t.common.retry,
          onPressed: () {
            setState(() => _inviteValid = null);
            _probeInvite();
          },
        ),
      ],
    );
  }

  /// The dead end for a dead invite: says what happened and where to go,
  /// instead of a form that can only be refused after five fields.
  Widget _buildInviteInvalid(
      AppColorsExtension colors, AppTypographyExtension typography) {
    return Column(
      key: const Key('invite_dead_end'),
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
        const SizedBox(height: 40),
        Icon(Icons.link_off, size: 40, color: colors.orangeAlert),
        const SizedBox(height: 16),
        Text(
          t.auth.invite_invalid_title,
          style: typography.display.copyWith(
              fontSize: 24, color: Colors.white, fontWeight: FontWeight.w600),
          textAlign: TextAlign.center,
        ),
        const SizedBox(height: 12),
        Text(
          t.auth.invite_invalid,
          style: typography.body
              .copyWith(color: colors.textSecondary, fontSize: 14),
          textAlign: TextAlign.center,
        ),
        const SizedBox(height: 32),
        PrimaryPillButton(
          key: const Key('invite_dead_end_login_button'),
          text: t.auth.sign_in,
          onPressed: () => context.go('/'),
        ),
      ],
    );
  }

  Widget _buildRegisterForm(AppColorsExtension colors,
      AppTypographyExtension typography, bool isDesktop) {
    final isLoading = ref.watch(authControllerProvider).isLoading;
    final bool isFormFilled = _accountNameController.text.isNotEmpty &&
        _nameController.text.isNotEmpty &&
        _emailController.text.isNotEmpty &&
        _passwordController.text.isNotEmpty &&
        _confirmPasswordController.text.isNotEmpty;

    return Form(
      key: _formKey,
      // AutofillGroup + per-field hints tell browsers and password managers
      // exactly what each field is. Without them an extension guesses, and a
      // wrong guess overlays the field — the confirm-password input was
      // unclickable under one until this landed.
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
            const SizedBox(height: 32),

            // Title
            Text(
              t.auth.register_title,
              style: typography.display.copyWith(
                  fontSize: 28,
                  color: Colors.white,
                  fontWeight: FontWeight.w600),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 8),
            Text(
              // Invited customers are ACTIVATING a contract that was already
              // sold; "start using Replider today" is self-service copy
              // written for a door that is closed.
              _hasInvite
                  ? t.auth.register_subtitle_invite
                  : t.auth.register_subtitle,
              style: typography.body
                  .copyWith(color: colors.textSecondary, fontSize: 14),
              textAlign: TextAlign.center,
            ),
            if (widget.couponCode != null &&
                widget.couponCode!.trim().isNotEmpty) ...[
              const SizedBox(height: 16),
              _CouponBanner(code: widget.couponCode!),
            ],
            if (_hasInvite && _inviteValid == true) ...[
              const SizedBox(height: 16),
              _InviteBanner(
                key: const Key('invite_banner_valid'),
                text: t.auth.invite_banner(email: _emailController.text),
                ok: true,
              ),
            ],
            if (_hasInvite && _inviteValid == false) ...[
              const SizedBox(height: 16),
              _InviteBanner(
                key: const Key('invite_banner_invalid'),
                text: t.auth.invite_invalid,
                ok: false,
              ),
            ],
            const SizedBox(height: 32),

            // Inputs. Autofocus on the first field + textInputAction.next
            // on intermediates lets the user blast through the form with
            // the keyboard alone (Enter on mobile, Tab on web). Submitting
            // from the last password field fires the Register button.
            CustomTextField(
              key: const Key('account_name_input'),
              hint: t.auth.account_name,
              prefixIcon: Icons.business,
              autofillHints: const [AutofillHints.organizationName],
              controller: _accountNameController,
              autofocus: true,
              onChanged: _onInputChanged,
              validator: (v) =>
                  v != null && v.isNotEmpty ? null : t.auth.field_required,
            ),
            const SizedBox(height: 16),
            CustomTextField(
              key: const Key('name_input'),
              hint: t.auth.full_name,
              prefixIcon: Icons.person_outline,
              autofillHints: const [AutofillHints.name],
              controller: _nameController,
              onChanged: _onInputChanged,
              validator: (v) =>
                  v != null && v.isNotEmpty ? null : t.auth.field_required,
            ),
            const SizedBox(height: 16),
            CustomTextField(
              key: const Key('email_input'),
              hint: t.auth.email_placeholder,
              prefixIcon: Icons.mail_outline,
              autofillHints: const [AutofillHints.email],
              controller: _emailController,
              keyboardType: TextInputType.emailAddress,
              readOnly: _hasInvite && _inviteValid == true,
              onChanged: _onInputChanged,
              validator: (v) => isValidEmail(v) ? null : t.auth.invalid_email,
            ),
            const SizedBox(height: 16),
            CustomTextField(
              key: const Key('password_input'),
              hint: t.auth.password_placeholder,
              prefixIcon: Icons.lock_outline,
              isPassword: true,
              autofillHints: const [AutofillHints.newPassword],
              controller: _passwordController,
              textInputAction: TextInputAction.next,
              onChanged: _onInputChanged,
              validator: (v) => v != null && v.length >= 8
                  ? null
                  : t.auth.min_characters(n: 8),
            ),
            const SizedBox(height: 16),
            CustomTextField(
              key: const Key('confirm_password_input'),
              hint: t.auth.confirm_password,
              prefixIcon: Icons.lock_outline,
              isPassword: true,
              autofillHints: const [AutofillHints.newPassword],
              controller: _confirmPasswordController,
              onChanged: _onInputChanged,
              onSubmitted: (_) => _submit(),
              validator: (v) => v == _passwordController.text
                  ? null
                  : t.auth.passwords_dont_match,
            ),
            const SizedBox(height: 16),

            // LGPD: explicit consent checkbox. Required by the backend
            // (TERMS_NOT_ACCEPTED is returned as 400 when missing) and by
            // LGPD Art. 8 — the contract is only valid with explicit accept.
            _termsCheckbox(colors, typography),

            const SizedBox(height: 24),

            // Register Button
            PrimaryPillButton(
              key: const Key('register_button'),
              text: t.auth.register_title,
              isLoading: isLoading,
              onPressed:
                  isFormFilled && _termsAccepted && !isLoading ? _submit : null,
            ),
            const SizedBox(height: 32),

            // Divider + Google sign-up. Google auto-creates the account on
            // first sign-in (auto-verified), so the same one-tap button that
            // logs returning users in also registers new ones — the highest
            // -friction email/password path stops being the only option.
            //
            // Hidden entirely when registration is paused: the Google flow
            // does not carry the invite token, so with the door closed it
            // could only fail with a generic error — or worse, mint an
            // account outside the invite's contract.
            if (isGoogleSignInAvailable &&
                (ref
                        .watch(publicConfigProvider)
                        .asData
                        ?.value
                        .registrationEnabled ??
                    false)) ...[
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
              const SizedBox(height: 16),
              const Row(
                children: [
                  Expanded(child: GoogleSignInButton()),
                ],
              ),
              const SizedBox(height: 32),
            ],

            // Bottom Prompt
            Wrap(
              alignment: WrapAlignment.center,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                Text(
                  t.auth.already_have_account,
                  style: typography.body
                      .copyWith(color: colors.textSecondary, fontSize: 14),
                ),
                TextButton(
                  onPressed: () => context.go('/'),
                  style: TextButton.styleFrom(
                    padding: const EdgeInsets.only(left: 4),
                    minimumSize: Size.zero,
                    tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                  ),
                  child: Text(
                    t.auth.sign_in,
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

  /// LGPD consent block: a Material Checkbox followed by a rich-text legend
  /// with two clickable links — Terms and Privacy Policy. We open the legal
  /// pages on the marketing site (replider.com/legal/*) so the same content
  /// the public sees is the one the user signs.
  Widget _termsCheckbox(
      AppColorsExtension colors, AppTypographyExtension typography) {
    return Row(
      key: const Key('terms_accepted_row'),
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: 24,
          height: 24,
          child: Checkbox(
            key: const Key('terms_accepted_checkbox'),
            value: _termsAccepted,
            activeColor: colors.bluePrimary,
            onChanged: (v) => setState(() => _termsAccepted = v ?? false),
          ),
        ),
        const SizedBox(width: 8),
        Expanded(
          child: GestureDetector(
            onTap: () => setState(() => _termsAccepted = !_termsAccepted),
            child: Text.rich(
              TextSpan(
                style: typography.body.copyWith(
                    color: colors.textSecondary, fontSize: 13, height: 1.4),
                children: [
                  TextSpan(text: t.auth.consent_prefix),
                  TextSpan(
                    text: t.auth.consent_terms,
                    style: TextStyle(
                        color: colors.bluePrimary,
                        decoration: TextDecoration.underline),
                    recognizer: TapGestureRecognizer()
                      ..onTap = () => _openLegal(_legalUrl('terms')),
                  ),
                  TextSpan(text: t.auth.consent_and),
                  TextSpan(
                    text: t.auth.consent_privacy,
                    style: TextStyle(
                        color: colors.bluePrimary,
                        decoration: TextDecoration.underline),
                    recognizer: TapGestureRecognizer()
                      ..onTap = () => _openLegal(_legalUrl('privacy')),
                  ),
                  const TextSpan(text: '.'),
                ],
              ),
            ),
          ),
        ),
      ],
    );
  }

  /// Builds the marketing-site legal URL, picking the language path based
  /// on the device locale. The platform locale (not
  /// LocaleSettings.currentLocale) is deliberate: at this point of the
  /// register flow the Slang locale may still be the 'en' base fallback
  /// while the device is genuinely Portuguese.
  ///
  /// The Astro site routes use English slugs (/en/terms, /en/privacy)
  /// with Portuguese mirrors (/pt/terms, /pt/privacy). Route slugs stay
  /// English so the URL space is consistent across languages.
  String _legalUrl(String slug) {
    final deviceLang = WidgetsBinding
        .instance.platformDispatcher.locale.languageCode
        .toLowerCase();
    final isPt = deviceLang == 'pt';
    return 'https://replider.com/${isPt ? "pt" : "en"}/$slug';
  }

  Future<void> _openLegal(String url) async {
    final uri = Uri.parse(url);
    if (await canLaunchUrl(uri)) {
      await launchUrl(uri, mode: LaunchMode.externalApplication);
    }
  }
}

/// Renders a discount banner when the register URL carried `?coupon=`.
/// Reads the public lookup endpoint to confirm the code is valid
/// before highlighting the percent-off line — invalid codes show a
/// soft warning so the user knows their invite link expired.
class _CouponBanner extends ConsumerWidget {
  final String code;

  const _CouponBanner({required this.code});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final async = ref.watch(couponPublicLookupProvider(code));
    return async.when(
      loading: () => const SizedBox(
        height: 32,
        child: Center(
            child: SizedBox(
                width: 16,
                height: 16,
                child: CircularProgressIndicator(strokeWidth: 2))),
      ),
      error: (_, __) => const SizedBox.shrink(),
      data: (lookup) {
        if (lookup == null) return const SizedBox.shrink();
        final ok = lookup.valid;
        final color = ok ? colors.limeGlow : colors.orangeAlert;
        return Container(
          padding: const EdgeInsets.all(12),
          decoration: BoxDecoration(
            color: color.withValues(alpha: 0.18),
            borderRadius: BorderRadius.circular(10),
            border: Border.all(color: color.withValues(alpha: 0.5)),
          ),
          child: Row(
            children: [
              Icon(
                ok ? Icons.local_offer_outlined : Icons.warning_amber_outlined,
                color: color,
                size: 18,
              ),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  ok
                      ? t.auth.coupon_banner_valid(
                          code: lookup.code,
                          percent: '${lookup.percentOff}',
                        )
                      : t.auth.coupon_banner_invalid(code: lookup.code),
                  style: TextStyle(
                    color: color,
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}

/// The invite's state, said out loud before anyone types a password: a valid
/// link names the e-mail it is bound to; a dead one says to ask for a new
/// link instead of letting the form fail at the end.
class _InviteBanner extends StatelessWidget {
  const _InviteBanner({super.key, required this.text, required this.ok});

  final String text;
  final bool ok;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final tone = ok ? colors.whatsappGreen : colors.orangeAlert;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        color: tone.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: tone.withValues(alpha: 0.35)),
      ),
      child: Row(
        children: [
          Icon(ok ? Icons.mark_email_read_outlined : Icons.link_off,
              size: 18, color: tone),
          const SizedBox(width: 8),
          Expanded(
            child: Text(text,
                style: TextStyle(color: colors.textSecondary, fontSize: 13)),
          ),
        ],
      ),
    );
  }
}
