import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/design_system/components/cached_network_image.dart';
import 'package:app/features/dashboard/domain/models/conversation_model.dart';
import 'package:app/features/dashboard/domain/models/lead_profile_model.dart';
import 'package:app/features/dashboard/domain/models/message_attachment.dart';
import 'package:app/features/dashboard/domain/models/message_model.dart';
import 'package:app/features/dashboard/presentation/controllers/dashboard_controller.dart';
import 'package:app/features/dashboard/presentation/widgets/contact_detail_sheet.dart';
import 'package:app/i18n/strings.g.dart';

ConversationModel _conversation() => ConversationModel(
      id: 'conv-1',
      channelId: 'chan-1',
      contactExternalId: '5511999999999',
      contactName: 'Jane Doe',
      lastMessage: 'hi',
      lastMessageAt: DateTime(2026, 1, 1, 10, 30),
      source: ConversationSource.whatsapp,
    );

MessageModel _imageMessage({String? mediaUrl}) => MessageModel(
      id: 'msg-1',
      conversationId: 'conv-1',
      content: '',
      senderType: SenderType.customer,
      createdAt: DateTime(2026, 1, 1, 10, 30),
      messageType: MessageType.image,
      mediaUrl: mediaUrl,
    );

/// Fake actions notifier capturing the manual metadata edit (US-014)
/// so widget tests can assert the save path without a real Dio call.
class _FakeConversationActions extends ConversationActions {
  String? leadId;
  String? fieldKey;
  String? value;

  @override
  Future<void> updateLeadMetadata(
    String conversationId,
    String leadId,
    String fieldKey,
    String value,
  ) async {
    this.leadId = leadId;
    this.fieldKey = fieldKey;
    this.value = value;
  }
}

