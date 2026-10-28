import 'dart:typed_data';

import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/dashboard/domain/models/email_risk.dart';
import 'package:app/features/dashboard/presentation/widgets/email_risk_banner.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

// The banner exists for one scenario above all: a supplier you have paid
// before asking for money at a new account. That is the last moment anyone
// can stop the transfer, so it must not look like every other notice.

EmailRisk _risk(List<(String, String)> signals, {int score = 60}) => EmailRisk(
      score: score,
      signals: [
        for (final (kind, detail) in signals)
          EmailRiskSignal(kind: kind, detail: detail),
      ],
    );

class _ThrowingAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(RequestOptions options, Stream<Uint8List>? _,
          Future<void>? __) async =>
      throw DioException(requestOptions: options, message: 'offline no teste');

  @override
  void close({bool force = false}) {}
}

Future<void> _pump(WidgetTester tester, EmailRisk risk) async {
  tester.view.physicalSize = const Size(900, 1400);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  final dio = Dio(BaseOptions(baseUrl: 'https://api.test/v1'))
    ..httpClientAdapter = _ThrowingAdapter();

  await tester.pumpWidget(ProviderScope(
    overrides: [dioProvider.overrideWithValue(dio)],
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      home: Scaffold(
        body: EmailRiskBanner(
          risk: risk,
          messageId: 'msg-1',
          conversationId: 'conv-1',
        ),
      ),
    ),
  ));
  await tester.pumpAndSettle();
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  // The loudest state, and the reason the feature exists.
  testWidgets('a changed payment instruction says to call, not just to look',
      (tester) async {
    await _pump(
        tester,
        _risk([
          (
            'changed_payment_details',
            'dados de pagamento diferentes dos que este contato sempre usou: Ag 9999 C/C 111222-3'
          ),
        ]));

    expect(
      tester.widget<Text>(find.byKey(const Key('email_risk_title_msg-1'))).data,
      t.email_risk.payment_title,
    );
    // The instruction, not just the warning: confirm by phone on a number the
    // email did not give you.
    expect(find.byKey(const Key('email_risk_action_msg-1')), findsOneWidget);
    // And the evidence is the account itself, which is what gets checked
    // against the invoice on the desk.
    expect(find.textContaining('Ag 9999 C/C 111222-3'), findsOneWidget);
  });

  testWidgets('a quarantined message without a payment change is quieter',
      (tester) async {
    await _pump(
        tester,
        _risk([
          ('auth_failed', 'o domínio medl1ne.com.br não autorizou este envio'),
          ('lookalike_domain', 'parecido com medline.com.br, que você já usa'),
        ]));

    expect(
      tester.widget<Text>(find.byKey(const Key('email_risk_title_msg-1'))).data,
      t.email_risk.quarantine_title,
    );
    // No phone instruction: nothing here is about money moving.
    expect(find.byKey(const Key('email_risk_action_msg-1')), findsNothing);
  });

  testWidgets('a low-score finding is an observation, not an alarm',
      (tester) async {
    await _pump(
        tester,
        _risk([('first_contact', 'primeiro contato deste endereço')],
            score: 5));

    expect(
      tester.widget<Text>(find.byKey(const Key('email_risk_title_msg-1'))).data,
      t.email_risk.attention_title,
    );
  });

  // Every signal is shown with its evidence: a score alone is not something
  // a salesperson can confirm.
  testWidgets('every finding is listed in words', (tester) async {
    await _pump(
        tester,
        _risk([
          ('auth_failed', 'o domínio X não autorizou este envio'),
          ('new_domain', 'domínio registrado há 3 dias'),
          ('reply_to_mismatch', 'sua resposta iria para outro.com'),
        ]));

    expect(find.textContaining('não autorizou'), findsOneWidget);
    expect(find.textContaining('3 dias'), findsOneWidget);
    expect(find.textContaining('outro.com'), findsOneWidget);
  });

  // A false positive that cannot be answered teaches the reader to scroll
  // past the banner, and then the real one is invisible too.
  testWidgets('the warning can always be answered', (tester) async {
    await _pump(tester, _risk([('auth_failed', 'algo')]));

    expect(find.text(t.email_risk.legit), findsOneWidget);
  });

  // Vouching is a security decision, so it is confirmed and the confirmation
  // says what it actually means.
  testWidgets('vouching asks first, and says it silences the sender',
      (tester) async {
    await _pump(tester, _risk([('auth_failed', 'algo')]));

    await tester.tap(find.byKey(const Key('email_risk_legit_msg-1')));
    await tester.pumpAndSettle();

    expect(find.text(t.email_risk.legit_confirm), findsOneWidget);
  });

  // The other half of the loop: one report is one company's judgement, two
  // independent ones block the sender for everybody.
  testWidgets('the reader can report a scam, not only dismiss a warning',
      (tester) async {
    await _pump(tester, _risk([('auth_failed', 'algo')]));

    expect(find.text(t.email_risk.scam), findsOneWidget);
  });

  // The reason is the highest-value part of a report, and still optional:
  // demanding it would cost reports from people in a hurry, and a report
  // with no explanation still protects the network.
  testWidgets('reporting asks why, without requiring an answer',
      (tester) async {
    await _pump(tester, _risk([('auth_failed', 'algo')]));

    await tester.tap(find.byKey(const Key('email_risk_scam_msg-1')));
    await tester.pumpAndSettle();

    expect(find.text(t.email_risk.scam_title), findsOneWidget);
    expect(find.byType(TextField), findsOneWidget);
  });

  test('a risk with no signals is no risk at all', () {
    expect(EmailRisk.fromJson(null), isNull);
    expect(EmailRisk.fromJson(<String, dynamic>{'score': 60}), isNull);
    expect(
        EmailRisk.fromJson(<String, dynamic>{
          'score': 60,
          'signals': <dynamic>[
            <String, dynamic>{'kind': 'auth_failed', 'detail': '   '},
          ],
        }),
        isNull,
        reason: 'a signal with no evidence is nothing to show');
  });

  test('the quarantine threshold matches the server', () {
    expect(_risk([('x', 'y')], score: 49).isQuarantined, isFalse);
    expect(_risk([('x', 'y')], score: 50).isQuarantined, isTrue);
  });
}
