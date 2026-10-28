import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/sales/presentation/controllers/relationship_controller.dart';
import 'package:app/features/sales/presentation/screens/relationship_screen.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

// Nothing on this screen sends anything, and the screen has to say so — a
// surface that quietly schedules messages to customers is one nobody should
// discover by watching their outbox.

Future<void> _pump(
  WidgetTester tester, {
  List<RelationshipDate> dates = const [],
  List<SectorDate> sectors = const [],
}) async {
  tester.view.physicalSize = const Size(900, 1600);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(ProviderScope(
    overrides: [
      dioProvider.overrideWithValue(Dio()),
      relationshipDatesProvider.overrideWith((ref) async => dates),
      sectorDatesProvider.overrideWith((ref) async => sectors),
    ],
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      home: const Scaffold(body: RelationshipBody()),
    ),
  ));
  await tester.pumpAndSettle();
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  testWidgets('the approval rule is on the screen, not in a doc',
      (tester) async {
    await _pump(tester);

    expect(find.text(t.relationship.greeting_only), findsOneWidget);
  });

  testWidgets('an empty list explains how one fills up', (tester) async {
    await _pump(tester);

    expect(find.byKey(const Key('relationship_empty')), findsOneWidget);
    expect(find.text(t.relationship.empty_hint), findsOneWidget);
  });

  // No year on a recurring date: what matters about a birthday is that it
  // comes round.
  testWidgets('a recurring date shows day and month only', (tester) async {
    await _pump(tester, dates: const [
      RelationshipDate(
          id: 'd1', kind: 'birthday', contactId: 'c1', month: 5, day: 12),
    ]);

    expect(find.text('12/05'), findsOneWidget);
  });

  testWidgets('a one-off date shows its full date', (tester) async {
    await _pump(tester, dates: [
      RelationshipDate(
          id: 'd1',
          kind: 'maintenance',
          contactId: 'c1',
          dueAt: DateTime(2026, 9, 3),
          label: 'Autoclave'),
    ]);

    expect(find.text('03/09/2026'), findsOneWidget);
    expect(find.text('Autoclave'), findsOneWidget);
  });

  // Otherwise somebody adds Dia do Médico and waits for six hundred greetings
  // that never come.
  testWidgets('an account-wide date says it never sends on its own',
      (tester) async {
    await _pump(tester, dates: const [
      RelationshipDate(
          id: 'd1', kind: 'sector', month: 10, day: 18, label: 'Dia do Médico'),
    ]);

    expect(
        find.byKey(const Key('relationship_account_wide_d1')), findsOneWidget);
  });

  testWidgets('a per-contact date carries no account-wide warning',
      (tester) async {
    await _pump(tester, dates: const [
      RelationshipDate(
          id: 'd1', kind: 'birthday', contactId: 'c1', month: 5, day: 12),
    ]);

    expect(find.byKey(const Key('relationship_account_wide_d1')), findsNothing);
  });

  // A date with no label is still a date — it falls back to what kind it is
  // rather than rendering an empty row.
  testWidgets('an unlabelled date shows its kind', (tester) async {
    await _pump(tester, dates: const [
      RelationshipDate(
          id: 'd1', kind: 'birthday', contactId: 'c1', month: 5, day: 12),
    ]);

    expect(find.text(t.relationship.kind_birthday), findsOneWidget);
  });

  // The catalogue is what stops somebody having to research that Dia do
  // Fonoaudiólogo is 9 December.
  testWidgets('the sector picker offers the shipped catalogue', (tester) async {
    await _pump(tester, sectors: const [
      SectorDate(key: 'medico', label: 'Dia do Médico', month: 10, day: 18),
    ]);

    await tester.tap(find.byKey(const Key('relationship_add')));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('relationship_kind')));
    await tester.pumpAndSettle();
    await tester.tap(find.text(t.relationship.kind_sector).last);
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('relationship_sector')), findsOneWidget);
    // And the day/month fields are gone: the catalogue fills them.
    expect(find.byKey(const Key('relationship_day')), findsNothing);
  });

  // The button used to look ready over an empty form. Pressing it spent a
  // round trip to be told "informe uma data" in a snackbar that the sheet
  // itself half-covers.
  testWidgets('the save button is dead until there is a date', (tester) async {
    await _pump(tester);

    await tester.tap(find.byKey(const Key('relationship_add')));
    await tester.pumpAndSettle();

    FilledButton save() =>
        tester.widget<FilledButton>(find.byKey(const Key('relationship_save')));

    expect(save().onPressed, isNull);

    await tester.enterText(find.byKey(const Key('relationship_day')), '12');
    await tester.pump();
    expect(save().onPressed, isNull,
        reason: 'a day with no month is not a date');

    await tester.enterText(find.byKey(const Key('relationship_month')), '5');
    await tester.pump();
    expect(save().onPressed, isNotNull);

    // An impossible date is refused here, not after a round trip.
    await tester.enterText(find.byKey(const Key('relationship_day')), '31');
    await tester.enterText(find.byKey(const Key('relationship_month')), '2');
    await tester.pump();
    expect(save().onPressed, isNull);

    // 29 February stays valid: somebody born on it has a birthday, and the
    // API agrees.
    await tester.enterText(find.byKey(const Key('relationship_day')), '29');
    await tester.pump();
    expect(save().onPressed, isNotNull);
  });

  testWidgets('a sector date needs a sector picked', (tester) async {
    await _pump(tester, sectors: const [
      SectorDate(key: 'medico', label: 'Dia do Médico', month: 10, day: 18),
    ]);

    await tester.tap(find.byKey(const Key('relationship_add')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('relationship_kind')));
    await tester.pumpAndSettle();
    await tester.tap(find.text(t.relationship.kind_sector).last);
    await tester.pumpAndSettle();

    expect(
        tester
            .widget<FilledButton>(find.byKey(const Key('relationship_save')))
            .onPressed,
        isNull);

    await tester.tap(find.byKey(const Key('relationship_sector')));
    await tester.pumpAndSettle();
    await tester.tap(find.textContaining('Dia do Médico').last);
    await tester.pumpAndSettle();

    expect(
        tester
            .widget<FilledButton>(find.byKey(const Key('relationship_save')))
            .onPressed,
        isNotNull);
  });
}
