import 'dart:ui';

import 'package:app/i18n/strings.g.dart';

enum ChannelType { whatsapp, telegram, instagram, webchat, sms, email }

/// Meta's official guide for Instagram messaging prerequisites: IG
/// professional account linked to a Facebook Page + an access token
/// carrying instagram_manage_messages. Linked from every Instagram
/// config form (settings dialog + onboarding connect step).
const kInstagramSetupHelpUrl =
    'https://developers.facebook.com/docs/messenger-platform/instagram/get-started';

/// Replider's own toll-free-first Twilio walkthrough (published from
/// docs/guides/sms-twilio-setup.md). Linked from every SMS config form
/// (settings dialog + onboarding connect step).
const kSmsSetupHelpUrl = 'https://replider.com/en/blog/sms-twilio-setup';

/// Replider's outbound-email setup guide (SMTP + the SPF/DKIM/DMARC records
/// the domain must publish). Linked from the email config form: a sending
/// domain without authentication is not "slightly worse" — Gmail and Outlook
/// reject or spam-folder it outright, and the failure is invisible from the
/// sender's side.
const kEmailSetupHelpUrl = 'https://replider.com/pt/blog/email-dominio-setup';

/// Mirrors the backend's sensitive-key rule (domain.SensitiveConfigKey):
/// credential-shaped config keys (*_token/*_secret/*_key/password) come
/// back MASKED from GET /channels ("abcd****"). On edit, an empty field
/// for one of these means "keep the stored credential" — the backend
/// preserves the value when the key is omitted, so forms drop these
/// keys instead of blocking the save or re-sending the mask.
bool isSensitiveConfigKey(String key) =>
    key == 'password' ||
    // `_password` covers the email channel's smtp_password (US-M003): an
    // exact-match-only rule left it unmasked in every channel read.
    key.endsWith('_password') ||
    key.endsWith('_token') ||
    key.endsWith('_secret') ||
    key.endsWith('_key');

const _kApiBaseUrl = String.fromEnvironment('API_BASE_URL');

/// Absolute inbound webhook URL for a channel. Shown in the SMS config UI
/// so the admin can paste it as the "A message comes in" webhook on their
/// Twilio number. Best effort — falls back to the production API when the
/// dashboard was built without API_BASE_URL (mirrors the webchat snippet).
String channelWebhookUrl(String webhookToken) {
  final base =
      _kApiBaseUrl.isEmpty ? 'https://api.replider.com/v1' : _kApiBaseUrl;
  return '$base/webhooks/$webhookToken';
}

/// Warmup/budget state of an email channel's mailbox, derived server-side by
/// the SAME rules the send gate applies — so the screen and a refusal can
/// never disagree about today's budget.
class EmailWarmupStatus {
  const EmailWarmupStatus({
    required this.started,
    required this.warming,
    this.day = 0,
    required this.rampDays,
    required this.dailyCap,
    required this.sentToday,
    required this.remaining,
  });

  final bool started;
  final bool warming;
  final int day;
  final int rampDays;
  final int dailyCap;
  final int sentToday;
  final int remaining;

  factory EmailWarmupStatus.fromJson(Map<String, dynamic> json) =>
      EmailWarmupStatus(
        started: json['started'] as bool? ?? false,
        warming: json['warming'] as bool? ?? false,
        day: (json['day'] as num?)?.toInt() ?? 0,
        rampDays: (json['ramp_days'] as num?)?.toInt() ?? 0,
        dailyCap: (json['daily_cap'] as num?)?.toInt() ?? 0,
        sentToday: (json['sent_today'] as num?)?.toInt() ?? 0,
        remaining: (json['remaining'] as num?)?.toInt() ?? 0,
      );
}

class ChannelModel {
  final String id;
  final String accountId;
  final String? agentId; // nullable — null means human-only channel
  final String name;
  final ChannelType type;
  final bool isActive;
  final Map<String, dynamic> config;
  final String
      webhookToken; // public channel identifier (ULID), safe to ship to customer sites
  final String createdAt;

