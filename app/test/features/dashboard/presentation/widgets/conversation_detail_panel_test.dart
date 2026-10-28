import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import 'package:app/core/error/api_error_parser.dart';
import 'package:app/core/network/websocket_service.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/design_system/components/cached_network_image.dart';
import 'package:app/features/dashboard/domain/models/conversation_model.dart';
import 'package:app/features/dashboard/domain/models/message_model.dart';
import 'package:app/features/dashboard/presentation/widgets/takeover_bar.dart';
import 'package:app/features/dashboard/presentation/controllers/dashboard_controller.dart';
import 'package:app/features/dashboard/presentation/widgets/conversation_detail_panel.dart';
import 'package:app/features/dashboard/presentation/widgets/notes_panel.dart';
import 'package:app/features/settings/domain/models/tag_model.dart';
import 'package:app/features/settings/domain/models/team_member_model.dart';
import 'package:app/features/settings/presentation/controllers/tag_controller.dart';
import 'package:app/features/settings/presentation/controllers/team_controller.dart';
import 'package:app/i18n/strings.g.dart';

ConversationModel _conversation({
  String? avatarUrl,
  bool botPaused = false,
  ConversationSource source = ConversationSource.whatsapp,
  String contactExternalId = '5511999999999',
}) {
  return ConversationModel(
    id: 'conv-1',
    isBotPaused: botPaused,
    channelId: 'chan-1',
    contactExternalId: contactExternalId,
    contactName: 'Alice',
    contactAvatarUrl: avatarUrl,
    lastMessage: 'olá',
    lastMessageAt: DateTime(2026, 1, 1),
    source: source,
  );
}

Widget _panelHarness(ConversationModel conversation,
    {List<MessageModel> messages = const []}) {
  return ProviderScope(
    overrides: [
      selectedConversationProvider.overrideWith(() => _FakeSelected('conv-1')),
      conversationsProvider.overrideWith((ref) => Future.value([conversation])),
      messagesProvider('conv-1').overrideWith((ref) => Future.value(messages)),
      wsEventsProvider.overrideWith((ref) => const Stream.empty()),
      // Header lives inside the panel alongside the actions bar and notes
      // panel, which otherwise hit the network (team/tags/notes) and leave
      // pending timers. Stub them so only the avatar under test renders.
      teamMembersProvider.overrideWith(() => _FakeTeam()),
      tagControllerProvider.overrideWith(() => _FakeTags()),
      conversationNotesProvider('conv-1')
          .overrideWith((ref) => Future.value(const [])),
    ],
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      home: const Scaffold(body: ConversationDetailPanel()),
    ),
  );
}

