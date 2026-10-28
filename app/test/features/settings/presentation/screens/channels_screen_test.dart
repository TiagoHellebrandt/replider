import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:dio/dio.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/features/settings/presentation/controllers/email_dns_controller.dart';
import 'package:app/design_system/components/channel_tile.dart';
import 'package:app/features/settings/domain/models/channel_model.dart';
import 'package:app/features/settings/presentation/controllers/channels_controller.dart';
import 'package:app/features/settings/presentation/screens/channels_screen.dart';
import 'package:app/i18n/strings.g.dart';

class _FakeChannelsList extends ChannelsList {
  @override
  Future<List<ChannelModel>> build() async => [];

  @override
  Future<void> load() async {
    state = const AsyncValue.data([]);
  }
}

const _smsChannel = ChannelModel(
  id: 'ch-sms-1',
  accountId: 'acc-1',
  name: 'Twilio SMS',
  type: ChannelType.sms,
  isActive: true,
  config: {'account_sid': 'AC***', 'from_number': '+15551234567'},
  webhookToken: '01HWEBHOOKTOKEN',
  createdAt: '2026-07-01T00:00:00Z',
  // Keeps the card's webhook-health badge (and its network poll) out of
  // the test — the badge only renders for fully-configured channels.
  isFullyConfigured: false,
);

class _FakeSmsChannelsList extends ChannelsList {
  @override
  Future<List<ChannelModel>> build() async => [_smsChannel];

  @override
  Future<void> load() async {
    state = const AsyncValue.data([_smsChannel]);
  }
}

/// A mailbox connected by Google consent whose grant the poller can no
/// longer use: the case that used to be invisible (generic 502 on send, a
/// silent poll failure) and un-editable (the SMTP form refused @gmail.com).
const _gmailChannel = ChannelModel(
  id: 'ch-gmail-1',
  accountId: 'acc-1',
  name: 'dona@gmail.com',
  type: ChannelType.email,
  isActive: true,
  config: {
    'auth_method': 'google_oauth',
    'from_address': 'dona@gmail.com',
    'oauth_refresh_token': 'refr****',
  },
  webhookToken: '01HWEBHOOKTOKEN2',
  createdAt: '2026-08-01T00:00:00Z',
  isFullyConfigured: false,
  imapLastError: '[EMAIL_OAUTH_NOT_CONNECTED] revoked',
  imapFailureCount: 3,
);

class _FakeGmailChannelsList extends ChannelsList {
  @override
  Future<List<ChannelModel>> build() async => [_gmailChannel];

  @override
  Future<void> load() async {
    state = const AsyncValue.data([_gmailChannel]);
  }
}

