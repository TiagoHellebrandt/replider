import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/design_system/components/cached_network_image.dart';
import 'package:app/features/dashboard/domain/models/conversation_model.dart';
import 'package:app/features/dashboard/presentation/widgets/conversation_tile.dart';
import 'package:app/i18n/strings.g.dart';

ConversationModel _conversation({
  String? avatarUrl,
  String leadId = '',
  String leadStageLabel = '',
  String inboxCategory = '',
}) {
  return ConversationModel(
    id: 'conv-1',
    channelId: 'chan-1',
    contactExternalId: '5511999999999',
    contactName: 'Alice',
    contactAvatarUrl: avatarUrl,
    lastMessage: 'olá',
    lastMessageAt: DateTime(2026, 1, 1),
    source: ConversationSource.whatsapp,
    leadId: leadId,
    leadStageLabel: leadStageLabel,
    inboxCategory: inboxCategory,
  );
}

Widget _harness(ConversationModel conversation) {
  return ProviderScope(
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      home: Scaffold(
        body: ConversationTile(conversation: conversation),
      ),
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

  // US-M024. An attendant about to answer needs to know whether this is a
  // stranger or a deal in negotiation.
  testWidgets('shows the pipeline stage when the contact is a lead',
      (tester) async {
    await tester.pumpWidget(
        _harness(_conversation(leadId: 'lead1', leadStageLabel: 'Negociação')));
    await tester.pump();

    expect(find.text('Negociação'), findsOneWidget);
    expect(
        find.byKey(const Key('conversation_lead_chip_conv-1')), findsOneWidget);
  });

  // A pure support conversation must render exactly as before — no chip
  // that means nothing.
  testWidgets('shows no chip when the contact is not a lead', (tester) async {
    await tester.pumpWidget(_harness(_conversation()));
    await tester.pump();

    expect(
        find.byKey(const Key('conversation_lead_chip_conv-1')), findsNothing);
  });

  testWidgets('renders contact avatar through cached AppNetworkImage',
      (WidgetTester tester) async {
    await tester
        .pumpWidget(_harness(_conversation(avatarUrl: 'https://cdn/a.png')));
    await tester.pump(); // avoid network fetch in pumpAndSettle

    expect(find.byType(AppNetworkImage), findsOneWidget);
  });

  testWidgets('falls back to initial-letter avatar when no photo',
      (WidgetTester tester) async {
    await tester.pumpWidget(_harness(_conversation(avatarUrl: null)));
    await tester.pump();

    expect(find.byType(AppNetworkImage), findsNothing);
    expect(find.text('A'), findsOneWidget);
  });

  // ── Inbox triage (§6.10 camada 3) ────────────────────────────────────────
  //
  // Only two of the seven categories earn a chip. A badge that is always
  // there stops being read — labelling every row "cliente" or "outros" would
  // add noise to every one and information to none.

  // Before everything, including the pipeline stage: a message we believe is
  // a scam must not read as an ordinary supplier invoice in the list.
  testWidgets('a quarantined conversation is marked in the list',
      (tester) async {
    await tester
        .pumpWidget(_harness(_conversation(inboxCategory: 'quarantine')));
    await tester.pump();

    expect(find.byKey(const Key('conversation_category_chip_conv-1')),
        findsOneWidget);
    expect(find.text(t.dashboard.category_quarantine), findsOneWidget);
  });

  testWidgets('a hot lead is marked in the list', (tester) async {
    await tester.pumpWidget(_harness(_conversation(inboxCategory: 'hot_lead')));
    await tester.pump();

    expect(find.text(t.dashboard.category_hot_lead), findsOneWidget);
  });

  testWidgets('an ordinary conversation gets no category chip', (tester) async {
    for (final category in ['customer', 'other', 'newsletter', '']) {
      await tester.pumpWidget(_harness(_conversation(inboxCategory: category)));
      await tester.pump();

      expect(find.byKey(const Key('conversation_category_chip_conv-1')),
          findsNothing,
          reason: 'category $category must not earn a chip');
    }
  });
}
