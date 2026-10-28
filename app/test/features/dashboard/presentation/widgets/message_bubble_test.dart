import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/design_system/components/cached_network_image.dart';
import 'package:app/features/dashboard/domain/models/message_model.dart';
import 'package:app/features/dashboard/presentation/widgets/message_bubble.dart';
import 'package:app/i18n/strings.g.dart';

MessageModel _imageMessage({String? mediaUrl}) {
  return MessageModel(
    id: 'msg-1',
    conversationId: 'conv-1',
    content: '',
    senderType: SenderType.customer,
    createdAt: DateTime(2026, 1, 1, 10, 30),
    messageType: MessageType.image,
    mediaUrl: mediaUrl,
  );
}

Widget _harness(MessageModel message) {
  return MaterialApp(
    theme: AppTheme.darkTheme,
    home: Scaffold(
      body: MessageBubble(message: message),
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

  // Seen in the live inbox: a customer's "Olá, pode me ajudar?" rendered in a
  // bubble as wide as the cap, with the text shoved against its right edge —
  // so the left/right alignment stopped saying who spoke, which is the one
  // job it has. Cause: an `Align` around the (empty) attachment list, which
  // expands to whatever width it is offered even with nothing inside.
  testWidgets('a short message gets a short bubble', (tester) async {
    tester.view.physicalSize = const Size(1000, 800);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(_harness(MessageModel(
      id: 'm1',
      conversationId: 'c1',
      content: 'Oi',
      senderType: SenderType.customer,
      createdAt: DateTime(2026, 1, 1, 10, 30),
    )));
    await tester.pump();

    final width =
        tester.getSize(find.byKey(const Key('message_bubble_body'))).width;
    // The cap is 65% of the panel; "Oi" is nowhere near it.
    expect(width, lessThan(1000 * 0.65),
        reason: 'the bubble is stretched to the cap by an empty Align');
    expect(width, lessThan(200));
  });

  // The clock and the text belong on the speaker's side.
  testWidgets('an inbound bubble is laid out from the left', (tester) async {
    await tester.pumpWidget(_harness(MessageModel(
      id: 'm1',
      conversationId: 'c1',
      content: 'Oi',
      senderType: SenderType.customer,
      createdAt: DateTime(2026, 1, 1, 10, 30),
    )));
    await tester.pump();

    final column = tester.widget<Column>(find.descendant(
      of: find.byKey(const Key('message_bubble_body')),
      matching: find.byType(Column),
    ));
    expect(column.crossAxisAlignment, CrossAxisAlignment.start);
  });

  testWidgets('an outbound bubble is laid out from the right', (tester) async {
    await tester.pumpWidget(_harness(MessageModel(
      id: 'm1',
      conversationId: 'c1',
      content: 'Oi',
      senderType: SenderType.agent,
      createdAt: DateTime(2026, 1, 1, 10, 30),
    )));
    await tester.pump();

    final column = tester.widget<Column>(find.descendant(
      of: find.byKey(const Key('message_bubble_body')),
      matching: find.byType(Column),
    ));
    expect(column.crossAxisAlignment, CrossAxisAlignment.end);
  });

  testWidgets('renders image attachment through cached AppNetworkImage',
      (WidgetTester tester) async {
    await tester
        .pumpWidget(_harness(_imageMessage(mediaUrl: 'https://cdn/x.jpg')));
    await tester.pump(); // avoid network fetch hanging pumpAndSettle

    expect(find.byType(AppNetworkImage), findsOneWidget);
  });

  testWidgets('still renders AppNetworkImage (fallback) when url is null',
      (WidgetTester tester) async {
    await tester.pumpWidget(_harness(_imageMessage(mediaUrl: null)));
    await tester.pump();

    // AppNetworkImage handles the null/empty url internally via its fallback,
    // so the widget is still mounted and no raw Image.network is used.
    expect(find.byType(AppNetworkImage), findsOneWidget);
  });

  // US-009 (review fixes): an outbound message whose channel send failed
  // must show the "not delivered" marker so the attendant knows the
  // customer never received it.
  testWidgets('shows not-delivered indicator on a send-failed message',
      (WidgetTester tester) async {
    final failed = MessageModel(
      id: 'msg-2',
      conversationId: 'conv-1',
      content: 'did this arrive?',
      senderType: SenderType.agent,
      createdAt: DateTime(2026, 1, 1, 10, 30),
      failedAt: DateTime(2026, 1, 1, 10, 31),
      failedReason: 'twilio: 21610 blocked recipient',
    );
    await tester.pumpWidget(_harness(failed));
    await tester.pump();

    expect(find.text(t.conversation.message_send_failed), findsOneWidget);
    expect(find.byIcon(Icons.error_outline), findsOneWidget);
  });

  testWidgets('no failure indicator on a delivered message',
      (WidgetTester tester) async {
    final sent = MessageModel(
      id: 'msg-3',
      conversationId: 'conv-1',
      content: 'all good',
      senderType: SenderType.agent,
      createdAt: DateTime(2026, 1, 1, 10, 30),
    );
    await tester.pumpWidget(_harness(sent));
    await tester.pump();

    expect(find.text(t.conversation.message_send_failed), findsNothing);
    expect(sent.sendFailed, isFalse);
  });
  // Reported from the phone: a dark shape at the bottom-left of an outbound
  // bubble, "like a corner being pulled into the middle of the message".
  //
  // The bubble itself draws clean — rendering it to a PNG shows a plain
  // rounded rectangle with no artifact. What sits on top is a Material text
  // selection handle: every message body was a SelectableText, so a long press
  // selected a word and left the handle painted over the bubble.
  //
  // It is also a gesture bug independent of the pixels. The bubble wraps that
  // same text in its own long-press (the copy menu), so two long-press
  // recognizers entered the arena for one touch and the deeper one — the
  // selection — tends to take it.
  testWidgets('an outbound bubble has no selectable text to leave a handle on',
      (tester) async {
    await tester.pumpWidget(_harness(MessageModel(
      id: 'msg-sel',
      conversationId: 'conv-1',
      content: 'Salve Maria, não consegui visualizar a imagem que mandou',
      senderType: SenderType.agent,
      senderLabel: 'Tiago',
      createdAt: DateTime(2026, 8, 18, 13, 52),
    )));
    await tester.pumpAndSettle();

    expect(
      find.byType(SelectableText),
      findsNothing,
      reason: 'a SelectableText here competes with the bubble long-press and '
          'leaves a selection handle drawn over the message',
    );
    // The text still has to be on screen — turning selection off must not
    // turn the message off.
    expect(find.textContaining('Salve Maria'), findsOneWidget);
  });

  testWidgets('the long press still opens the copy menu', (tester) async {
    await tester.pumpWidget(_harness(MessageModel(
      id: 'msg-menu',
      conversationId: 'conv-1',
      content: 'Salve Maria',
      senderType: SenderType.agent,
      senderLabel: 'Tiago',
      createdAt: DateTime(2026, 8, 18, 13, 52),
    )));
    await tester.pumpAndSettle();

    await tester.longPress(find.byKey(const Key('message_bubble_body')));
    await tester.pumpAndSettle();

    // Copying is what the long press is FOR — it is the reason free selection
    // can be given up.
    expect(find.text(t.message_actions.copy_text), findsOneWidget);
    expect(find.text(t.message_actions.copy_message_id), findsOneWidget);
  });

  // The transcript sits inside a SelectionArea (conversation_detail_panel):
  // the text must be selectable with the mouse there, AND the bubble's own
  // long press must still win — it is the deeper recognizer. Both, or the
  // fix for one is the bug of the other (2026-08-18 shipped exactly that).
  testWidgets(
      'inside a SelectionArea the text selects and the menu still opens',
      (tester) async {
    final message = MessageModel(
      id: 'msg-sel',
      conversationId: 'conv-1',
      content: 'Selecione isto',
      senderType: SenderType.customer,
      createdAt: DateTime(2026, 8, 18, 13, 52),
    );
    String? selected;
    await tester.pumpWidget(MaterialApp(
      theme: AppTheme.darkTheme,
      home: Scaffold(
        body: SelectionArea(
          onSelectionChanged: (c) => selected = c?.plainText,
          child: MessageBubble(message: message),
        ),
      ),
    ));
    await tester.pumpAndSettle();

    // Mouse drag across the text selects it.
    final textCenter = tester.getCenter(find.text('Selecione isto'));
    final gesture = await tester.startGesture(
      textCenter - const Offset(40, 0),
      kind: PointerDeviceKind.mouse,
    );
    await tester.pump();
    await gesture.moveTo(textCenter + const Offset(40, 0));
    await tester.pump();
    await gesture.up();
    await tester.pumpAndSettle();
    expect(selected, isNotNull);
    expect(selected, isNotEmpty,
        reason: 'a mouse drag over the bubble must select text');

    // And the long press on the bubble still opens the copy menu.
    await tester.longPress(find.byKey(const Key('message_bubble_body')));
    await tester.pumpAndSettle();
    expect(find.text(t.message_actions.copy_text), findsOneWidget);
  });

  // A shared contact is a CARD — name, business, number — not a line of
  // prose. It read as an ordinary message and nobody saw the number in it.
  group('shared contact card', () {
    MessageModel contactMessage({List<Map<String, dynamic>>? contacts}) =>
        MessageModel(
          id: 'm-card',
          conversationId: 'c1',
          content:
              '📇 Contato compartilhado: Gerente Paulo celular (+55 65 9333-4444)',
          senderType: SenderType.customer,
          createdAt: DateTime(2026, 1, 1, 10, 30),
          metadata: {
            'contacts': contacts ??
                [
                  {
                    'name': {'formatted_name': 'Gerente Paulo celular'},
                    'phones': [
                      {'phone': '+55 65 9333-4444', 'wa_id': '556593334444'}
                    ],
                    'business_name': 'Rede São Benedito',
                    'business_description':
                        'Loja pertencente a Rede Exemplo Fictício',
                  }
                ],
          },
        );

    testWidgets('renders name, business, description and the number',
        (tester) async {
      await tester.pumpWidget(_harness(contactMessage()));
      await tester.pump();

      expect(find.byKey(const Key('shared_contact_card')), findsOneWidget);
      expect(find.text(t.conversation.shared_contact), findsOneWidget);
      expect(find.text('Gerente Paulo celular'), findsOneWidget);
      expect(find.text('Rede São Benedito'), findsOneWidget);
      expect(find.text('Loja pertencente a Rede Exemplo Fictício'),
          findsOneWidget);
      expect(find.text('+55 65 9333-4444'), findsOneWidget);
      // The prose fallback is NOT shown on top of the card.
      expect(find.textContaining('📇'), findsNothing);
    });

    testWidgets('a business name equal to the saved name is not repeated',
        (tester) async {
      await tester.pumpWidget(_harness(contactMessage(contacts: [
        {
          'name': {'formatted_name': 'Hospital Regional'},
          'phones': [
            {'phone': '+55 65 3000-0000'}
          ],
          'business_name': 'hospital  regional',
        }
      ])));
      await tester.pump();

      expect(find.text('Hospital Regional'), findsOneWidget);
      expect(find.text('hospital  regional'), findsNothing);
    });

    testWidgets('tapping the number copies it', (tester) async {
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

      await tester.pumpWidget(_harness(contactMessage()));
      await tester.pump();
      await tester
          .tap(find.byKey(const Key('shared_contact_phone_+55 65 9333-4444')));
      await tester.pump();

      expect(copied, ['+55 65 9333-4444']);
      expect(find.text(t.conversation.shared_contact_copied), findsOneWidget);
    });

    testWidgets('a text message with no contacts metadata is unchanged',
        (tester) async {
      await tester.pumpWidget(_harness(MessageModel(
        id: 'm-plain',
        conversationId: 'c1',
        content: 'só texto',
        senderType: SenderType.customer,
        createdAt: DateTime(2026, 1, 1, 10, 30),
        // Any metadata WITHOUT a contacts payload. (It used to say
        // unsupported_type here, which now legitimately renders the
        // notice card instead of the text.)
        metadata: const {'wa_message_id': 'wamid.x'},
      )));
      await tester.pump();

      expect(find.byKey(const Key('shared_contact_card')), findsNothing);
      expect(find.text('só texto'), findsOneWidget);
    });
  });

  // Content Meta refused to deliver is an EVENT, not a customer utterance:
  // it renders as a notice card — title, no speech bubble, no sender label —
  // keyed off metadata.unsupported_type (the server's stamp), never off the
  // placeholder text.
  testWidgets('unsupported content renders as a notice card, not a bubble',
      (tester) async {
    await tester.pumpWidget(_harness(MessageModel(
      id: 'm-unsup',
      conversationId: 'conv-1',
      content: '⚠️ O cliente enviou uma mensagem que o WhatsApp não entrega…',
      senderType: SenderType.customer,
      createdAt: DateTime(2026, 8, 26, 16, 48),
      metadata: const {'unsupported_type': 'unsupported'},
    )));
    await tester.pump();

    expect(
        find.byKey(const Key('unsupported_content_m-unsup')), findsOneWidget);
    expect(find.text(t.dashboard.wa_unsupported_title), findsOneWidget);
    // The speech-bubble body must NOT render — no warning in the
    // customer's mouth.
    expect(find.byKey(const Key('message_bubble_body')), findsNothing);
    expect(find.textContaining('⚠️'), findsNothing);
  });

  testWidgets('a normal customer message still gets its bubble',
      (tester) async {
    await tester.pumpWidget(_harness(MessageModel(
      id: 'm-ok',
      conversationId: 'conv-1',
      content: 'bom dia',
      senderType: SenderType.customer,
      createdAt: DateTime(2026, 8, 26, 16, 48),
    )));
    await tester.pump();

    expect(find.byKey(const Key('message_bubble_body')), findsOneWidget);
    expect(find.byKey(const Key('unsupported_content_m-ok')), findsNothing);
  });
}
