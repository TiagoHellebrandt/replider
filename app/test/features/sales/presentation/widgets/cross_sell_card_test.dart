import 'dart:async';

import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/sales/presentation/controllers/cross_sell_controller.dart';
import 'package:app/features/sales/presentation/widgets/cross_sell_card.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

// Every suggestion carries its reason. A salesperson who can say "os clientes
// que compram luva com a gente costumam levar máscara" is making an argument;
// one reading an unexplained product name is guessing out loud.

Future<void> _pump(WidgetTester tester, CrossSellResult? result) async {
  await tester.pumpWidget(ProviderScope(
    overrides: [
      dioProvider.overrideWithValue(Dio()),
      if (result != null)
        crossSellProvider('c1').overrideWith((ref) async => result),
    ],
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      home: const Scaffold(body: CrossSellCard(contactId: 'c1')),
    ),
  ));
  await tester.pumpAndSettle();
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  testWidgets('a suggestion always says why', (tester) async {
    await _pump(
      tester,
      const CrossSellResult(suggestions: [
        CrossSellSuggestion(
            suggest: 'mascara cirurgica',
            because: 'luva de procedimento',
            confidence: 0.8,
            customers: 8),
      ]),
    );

    expect(
        find.byKey(const Key('cross_sell_mascara cirurgica')), findsOneWidget);
    // In customers, not a percentage: "8 clientes que compram luva também
    // levam" is a sentence somebody can repeat on a call.
    expect(
      find.text(t.cross_sell.evidence(count: 8, item: 'luva de procedimento')),
      findsOneWidget,
    );
  });

  // A distributor asked to trust a suggestion deserves to know it was not
  // learned from a competitor's sales.
  testWidgets('says where the pattern came from', (tester) async {
    await _pump(
      tester,
      const CrossSellResult(suggestions: [
        CrossSellSuggestion(suggest: 'mascara', because: 'luva', customers: 5),
      ]),
    );

    expect(find.text(t.cross_sell.learned_here), findsOneWidget);
  });

  // An empty panel with no explanation reads as broken.
  testWidgets('an empty answer explains itself', (tester) async {
    await _pump(tester, const CrossSellResult());

    expect(find.byKey(const Key('cross_sell_empty')), findsOneWidget);
    expect(find.text(t.cross_sell.empty), findsOneWidget);
  });

  // This is a helpful extra next to the purchase history, not the reason the
  // screen exists — a spinner here would make a working screen look broken.
  testWidgets('renders nothing while it is still loading', (tester) async {
    await tester.pumpWidget(ProviderScope(
      overrides: [
        dioProvider.overrideWithValue(Dio()),
        // A future that never completes, rather than a delayed one: a
        // pending Timer fails the test at teardown.
        crossSellProvider('c1')
            .overrideWith((ref) => Completer<CrossSellResult>().future),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const Scaffold(body: CrossSellCard(contactId: 'c1')),
      ),
    ));
    await tester.pump();

    expect(find.text(t.cross_sell.title), findsNothing);
    expect(find.byType(CircularProgressIndicator), findsNothing);
  });
}
