import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/sales/presentation/controllers/sales_record_controller.dart';
import 'package:app/features/sales/presentation/widgets/register_sale_sheet.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

// This form feeds the sales record. The tests are about the money being
// right, and about the person seeing the margin before they commit to it.

class _FakeActions implements SaleActions {
  int? gross;
  int? cost;
  int? tax;
  int? cycle;
  bool? fromAI;
  String? contactId;
  String? leadId;

  @override
  Future<Sale> register({
    String contactId = '',
    String leadId = '',
    required int grossCents,
    int costCents = 0,
    int taxCents = 0,
    int cycleDays = 0,
    DateTime? soldAt,
    List<SaleItem> items = const [],
    String notes = '',
    bool fromAI = false,
  }) async {
    gross = grossCents;
    cost = costCents;
    tax = taxCents;
    cycle = cycleDays;
    this.fromAI = fromAI;
    this.contactId = contactId;
    this.leadId = leadId;
    return Sale(id: 's1', contactId: contactId, grossCents: grossCents);
  }

  @override
  Future<SaleProposal> extract(String conversationId) async =>
      const SaleProposal(found: false);
}

Future<_FakeActions> _pump(WidgetTester tester,
    {SaleProposal? proposal,
    String contactId = 'c1',
    String leadId = ''}) async {
  tester.view.physicalSize = const Size(900, 2000);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  final actions = _FakeActions();
  await tester.pumpWidget(ProviderScope(
    overrides: [
      saleActionsProvider.overrideWithValue(actions),
      dioProvider.overrideWithValue(Dio()),
    ],
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      home: Scaffold(
        body: RegisterSaleSheet(
            contactId: contactId, leadId: leadId, proposal: proposal),
      ),
    ),
  ));
  await tester.pumpAndSettle();
  return actions;
}

