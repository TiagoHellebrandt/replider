import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/features/dashboard/domain/models/message_model.dart';
import 'package:app/features/dashboard/presentation/widgets/email_inbox_view.dart';
import 'package:app/i18n/strings.g.dart';

// Email as a TRADITIONAL inbox grouped by contact: a list of mails, and a
// click opens the READER over the whole panel — no accordion, no fixed-height
// iframe trapping the scroll.

MessageModel _mail({
  required String id,
  required DateTime at,
  String? subject,
  String content = 'corpo do e-mail\nsegunda linha',
  SenderType sender = SenderType.customer,
}) {
  return MessageModel(
    id: id,
    conversationId: 'conv-1',
    content: content,
    senderType: sender,
    createdAt: at,
    metadata: subject == null ? null : {'subject': subject},
  );
}

Future<void> _pump(WidgetTester tester, List<MessageModel> messages) async {
  tester.view.physicalSize = const Size(1000, 1600);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(ProviderScope(
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      home: Scaffold(
        body: EmailInboxView(
          messages: messages,
          scrollController: ScrollController(),
        ),
      ),
    ),
  ));
  await tester.pump();
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  testWidgets('the list shows one row per mail, subject leading',
      (tester) async {
    await _pump(tester, [
      _mail(
          id: 'm1',
          at: DateTime(2026, 8, 25, 10),
          subject: 'Proposta tomógrafo'),
      _mail(id: 'm2', at: DateTime(2026, 8, 25, 12), subject: 'Re: Proposta'),
    ]);

    expect(find.byKey(const Key('email_inbox_view')), findsOneWidget);
    expect(find.text('Proposta tomógrafo'), findsOneWidget);
    expect(find.text('Re: Proposta'), findsOneWidget);
    // Nothing is open until a click — an inbox, not an accordion.
    expect(find.byKey(const Key('email_reader_m1')), findsNothing);
    expect(find.byKey(const Key('email_reader_m2')), findsNothing);
  });

  testWidgets('clicking a row opens the reader over the whole panel',
      (tester) async {
    await _pump(tester, [
      _mail(
          id: 'm1',
          at: DateTime(2026, 8, 25, 10),
          subject: 'Proposta',
          content: 'texto completo da proposta'),
    ]);

    await tester.tap(find.byKey(const Key('email_row_m1')));
    await tester.pump();

    expect(find.byKey(const Key('email_reader_m1')), findsOneWidget);
    expect(find.byKey(const Key('email_reader_subject')), findsOneWidget);
    expect(find.text('texto completo da proposta'), findsOneWidget);
    // The list is gone — the reader owns the panel.
    expect(find.byKey(const Key('email_inbox_view')), findsNothing);
  });

  testWidgets('back returns to the list', (tester) async {
    await _pump(tester, [
      _mail(id: 'm1', at: DateTime(2026, 8, 25, 10), subject: 'Proposta'),
    ]);

    await tester.tap(find.byKey(const Key('email_row_m1')));
    await tester.pump();
    await tester.tap(find.byKey(const Key('email_reader_back')));
    await tester.pump();

    expect(find.byKey(const Key('email_inbox_view')), findsOneWidget);
    expect(find.byKey(const Key('email_reader_m1')), findsNothing);
  });

  testWidgets('a mail with no subject says so instead of a blank line',
      (tester) async {
    await _pump(tester, [
      _mail(id: 'm1', at: DateTime(2026, 8, 25, 10)),
    ]);

    expect(find.text(t.email_inbox.no_subject), findsOneWidget);
  });

  // The verdict must be visible ON THE ROW: a forged sender flagged only
  // inside the opened message is a warning after the fact.
  testWidgets('a failed sender-auth verdict shows on the list row',
      (tester) async {
    final failed = MessageModel(
      id: 'm1',
      conversationId: 'conv-1',
      content: 'boleto atualizado em anexo',
      senderType: SenderType.customer,
      createdAt: DateTime(2026, 8, 25, 10),
      metadata: const {
        'subject': 'Boleto',
        'email_auth': {'suspicious': true, 'trustworthy': false},
      },
    );
    await _pump(tester, [failed]);

    expect(find.byKey(const Key('email_auth_failed')), findsOneWidget);
  });

  // Verified is SILENT — badge wallpaper is how the one that matters gets
  // ignored.
  testWidgets('a verified sender gets no badge', (tester) async {
    final verified = MessageModel(
      id: 'm1',
      conversationId: 'conv-1',
      content: 'salve',
      senderType: SenderType.customer,
      createdAt: DateTime(2026, 8, 25, 10),
      metadata: const {
        'subject': 'Oi',
        'email_auth': {'trustworthy': true, 'suspicious': false},
      },
    );
    await _pump(tester, [verified]);

    expect(find.byKey(const Key('email_auth_failed')), findsNothing);
    expect(find.byKey(const Key('email_auth_unknown')), findsNothing);
  });
}
