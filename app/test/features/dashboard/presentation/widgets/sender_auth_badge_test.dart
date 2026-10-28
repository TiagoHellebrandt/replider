import 'package:app/design_system/app_theme.dart';
import 'package:app/features/dashboard/domain/models/sender_authenticity.dart';
import 'package:app/features/dashboard/presentation/widgets/sender_auth_badge.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:flutter/material.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:app/features/dashboard/domain/models/conversation_model.dart';

// Before this, anyone could put a clinic's address in the From line and land
// in the funnel as that lead's reply. Three states, never two: "ninguém
// checou" is a different statement from both "verificado" and "forjado".

Map<String, dynamic> _meta({
  String spf = 'unknown',
  String dkim = 'unknown',
  String dmarc = 'unknown',
  bool trustworthy = false,
  bool suspicious = false,
}) =>
    <String, dynamic>{
      'email_auth': <String, dynamic>{
        'spf': spf,
        'dkim': dkim,
        'dmarc': dmarc,
        'trustworthy': trustworthy,
        'suspicious': suspicious,
      },
    };

Future<void> _pump(WidgetTester tester, Map<String, dynamic>? metadata) async {
  final badge = SenderAuthBadge.forMessage(metadata,
      isInbound: true, source: ConversationSource.email);
  await tester.pumpWidget(MaterialApp(
    theme: AppTheme.darkTheme,
    home: Scaffold(body: badge ?? const SizedBox.shrink()),
  ));
  await tester.pumpAndSettle();
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  group('reading the verdict', () {
    test('a passing check is verified', () {
      expect(
          SenderAuthenticity.fromMetadata(
              _meta(spf: 'pass', trustworthy: true)),
          SenderAuthenticity.verified);
    });

    test('a failing check is failed', () {
      expect(
          SenderAuthenticity.fromMetadata(_meta(spf: 'fail', suspicious: true)),
          SenderAuthenticity.failed);
    });

    // The one that matters. Absent metadata must never render as a checkmark.
    test('absent or unreadable metadata is unknown, never verified', () {
      for (final m in <Map<String, dynamic>?>[
        null,
        <String, dynamic>{},
        <String, dynamic>{'email_auth': 'nao e um mapa'},
        _meta(),
      ]) {
        expect(SenderAuthenticity.fromMetadata(m), SenderAuthenticity.unknown);
      }
    });

    // A failure outranks a pass: something was asserted and disproved.
    test('suspicious wins over trustworthy', () {
      expect(
          SenderAuthenticity.fromMetadata(
              _meta(trustworthy: true, suspicious: true)),
          SenderAuthenticity.failed);
    });
  });

  // Marking the normal case makes the badge wallpaper, and then the one that
  // matters reads as more of the same.
  testWidgets('a verified sender gets no badge', (tester) async {
    await _pump(tester, _meta(spf: 'pass', trustworthy: true));

    expect(find.byType(InkWell), findsNothing);
  });

  testWidgets('an unchecked sender says so, without crying wolf',
      (tester) async {
    await _pump(tester, _meta());

    expect(find.text(t.sender_auth.badge_unknown), findsOneWidget);
    expect(find.text(t.sender_auth.badge_failed), findsNothing);
  });

  testWidgets('a forged sender is flagged', (tester) async {
    await _pump(tester, _meta(spf: 'fail', dmarc: 'fail', suspicious: true));

    expect(find.text(t.sender_auth.badge_failed), findsOneWidget);
  });

  // The explanation says what to DO, then shows the raw checks — that is
  // what gets forwarded to whoever runs the mail server.
  testWidgets('the failed explanation tells the reader to confirm by phone',
      (tester) async {
    await _pump(tester,
        _meta(spf: 'fail', dkim: 'none', dmarc: 'fail', suspicious: true));

    await tester.tap(find.byKey(const Key('sender_auth_failed')));
    await tester.pumpAndSettle();

    expect(find.text(t.sender_auth.title_failed), findsOneWidget);
    expect(find.textContaining('SPF: fail'), findsOneWidget);
    expect(find.textContaining('DKIM: none'), findsOneWidget);
  });

  testWidgets('the unknown explanation does not accuse anyone', (tester) async {
    await _pump(tester, _meta());

    await tester.tap(find.byKey(const Key('sender_auth_unknown')));
    await tester.pumpAndSettle();

    expect(find.text(t.sender_auth.title_unknown), findsOneWidget);
    expect(find.text(t.sender_auth.title_failed), findsNothing);
  });

  // "Verificamos que você mesmo mandou" is noise.
  testWidgets('outbound messages never carry a badge', (tester) async {
    expect(
        SenderAuthBadge.forMessage(_meta(),
            isInbound: false, source: ConversationSource.email),
        isNull);
  });

  // The badge is an EMAIL fact. On WhatsApp, Telegram, Instagram or SMS there
  // is no SPF/DKIM/DMARC verdict to have, so `fromMetadata` returned `unknown`
  // for every message and the badge stamped "Remetente não verificado" under
  // every line a customer ever wrote — implying a risk on a channel where the
  // check is not even possible.
  test('a non-email channel never carries the badge', () {
    for (final source in [
      ConversationSource.whatsapp,
      ConversationSource.telegram,
      ConversationSource.instagram,
      ConversationSource.sms,
      ConversationSource.internal,
    ]) {
      expect(
        SenderAuthBadge.forMessage(null, isInbound: true, source: source),
        isNull,
        reason: '$source has no mail server to have checked anything',
      );
    }
  });

  test('an unknown channel is treated as not-email', () {
    // Hiding a badge beats inventing one.
    expect(SenderAuthBadge.forMessage(null, isInbound: true), isNull);
  });

  test('email with no verdict still warns', () {
    // The original purpose survives: on a real mail channel, "nobody checked"
    // is a different statement from "verified" and must not render as one.
    expect(
      SenderAuthBadge.forMessage(null,
          isInbound: true, source: ConversationSource.email),
      isNotNull,
    );
  });
}