Future<void> _type(WidgetTester tester, String label, String value) async {
  await tester.enterText(find.byKey(Key('sale_field_$label')), value);
  await tester.pumpAndSettle();
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  // The form talks in reais because that is what a person says out loud; the
  // wire talks in cents because money in fractions accumulates error.
  testWidgets('sends cents, not reais', (tester) async {
    final actions = await _pump(tester);

    await _type(tester, t.sales_record.gross, '1500,00');
    await tester.tap(find.byKey(const Key('sale_submit')));
    await tester.pumpAndSettle();

    expect(actions.gross, 150000);
  });

  // The form used to convert with `replaceAll('.', '').replaceAll(',', '.')`,
  // which destroys the US notation: 1,234.56 lost its point, gained a decimal
  // comma and landed as 1.23456. A US customer recorded a sale of $1,234.56 and
  // the database stored $1.23 — a thousandfold error, silent, on the very
  // figure everything downstream is calculated from.
  testWidgets('a US-notation amount records the amount that was typed',
      (tester) async {
    final actions = await _pump(tester);

    await _type(tester, t.sales_record.gross, '1,234.56');
    await tester.tap(find.byKey(const Key('sale_submit')));
    await tester.pumpAndSettle();

    expect(actions.gross, 123456);
  });

  // The same amount in the other notation has to reach the same cents, or one
  // of the two customers is recording a different sale from the one they read.
  testWidgets('both notations record the same sale', (tester) async {
    final us = await _pump(tester);
    await _type(tester, t.sales_record.gross, '1,234.56');
    await tester.tap(find.byKey(const Key('sale_submit')));
    await tester.pumpAndSettle();

    final br = await _pump(tester);
    await _type(tester, t.sales_record.gross, '1.234,56');
    await tester.tap(find.byKey(const Key('sale_submit')));
    await tester.pumpAndSettle();

    expect(us.gross, br.gross);
    expect(us.gross, 123456);
  });

  // The margin, shown while they type. Finding it out afterwards, in a
  // report, is how a good month turns out to have been a bad one.
  testWidgets('shows what the company keeps as the numbers are typed',
      (tester) async {
    await _pump(tester);

    await _type(tester, t.sales_record.gross, '1000,00');
    await _type(tester, t.sales_record.cost, '720,00');
    await _type(tester, t.sales_record.tax, '140,00');

    expect(find.byKey(const Key('sale_net_preview')), findsOneWidget);
    expect(find.text('R\$ 140,00'), findsOneWidget);
  });

  // Warned, never blocked: discounts, loss leaders and typos all look like
  // this, and only a person can tell which.
  testWidgets('warns about an underwater sale without blocking it',
      (tester) async {
    final actions = await _pump(tester);

    await _type(tester, t.sales_record.gross, '1000,00');
    await _type(tester, t.sales_record.cost, '1200,00');

    expect(find.byKey(const Key('sale_underwater_warning')), findsOneWidget);

    await tester.tap(find.byKey(const Key('sale_submit')));
    await tester.pumpAndSettle();
    expect(actions.gross, 100000, reason: 'recorded anyway');
  });

  // Nothing to warn about before there are numbers.
  testWidgets('an empty form shows no margin and no warning', (tester) async {
    await _pump(tester);

    expect(find.byKey(const Key('sale_net_preview')), findsNothing);
    expect(find.byKey(const Key('sale_underwater_warning')), findsNothing);
  });

  // ── The AI proposal ──────────────────────────────────────────────────────

  // The whole interaction is one sentence, so it leads — and the fields
  // arrive filled, because retyping what the model just read is the work the
  // feature was meant to remove.
  testWidgets('an AI reading leads with its question and fills the fields',
      (tester) async {
    await _pump(tester,
        proposal: const SaleProposal(
          found: true,
          question: 'fechou 200 cx de luvas, ciclo ~30d?',
          grossCents: 450000,
          cycleDays: 30,
        ));

    expect(find.byKey(const Key('sale_ai_question')), findsOneWidget);
    expect(find.text('fechou 200 cx de luvas, ciclo ~30d?'), findsOneWidget);
    // In the account's own notation: the person types over this text, so it
    // has to be written the way they read it — and re-parsing an untouched
    // field must return the amount it started with.
    expect(find.text('4500,00'), findsOneWidget);
    expect(find.text('30'), findsOneWidget);
  });

  // Confirming a proposal records BOTH halves: the model found it, this
  // person said yes.
  testWidgets('confirming a proposal records it as AI-extracted',
      (tester) async {
    final actions = await _pump(tester,
        proposal: const SaleProposal(
            found: true, question: 'fechou?', grossCents: 450000));

    await tester.tap(find.byKey(const Key('sale_submit')));
    await tester.pumpAndSettle();

    expect(actions.fromAI, isTrue);
  });

  // "Não encontrei uma venda aqui" must not become a filled form.
  testWidgets('a reading that found nothing leaves the form empty',
      (tester) async {
    final actions =
        await _pump(tester, proposal: const SaleProposal(found: false));

    expect(find.byKey(const Key('sale_ai_question')), findsNothing);

    await _type(tester, t.sales_record.gross, '100,00');
    await tester.tap(find.byKey(const Key('sale_submit')));
    await tester.pumpAndSettle();

    expect(actions.fromAI, isFalse, reason: 'typed by a person, not extracted');
  });

  // The one number that makes a customer come back on their own instead of
  // being remembered.
  testWidgets('the repurchase cycle is sent', (tester) async {
    final actions = await _pump(tester);

    await _type(tester, t.sales_record.gross, '500,00');
    await _type(tester, t.sales_record.cycle, '45');
    await tester.tap(find.byKey(const Key('sale_submit')));
    await tester.pumpAndSettle();

    expect(actions.cycle, 45);
  });

  // A sale with no value is refused by the server ("informe o valor da
  // venda"), and that refusal used to cost a round trip and arrive in a
  // snackbar underneath this sheet.
  testWidgets('a sale with no value cannot be submitted', (tester) async {
    final actions = await _pump(tester);

    FilledButton submit() =>
        tester.widget<FilledButton>(find.byKey(const Key('sale_submit')));

    expect(submit().onPressed, isNull);

    await _type(tester, t.sales_record.gross, '0,00');
    expect(submit().onPressed, isNull, reason: 'zero is not a sale');

    await _type(tester, t.sales_record.gross, '10,00');
    expect(submit().onPressed, isNotNull);
    expect(actions.gross, isNull, reason: 'still nothing sent');
  });

  // Every other money field on this form says what it means. "Imposto" alone
  // left it open whether it wanted reais or a percentage.
  testWidgets('the tax field says what it wants', (tester) async {
    await _pump(tester);

    expect(find.text(t.sales_record.tax_hint), findsOneWidget);
  });

  // From a lead's card the customer may be unknown to the app (no channel
  // address yet). The lead id is what the server resolves the customer from
  // and what marks the deal won; the sheet used to send the lead's own id
  // AS the customer and get CONTACT_NOT_FOUND back.
  testWidgets('a sale from a lead sends the lead id, not the lead as customer',
      (tester) async {
    final actions = await _pump(tester, contactId: '', leadId: 'lead-1');

    await _type(tester, t.sales_record.gross, '1500');
    await tester.tap(find.byKey(const Key('sale_submit')));
    await tester.pumpAndSettle();

    expect(actions.leadId, 'lead-1');
    expect(actions.contactId, '');
    expect(actions.gross, 150000);
  });
}