Widget _harness(
  List<MessageModel> media, {
  LeadProfileModel profile = const LeadProfileModel.empty(),
  ConversationActions Function()? actions,
}) {
  return ProviderScope(
    overrides: [
      conversationMediaProvider('conv-1')
          .overrideWith((ref) => Future.value(media)),
      contactStatsProvider('conv-1').overrideWith(
        (ref) => Future.value(
          const ContactStatsModel(totalConversations: 1, totalMessages: 1),
        ),
      ),
      salesProfileProvider('conv-1')
          .overrideWith((ref) => Future.value(profile)),
      if (actions != null) conversationActionsProvider.overrideWith(actions),
    ],
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      home: Scaffold(
        body: ContactDetailSheet(conversation: _conversation()),
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

  testWidgets('image gallery renders thumbnails through cached AppNetworkImage',
      (tester) async {
    await tester
        .pumpWidget(_harness([_imageMessage(mediaUrl: 'https://cdn/x.jpg')]));
    await tester.pump();

    // Switch to the Files tab where the gallery lives, letting the tab
    // animation settle and the media FutureProvider resolve.
    await tester.tap(find.text(t.contact_sheet.tab_files));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));
    await tester.pump(); // avoid network fetch hanging pumpAndSettle

    expect(find.byType(AppNetworkImage), findsOneWidget);
  });

  testWidgets(
      'AppNetworkImage still renders (fallback) when media url is empty',
      (tester) async {
    await tester.pumpWidget(_harness([_imageMessage(mediaUrl: null)]));
    await tester.pump();

    await tester.tap(find.text(t.contact_sheet.tab_files));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));
    await tester.pump();

    expect(find.byType(AppNetworkImage), findsOneWidget);
  });

  testWidgets('sales profile shows empty state when not in a funnel',
      (tester) async {
    await tester.pumpWidget(_harness(const []));
    await tester.pump();

    expect(find.text(t.sales.lead_profile.section_title.toUpperCase()),
        findsOneWidget);
    expect(find.text(t.sales.lead_profile.empty), findsOneWidget);
  });

  testWidgets('sales profile renders fields, origin badges and buy intent',
      (tester) async {
    const profile = LeadProfileModel(
      inFunnel: true,
      leadId: 'lead-1',
      funnelId: 'funnel-1',
      status: 'qualified',
      score: 42,
      fields: [
        LeadFieldModel(
          key: 'name',
          label: 'Name',
          type: 'text',
          value: 'Jane',
          source: LeadFieldSource.ai,
        ),
        LeadFieldModel(
          key: 'email',
          label: 'Email',
          type: 'text',
          value: 'jane@x.com',
          source: LeadFieldSource.human,
        ),
        LeadFieldModel(
          key: 'budget',
          label: 'Budget',
          type: 'number',
          required: true,
        ),
      ],
    );
    await tester.pumpWidget(_harness(const [], profile: profile));
    await tester.pump();

    // Collected values surface with their origin badges.
    expect(find.text('Jane'), findsOneWidget);
    expect(find.text('jane@x.com'), findsOneWidget);
    expect(find.text(t.sales.lead_profile.origin_ai), findsOneWidget);
    expect(find.text(t.sales.lead_profile.origin_human), findsOneWidget);

    // Required-but-empty field is highlighted as pending.
    expect(find.text(t.sales.lead_profile.pending), findsOneWidget);

    // Status label is localized (not the raw enum value).
    expect(
      find.textContaining(t.sales.lead_profile.lead_status.qualified),
      findsOneWidget,
    );
  });

  testWidgets('sales profile shows buy-intent chip when intent detected',
      (tester) async {
    final profile = LeadProfileModel(
      inFunnel: true,
      leadId: 'lead-1',
      status: 'responded',
      buyIntentAt: DateTime(2026, 1, 2),
    );
    await tester.pumpWidget(_harness(const [], profile: profile));
    await tester.pump();

    expect(find.text(t.sales.lead_profile.buy_intent), findsOneWidget);
  });

  testWidgets('each field exposes an edit pencil when a lead exists',
      (tester) async {
    const profile = LeadProfileModel(
      inFunnel: true,
      leadId: 'lead-1',
      fields: [
        LeadFieldModel(key: 'name', label: 'Name', type: 'text', value: 'Jane'),
        LeadFieldModel(key: 'budget', label: 'Budget', type: 'number'),
      ],
    );
    await tester.pumpWidget(_harness(const [], profile: profile));
    await tester.pump();

    expect(find.byIcon(Icons.edit_outlined), findsNWidgets(2));
  });

  testWidgets('no edit pencil when the profile carries no lead id',
      (tester) async {
    const profile = LeadProfileModel(
      inFunnel: true,
      fields: [
        LeadFieldModel(key: 'name', label: 'Name', type: 'text', value: 'Jane'),
      ],
    );
    await tester.pumpWidget(_harness(const [], profile: profile));
    await tester.pump();

    expect(find.byIcon(Icons.edit_outlined), findsNothing);
  });

  testWidgets('editing a field saves the new value with the human source path',
      (tester) async {
    tester.view.physicalSize = const Size(1000, 2400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    final fake = _FakeConversationActions();
    const profile = LeadProfileModel(
      inFunnel: true,
      leadId: 'lead-1',
      fields: [
        LeadFieldModel(key: 'name', label: 'Name', type: 'text', value: 'Jane'),
      ],
    );
    await tester.pumpWidget(
      _harness(const [], profile: profile, actions: () => fake),
    );
    await tester.pump();

    await tester.tap(find.byIcon(Icons.edit_outlined).first);
    await tester.pumpAndSettle();

    // The type-aware dialog opened with Save/Cancel actions.
    expect(find.text(t.common.save), findsOneWidget);
    expect(find.text(t.common.cancel), findsOneWidget);

    await tester.enterText(find.byType(TextField).first, 'Janet');
    await tester.tap(find.text(t.common.save));
    await tester.pumpAndSettle();

    expect(fake.leadId, 'lead-1');
    expect(fake.fieldKey, 'name');
    expect(fake.value, 'Janet');
  });

  // Pivot check (2026-08): the sheet leads with the SALES card, the support
  // bookkeeping is a footer whisper, and email attachments show in Arquivos.
  testWidgets('the sales card renders above, stats demoted to the footer',
      (tester) async {
    await tester.pumpWidget(_harness(const []));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('contact_sales_card')), findsOneWidget);
    expect(find.byKey(const Key('contact_footer_stats')), findsOneWidget);
    // The old headline stat labels are gone.
    expect(find.text(t.contact_sheet.stat_customer_since), findsNothing);
  });

  testWidgets('a lead with an id offers the new-proposal action',
      (tester) async {
    await tester.pumpWidget(_harness(const [],
        profile: const LeadProfileModel(
          inFunnel: true,
          leadId: 'lead-1',
          score: 10,
        )));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('contact_new_proposal')), findsOneWidget);
  });

  testWidgets('email attachments appear in the files tab', (tester) async {
    final mail = MessageModel(
      id: 'm-mail',
      conversationId: 'conv-1',
      content: 'segue anexo',
      senderType: SenderType.customer,
      createdAt: DateTime(2026, 8, 26, 20, 8),
      attachments: const [
        MessageAttachment(
          id: 'att-1',
          filename: 'DAS.pdf',
          contentType: 'application/pdf',
          sizeBytes: 157878,
          isInline: false,
          scanStatus: AttachmentScanStatus.clean,
        ),
      ],
    );
    await tester.pumpWidget(_harness([mail]));
    await tester.pumpAndSettle();

    await tester.tap(find.text(t.contact_sheet.tab_files));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('files_attachments_m-mail')), findsOneWidget);
    expect(find.text('DAS.pdf'), findsOneWidget);
  });

  testWidgets('each email group is titled with its subject and date',
      (tester) async {
    // Same PDF arriving in two different emails — without the per-email
    // header the two rows are indistinguishable.
    MessageModel mail(String id, DateTime at, {String? subject}) =>
        MessageModel(
          id: id,
          conversationId: 'conv-1',
          content: 'segue anexo',
          senderType: SenderType.customer,
          createdAt: at,
          metadata: subject == null ? null : {'subject': subject},
          attachments: const [
            MessageAttachment(
              id: 'att-1',
              filename: 'DAS.pdf',
              contentType: 'application/pdf',
              sizeBytes: 157878,
              isInline: false,
              scanStatus: AttachmentScanStatus.clean,
            ),
          ],
        );
    await tester.pumpWidget(_harness([
      mail('m-1', DateTime(2026, 8, 26, 20, 8), subject: 'DAS de agosto'),
      mail('m-2', DateTime(2026, 8, 27, 9, 15)),
    ]));
    await tester.pumpAndSettle();

    await tester.tap(find.text(t.contact_sheet.tab_files));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('files_email_header_m-1')), findsOneWidget);
    expect(find.text('DAS de agosto'), findsOneWidget);
    expect(find.textContaining('26/08/2026'), findsOneWidget);
    // A subject-less email still gets a header, with an honest fallback.
    expect(find.text(t.contact_sheet.files_email_no_subject), findsOneWidget);
  });
}
