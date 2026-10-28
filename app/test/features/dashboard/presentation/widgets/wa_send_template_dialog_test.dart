import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/features/dashboard/presentation/widgets/wa_send_template_dialog.dart';
import 'package:app/features/settings/domain/models/whatsapp_template_model.dart';
import 'package:app/features/settings/presentation/controllers/whatsapp_config_controller.dart';
import 'package:app/i18n/strings.g.dart';

// The template dialog stopped being a blind dropdown (2026-08-28): it now
// renders the body as the customer will receive it, with the {{n}} values
// applied live, and refuses to send while any variable is blank.

WhatsAppTemplateModel _tpl({
  String name = 'retomar_conversa',
  String body = 'Olá {{1}}! Tentamos falar sobre {{2}}. Pode responder?',
}) {
  return WhatsAppTemplateModel.fromJson({
    'name': name,
    'status': 'APPROVED',
    'language': 'pt_BR',
    'category': 'UTILITY',
    'components': [
      {
        'type': 'BODY',
        'text': body,
        'example': {
          'body_text': [
            ['Maria', 'o orçamento']
          ]
        },
      },
    ],
  });
}

class _FakeTemplates extends WhatsAppTemplates {
  _FakeTemplates(this._seed);
  final List<WhatsAppTemplateModel> _seed;

  @override
  Future<List<WhatsAppTemplateModel>> build(String channelId) async => _seed;
}

Future<void> _pump(WidgetTester tester,
    {List<WhatsAppTemplateModel>? templates, String? contactName}) async {
  tester.view.physicalSize = const Size(1000, 1400);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(ProviderScope(
    overrides: [
      whatsAppTemplatesProvider
          .overrideWith(() => _FakeTemplates(templates ?? [_tpl()])),
    ],
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      home: Scaffold(
        body: WaSendTemplateDialog(
          channelId: 'ch-1',
          recipientPhone: '556599990000',
          contactName: contactName,
        ),
      ),
    ),
  ));
  await tester.pumpAndSettle();
}

Future<void> _pickFirstTemplate(WidgetTester tester) async {
  await tester.tap(find.byKey(const Key('template_dropdown')));
  await tester.pumpAndSettle();
  await tester.tap(find.textContaining('retomar_conversa').last);
  await tester.pumpAndSettle();
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  testWidgets('selecting a template shows the as-received preview',
      (tester) async {
    await _pump(tester, contactName: 'Henrique');
    expect(find.byKey(const Key('template_preview')), findsNothing);

    await _pickFirstTemplate(tester);

    expect(find.byKey(const Key('template_preview')), findsOneWidget);
    // {{1}} pre-filled with the contact's name, {{2}} with Meta's example —
    // the preview never shows raw braces. Scoped to the preview: the same
    // values also live in the variable FIELDS below it.
    Finder inPreview(String text) => find.descendant(
        of: find.byKey(const Key('template_preview')),
        matching: find.textContaining(text));
    expect(inPreview('Olá Henrique!'), findsOneWidget);
    expect(inPreview('o orçamento'), findsOneWidget);
    expect(inPreview('{{'), findsNothing);
  });

  testWidgets('typing a variable rewrites the preview live', (tester) async {
    await _pump(tester, contactName: 'Henrique');
    await _pickFirstTemplate(tester);

    await tester.enterText(
        find.byKey(const Key('template_var_2')), 'a proposta do tomógrafo');
    await tester.pumpAndSettle();

    expect(
        find.descendant(
            of: find.byKey(const Key('template_preview')),
            matching: find.textContaining('a proposta do tomógrafo')),
        findsOneWidget);
  });

  testWidgets('send stays dead while a variable is blank', (tester) async {
    await _pump(tester); // no contact name → {{1}} seeds from example
    await _pickFirstTemplate(tester);

    ElevatedButton send() =>
        tester.widget<ElevatedButton>(find.byKey(const Key('template_send')));
    // Examples filled both vars, so it is enabled…
    expect(send().onPressed, isNotNull);

    // …and blanking one kills it: "{{2}}" must never reach a customer.
    await tester.enterText(find.byKey(const Key('template_var_2')), '');
    await tester.pumpAndSettle();
    expect(send().onPressed, isNull);
  });

  testWidgets('the create sheet opens with an honest approval notice',
      (tester) async {
    await _pump(tester);

    await tester.tap(find.byKey(const Key('template_create_new')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('create_template_notice')), findsOneWidget);
    // The first suggestion pre-fills the form.
    final name =
        tester.widget<TextField>(find.byKey(const Key('create_template_name')));
    expect(name.controller!.text, 'retomar_conversa');

    // "Do zero" clears it.
    await tester.tap(find.byKey(const Key('template_suggestion_2')));
    await tester.pumpAndSettle();
    expect(name.controller!.text, isEmpty);
  });
}
