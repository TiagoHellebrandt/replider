import 'package:app/core/auth/session_ending_banner.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

// Ser posto para fora uma vez por dia é decisão do servidor (token de 24h, sem
// refresh). O que a tela pode consertar é a surpresa.

Future<void> _pump(WidgetTester tester, Duration remaining) async {
  await tester.pumpWidget(ProviderScope(
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      home: Scaffold(body: SessionEndingBanner(remaining: remaining)),
    ),
  ));
  await tester.pump();
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  testWidgets('diz quanto falta, e o que fazer com isso', (tester) async {
    await _pump(tester, const Duration(minutes: 12));

    expect(
        find.text(t.auth.session_ending_title(minutes: '12')), findsOneWidget);
    expect(find.text(t.auth.session_ending_body), findsOneWidget);
    expect(find.byKey(const Key('session_ending_cta')), findsOneWidget);
  });

  // Arredondar para baixo diria "0 min" a quem ainda tem quarenta segundos —
  // a notícia errada no último momento em que ela ainda serve para algo.
  testWidgets('o último minuto ainda é um minuto', (tester) async {
    await _pump(tester, const Duration(seconds: 40));

    expect(
        find.text(t.auth.session_ending_title(minutes: '1')), findsOneWidget);
  });
}
