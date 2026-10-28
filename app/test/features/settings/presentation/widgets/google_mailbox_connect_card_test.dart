import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/features/settings/domain/models/channel_model.dart';
import 'package:app/features/settings/presentation/controllers/channels_controller.dart';
import 'package:app/features/settings/presentation/widgets/google_mailbox_connect_card.dart';
import 'package:app/i18n/strings.g.dart';

/// The card is the entire visible half of pillar 1. What it must not do is
/// claim something happened that did not.

class _NoChannels extends ChannelsList {
  @override
  Future<List<ChannelModel>> build() async => const [];

  @override
  Future<void> load() async {}
}

class _WithMailbox extends ChannelsList {
  @override
  Future<List<ChannelModel>> build() async => [
        ChannelModel.fromJson(const {
          'id': 'ch1',
          'account_id': 'acc',
          'name': 'vendas@clinica.com',
          'type': 'email',
          'is_active': true,
        }),
      ];

  @override
  Future<void> load() async {}
}

void main() {
  setUp(() => LocaleSettings.setLocale(AppLocale.en));

  Future<void> pump(WidgetTester tester, {bool connected = false}) =>
      tester.pumpWidget(
        ProviderScope(
          overrides: [
            channelsListProvider
                .overrideWith(connected ? _WithMailbox.new : _NoChannels.new),
          ],
          child: MaterialApp(
            theme: AppTheme.darkTheme,
            home: const Scaffold(body: GoogleMailboxConnectCard()),
          ),
        ),
      );

  testWidgets('offers the one-click path and says what it costs',
      (tester) async {
    await pump(tester);
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('gmail_connect_card')), findsOneWidget);
    expect(find.text(t.channels.gmail_connect_cta), findsOneWidget);
    // The two facts the person is actually deciding on: nothing to type, and
    // the replies stay where they already read them.
    expect(find.text(t.channels.gmail_connect_help), findsOneWidget);
  });

  // The OAuth ends in another tab; the honest signal is the account's own
  // channel list. With a mailbox present the card says CONNECTED instead of
  // offering to connect again — the wizard used to give no feedback at all.
  testWidgets('a connected mailbox renders as connected', (tester) async {
    await pump(tester, connected: true);
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('gmail_connected_state')), findsOneWidget);
    expect(find.text(t.channels.gmail_connect_cta), findsNothing);
  });

  group('the callback flag', () {
    test('cancelling is not reported as a failure', () {
      // Somebody pressed cancel on the consent screen. Telling them something
      // broke is a lie, and it is the message that would send them to support.
      expect(gmailConnectMessage('denied'), t.channels.gmail_denied);
      expect(gmailConnectMessage('denied'), isNot(t.channels.gmail_failed));
    });

    test('a first connection and a repair read differently', () {
      // To the person one is setup and the other is "the thing that broke is
      // fixed" — same server call, different sentence.
      expect(gmailConnectMessage('connected'), t.channels.gmail_connected);
      expect(gmailConnectMessage('reconnected'), t.channels.gmail_reconnected);
      expect(gmailConnectMessage('connected'),
          isNot(gmailConnectMessage('reconnected')));
    });

    test('a failure says so', () {
      expect(gmailConnectMessage('error'), t.channels.gmail_failed);
    });

    test('no flag says nothing at all', () {
      // Landing on the channels screen for any other reason must not pop a
      // message about a connection nobody started.
      expect(gmailConnectMessage(null), isNull);
      expect(gmailConnectMessage(''), isNull);
      expect(gmailConnectMessage('something-else'), isNull);
    });
  });
}