void main() {
  setUpAll(() {
    FlutterSecureStorage.setMockInitialValues({});
  });

  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
  });

  testWidgets(
      'ConversationDetailPanel error state uses parseApiError, not raw toString',
      (WidgetTester tester) async {
    final error = Exception('secret-stack-detail');

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          // A conversation is selected so the outer conversationsProvider
          // error branch (not the empty state) renders.
          selectedConversationProvider.overrideWith(
            () => _FakeSelected('conv-1'),
          ),
          // Force the panel into its error branch.
          conversationsProvider.overrideWith((ref) => Future.error(error)),
          // Don't connect to a live WebSocket in tests.
          wsEventsProvider.overrideWith((ref) => const Stream.empty()),
        ],
        child: MaterialApp(
          theme: AppTheme.darkTheme,
          home: const Scaffold(body: ConversationDetailPanel()),
        ),
      ),
    );
    await tester.pumpAndSettle();

    // Localised, parsed message is shown.
    expect(find.text(parseApiError(error)), findsOneWidget);
    // Raw exception detail must NEVER leak to the UI.
    expect(find.textContaining('secret-stack-detail'), findsNothing);
  });

  testWidgets('header avatar renders through cached AppNetworkImage',
      (WidgetTester tester) async {
    await tester.pumpWidget(
        _panelHarness(_conversation(avatarUrl: 'https://cdn/a.png')));
    await tester.pump(); // avoid network fetch in pumpAndSettle

    expect(find.byType(AppNetworkImage), findsOneWidget);
  });

  // On email the address IS the identity: a display name alone hides who
  // actually wrote, on the one channel where anyone can claim any name.
  testWidgets('email conversations show the sender address in the header',
      (tester) async {
    await tester.pumpWidget(_panelHarness(_conversation(
        source: ConversationSource.email,
        contactExternalId: 'tiago@gmail.com')));
    await tester.pump();

    expect(
        find.byKey(const Key('conversation_sender_address')), findsOneWidget);
    expect(find.text('tiago@gmail.com'), findsOneWidget);
  });

  testWidgets('non-email conversations do not show an address line',
      (tester) async {
    await tester.pumpWidget(_panelHarness(_conversation()));
    await tester.pump();

    expect(find.byKey(const Key('conversation_sender_address')), findsNothing);
  });

  testWidgets('header falls back to initial-letter avatar when no photo',
      (WidgetTester tester) async {
    await tester.pumpWidget(_panelHarness(_conversation(avatarUrl: null)));
    await tester.pump();

    expect(find.byType(AppNetworkImage), findsNothing);
  });

  // O botão mais usado do produto. Aceso sobre a caixa vazia ele engolia o
  // clique: `_sendMessage` volta na primeira linha, sem uma palavra.
  testWidgets('o botão de enviar só acende com algo escrito', (tester) async {
    tester.view.physicalSize = const Size(1200, 1600);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    // Bot pausado: é o estado em que o composer aceita digitação.
    await tester.pumpWidget(_panelHarness(_conversation(
        botPaused: true,
        // Telegram não tem janela de 24h, que travaria o composer.
        source: ConversationSource.telegram)));
    await tester.pumpAndSettle();

    // O ícone aparece mais de uma vez na tela (composer e ações); o do
    // composer é o último na ordem de construção.
    final sendIcon = find.byIcon(Icons.send).last;

    IconButton button() => tester.widget<IconButton>(
        find.ancestor(of: sendIcon, matching: find.byType(IconButton)).first);

    expect(button().onPressed, isNull, reason: 'caixa vazia');

    await tester.enterText(find.byType(TextField).last, 'oi');
    await tester.pump();
    expect(button().onPressed, isNotNull);

    await tester.enterText(find.byType(TextField).last, '   ');
    await tester.pump();
    expect(button().onPressed, isNull,
        reason: 'espaço em branco não é mensagem');
  });

  // EMAIL is not a chat: no bot ever answers it server-side, so the takeover
  // bar ("Em atendimento por…", "Devolver ao bot"), the reply suggestions and
  // the AI-waiting banner are all noise. The composer is a mail composer —
  // subject line + tall body + an explicit send button — and it works
  // WITHOUT assuming the conversation.
  group('email composer', () {
    MessageModel mail(String id, String subject) => MessageModel(
          id: id,
          conversationId: 'conv-1',
          content: 'corpo',
          senderType: SenderType.customer,
          createdAt: DateTime(2026, 8, 25, 10),
          metadata: {'subject': subject},
        );

    // The composer starts FOLDED — a reply chip, not a permanent
    // subject+body block eating ~180px on every read.
    Future<void> expandComposer(WidgetTester tester) async {
      await tester.tap(find.byKey(const Key('email_composer_reply_chip')));
      await tester.pumpAndSettle();
    }

    testWidgets('email hides chat furniture and shows the mail composer',
        (tester) async {
      tester.view.physicalSize = const Size(1200, 1600);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      await tester.pumpWidget(_panelHarness(
        _conversation(
            source: ConversationSource.email,
            contactExternalId: 'a@b.com',
            botPaused: false),
        messages: [mail('m1', 'Proposta tomógrafo')],
      ));
      await tester.pumpAndSettle();

      // Folded by default: chip only, no fields.
      expect(find.byType(TakeoverBar), findsNothing);
      expect(
          find.byKey(const Key('email_composer_reply_chip')), findsOneWidget);
      expect(find.byKey(const Key('email_composer_body')), findsNothing);

      await expandComposer(tester);

      expect(find.byKey(const Key('email_composer_subject')), findsOneWidget);
      expect(find.byKey(const Key('email_composer_body')), findsOneWidget);
      expect(find.byKey(const Key('email_composer_send')), findsOneWidget);
      // No "assume conversation" gate: the body accepts typing immediately.
      final body = tester
          .widget<TextField>(find.byKey(const Key('email_composer_body')));
      expect(body.enabled, isTrue);
    });

    testWidgets('the chevron folds the composer back to the chip',
        (tester) async {
      tester.view.physicalSize = const Size(1200, 1600);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      await tester.pumpWidget(_panelHarness(
        _conversation(
            source: ConversationSource.email, contactExternalId: 'a@b.com'),
        messages: [mail('m1', 'Proposta')],
      ));
      await tester.pumpAndSettle();
      await expandComposer(tester);

      await tester.tap(find.byKey(const Key('email_composer_collapse')));
      await tester.pumpAndSettle();

      expect(
          find.byKey(const Key('email_composer_reply_chip')), findsOneWidget);
      expect(find.byKey(const Key('email_composer_body')), findsNothing);
    });

    testWidgets('subject prefills as Re: of the newest inbound subject',
        (tester) async {
      tester.view.physicalSize = const Size(1200, 1600);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      await tester.pumpWidget(_panelHarness(
        _conversation(
            source: ConversationSource.email, contactExternalId: 'a@b.com'),
        messages: [mail('m1', 'Proposta tomógrafo')],
      ));
      await tester.pumpAndSettle();
      await expandComposer(tester);

      final subject = tester
          .widget<TextField>(find.byKey(const Key('email_composer_subject')));
      expect(subject.controller!.text, 'Re: Proposta tomógrafo');
    });

    testWidgets('an inbound already starting with Re: is not double-prefixed',
        (tester) async {
      tester.view.physicalSize = const Size(1200, 1600);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      await tester.pumpWidget(_panelHarness(
        _conversation(
            source: ConversationSource.email, contactExternalId: 'a@b.com'),
        messages: [mail('m1', 'Re: Proposta')],
      ));
      await tester.pumpAndSettle();
      await expandComposer(tester);

      final subject = tester
          .widget<TextField>(find.byKey(const Key('email_composer_subject')));
      expect(subject.controller!.text, 'Re: Proposta');
    });

    // Design A (2026-08-27): while a mail is open in the reader, the panel's
    // contact header and notes bar yield — the reader header already names
    // the sender, and the stacked chrome was burying the message.
    testWidgets('opening a mail hides the contact header and notes bar',
        (tester) async {
      tester.view.physicalSize = const Size(1200, 1600);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      await tester.pumpWidget(_panelHarness(
        _conversation(
            source: ConversationSource.email, contactExternalId: 'a@b.com'),
        messages: [mail('m1', 'Proposta')],
      ));
      await tester.pumpAndSettle();
      expect(
          find.byKey(const Key('conversation_sender_address')), findsOneWidget);
      expect(find.byType(NotesPanel), findsOneWidget);

      await tester.tap(find.byKey(const Key('email_row_m1')));
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('email_reader_m1')), findsOneWidget);
      expect(
          find.byKey(const Key('conversation_sender_address')), findsNothing);
      expect(find.byType(NotesPanel), findsNothing);

      await tester.tap(find.byKey(const Key('email_reader_back')));
      await tester.pumpAndSettle();
      expect(
          find.byKey(const Key('conversation_sender_address')), findsOneWidget);
    });

    testWidgets('send button only lights with a non-blank body',
        (tester) async {
      tester.view.physicalSize = const Size(1200, 1600);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      await tester.pumpWidget(_panelHarness(
        _conversation(
            source: ConversationSource.email, contactExternalId: 'a@b.com'),
        messages: [mail('m1', 'Proposta')],
      ));
      await tester.pumpAndSettle();
      await expandComposer(tester);

      InkWell button() =>
          tester.widget<InkWell>(find.byKey(const Key('email_composer_send')));
      expect(button().onTap, isNull);

      await tester.enterText(
          find.byKey(const Key('email_composer_body')), 'segue proposta');
      await tester.pump();
      expect(button().onTap, isNotNull);
    });
  });

  // 24h window closed + nobody assumed: the three-layer stack (orange
  // banner + takeover bar + dead composer repeating the bar's hint)
  // collapses into ONE footer with the two actions that work.
  testWidgets('closed window shows the unified footer instead of the stack',
      (tester) async {
    tester.view.physicalSize = const Size(1200, 1600);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(_panelHarness(
      _conversation(), // whatsapp, bot active (not assumed)
      messages: [
        MessageModel(
          id: 'm1',
          conversationId: 'conv-1',
          content: 'oi',
          senderType: SenderType.customer,
          // Two days old: the 24h window is closed.
          createdAt: DateTime.now().subtract(const Duration(days: 2)),
        ),
      ],
    ));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('window_closed_footer')), findsOneWidget);
    expect(find.byKey(const Key('window_closed_template')), findsOneWidget);
    expect(find.byKey(const Key('window_closed_assume')), findsOneWidget);
    // The dead stack is gone: no takeover bar, no composer.
    expect(find.byType(TakeoverBar), findsNothing);
    expect(find.byIcon(Icons.attach_file), findsNothing);
  });

  testWidgets('an open window keeps the normal composer stack', (tester) async {
    tester.view.physicalSize = const Size(1200, 1600);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(_panelHarness(
      _conversation(),
      messages: [
        MessageModel(
          id: 'm1',
          conversationId: 'conv-1',
          content: 'oi',
          senderType: SenderType.customer,
          createdAt: DateTime.now().subtract(const Duration(hours: 1)),
        ),
      ],
    ));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('window_closed_footer')), findsNothing);
    expect(find.byType(TakeoverBar), findsOneWidget);
  });
}

class _FakeSelected extends SelectedConversation {
  _FakeSelected(this._seed);

  final String _seed;

  @override
  String? build() => _seed;
}

class _FakeTeam extends TeamMembers {
  @override
  Future<List<TeamMemberModel>> build() async => const [];
}

class _FakeTags extends TagController {
  @override
  Future<List<TagModel>> build() async => const [];
}
