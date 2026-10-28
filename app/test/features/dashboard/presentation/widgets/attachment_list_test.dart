import 'package:app/design_system/app_theme.dart';
import 'package:app/features/dashboard/domain/models/message_attachment.dart';
import 'package:app/features/dashboard/presentation/widgets/attachment_list.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

// Attachments used to vanish silently. Now they arrive with a verdict, and
// the verdict has to be readable BEFORE the click — discovering that a file
// is blocked by clicking it is how a product implies a safety it never
// checked.

MessageAttachment _att({
  String id = 'a1',
  String filename = 'termo.pdf',
  int size = 2048,
  bool inline = false,
  String contentType = 'application/pdf',
  AttachmentScanStatus status = AttachmentScanStatus.clean,
  String? signature,
}) =>
    MessageAttachment(
      id: id,
      filename: filename,
      contentType: contentType,
      sizeBytes: size,
      isInline: inline,
      scanStatus: status,
      scanSignature: signature,
    );

Future<void> _pump(
  WidgetTester tester,
  List<MessageAttachment> items, {
  Size size = const Size(900, 1200),
  bool includeLargeInline = false,
}) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(ProviderScope(
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      home: Scaffold(
          body: AttachmentList(
              attachments: items, includeLargeInline: includeLargeInline)),
    ),
  ));
  await tester.pumpAndSettle();
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  testWidgets('a clean file shows its name, size and verdict', (tester) async {
    await _pump(tester, [_att()]);

    expect(find.text('termo.pdf'), findsOneWidget);
    expect(
      tester.widget<Text>(find.byKey(const Key('attachment_status_a1'))).data,
      '2 KB · ${t.attachment.status_clean}',
    );
    expect(find.text(t.attachment.open), findsOneWidget);
  });

  // The distinction the whole feature exists for.
  testWidgets('an unscanned file does not read as a checked one',
      (tester) async {
    await _pump(tester, [_att(status: AttachmentScanStatus.skipped)]);

    expect(
      tester.widget<Text>(find.byKey(const Key('attachment_status_a1'))).data,
      contains(t.attachment.status_skipped),
    );
    expect(find.text(t.attachment.status_clean), findsNothing);
    // Not a plain "Abrir": opening it is a decision, and the label says so.
    expect(find.text(t.attachment.download_anyway), findsOneWidget);
    expect(find.text(t.attachment.open), findsNothing);
  });

  testWidgets('pending and failed are shown apart from each other',
      (tester) async {
    await _pump(tester, [
      _att(id: 'a1', status: AttachmentScanStatus.pending),
      _att(id: 'a2', filename: 'b.pdf', status: AttachmentScanStatus.failed),
    ]);

    expect(
      tester.widget<Text>(find.byKey(const Key('attachment_status_a1'))).data,
      contains(t.attachment.status_pending),
    );
    expect(
      tester.widget<Text>(find.byKey(const Key('attachment_status_a2'))).data,
      contains(t.attachment.status_failed),
    );
  });

  // No way through, and the button says so: a "download anyway" on a flagged
  // file would make the scan decorative.
  testWidgets('an infected file offers an explanation, not a way through',
      (tester) async {
    await _pump(tester, [
      _att(
          status: AttachmentScanStatus.infected,
          signature: 'Eicar-Test-Signature'),
    ]);

    expect(find.text(t.attachment.why_blocked), findsOneWidget);
    expect(find.text(t.attachment.download_anyway), findsNothing);
    expect(find.text(t.attachment.open), findsNothing);
  });

  testWidgets('the infected explanation names the signature', (tester) async {
    await _pump(tester, [
      _att(
          status: AttachmentScanStatus.infected,
          signature: 'Eicar-Test-Signature'),
    ]);

    await tester.tap(find.byKey(const Key('attachment_open_a1')));
    await tester.pumpAndSettle();

    // "Infectado" with no name is impossible to argue with when it is a
    // false positive.
    expect(find.textContaining('Eicar-Test-Signature'), findsOneWidget);
    // One button: there is exactly one thing to do with a flagged file.
    expect(find.text(t.common.close), findsOneWidget);
    expect(find.text(t.common.cancel), findsNothing);
  });

  testWidgets('opening an unscanned file asks first', (tester) async {
    await _pump(tester, [_att(status: AttachmentScanStatus.skipped)]);

    await tester.tap(find.byKey(const Key('attachment_open_a1')));
    await tester.pumpAndSettle();

    expect(find.text(t.attachment.blocked_title), findsOneWidget);
    expect(find.textContaining(t.attachment.download_anyway_confirm),
        findsOneWidget);
  });

  // A signature logo is not what a person means by "anexo".
  testWidgets('inline furniture is not listed', (tester) async {
    await _pump(tester, [
      _att(id: 'a1', filename: 'logo.png', inline: true),
      _att(id: 'a2', filename: 'contrato.pdf'),
    ]);

    expect(find.text('contrato.pdf'), findsOneWidget);
    expect(find.text('logo.png'), findsNothing);
  });

  testWidgets('a message with no attachments renders nothing', (tester) async {
    await _pump(tester, const []);

    expect(find.byType(TextButton), findsNothing);
  });

  test('an unknown scan status reads as pending, never as clean', () {
    expect(AttachmentScanStatus.fromWire('something_new'),
        AttachmentScanStatus.pending);
    expect(AttachmentScanStatus.fromWire(null), AttachmentScanStatus.pending);
    expect(AttachmentScanStatus.fromWire('clean').isDownloadable, isTrue);
    for (final s in [
      AttachmentScanStatus.pending,
      AttachmentScanStatus.skipped,
      AttachmentScanStatus.failed,
      AttachmentScanStatus.infected,
    ]) {
      expect(s.isDownloadable, isFalse, reason: '$s must not be downloadable');
    }
  });

  test('size reads in whole units below a megabyte', () {
    expect(_att(size: 900).readableSize, '900 B');
    expect(_att(size: 2048).readableSize, '2 KB');
    expect(_att(size: 5 * 1024 * 1024).readableSize, '5.0 MB');
  });

  // A pasted PHOTO is content, not furniture: the email text mode lists it
  // as an openable attachment (the formatted view renders it in place
  // instead). The signature logo stays hidden either way.
  testWidgets('a large inline image is listed only when asked', (tester) async {
    final photo = _att(
        id: 'foto',
        filename: 'foto.jpg',
        inline: true,
        contentType: 'image/jpeg',
        size: 4 * 1024 * 1024);

    await _pump(tester, [photo]);
    expect(find.byKey(const Key('attachment_foto')), findsNothing);

    await _pump(tester, [photo], includeLargeInline: true);
    expect(find.byKey(const Key('attachment_foto')), findsOneWidget);
  });

  testWidgets('a small inline logo is never listed', (tester) async {
    final logo = _att(
        id: 'logo',
        filename: 'logo.png',
        inline: true,
        contentType: 'image/png',
        size: 20 * 1024);

    await _pump(tester, [logo], includeLargeInline: true);
    expect(find.byKey(const Key('attachment_logo')), findsNothing);
  });
}
