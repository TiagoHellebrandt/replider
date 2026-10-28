import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/sales/domain/models/product_model.dart';
import 'package:app/features/sales/domain/models/proposal.dart';
import 'package:app/features/sales/presentation/controllers/products_controller.dart';
import 'package:app/features/sales/presentation/controllers/proposal_controller.dart';
import 'package:app/features/sales/presentation/screens/proposal_screen.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

// Composing a priced offer. These pin the properties that decide whether the
// number a customer reads is defensible, and the one promise the whole product
// rests on: nothing leaves without a human.

const _gauze = ProductModel(
  id: 'p1',
  accountId: 'a1',
  name: 'Gaze estéril',
  priceCents: 4990,
  currency: 'BRL',
);
const _glove = ProductModel(
  id: 'p2',
  accountId: 'a1',
  name: 'Luva cirúrgica',
  priceCents: 12550,
  currency: 'BRL',
);
const _swabUSD = ProductModel(
  id: 'p3',
  accountId: 'a1',
  name: 'Swab',
  priceCents: 500,
  currency: 'USD',
);
const _retired = ProductModel(
  id: 'p4',
  accountId: 'a1',
  name: 'Produto descontinuado',
  priceCents: 1000,
  currency: 'BRL',
  active: false,
);

class _Products extends ProductsList {
  _Products(this._seed);
  final List<ProductModel> _seed;
  @override
  Future<List<ProductModel>> build() async => _seed;
}

class _FakeActions implements ProposalActions {
  int creates = 0;
  int approvals = 0;
  List<ProposalLine>? lines;
  String? body;
  Object? failWith;

  @override
  Future<Proposal> create({
    required String leadId,
    required List<ProposalLine> lines,
    String body = '',
  }) async {
    if (failWith != null) throw failWith!;
    creates++;
    this.lines = lines;
    this.body = body;
    return _draft;
  }

  @override
  Future<Proposal> sendForApproval({required String proposalId}) async {
    approvals++;
    return _draft;
  }

  int downloads = 0;
  bool downloadSucceeds = true;

  @override
  Future<bool> downloadPdf({required String proposalId}) async {
    if (failWith != null) throw failWith!;
    downloads++;
    return downloadSucceeds;
  }
}

const _draft = Proposal(
  id: 'prop-1',
  leadId: 'lead-1',
  status: 'draft',
  currency: 'BRL',
  body: 'Segue a proposta.',
  items: [],
  totalCents: 40070,
  rendered: 'Segue a proposta.\n\n• 3 × Gaze estéril — 149,70 BRL',
);