  /// Derived server-side: true when all credentials needed to call the
  /// upstream provider are present (for WhatsApp: phone_number_id +
  /// waba_id + access_token). Stub channels created manually without
  /// Embedded Signup show this as false and get an "Incompleto" badge.
  final bool isFullyConfigured;

  /// Present only on email channels with a known mailbox: the warmup state
  /// and today's send budget (Item 1 — the guard fails closed, so the screen
  /// says where the ramp is instead of the user discovering it on a refusal).
  final EmailWarmupStatus? emailWarmup;

  /// The IMAP poller's last outcome for an email channel (server-derived
  /// `imap_last_error` / `imap_failure_count` / `imap_last_polled_at`). A
  /// failure count above zero means replies are NOT being read — a revoked
  /// Google grant or a changed password shows up here and nowhere else.
  final String imapLastError;
  final int imapFailureCount;
  final String? imapLastPolledAt;

  const ChannelModel({
    required this.id,
    required this.accountId,
    this.agentId,
    required this.name,
    required this.type,
    required this.isActive,
    this.config = const {},
    this.webhookToken = '',
    required this.createdAt,
    this.isFullyConfigured = true,
    this.emailWarmup,
    this.imapLastError = '',
    this.imapFailureCount = 0,
    this.imapLastPolledAt,
  });

  /// Whether this channel has an AI agent assigned.
  bool get hasAgent => agentId != null && agentId!.isNotEmpty;

  /// An email channel connected by Google consent (`auth_method ==
  /// google_oauth`): it has no SMTP host or password to edit, and its
  /// @gmail.com address is legitimate — the SMTP form must not be shown.
  bool get isGoogleOAuth =>
      type == ChannelType.email &&
      (config['auth_method']?.toString().toLowerCase() ?? '') == 'google_oauth';

  /// Whether the channel needs to be connected again: the poller is failing
  /// (any failure count), or the provider explicitly refused the grant.
  bool get needsReconnect =>
      imapFailureCount > 0 ||
      imapLastError.contains('EMAIL_OAUTH_NOT_CONNECTED');

  factory ChannelModel.fromJson(Map<String, dynamic> json) {
    final typeStr = json['type'] as String? ?? 'whatsapp';
    final rawConfig = json['config'];
    Map<String, dynamic> config = {};
    if (rawConfig is Map<String, dynamic>) {
      config = rawConfig;
    }

    final rawAgentId = json['agent_id'] as String?;

    return ChannelModel(
      id: json['id'] as String,
      accountId: json['account_id'] as String? ?? '',
      agentId:
          (rawAgentId != null && rawAgentId.isNotEmpty) ? rawAgentId : null,
      name: json['name'] as String,
      type: ChannelType.values.firstWhere(
        (t) => t.name == typeStr,
        orElse: () => ChannelType.whatsapp,
      ),
      isActive: json['is_active'] as bool? ?? false,
      config: config,
      webhookToken: json['webhook_token'] as String? ?? '',
      createdAt: json['created_at'] as String? ?? '',
      isFullyConfigured: json['is_fully_configured'] as bool? ?? true,
      emailWarmup: json['email_warmup'] is Map<String, dynamic>
          ? EmailWarmupStatus.fromJson(
              json['email_warmup'] as Map<String, dynamic>)
          : null,
      imapLastError: json['imap_last_error'] as String? ?? '',
      imapFailureCount: (json['imap_failure_count'] as num?)?.toInt() ?? 0,
      imapLastPolledAt: json['imap_last_polled_at'] as String?,
    );
  }

  ChannelModel copyWith({
    String? name,
    bool? isActive,
    Map<String, dynamic>? config,
    String? Function()? agentId,
  }) {
    return ChannelModel(
      id: id,
      accountId: accountId,
      agentId: agentId != null ? agentId() : this.agentId,
      name: name ?? this.name,
      type: type,
      isActive: isActive ?? this.isActive,
      config: config ?? this.config,
      webhookToken: webhookToken,
      createdAt: createdAt,
      isFullyConfigured: isFullyConfigured,
      emailWarmup: emailWarmup,
      imapLastError: imapLastError,
      imapFailureCount: imapFailureCount,
      imapLastPolledAt: imapLastPolledAt,
    );
  }
}