void main() {
  setUpAll(() {
    FlutterSecureStorage.setMockInitialValues({});
  });

  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
  });

  Widget harness({bool withSmsChannel = false}) {
    return ProviderScope(
      overrides: [
        channelsListProvider.overrideWith(
            withSmsChannel ? _FakeSmsChannelsList.new : _FakeChannelsList.new),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const ChannelsScreen(),
      ),
    );
  }

  Finder tileOf(ChannelType type) =>
      find.byWidgetPredicate((w) => w is ChannelTile && w.type == type);

  // Two doors since 2026-08-18: email and WhatsApp. Telegram, Instagram and
  // SMS are not offered to a new channel — a hospital-equipment seller reaches
  // her buyers on the first two, and five tiles truncated every label.
  testWidgets('empty state offers email and WhatsApp, nothing else',
      (tester) async {
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(harness());
    await tester.pump();

    expect(tileOf(ChannelType.email), findsOneWidget);
    expect(tileOf(ChannelType.whatsapp), findsOneWidget);
    expect(tileOf(ChannelType.telegram), findsNothing);
    expect(tileOf(ChannelType.instagram), findsNothing);
    expect(tileOf(ChannelType.sms), findsNothing);
  });

  // The Gmail card is the door; the SMTP form is the graduation path, folded;
  // and while it is folded there is exactly ONE connect button on screen —
  // the card's. Two "Conectar" side by side was a coin flip.
  testWidgets('email dialog: Gmail first, SMTP folded, no second Connect',
      (tester) async {
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(harness());
    await tester.pump();
    await tester.tap(tileOf(ChannelType.email));
    await tester.pump();

    expect(find.text(t.settings.channel_dialog_add_title), findsOneWidget);
    expect(find.byKey(const Key('gmail_connect_card')), findsOneWidget);
    expect(find.byKey(const Key('channel_dialog_submit')), findsNothing);
    // No channel-name field on add: it defaults to the type's label.
    expect(find.text(t.settings.channel_name_label), findsNothing);
    // Only the two tiles inside the dialog too.
    expect(find.byType(ChannelTile), findsNWidgets(2 + 2));

    await tester.tap(find.byKey(const Key('channel_manual_toggle')));
    await tester.pump();

    expect(find.byKey(const Key('channel_dialog_submit')), findsOneWidget);
    // The "responsible agent" picker is v1 residue: agent screens were cut
    // on 2026-08-18, so a NEW channel must not offer a dropdown over a list
    // nobody can populate. (Editing a legacy channel that has one still
    // shows it, so the assignment can be removed.)
    expect(find.text(t.settings.channel_responsible_agent), findsNothing);
  });

  testWidgets(
      'SMS channel card shows the copyable Webhook URL button and the '
      'edit dialog shows the webhook URL box', (tester) async {
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(harness(withSmsChannel: true));
    await tester.pump();

    // Card action to copy the Twilio webhook URL.
    expect(find.text(t.settings.channel_webhook_url_btn), findsOneWidget);

    final copied = <String>[];
    tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
      SystemChannels.platform,
      (call) async {
        if (call.method == 'Clipboard.setData') {
          copied.add((call.arguments as Map)['text'] as String);
        }
        return null;
      },
    );
    addTearDown(() => tester.binding.defaultBinaryMessenger
        .setMockMethodCallHandler(SystemChannels.platform, null));

    await tester.tap(find.text(t.settings.channel_webhook_url_btn));
    await tester.pump();
    expect(copied, [channelWebhookUrl('01HWEBHOOKTOKEN')]);
    expect(find.text(t.settings.channel_webhook_url_copied), findsOneWidget);

    // Edit dialog surfaces the URL with copy + Twilio instructions.
    await tester.tap(find.byTooltip(t.settings.channel_edit_tooltip));
    await tester.pump();
    expect(find.text(t.settings.channel_webhook_url_label), findsOneWidget);
    expect(find.text(t.settings.channel_webhook_url_help), findsOneWidget);
    expect(
      find.text(channelWebhookUrl('01HWEBHOOKTOKEN')),
      findsOneWidget,
    );
  });

  // The email card's DNS and bulk cards poll on a timer unless the state
  // they get is settled; hand them settled states so no timer leaks.
  Widget gmailHarness() {
    return ProviderScope(
      overrides: [
        channelsListProvider.overrideWith(_FakeGmailChannelsList.new),
        dioProvider.overrideWithValue(Dio()),
        emailDnsProvider(_gmailChannel.id).overrideWith((ref) async =>
            const EmailDnsSetup(
                domain: 'gmail.com', records: [], verified: true)),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const ChannelsScreen(),
      ),
    );
  }

  testWidgets('a failing Google mailbox wears a Reconectar badge on its card',
      (tester) async {
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(gmailHarness());
    await tester.pump();
    await tester.pump();

    expect(
        find.byKey(const Key('channel_imap_reconnect_badge')), findsOneWidget);
    expect(find.text(t.settings.channel_reconnect_action), findsOneWidget);
    expect(
      find.byTooltip(t.settings.channel_imap_failing(
          count: '3', error: '[EMAIL_OAUTH_NOT_CONNECTED] revoked')),
      findsOneWidget,
    );
  });

  // Editing the consented mailbox shows what it is and how to fix it —
  // never the SMTP form, which asked for a host that does not exist and
  // refused the @gmail.com address on save.
  testWidgets('editing a Google mailbox hides SMTP and offers a reconnect',
      (tester) async {
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(gmailHarness());
    await tester.pump();
    await tester.pump();

    await tester.tap(find.byTooltip(t.settings.channel_edit_tooltip));
    await tester.pump();

    expect(find.byKey(const Key('channel_google_edit_block')), findsOneWidget);
    expect(
      find.text(t.settings.channel_google_connected(address: 'dona@gmail.com')),
      findsOneWidget,
    );
    expect(find.byKey(const Key('channel_google_reconnect')), findsOneWidget);
    expect(find.text(t.channels.email_field_smtp_host), findsNothing,
        reason: 'no SMTP host to type for a consented mailbox');
    expect(find.text(t.channels.email_field_smtp_password), findsNothing);
    // The name is still editable and the save button is there.
    expect(find.byKey(const Key('channel_dialog_submit')), findsOneWidget);
  });
}