Future<_FakeActions> _pump(
  WidgetTester tester, {
  List<ProductModel> catalogue = const [_gauze, _glove],
  List<Proposal> history = const [],
}) async {
  tester.view.physicalSize = const Size(1000, 2200);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  final actions = _FakeActions();
  await tester.pumpWidget(ProviderScope(
    overrides: [
      productsListProvider.overrideWith(() => _Products(catalogue)),
      proposalActionsProvider.overrideWithValue(actions),
      leadProposalsProvider('lead-1').overrideWith((ref) async => history),
      dioProvider.overrideWithValue(Dio()),
    ],
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      home: const ProposalScreen(leadId: 'lead-1'),
    ),
  ));
  await tester.pumpAndSettle();
  return actions;
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  testWidgets('the total carries its currency and follows the quantities',
      (tester) async {
    await _pump(tester);

    // Nothing chosen: no total to state.
    expect(find.byKey(const Key('proposal_total')), findsNothing);

    await tester.tap(find.byKey(const Key('proposal_plus_p1')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('proposal_plus_p1')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('proposal_plus_p1')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('proposal_plus_p2')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('proposal_plus_p2')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('proposal_qty_p1')), findsOneWidget);
    expect(
      tester.widget<Text>(find.byKey(const Key('proposal_qty_p1'))).data,
      '3',
    );
    // 3 × 49,90 + 2 × 125,50 = 400,70 — and never a bare number.
    expect(
      tester.widget<Text>(find.byKey(const Key('proposal_total'))).data,
      'R\$ 400,70',
    );
  });

  testWidgets('removing the last unit removes the line', (tester) async {
    await _pump(tester);

    await tester.tap(find.byKey(const Key('proposal_plus_p1')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('proposal_minus_p1')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('proposal_total')), findsNothing);
    // With nothing chosen there is nothing to propose.
    final button =
        tester.widget<FilledButton>(find.byKey(const Key('proposal_send')));
    expect(button.onPressed, isNull);
  });

  // Summing BRL and USD invents a number; picking one misstates the rest. The
  // server refuses too — this stops the work before it is wasted.
  testWidgets('mixed currencies refuse instead of showing a total',
      (tester) async {
    final actions = await _pump(tester, catalogue: const [_gauze, _swabUSD]);

    await tester.tap(find.byKey(const Key('proposal_plus_p1')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('proposal_plus_p3')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('proposal_mixed_currency')), findsOneWidget);
    expect(find.byKey(const Key('proposal_total')), findsNothing);

    final button =
        tester.widget<FilledButton>(find.byKey(const Key('proposal_send')));
    expect(button.onPressed, isNull);
    expect(actions.creates, 0);
  });

  // Offering something the company stopped selling is a promise it cannot keep.
  testWidgets('a retired product is not offerable at all', (tester) async {
    await _pump(tester, catalogue: const [_gauze, _retired]);

    expect(find.byKey(const Key('proposal_product_p1')), findsOneWidget);
    expect(find.byKey(const Key('proposal_product_p4')), findsNothing);
    expect(find.text('Produto descontinuado'), findsNothing);
  });

  // The one promise everything rests on: the button goes to the queue, and the
  // line under it says what does not happen.
  testWidgets('sending goes to the approval queue and says so', (tester) async {
    final actions = await _pump(tester);

    expect(find.text(t.proposal.approval_note), findsOneWidget);

    await tester.tap(find.byKey(const Key('proposal_plus_p1')));
    await tester.pumpAndSettle();
    await tester.enterText(
        find.byKey(const Key('proposal_body')), 'Segue a proposta.');
    await tester.tap(find.byKey(const Key('proposal_send')));
    await tester.pumpAndSettle();

    expect(actions.creates, 1);
    expect(actions.approvals, 1);
    expect(actions.lines, hasLength(1));
    expect(actions.lines!.first.productId, 'p1');
    expect(actions.lines!.first.quantity, 1);
    expect(actions.body, 'Segue a proposta.');

    // And what the reviewer will read is shown, rendered by the server so the
    // two can never disagree.
    expect(find.byKey(const Key('proposal_queued')), findsOneWidget);
    expect(
        find.textContaining('3 × Gaze estéril — 149,70 BRL'), findsOneWidget);
  });

  // The file the seller forwards, on the same card as the queue.
  //
  // Being second with the price is the commonest way to lose a deal in this
  // market. Until this button existed the screen could compose the lines and
  // the total and then hand over nothing — the work finished and the seller
  // went back to Word.
  testWidgets('the queued offer can be downloaded as a PDF', (tester) async {
    final actions = await _pump(tester);

    await tester.tap(find.byKey(const Key('proposal_plus_p1')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('proposal_send')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('proposal_download_pdf')), findsOneWidget);
    await tester.tap(find.byKey(const Key('proposal_download_pdf')));
    await tester.pumpAndSettle();

    expect(actions.downloads, 1);
    // Downloading is NOT sending: the approval queue is still the only way an
    // offer reaches a customer.
    expect(actions.approvals, 1);
  });

  // A platform that refused to write the file must say so. Silence reads as
  // "saved", and the seller finds out in front of the customer.
  testWidgets('a refused save is reported, not swallowed', (tester) async {
    final actions = await _pump(tester);
    actions.downloadSucceeds = false;

    await tester.tap(find.byKey(const Key('proposal_plus_p1')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('proposal_send')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('proposal_download_pdf')));
    await tester.pumpAndSettle();

    expect(find.text(t.proposal.download_failed), findsOneWidget);
  });

  testWidgets('a failed send reports the reason and queues nothing',
      (tester) async {
    final actions = await _pump(tester);
    actions.failWith = Exception('boom');

    await tester.tap(find.byKey(const Key('proposal_plus_p1')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('proposal_send')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('proposal_error')), findsOneWidget);
    expect(find.byKey(const Key('proposal_queued')), findsNothing);
    expect(actions.approvals, 0);
  });

  // A picker over an empty catalogue is a dead end.
  testWidgets('with no products it says what is missing and where it lives',
      (tester) async {
    await _pump(tester, catalogue: const []);

    expect(find.byKey(const Key('proposal_no_products')), findsOneWidget);
    expect(find.byKey(const Key('proposal_go_products')), findsOneWidget);
    expect(find.byKey(const Key('proposal_send')), findsNothing);
  });

  // The reason the object has a history: the last offer shapes the next one.
  testWidgets('previous offers to this customer are shown', (tester) async {
    await _pump(tester, history: const [
      Proposal(
        id: 'old-1',
        leadId: 'lead-1',
        status: 'sent',
        currency: 'BRL',
        body: '',
        items: [],
        totalCents: 25000,
        rendered: '',
        sentAt: null,
      ),
    ]);

    expect(find.byKey(const Key('proposal_history')), findsOneWidget);
    expect(find.text('R\$ 250,00'), findsOneWidget);
  });

  testWidgets('offers a way back', (tester) async {
    await _pump(tester);
    expect(find.byKey(const Key('proposal_back')), findsOneWidget);
  });
}