extension ChannelTypeExt on ChannelType {
  // No gated channel types right now (Instagram went live in US-006).
  // Keep the flag — the next channel type (SMS) lands behind it first.
  bool get isComingSoon => false;

  String get label {
    switch (this) {
      case ChannelType.whatsapp:
        return 'WhatsApp';
      case ChannelType.telegram:
        return 'Telegram';
      case ChannelType.instagram:
        return 'Instagram';
      case ChannelType.webchat:
        return t.channels.type_webchat_label;
      case ChannelType.sms:
        return 'SMS';
      case ChannelType.email:
        return 'E-mail';
    }
  }

  /// Single config key for each channel type.
  /// Webchat has no single "credential" — it uses a config object.
  String? get configKey {
    switch (this) {
      case ChannelType.whatsapp:
        return 'phone_number_id';
      case ChannelType.telegram:
        return 'bot_token';
      case ChannelType.instagram:
        return 'page_id';
      case ChannelType.sms:
        return 'account_sid';
      case ChannelType.email:
        return 'from_address';
      case ChannelType.webchat:
        return null;
    }
  }

  /// Extra fields for channel types that require multiple credentials.
  /// Returns a list of {key, label, hint} maps.
  ///
  /// WhatsApp returns three fields. The preferred path is Embedded Signup
  /// (which fills all three automatically), but we keep the manual form
  /// as a fallback for devs who already have System User tokens from
  /// Business Manager — and as Plan B during the Meta App Review demo if
  /// the embedded popup misbehaves.
  List<Map<String, String>> get multiFieldConfig {
    switch (this) {
      case ChannelType.whatsapp:
        return [
          {
            'key': 'phone_number_id',
            'label': 'Phone Number ID',
            'hint': '123456789012345',
          },
          {
            'key': 'waba_id',
            'label': 'WhatsApp Business Account ID',
            'hint': '987654321098765',
          },
          {
            'key': 'access_token',
            'label': 'Access Token (System User)',
            'hint': 'EAAxxxxxxxxxx...',
          },
        ];
      case ChannelType.instagram:
        // Manual-token MVP (mirrors Telegram's paste-a-token pattern).
        // Requires an IG professional account linked to a Facebook Page
        // and a token with instagram_manage_messages — the dialog links
        // to the Meta docs explaining both.
        return [
          {
            'key': 'page_id',
            'label': 'Page ID',
            'hint': '17841400000000000',
          },
          {
            'key': 'access_token',
            'label': 'Access Token',
            'hint': 'IGAAxxxxxxxxxx...',
          },
        ];
      case ChannelType.sms:
        // BYO Twilio: the customer pastes credentials from the Twilio
        // console. `obscure: true` hides the Auth Token as it's typed
        // (it's a full-account credential, not a scoped token).
        return [
          {
            'key': 'account_sid',
            'label': 'Account SID',
            'hint': 'ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
          },
          {
            'key': 'auth_token',
            'label': 'Auth Token',
            'hint': '••••••••••••••••',
            'obscure': 'true',
          },
          {
            'key': 'from_number',
            'label': 'From Number (E.164)',
            'hint': '+15551234567',
          },
        ];
      case ChannelType.email:
        // BYO SMTP. from_address is the identity every deliverability
        // signal attaches to, so it leads the form; the password is
        // obscured like any full-account credential. Labels come from
        // slang (this getter runs at render time) — they were PT literals,
        // so an EN locale saw a Portuguese form.
        return [
          {
            'key': 'from_address',
            'label': t.channels.email_field_from,
            'hint': 'vendas@suaempresa.com.br',
          },
          {
            'key': 'smtp_host',
            'label': t.channels.email_field_smtp_host,
            'hint': 'smtp.suaempresa.com.br',
          },
          {
            'key': 'smtp_port',
            'label': t.channels.email_field_smtp_port,
            'hint': '587',
          },
          {
            'key': 'smtp_username',
            'label': t.channels.email_field_smtp_user,
            'hint': 'vendas@suaempresa.com.br',
          },
          {
            'key': 'smtp_password',
            'label': t.channels.email_field_smtp_password,
            'hint': '••••••••••••',
            'obscure': 'true',
          },
          // Reading the replies uses the SAME login. Left blank, the poller
          // falls back to the SMTP host and port 993 — right for every provider
          // that hosts the mailbox it sends through (on Locaweb both are
          // literally email-ssl.com.br). The fields exist only for the ones
          // that split them, which is why they are last and marked optional.
          {
            'key': 'imap_host',
            'label': t.channels.email_field_imap_host,
            'hint': t.channels.email_field_imap_hint,
            // 'optional' is what the submit VALIDATION reads. Before it
            // existed the label said optional while the code required every
            // field — leaving these blank made "Conectar" return in silence.
            'optional': 'true',
          },
          {
            'key': 'imap_port',
            'label': t.channels.email_field_imap_port,
            'hint': '993',
            'optional': 'true',
          },
        ];
      case ChannelType.telegram:
      case ChannelType.webchat:
        return [];
    }
  }

