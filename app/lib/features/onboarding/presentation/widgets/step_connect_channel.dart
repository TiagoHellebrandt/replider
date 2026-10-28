import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:app/core/analytics/analytics_client.dart';
import 'package:app/design_system/components/channel_tile.dart';
import 'package:app/design_system/theme_extensions.dart';
import 'package:app/features/onboarding/presentation/controllers/onboarding_controller.dart';
import 'package:app/features/settings/domain/models/channel_model.dart';
import 'package:app/features/settings/presentation/widgets/google_mailbox_connect_card.dart';
import 'package:app/features/settings/presentation/widgets/whatsapp_embedded_signup_button.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:url_launcher/url_launcher.dart';

// "Como conectar o WhatsApp" tutorial URL. Empty until the video is
// recorded + hosted — the help link below the Embedded Signup button is
// hidden while this is empty so we never open a dead link. Supply at
// build time once the video exists:
//   --dart-define=WHATSAPP_HELP_VIDEO_URL=https://youtu.be/xxxx
const _kWhatsAppHelpVideoUrl =
    String.fromEnvironment('WHATSAPP_HELP_VIDEO_URL');

class StepConnectChannel extends ConsumerStatefulWidget {
  const StepConnectChannel({super.key});

  @override
  ConsumerState<StepConnectChannel> createState() => _StepConnectChannelState();
}

class _StepConnectChannelState extends ConsumerState<StepConnectChannel> {
  final _emailFromController = TextEditingController();
  final _emailHostController = TextEditingController();
  final _emailPortController = TextEditingController(text: '587');
  final _emailUserController = TextEditingController();
  final _emailPasswordController = TextEditingController();
  final _telegramTokenController = TextEditingController();
  final _whatsappPhoneIdController = TextEditingController();
  final _instagramPageIdController = TextEditingController();
  final _instagramTokenController = TextEditingController();
  final _smsAccountSidController = TextEditingController();
  final _smsAuthTokenController = TextEditingController();
  final _smsFromNumberController = TextEditingController();

  /// The SMTP fields are the graduation path (own domain, volume), not the
  /// door. Folded by default so the one-click Gmail card is the first —
  /// and for most people the only — thing on the email tile.
  bool _showSmtp = false;

  /// WhatsApp, Telegram, Instagram and SMS are not what the first offer goes
  /// out on. Behind "Outros canais" by default: five tiles above the Gmail
  /// card made the one-click door look like one option among many. Opens
  /// itself when a resumed draft already picked one of them.
  bool _showOtherChannels = false;

