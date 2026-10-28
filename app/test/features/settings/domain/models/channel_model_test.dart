import 'package:flutter_test/flutter_test.dart';
import 'package:app/features/settings/domain/models/channel_model.dart';

void main() {
  group('ChannelModel', () {
    final fullJson = <String, dynamic>{
      'id': 'ch-1',
      'agent_id': 'agent-1',
      'name': 'Main WhatsApp',
      'type': 'whatsapp',
      'is_active': true,
      'config': {'phone_number_id': '123', 'access_token': 'EAABwz'},
      'created_at': '2026-01-01T00:00:00Z',
    };

    test('fromJson should parse all fields correctly', () {
      final channel = ChannelModel.fromJson(fullJson);

      expect(channel.id, 'ch-1');
      expect(channel.agentId, 'agent-1');
      expect(channel.name, 'Main WhatsApp');
      expect(channel.type, ChannelType.whatsapp);
      expect(channel.isActive, true);
      expect(
          channel.config, {'phone_number_id': '123', 'access_token': 'EAABwz'});
      expect(channel.createdAt, '2026-01-01T00:00:00Z');
    });

    test('fromJson should parse telegram type', () {
      final json = <String, dynamic>{
        'id': 'ch-2',
        'agent_id': 'agent-1',
        'name': 'Telegram Bot',
        'type': 'telegram',
        'is_active': true,
        'config': {'bot_token': 'abc123'},
        'created_at': '2026-02-01T00:00:00Z',
      };

      final channel = ChannelModel.fromJson(json);

      expect(channel.type, ChannelType.telegram);
    });

    test('fromJson should parse instagram type', () {
      final json = <String, dynamic>{
        'id': 'ch-3',
        'agent_id': 'agent-1',
        'name': 'Instagram DM',
        'type': 'instagram',
        'is_active': false,
        'config': {'page_id': 'page-456'},
        'created_at': '2026-03-01T00:00:00Z',
      };

      final channel = ChannelModel.fromJson(json);

      expect(channel.type, ChannelType.instagram);
      expect(channel.isActive, false);
    });

    test('fromJson should default to whatsapp for unknown type', () {
      final json = <String, dynamic>{
        'id': 'ch-4',
        'agent_id': 'agent-1',
        'name': 'Unknown',
        'type': 'unknown_type',
        'is_active': false,
        'created_at': '2026-01-01T00:00:00Z',
      };

      final channel = ChannelModel.fromJson(json);

      expect(channel.type, ChannelType.whatsapp);
    });

    test('fromJson should default missing type to whatsapp', () {
      final json = <String, dynamic>{
        'id': 'ch-5',
        'agent_id': 'agent-1',
        'name': 'No Type',
        'is_active': true,
        'created_at': '2026-01-01T00:00:00Z',
      };

      final channel = ChannelModel.fromJson(json);

      expect(channel.type, ChannelType.whatsapp);
    });

    test('fromJson should default config to empty map when missing', () {
      final json = <String, dynamic>{
        'id': 'ch-6',
        'agent_id': 'agent-1',
        'name': 'No Config',
        'type': 'telegram',
        'is_active': true,
        'created_at': '2026-01-01T00:00:00Z',
      };

      final channel = ChannelModel.fromJson(json);

      expect(channel.config, isEmpty);
    });

    test('fromJson should default is_active to false when missing', () {
      final json = <String, dynamic>{
        'id': 'ch-7',
        'agent_id': 'agent-1',
        'name': 'Defaults',
        'type': 'whatsapp',
        'created_at': '2026-01-01T00:00:00Z',
      };

      final channel = ChannelModel.fromJson(json);

      expect(channel.isActive, false);
    });

    test('fromJson should default agent_id to null when missing', () {
      final json = <String, dynamic>{
        'id': 'ch-8',
        'name': 'No Agent',
        'type': 'whatsapp',
        'created_at': '2026-01-01T00:00:00Z',
      };

      final channel = ChannelModel.fromJson(json);

      expect(channel.agentId, isNull);
      expect(channel.hasAgent, isFalse);
    });

    test('hasAgent should return true when agent_id is set', () {
      final channel = ChannelModel.fromJson(fullJson);
      expect(channel.hasAgent, isTrue);
    });

    test('copyWith should update specified fields only', () {
      final channel = ChannelModel.fromJson(fullJson);
      final updated = channel.copyWith(
        name: 'Updated Name',
        isActive: false,
      );

      expect(updated.name, 'Updated Name');
      expect(updated.isActive, false);
      expect(updated.id, channel.id);
      expect(updated.agentId, channel.agentId);
      expect(updated.type, channel.type);
      expect(updated.config, channel.config);
      expect(updated.createdAt, channel.createdAt);
    });
  });

  group('ChannelType', () {
    test('should have all expected values', () {
      expect(ChannelType.values, hasLength(6));
      expect(ChannelType.values, contains(ChannelType.whatsapp));
      expect(ChannelType.values, contains(ChannelType.telegram));
      expect(ChannelType.values, contains(ChannelType.instagram));
      expect(ChannelType.values, contains(ChannelType.webchat));
      expect(ChannelType.values, contains(ChannelType.sms));
      expect(ChannelType.values, contains(ChannelType.email));
    });

    // Email is BYO SMTP: the from address leads (every deliverability
    // signal attaches to it) and the password is obscured like any other
    // full-account credential.
    test('email exposes the SMTP fields the backend validates', () {
      final keys =
          ChannelType.email.multiFieldConfig.map((f) => f['key']).toList();
      expect(
        keys,
        containsAll(<String>[
          'from_address',
          'smtp_host',
          'smtp_username',
          'smtp_password',
        ]),
      );
      expect(keys.first, 'from_address');
      final password = ChannelType.email.multiFieldConfig
          .firstWhere((f) => f['key'] == 'smtp_password');
      expect(password['obscure'], 'true');
      // The SMTP password must read as a credential, or it comes back
      // unmasked from GET /channels and a client echoing what it fetched
      // overwrites the stored one.
      expect(isSensitiveConfigKey('smtp_password'), isTrue);
      // The lookup fields must NOT: they are filtered on in raw SQL.
      expect(isSensitiveConfigKey('smtp_host'), isFalse);
      expect(isSensitiveConfigKey('from_address'), isFalse);
    });

    test('label should return display name', () {
      expect(ChannelType.whatsapp.label, 'WhatsApp');
      expect(ChannelType.telegram.label, 'Telegram');
      expect(ChannelType.instagram.label, 'Instagram');
      expect(ChannelType.sms.label, 'SMS');
    });

    test('configKey should return correct key per type', () {
      expect(ChannelType.whatsapp.configKey, 'phone_number_id');
      expect(ChannelType.telegram.configKey, 'bot_token');
      expect(ChannelType.instagram.configKey, 'page_id');
      expect(ChannelType.sms.configKey, 'account_sid');
    });

    test('multiFieldConfig: WhatsApp exposes three creds, Instagram two', () {
      // WhatsApp's manual-fallback form exposes phone_number_id +
      // waba_id + access_token so devs without Embedded Signup can
      // still paste a System User token. Instagram's manual-token MVP
      // needs page_id + access_token (the shape the backend adapter
      // consumes). Telegram stays on the single-field path.
      final waFields =
          ChannelType.whatsapp.multiFieldConfig.map((f) => f['key']).toSet();
      expect(waFields, {'phone_number_id', 'waba_id', 'access_token'});
      final igFields =
          ChannelType.instagram.multiFieldConfig.map((f) => f['key']).toSet();
      expect(igFields, {'page_id', 'access_token'});
      expect(ChannelType.telegram.multiFieldConfig, isEmpty);
    });

    test('multiFieldConfig: SMS exposes the Twilio BYO creds, token obscured',
        () {
      // The shape the backend Twilio adapter consumes (US-009). The Auth
      // Token is a full-account credential — the form must obscure it.
      final smsFields = ChannelType.sms.multiFieldConfig;
      expect(smsFields.map((f) => f['key']).toList(),
          ['account_sid', 'auth_token', 'from_number']);
      final authToken = smsFields.firstWhere((f) => f['key'] == 'auth_token');
      expect(authToken['obscure'], 'true');
      final others = smsFields.where((f) => f['key'] != 'auth_token');
      for (final f in others) {
        expect(f['obscure'], isNull, reason: f['key']);
      }
    });

    test('fromJson should parse sms type', () {
      final json = <String, dynamic>{
        'id': 'ch-9',
        'name': 'SMS',
        'type': 'sms',
        'is_active': true,
        'config': {'account_sid': 'AC123', 'from_number': '+15551234567'},
        'created_at': '2026-07-01T00:00:00Z',
      };

      expect(ChannelModel.fromJson(json).type, ChannelType.sms);
    });

    test('no channel type is gated as coming soon', () {
      // Instagram went live in US-006, SMS in US-011 — every selectable
      // type must be connectable.
      for (final type in ChannelType.values) {
        expect(type.isComingSoon, isFalse, reason: type.name);
      }
    });
  });

  group('isSensitiveConfigKey', () {
    test('flags credential-shaped keys (mirrors backend rule)', () {
      // US-003: these come back masked from GET /channels — the edit
      // dialog must drop them (not re-send the mask) when untouched.
      expect(isSensitiveConfigKey('auth_token'), isTrue);
      expect(isSensitiveConfigKey('bot_token'), isTrue);
      expect(isSensitiveConfigKey('access_token'), isTrue);
      expect(isSensitiveConfigKey('app_secret'), isTrue);
      expect(isSensitiveConfigKey('api_key'), isTrue);
      expect(isSensitiveConfigKey('password'), isTrue);
    });

    test('leaves identifiers hydratable', () {
      expect(isSensitiveConfigKey('account_sid'), isFalse);
      expect(isSensitiveConfigKey('from_number'), isFalse);
      expect(isSensitiveConfigKey('phone_number_id'), isFalse);
      expect(isSensitiveConfigKey('waba_id'), isFalse);
      expect(isSensitiveConfigKey('page_id'), isFalse);
    });
  });

  group('channelWebhookUrl', () {
    test('builds the per-channel inbound webhook URL', () {
      // Tests run without --dart-define API_BASE_URL, so the production
      // fallback applies (mirrors the webchat snippet's best-effort base).
      expect(
        channelWebhookUrl('01HTOKEN'),
        'https://api.replider.com/v1/webhooks/01HTOKEN',
      );
    });
  });

  group('EmailWarmupStatus', () {
    // Item 1: the send gate fails CLOSED on this state, so what the screen
    // parses must mirror exactly what the server derived — a dropped field
    // here is a budget the user cannot see until a refusal explains it.
    test('parses the warmup decoration off an email channel', () {
      final ch = ChannelModel.fromJson(const <String, dynamic>{
        'id': 'ch1',
        'name': 'Comercial',
        'type': 'email',
        'is_active': true,
        'email_warmup': <String, dynamic>{
          'started': true,
          'warming': true,
          'day': 2,
          'ramp_days': 21,
          'daily_cap': 5,
          'sent_today': 3,
          'remaining': 2,
        },
      });

      final w = ch.emailWarmup;
      expect(w, isNotNull);
      expect(w!.warming, isTrue);
      expect(w.day, 2);
      expect(w.rampDays, 21);
      expect(w.remaining, 2);
    });

    test('absent decoration stays null — non-email channels carry none', () {
      final ch = ChannelModel.fromJson(const <String, dynamic>{
        'id': 'ch1',
        'name': 'Zap',
        'type': 'whatsapp',
        'is_active': true,
      });
      expect(ch.emailWarmup, isNull);
    });

    test('never-started renders as exactly that', () {
      final w = EmailWarmupStatus.fromJson(const <String, dynamic>{
        'started': false,
        'warming': false,
        'ramp_days': 21,
        'daily_cap': 0,
        'sent_today': 0,
        'remaining': 0,
      });
      expect(w.started, isFalse);
      expect(w.day, 0);
    });
  });

  group('IMAP health + Google OAuth (revoked-grant visibility)', () {
    Map<String, dynamic> gmailJson({Map<String, dynamic> extra = const {}}) => {
          'id': 'ch-gmail',
          'account_id': 'acc-1',
          'name': 'dona@gmail.com',
          'type': 'email',
          'is_active': true,
          'config': {
            'auth_method': 'google_oauth',
            'from_address': 'dona@gmail.com'
          },
          ...extra,
        };

    test('parses imap_* decoration off an email channel', () {
      final ch = ChannelModel.fromJson(gmailJson(extra: {
        'imap_last_error': '[EMAIL_OAUTH_NOT_CONNECTED] revoked',
        'imap_failure_count': 3,
        'imap_last_polled_at': '2026-09-01T08:00:00Z',
      }));
      expect(ch.imapLastError, '[EMAIL_OAUTH_NOT_CONNECTED] revoked');
      expect(ch.imapFailureCount, 3);
      expect(ch.imapLastPolledAt, '2026-09-01T08:00:00Z');
      expect(ch.needsReconnect, isTrue);
      expect(ch.copyWith(name: 'x').imapFailureCount, 3);
    });

    test('absent decoration means healthy — no reconnect badge', () {
      final ch = ChannelModel.fromJson(gmailJson());
      expect(ch.imapFailureCount, 0);
      expect(ch.imapLastError, '');
      expect(ch.imapLastPolledAt, isNull);
      expect(ch.needsReconnect, isFalse);
    });

    test('auth_method google_oauth marks the channel as a consented mailbox',
        () {
      expect(ChannelModel.fromJson(gmailJson()).isGoogleOAuth, isTrue);
      final smtp = ChannelModel.fromJson(gmailJson(extra: {
        'config': {'from_address': 'vendas@clinica.com.br', 'smtp_host': 'x'},
      }));
      expect(smtp.isGoogleOAuth, isFalse);
      // Only email channels can be one, whatever the config says.
      final wa = ChannelModel.fromJson(gmailJson(extra: {'type': 'whatsapp'}));
      expect(wa.isGoogleOAuth, isFalse);
    });
  });
}