  String get configHint {
    switch (this) {
      case ChannelType.whatsapp:
        return '123456789012345';
      case ChannelType.telegram:
        return '1234567890:AASomeBot_Token';
      case ChannelType.instagram:
        return 'Page ID';
      case ChannelType.sms:
        return 'ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx';
      case ChannelType.email:
        return 'vendas@suaempresa.com.br';
      case ChannelType.webchat:
        return '';
    }
  }

  String get configLabel {
    switch (this) {
      case ChannelType.whatsapp:
        return 'Phone Number ID';
      case ChannelType.telegram:
        return 'Bot Token';
      case ChannelType.instagram:
        return 'Page ID';
      case ChannelType.sms:
        return 'Account SID';
      case ChannelType.email:
        return t.channels.email_field_from;
      case ChannelType.webchat:
        return '';
    }
  }

  String get setupDescription {
    switch (this) {
      case ChannelType.whatsapp:
        return t.channels.setup_whatsapp;
      case ChannelType.telegram:
        return t.channels.setup_telegram;
      case ChannelType.instagram:
        return t.channels.setup_instagram;
      case ChannelType.webchat:
        return t.channels.setup_webchat;
      case ChannelType.sms:
        return t.channels.setup_sms;
      case ChannelType.email:
        return t.channels.setup_email;
    }
  }

  /// The official brand color for this channel type.
  /// Use this instead of hardcoding `Colors.green.shade600` etc.
  Color get brandColor {
    switch (this) {
      case ChannelType.whatsapp:
        return const Color(0xFF25D366);
      case ChannelType.telegram:
        return const Color(0xFF26A5E4);
      case ChannelType.instagram:
        return const Color(0xFFE1306C);
      case ChannelType.webchat:
        return const Color(0xFF4F46E5);
      case ChannelType.sms:
        // Twilio brand red.
        return const Color(0xFFF22F46);
      case ChannelType.email:
        // No vendor owns email — the platform's own accent.
        return const Color(0xFF6366F1);
    }
  }

  /// SVG asset path for the official channel logo.
  String get logoAsset {
    switch (this) {
      case ChannelType.whatsapp:
        return 'assets/whatsapp_logo.svg';
      case ChannelType.telegram:
        return 'assets/telegram_logo.svg';
      case ChannelType.instagram:
        return 'assets/instagram_logo.svg';
      case ChannelType.webchat:
        return 'assets/webchat_logo.svg';
      case ChannelType.sms:
        return 'assets/sms_logo.svg';
      case ChannelType.email:
        // No vendor logo for email; callers render a Material glyph
        // instead (see ChannelTile._logo).
        return '';
    }
  }
}