  @override
  void initState() {
    super.initState();
    // Resume: the draft restored channel_config (secrets stripped), but
    // the controllers started empty and SMTP folded — so a resumed setup
    // POSTed a credential-less config against a form that LOOKED blank.
    // Seed what the draft knows and unfold the section it belongs to; the
    // person re-types only the password, which is the deal the draft made.
    final cfg = ref.read(onboardingControllerProvider).channelConfig;
    String cfgStr(String k) => cfg[k] ?? '';
    _emailFromController.text = cfgStr('from_address');
    _emailHostController.text = cfgStr('smtp_host');
    if (cfgStr('smtp_port').isNotEmpty) {
      _emailPortController.text = cfgStr('smtp_port');
    }
    _emailUserController.text = cfgStr('smtp_username');
    _telegramTokenController.text = cfgStr('bot_token');
    _whatsappPhoneIdController.text = cfgStr('phone_number_id');
    _instagramPageIdController.text = cfgStr('page_id');
    _smsAccountSidController.text = cfgStr('account_sid');
    _smsFromNumberController.text = cfgStr('from_number');
    _showSmtp = cfgStr('smtp_host').isNotEmpty;
    final resumedType = ref.read(onboardingControllerProvider).channelType;
    _showOtherChannels = resumedType != null && resumedType != 'email';

    // Email is the default: it is the channel the first offer goes out on,
    // and a person who has never seen this screen should not have to pick
    // before she can connect. Deferred a frame — mutating the provider
    // during build is what the framework forbids.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      if (ref.read(onboardingControllerProvider).channelType == null) {
        ref
            .read(onboardingControllerProvider.notifier)
            .updateField('channelType', 'email');
      }
    });
  }

  @override
  void dispose() {
    _emailFromController.dispose();
    _emailHostController.dispose();
    _emailPortController.dispose();
    _emailUserController.dispose();
    _emailPasswordController.dispose();
    _telegramTokenController.dispose();
    _whatsappPhoneIdController.dispose();
    _instagramPageIdController.dispose();
    _instagramTokenController.dispose();
    _smsAccountSidController.dispose();
    _smsAuthTokenController.dispose();
    _smsFromNumberController.dispose();
    super.dispose();
  }

  void _selectChannel(String? type) {
    final notifier = ref.read(onboardingControllerProvider.notifier);
    notifier.updateField('channelType', type);
    // Reset config and controllers when switching
    notifier.updateField('channelConfig', <String, String>{});
    _emailFromController.clear();
    _emailHostController.clear();
    _emailPortController.text = '587';
    _emailUserController.clear();
    _emailPasswordController.clear();
    _telegramTokenController.clear();
    _whatsappPhoneIdController.clear();
    _instagramPageIdController.clear();
    _instagramTokenController.clear();
    _smsAccountSidController.clear();
    _smsAuthTokenController.clear();
    _smsFromNumberController.clear();
    if (type != null) {
      ref.read(analyticsClientProvider).track(
        OnboardingEvents.channelSelected,
        {'type': type},
      );
    }
  }

  /// Opens the "Como conectar o WhatsApp" help video in a new browser
  /// tab. Non-blocking — if the launcher fails (e.g., pop-up blocked)
  /// we surface a snackbar instead of silently failing.
  Future<void> _openWhatsAppHelp() async {
    final uri = Uri.parse(_kWhatsAppHelpVideoUrl);
    final ok = await launchUrl(uri, mode: LaunchMode.externalApplication);
    if (!ok && mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(t.onboarding.step_channel_whatsapp_help_failed)),
      );
    }
  }

  void _updateConfig() {
    final channelType = ref.read(onboardingControllerProvider).channelType;
    final notifier = ref.read(onboardingControllerProvider.notifier);

    if (channelType == 'email') {
      notifier.updateField('channelConfig', {
        'from_address': _emailFromController.text.trim(),
        'smtp_host': _emailHostController.text.trim(),
        'smtp_port': _emailPortController.text.trim(),
        // Most providers use the full address as the SMTP user; defaulting
        // to it saves a field the person would type identically anyway,
        // while still letting them override it.
        'smtp_username': _emailUserController.text.trim().isEmpty
            ? _emailFromController.text.trim()
            : _emailUserController.text.trim(),
        'smtp_password': _emailPasswordController.text,
      });
    } else if (channelType == 'telegram') {
      notifier.updateField('channelConfig', {
        'bot_token': _telegramTokenController.text.trim(),
      });
    } else if (channelType == 'whatsapp') {
      notifier.updateField('channelConfig', {
        'phone_number_id': _whatsappPhoneIdController.text.trim(),
      });
    } else if (channelType == 'instagram') {
      notifier.updateField('channelConfig', {
        'page_id': _instagramPageIdController.text.trim(),
        'access_token': _instagramTokenController.text.trim(),
      });
    } else if (channelType == 'sms') {
      notifier.updateField('channelConfig', {
        'account_sid': _smsAccountSidController.text.trim(),
        'auth_token': _smsAuthTokenController.text.trim(),
        'from_number': _smsFromNumberController.text.trim(),
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;
    final typography = Theme.of(context).extension<AppTypographyExtension>()!;
    final onboardingState = ref.watch(onboardingControllerProvider);
    final selectedType = onboardingState.channelType;

    // Shrink the hero on short viewports (< 700px) so the tiles
    // and config inputs stay visible without manual scroll.
    final screenHeight = MediaQuery.of(context).size.height;
    final compact = screenHeight < 700;

    return SingleChildScrollView(
      padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 16),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 440),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            SizedBox(height: compact ? 8 : 28),
            Center(
              child: Container(
                width: compact ? 48 : 72,
                height: compact ? 48 : 72,
                decoration: BoxDecoration(
                  color: colors.bluePrimary.withValues(alpha: 0.12),
                  shape: BoxShape.circle,
                ),
                child: Icon(
                  Icons.chat_bubble_outline,
                  size: compact ? 24 : 36,
                  color: colors.bluePrimary,
                ),
              ),
            ),
            SizedBox(height: compact ? 10 : 24),
            Text(
              t.onboarding.step_channel_title,
              style: typography.display.copyWith(
                fontSize: compact ? 20 : 24,
                color: Colors.white,
                fontWeight: FontWeight.w700,
              ),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 8),
            Text(
              t.onboarding.step_channel_subtitle,
              style: typography.body.copyWith(
                color: colors.textSecondary,
                fontSize: 14,
              ),
              textAlign: TextAlign.center,
            ),
            SizedBox(height: compact ? 16 : 32),

            // Email leads because it is the channel this product sells on:
            // in B2B healthcare the first approach goes by email and
            // WhatsApp is the follow-up. The tiles are hidden until asked
            // for: with email preselected the page opens on the Gmail card.
            //
            // A Wrap, not a Row: five tiles squeezed into one 500px row gave
            // each ~76px and truncated every label — worse on a phone. And
            // no speed badge on Telegram: "Mais rápido" beside the channel
            // the product actually sells on taught the wrong first move.
            // A non-email choice (a resumed draft, or one just made) keeps
            // its tile on screen whatever the fold says.
            if (_showOtherChannels ||
                (selectedType != null && selectedType != 'email')) ...[
              Wrap(
                spacing: 12,
                runSpacing: 12,
                children: [
                  SizedBox(
                    width: 150,
                    child: ChannelTile(
                      type: ChannelType.email,
                      isSelected: selectedType == 'email',
                      badgeText: t.onboarding.step_channel_badge_email,
                      badgeColor: colors.bluePrimary,
                      badgeIcon: Icons.alternate_email,
                      onTap: () => _selectChannel(
                        selectedType == 'email' ? null : 'email',
                      ),
                    ),
                  ),
                  SizedBox(
                    width: 150,
                    child: ChannelTile(
                      type: ChannelType.whatsapp,
                      isSelected: selectedType == 'whatsapp',
                      badgeText: t.onboarding.step_channel_badge_whatsapp,
                      badgeIcon: Icons.star_rounded,
                      onTap: () => _selectChannel(
                        selectedType == 'whatsapp' ? null : 'whatsapp',
                      ),
                    ),
                  ),
                  SizedBox(
                    width: 150,
                    child: ChannelTile(
                      type: ChannelType.telegram,
                      isSelected: selectedType == 'telegram',
                      onTap: () => _selectChannel(
                        selectedType == 'telegram' ? null : 'telegram',
                      ),
                    ),
                  ),
                  SizedBox(
                    width: 150,
                    child: ChannelTile(
                      type: ChannelType.instagram,
                      isSelected: selectedType == 'instagram',
                      onTap: () => _selectChannel(
                        selectedType == 'instagram' ? null : 'instagram',
                      ),
                    ),
                  ),
                  SizedBox(
                    width: 150,
                    child: ChannelTile(
                      type: ChannelType.sms,
                      isSelected: selectedType == 'sms',
                      onTap: () => _selectChannel(
                        selectedType == 'sms' ? null : 'sms',
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 24),
            ],

            if (selectedType == 'email') ...[
              // The mailbox she already has, one click. The card posts to
              // /channels/email/google/connect and opens Google's consent
              // screen; the callback creates the channel server-side, so
              // "Next" here has nothing to post for it.
              const GoogleMailboxConnectCard(),
              const SizedBox(height: 12),
              TextButton.icon(
                key: const Key('onboarding_email_smtp_toggle'),
                onPressed: () => setState(() => _showSmtp = !_showSmtp),
                icon: Icon(
                  _showSmtp ? Icons.expand_less : Icons.expand_more,
                  size: 18,
                ),
                label: Text(t.onboarding.email_smtp_toggle),
              ),
            ],
            // The other channels, on request. Placed under the email block
            // (not above it) so the door stays the first thing on the page;
            // when a non-email channel is chosen the tiles are already open
            // above, and the toggle only folds them back.
            if (selectedType == 'email' || selectedType == null)
              TextButton.icon(
                key: const Key('onboarding_other_channels_toggle'),
                onPressed: () =>
                    setState(() => _showOtherChannels = !_showOtherChannels),
                icon: Icon(
                  _showOtherChannels ? Icons.expand_less : Icons.expand_more,
                  size: 18,
                ),
                label: Text(_showOtherChannels
                    ? t.onboarding.step_channel_other_channels_hide
                    : t.onboarding.step_channel_other_channels),
              ),
            if (selectedType == 'email' && _showSmtp) ...[
              _buildInput(
                controller: _emailFromController,
                label: t.onboarding.email_from_address,
                hint: 'vendas@suaempresa.com.br',
                colors: colors,
              ),
              const SizedBox(height: 12),
              _buildInput(
                controller: _emailHostController,
                label: t.onboarding.email_smtp_host,
                hint: 'smtp.suaempresa.com.br',
                colors: colors,
              ),
              const SizedBox(height: 12),
              Row(
                children: [
                  SizedBox(
                    width: 110,
                    child: _buildInput(
                      controller: _emailPortController,
                      label: t.onboarding.email_smtp_port,
                      hint: '587',
                      colors: colors,
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: _buildInput(
                      controller: _emailPasswordController,
                      label: t.onboarding.email_smtp_password,
                      hint: '••••••••',
                      colors: colors,
                      obscure: true,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 16),
              // The DNS records are the part that decides whether anything
              // lands in an inbox, and they used to live only in Settings →
              // Canais, found by accident. They are named here, before the
              // channel exists, because SPF/DKIM/DMARC take hours to
              // propagate — someone who learns about them after finishing
              // onboarding has already lost a day.
              Container(
                key: const Key('onboarding_email_dns_notice'),
                padding: const EdgeInsets.all(16),
                decoration: BoxDecoration(
                  color: colors.backgroundSecondary,
                  borderRadius: BorderRadius.circular(12),
                  border: Border.all(color: colors.borderMedium),
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Icon(Icons.dns_outlined,
                            size: 18, color: colors.bluePrimary),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Text(
                            t.onboarding.email_dns_title,
                            style: typography.body.copyWith(
                              color: colors.textPrimary,
                              fontSize: 14,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 6),
                    Text(
                      t.onboarding.email_dns_body,
                      style: typography.body.copyWith(
                          color: colors.textSecondary,
                          fontSize: 13,
                          height: 1.4),
                    ),
                  ],
                ),
              ),
            ] else if (selectedType == 'telegram') ...[
              _buildInput(
                controller: _telegramTokenController,
                label: t.onboarding.telegram_bot_token,
                hint: '123456:ABC-DEF...',
                colors: colors,
              ),
            ] else if (selectedType == 'whatsapp') ...[
              // Primary path: Meta's Embedded Signup. Creates the channel
              // server-side in a single round-trip. On success we flip the
              // onboarding flag so "Próximo" skips the manual POST and
              // just advances.
              Builder(
                builder: (_) {
                  final onboardingNotifier =
                      ref.read(onboardingControllerProvider.notifier);
                  return Center(
                    child: WhatsAppEmbeddedSignupButton(
                      onConnected: () {
                        onboardingNotifier.markChannelConnected();
                        ref.read(analyticsClientProvider).track(
                          OnboardingEvents.channelConnected,
                          {'type': 'whatsapp'},
                        );
                      },
                    ),
                  );
                },
              ),
              if (_kWhatsAppHelpVideoUrl.isNotEmpty) ...[
                const SizedBox(height: 8),
                Center(
                  child: TextButton.icon(
                    onPressed: _openWhatsAppHelp,
                    icon: Icon(Icons.play_circle_outline,
                        size: 16, color: colors.textMuted),
                    label: Text(
                      t.onboarding.step_channel_whatsapp_help,
                      style: TextStyle(color: colors.textMuted, fontSize: 12),
                    ),
                  ),
                ),
              ],
              const SizedBox(height: 4),
              if (onboardingState.channelConnected)
                Container(
                  padding: const EdgeInsets.all(10),
                  decoration: BoxDecoration(
                    color: colors.limeGlow.withValues(alpha: 0.12),
                    borderRadius: BorderRadius.circular(8),
                    border: Border.all(
                      color: colors.limeGlow.withValues(alpha: 0.4),
                    ),
                  ),
                  child: Row(
                    children: [
                      Icon(Icons.check_circle,
                          size: 16, color: colors.limeGlow),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Text(
                          t.onboarding.step_channel_whatsapp_connected,
                          style: TextStyle(
                            color: colors.textPrimary,
                            fontSize: 12,
                            height: 1.4,
                          ),
                        ),
                      ),
                    ],
                  ),
                )
              else ...[
                Row(
                  children: [
                    Expanded(child: Divider(color: colors.borderLight)),
                    Padding(
                      padding: const EdgeInsets.symmetric(horizontal: 12),
                      child: Text(
                        t.onboarding.step_channel_whatsapp_or_manual,
                        style: TextStyle(
                          color: colors.textMuted,
                          fontSize: 11,
                        ),
                      ),
                    ),
                    Expanded(child: Divider(color: colors.borderLight)),
                  ],
                ),
                const SizedBox(height: 12),
                _buildInput(
                  controller: _whatsappPhoneIdController,
                  label: t.onboarding.whatsapp_phone_number_id,
                  hint: '1234567890',
                  colors: colors,
                ),
                const SizedBox(height: 8),
                Container(
                  padding: const EdgeInsets.all(10),
                  decoration: BoxDecoration(
                    color: colors.glassMedium,
                    borderRadius: BorderRadius.circular(8),
                  ),
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Icon(Icons.info_outline,
                          size: 14, color: colors.textMuted),
                      const SizedBox(width: 6),
                      Expanded(
                        child: Text(
                          t.onboarding.step_channel_whatsapp_manual_hint,
                          style: TextStyle(
                            color: colors.textMuted,
                            fontSize: 11,
                            height: 1.4,
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ] else if (selectedType == 'instagram') ...[
              _buildInput(
                controller: _instagramPageIdController,
                label: t.onboarding.instagram_page_id,
                hint: '17841400000000000',
                colors: colors,
              ),
              const SizedBox(height: 12),
              _buildInput(
                controller: _instagramTokenController,
                label: t.onboarding.instagram_access_token,
                hint: 'IGAAxxxxxxxxxx...',
                colors: colors,
              ),
              const SizedBox(height: 8),
              Container(
                padding: const EdgeInsets.all(10),
                decoration: BoxDecoration(
                  color: colors.glassMedium,
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Icon(Icons.info_outline, size: 14, color: colors.textMuted),
                    const SizedBox(width: 6),
                    Expanded(
                      child: Text(
                        t.onboarding.step_channel_instagram_hint,
                        style: TextStyle(
                          color: colors.textMuted,
                          fontSize: 11,
                          height: 1.4,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
              Center(
                child: TextButton.icon(
                  onPressed: () => launchUrl(
                    Uri.parse(kInstagramSetupHelpUrl),
                    mode: LaunchMode.externalApplication,
                  ),
                  icon: Icon(Icons.open_in_new,
                      size: 14, color: colors.textMuted),
                  label: Text(
                    t.settings.channel_instagram_help_link,
                    style: TextStyle(color: colors.textMuted, fontSize: 12),
                  ),
                ),
              ),
            ] else if (selectedType == 'sms') ...[
              _buildInput(
                controller: _smsAccountSidController,
                label: t.onboarding.sms_account_sid,
                hint: 'ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
                colors: colors,
              ),
              const SizedBox(height: 12),
              _buildInput(
                controller: _smsAuthTokenController,
                label: t.onboarding.sms_auth_token,
                hint: '••••••••••••••••',
                colors: colors,
                obscure: true,
              ),
              const SizedBox(height: 12),
              _buildInput(
                controller: _smsFromNumberController,
                label: t.onboarding.sms_from_number,
                hint: '+15551234567',
                colors: colors,
              ),
              const SizedBox(height: 8),
              Container(
                padding: const EdgeInsets.all(10),
                decoration: BoxDecoration(
                  color: colors.glassMedium,
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Icon(Icons.info_outline, size: 14, color: colors.textMuted),
                    const SizedBox(width: 6),
                    Expanded(
                      child: Text(
                        t.onboarding.step_channel_sms_hint,
                        style: TextStyle(
                          color: colors.textMuted,
                          fontSize: 11,
                          height: 1.4,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
              Center(
                child: TextButton.icon(
                  onPressed: () => launchUrl(
                    Uri.parse(kSmsSetupHelpUrl),
                    mode: LaunchMode.externalApplication,
                  ),
                  icon: Icon(Icons.open_in_new,
                      size: 14, color: colors.textMuted),
                  label: Text(
                    t.settings.channel_sms_help_link,
                    style: TextStyle(color: colors.textMuted, fontSize: 12),
                  ),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildInput({
    required TextEditingController controller,
    required String label,
    required String hint,
    required AppColorsExtension colors,
    bool obscure = false,
  }) {
    return TextField(
      controller: controller,
      obscureText: obscure,
      style: TextStyle(color: colors.textPrimary),
      onChanged: (_) => _updateConfig(),
      decoration: InputDecoration(
        labelText: label,
        labelStyle: TextStyle(color: colors.textMuted),
        hintText: hint,
        hintStyle: TextStyle(color: colors.textDisabled, fontSize: 12),
      ),
    );
  }
}
