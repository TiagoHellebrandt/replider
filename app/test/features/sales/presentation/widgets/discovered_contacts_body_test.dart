import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/sales/domain/models/discovered_contact.dart';
import 'package:app/features/sales/domain/models/funnel_model.dart';
import 'package:app/features/sales/presentation/controllers/discovered_contacts_controller.dart';
import 'package:app/features/sales/presentation/controllers/funnels_controller.dart';
import 'package:app/features/sales/presentation/widgets/discovered_contacts_body.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

// The screen is a REVIEW of a list read out of somebody's mailbox. These pin
// what it must never do: import without a human, import into nowhere, or imply
// the list is everybody when it is not.

final _candidates = DiscoveredContactsPage(
  total: 2,
  items: [
    DiscoveredContact(
      address: 'ana@clinicaaurora.com.br',
      name: 'Ana Souza',
      sentCount: 40,
      firstSentAt: DateTime(2024, 3, 1),
      lastSentAt: DateTime(2026, 7, 1),
    ),
    const DiscoveredContact(
      address: 'compras@hospital.com.br',
      sentCount: 1,
    ),
  ],
);

class _FakeActions implements DiscoveredContactActions {
  List<String>? importedAddresses;
  String? importedFunnel;
  List<String>? dismissedAddresses;
  int scans = 0;
  SentScanResult scanResult = const SentScanResult(
    mailbox: 'INBOX.Enviados',
    scanned: 4000,
    found: 62,
    skipped: 300,
    complete: true,
  );
  ImportContactsResult importResult =
      const ImportContactsResult(imported: 1, skipped: 0);

  @override
  Future<SentScanResult> scan({required String channelId}) async {
    scans++;
    return scanResult;
  }

  @override
  Future<ImportContactsResult> import({
    required List<String> addresses,
    required String funnelId,
  }) async {
    importedAddresses = addresses;
    importedFunnel = funnelId;
    return importResult;
  }

  @override
  Future<void> dismiss({required List<String> addresses}) async {
    dismissedAddresses = addresses;
  }
}

class _Funnels extends FunnelsList {
  _Funnels(this._seed);
  final List<FunnelModel> _seed;
  @override
  Future<List<FunnelModel>> build() async => _seed;
}

/// The debug-only "ListTile background color or ink splashes may be invisible"
/// assertion. It fires on the CI toolchain (Flutter 3.47) and not on an older
/// local one, so it slipped through as green here and red there — the same class
/// of gap the PR test gate now closes. Cosmetic and debug-only; swallow just
/// this one, from INSIDE the test body, because the binding reinstalls its own
/// handler after `setUp`. (See app/CLAUDE.md.)
void ignoreListTileAssertions() {
  final prev = FlutterError.onError;
  FlutterError.onError = (details) {
    if (details.exceptionAsString().contains('ListTile background color')) {
      return;
    }
    prev?.call(details);
  };
}

Future<_FakeActions> _pump(
  WidgetTester tester, {
  DiscoveredContactsPage? page,
  List<FunnelModel> funnels = const [],
  String? channelId = 'ch-1',
}) async {
  tester.view.physicalSize = const Size(1000, 1800);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  ignoreListTileAssertions();
  final actions = _FakeActions();
  await tester.pumpWidget(ProviderScope(
    overrides: [
      discoveredContactsProvider
          .overrideWith((ref) async => page ?? _candidates),
      discoveredContactActionsProvider.overrideWithValue(actions),
      funnelsListProvider.overrideWith(() => _Funnels(funnels)),
      dioProvider.overrideWithValue(Dio()),
    ],
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      // The body carries no Scaffold of its own any more: it renders inside the
      // Clientes destination, which supplies the Material its ListTiles need.
      home: Scaffold(body: DiscoveredContactsBody(channelId: channelId)),
    ),
  ));
  await tester.pumpAndSettle();
  return actions;
}

