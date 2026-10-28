import 'package:app/design_system/app_theme.dart';
import 'package:app/features/sales/presentation/controllers/contract_calendar_controller.dart';
import 'package:app/features/sales/presentation/widgets/contract_calendar_body.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

Future<void> _pump(WidgetTester tester, ContractCalendar calendar) async {
  tester.view.physicalSize = const Size(900, 1600);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(ProviderScope(
    overrides: [
      contractCalendarProvider.overrideWith((ref) async => calendar),
    ],
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      home: const Scaffold(body: ContractCalendarBody()),
    ),
  ));
  await tester.pumpAndSettle();
}

void main() {
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  // The card's whole job is to answer "who, what, and when". Everything else
  // on it is supporting detail.
  testWidgets('a window names the buyer, the clock and the incumbent',
      (tester) async {
    await _pump(
      tester,
      const ContractCalendar(
        states: ['MT'],
        keywords: ['monitor'],
        entries: [
          ContractWindow(
            pncpId: 'c1',
            buyerName: 'Hospital Santa Rosa',
            object: 'Aquisição de monitores multiparamétricos',
            supplierName: 'Concorrente Ltda',
            valueCents: 42000000,
            city: 'Cuiabá',
            uf: 'MT',
            daysLeft: 45,
            url: 'https://pncp.gov.br/app/contratos/c1',
          ),
        ],
      ),
    );

    expect(find.byKey(const Key('calendar_card_c1')), findsOneWidget);
    expect(find.byKey(const Key('calendar_when_c1')), findsOneWidget);
    expect(find.text('Hospital Santa Rosa'), findsOneWidget);
    expect(find.textContaining('Concorrente Ltda'), findsOneWidget);
    // The public record, so every claim built on the row can be checked.
    expect(find.byKey(const Key('calendar_open_c1')), findsOneWidget);
    // And the verb. A calendar whose only action is "open a government
    // website" is a list somebody reads once and never returns to.
    expect(find.byKey(const Key('calendar_work_c1')), findsOneWidget);
  });

  // Every other surface in this product ends in something you can do. This one
  // has to as well, or the date never becomes a deal.
  testWidgets('working a window puts the buyer in the funnel', (tester) async {
    final actions = _FakeCalendarActions();
    tester.view.physicalSize = const Size(900, 1600);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(ProviderScope(
      overrides: [
        contractCalendarProvider
            .overrideWith((ref) async => const ContractCalendar(states: [
                  'MT'
                ], keywords: [
                  'monitor'
                ], entries: [
                  ContractWindow(
                    pncpId: 'c9',
                    buyerName: 'Hospital Santa Rosa',
                    object: 'monitores',
                    daysLeft: 30,
                  ),
                ])),
        calendarActionsProvider.overrideWithValue(actions),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const Scaffold(body: ContractCalendarBody()),
      ),
    ));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('calendar_work_c9')));
    await tester.pumpAndSettle();

    expect(actions.worked, ['c9']);
    expect(find.text(t.calendar.worked(name: 'Hospital Santa Rosa')),
        findsOneWidget);
  });

  // The most actionable bit in the whole record: a body that rides other
  // bodies' price registries can buy from you without a tender at all.
  testWidgets('adhesion and price registry are called out', (tester) async {
    await _pump(
      tester,
      const ContractCalendar(
        states: ['MT'],
        keywords: ['monitor'],
        entries: [
          ContractWindow(
            pncpId: 'c2',
            buyerName: 'Município de Sinop',
            object: 'monitores',
            daysLeft: 120,
            priceRegistry: true,
            ridesOthers: true,
          ),
        ],
      ),
    );

    expect(find.text(t.calendar.price_registry), findsOneWidget);
    expect(find.text(t.calendar.rides_others), findsOneWidget);
  });

  // Three different facts render as zero rows. Showing the same blank for all
  // three is how a working feature gets reported as broken.
  //
  // One test each, deliberately: re-pumping a new ProviderScope into the same
  // element tree reuses the container, so a second calendar in the same test
  // would assert against the first one's value.
  testWidgets('no territory says where to set it', (tester) async {
    await _pump(tester, const ContractCalendar());
    expect(
        find.byKey(const Key('calendar_empty_no_territory')), findsOneWidget);
  });

  testWidgets('no words says to add products', (tester) async {
    await _pump(tester, const ContractCalendar(states: ['MT']));
    expect(find.byKey(const Key('calendar_empty_no_keywords')), findsOneWidget);
  });

  testWidgets('day-one empty says it is still reading', (tester) async {
    await _pump(
        tester,
        const ContractCalendar(
            states: ['MT'], keywords: ['monitor'], scanning: true));
    expect(find.byKey(const Key('calendar_empty_scanning')), findsOneWidget);
  });

  testWidgets('a swept territory with nothing due says so plainly',
      (tester) async {
    await _pump(
        tester, const ContractCalendar(states: ['MT'], keywords: ['monitor']));
    expect(find.byKey(const Key('calendar_empty')), findsOneWidget);
  });
}

class _FakeCalendarActions implements CalendarActions {
  final List<String> worked = [];

  @override
  Future<void> work({required String pncpId}) async => worked.add(pncpId);
}
