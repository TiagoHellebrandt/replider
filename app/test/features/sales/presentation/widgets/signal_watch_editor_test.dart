import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/features/sales/presentation/widgets/signal_watch_editor.dart';
import 'package:app/i18n/strings.g.dart';

void main() {
  setUpAll(() {
    FlutterSecureStorage.setMockInitialValues({});
  });
  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
  });

  group('parseWatchStates', () {
    test('accepts the 27, in the order typed, without duplicates', () {
      expect(parseWatchStates(' mt , sp'), ['MT', 'SP']);
      expect(parseWatchStates('SP, sp , Sp'), ['SP']);
    });

    // "XX" passes any two-letter shape test and then matches no tender ever
    // — a watch that looks saved and is quietly empty forever.
    test('drops anything that is not a federative unit', () {
      expect(parseWatchStates('XX, Mato Grosso, 12'), isEmpty);
      expect(parseWatchStates('XX, mt'), ['MT']);
    });

    test('an empty field is not a state', () {
      expect(parseWatchStates(''), isEmpty);
      expect(parseWatchStates('  ,  '), isEmpty);
    });
  });

  Future<void> openEditor(WidgetTester tester) async {
    await tester.pumpWidget(
      ProviderScope(
        child: MaterialApp(
          home: Consumer(
            builder: (context, ref, _) => Scaffold(
              body: Builder(
                builder: (inner) => TextButton(
                  onPressed: () => editSignalWatch(inner, ref, null),
                  child: const Text('open'),
                ),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
  }

  // The whole point of the field: a watch with no UF used to mean the whole
  // country, and the poll's scope is the union of every watch — so one
  // unscoped row dragged everybody else's search across Brazil.
  testWidgets('keywords alone do not enable save', (tester) async {
    await openEditor(tester);

    await tester.enterText(
        find.byKey(const Key('signals_watch_input')), 'oxigênio');
    await tester.pump();

    final save = tester
        .widget<FilledButton>(find.byKey(const Key('signals_watch_save')));
    expect(save.onPressed, isNull,
        reason: 'a watch with no state cannot be saved');
  });

  testWidgets('naming a state enables save', (tester) async {
    await openEditor(tester);

    await tester.enterText(
        find.byKey(const Key('signals_watch_input')), 'oxigênio');
    await tester.enterText(
        find.byKey(const Key('signals_watch_states_input')), 'MT');
    await tester.pump();

    final save = tester
        .widget<FilledButton>(find.byKey(const Key('signals_watch_save')));
    expect(save.onPressed, isNotNull);
  });

  testWidgets('a state that is not a state does not enable save',
      (tester) async {
    await openEditor(tester);

    await tester.enterText(
        find.byKey(const Key('signals_watch_input')), 'oxigênio');
    await tester.enterText(
        find.byKey(const Key('signals_watch_states_input')), 'XX');
    await tester.pump();

    final save = tester
        .widget<FilledButton>(find.byKey(const Key('signals_watch_save')));
    expect(save.onPressed, isNull);
  });

  // The dialog says why the field is there. Somebody who does not know that
  // an empty field means "the whole country" cannot make a good choice.
  testWidgets('the dialog explains what the state is for', (tester) async {
    await openEditor(tester);

    expect(find.text(t.signals.watch_states_body), findsOneWidget);
    expect(find.text(t.signals.watch_states_label), findsOneWidget);
  });
}