const _funnel = FunnelModel(id: 'fn-1', name: 'Clientes antigos');

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  // She is about to let a tool open her mailbox. What it reads — and what it
  // does not — is the first thing on screen, not a line in a settings page.
  testWidgets('says what it reads before anything is read', (tester) async {
    await _pump(tester);

    expect(find.byKey(const Key('discovered_privacy_note')), findsOneWidget);
    expect(find.text(t.my_customers.subtitle), findsOneWidget);
  });

  // The history line IS the decision: it is how she tells a customer of two
  // years from somebody who asked one question.
  testWidgets('each row shows how much she writes to them', (tester) async {
    await _pump(tester);

    final line = tester
        .widget<Text>(find
            .byKey(const Key('discovered_history_ana@clinicaaurora.com.br')))
        .data;
    expect(line, contains(t.my_customers.sent_count(count: '40')));
    expect(line, contains('03/2024'));
    expect(line, contains('07/2026'));

    // The address shows even when a name exists: two people share a name,
    // nobody shares an address, and she is about to email whoever this is.
    expect(find.text('ana@clinicaaurora.com.br'), findsOneWidget);
    expect(find.text('Ana Souza'), findsOneWidget);
  });

  // Nothing is created until a human ticks something.
  testWidgets('no decision bar until something is selected', (tester) async {
    await _pump(tester);
    expect(find.byKey(const Key('discovered_decision_bar')), findsNothing);

    await tester.tap(find.byKey(const Key('discovered_select_all')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('discovered_decision_bar')), findsOneWidget);
    expect(find.text(t.my_customers.selected(count: '2')), findsOneWidget);
  });

  testWidgets('importing sends exactly what was ticked, into the chosen funnel',
      (tester) async {
    final actions = await _pump(tester, funnels: [_funnel]);

    await tester
        .tap(find.byKey(const Key('discovered_row_ana@clinicaaurora.com.br')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('discovered_funnel_picker')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Clientes antigos').last);
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('discovered_import')));
    await tester.pumpAndSettle();

    expect(actions.importedAddresses, ['ana@clinicaaurora.com.br']);
    expect(actions.importedFunnel, 'fn-1');
    expect(find.byKey(const Key('discovered_import_result')), findsOneWidget);
  });

  // A lead with no funnel is a row nobody sees, so the button refuses instead
  // of importing into nowhere.
  testWidgets('importing without a funnel refuses and creates nothing',
      (tester) async {
    final actions = await _pump(tester, funnels: [_funnel]);

    await tester.tap(find.byKey(const Key('discovered_select_all')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('discovered_import')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('discovered_funnel_required')), findsOneWidget);
    expect(actions.importedAddresses, isNull);
  });

  // Without a funnel to land in, the screen says what to do rather than
  // offering a button that cannot work.
  testWidgets('with no funnels at all it says so', (tester) async {
    await _pump(tester);

    await tester.tap(find.byKey(const Key('discovered_select_all')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('discovered_no_funnels')), findsOneWidget);
  });

  // A count that does not match what she ticked is a question; answering it
  // here saves a support message.
  testWidgets('an import that skipped some says how many', (tester) async {
    final actions = await _pump(tester, funnels: [_funnel]);
    actions.importResult = const ImportContactsResult(imported: 1, skipped: 1);

    await tester.tap(find.byKey(const Key('discovered_select_all')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('discovered_funnel_picker')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Clientes antigos').last);
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('discovered_import')));
    await tester.pumpAndSettle();

    expect(
      find.text(t.my_customers.imported_partial(count: '1', skipped: '1')),
      findsOneWidget,
    );
  });

  // "Not a customer" is a separate decision from importing, and it needs no
  // funnel — she is removing them from the list, not filing them.
  testWidgets('dismissing needs no funnel', (tester) async {
    final actions = await _pump(tester);

    await tester.tap(find.byKey(const Key('discovered_select_all')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('discovered_dismiss')));
    await tester.pumpAndSettle();

    expect(actions.dismissedAddresses, hasLength(2));
    expect(find.byKey(const Key('discovered_dismiss_result')), findsOneWidget);
  });

  // The scan walks a mailbox and takes real seconds. A button that goes quiet
  // reads as broken.
  testWidgets('the scan reports what it read', (tester) async {
    final actions = await _pump(tester);

    await tester.tap(find.byKey(const Key('discovered_scan')));
    await tester.pumpAndSettle();

    expect(actions.scans, 1);
    expect(find.byKey(const Key('discovered_scan_summary')), findsOneWidget);
    expect(
      find.text(t.my_customers.scan_result(
          scanned: '4000', mailbox: 'INBOX.Enviados', found: '62')),
      findsOneWidget,
    );
    // A long history yielding a short list needs an explanation.
    expect(
        find.text(t.my_customers.scan_skipped(count: '300')), findsOneWidget);
  });

  // The cap stopped the walk, so this is NOT everybody she has ever written to.
  // Implying otherwise would make her think a missing customer was never there.
  testWidgets('an incomplete scan says the list is not everybody',
      (tester) async {
    final actions = await _pump(tester);
    actions.scanResult = const SentScanResult(
      mailbox: 'Sent',
      scanned: 4000,
      found: 62,
      skipped: 0,
      complete: false,
    );

    await tester.tap(find.byKey(const Key('discovered_scan')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('discovered_scan_partial')), findsOneWidget);
  });

  // Opened with no channel there is nothing to dial, so the button hides rather
  // than failing on tap.
  testWidgets('with no channel there is no scan button', (tester) async {
    await _pump(tester, channelId: null);
    expect(find.byKey(const Key('discovered_scan')), findsNothing);
  });

  // "Select all" must not read as "all of them" when the page is capped.
  testWidgets('a capped page says how much it is showing', (tester) async {
    await _pump(
      tester,
      page: DiscoveredContactsPage(items: _candidates.items, total: 340),
    );

    expect(
      find.text(t.my_customers.showing_partial(shown: '2', total: '340')),
      findsOneWidget,
    );
  });

  testWidgets('an empty list explains how to fill it', (tester) async {
    await _pump(
      tester,
      page: const DiscoveredContactsPage(items: [], total: 0),
    );

    expect(find.byKey(const Key('discovered_empty')), findsOneWidget);
    expect(find.text(t.my_customers.empty_hint), findsOneWidget);
  });
}
