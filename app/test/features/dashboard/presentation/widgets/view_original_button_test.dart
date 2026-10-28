import 'dart:typed_data';

import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/dashboard/presentation/widgets/view_original_button.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

// The body a bubble renders is plain text by design. This is how the reader
// gets the sender's formatting back — in a browser, where the server's CSP
// can actually be enforced.

/// A Dio whose adapter always throws, so the tap fails fast instead of
/// dialing the network — and its request-timeout Timer resolves rather than
/// outliving the widget tree.
Dio _throwingDio() {
  final dio = Dio(BaseOptions(baseUrl: 'https://api.test/v1'));
  dio.httpClientAdapter = _ThrowingAdapter();
  return dio;
}

class _ThrowingAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(RequestOptions options, Stream<Uint8List>? _,
          Future<void>? __) async =>
      throw DioException(requestOptions: options, message: 'offline no teste');

  @override
  void close({bool force = false}) {}
}

Future<void> _pump(WidgetTester tester) async {
  await tester.pumpWidget(ProviderScope(
    overrides: [dioProvider.overrideWithValue(_throwingDio())],
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      home: const Scaffold(body: ViewOriginalButton(messageId: 'msg-1')),
    ),
  ));
  await tester.pumpAndSettle();
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  // The key is written only when the markup was actually stored, so a
  // missing key means the button never appears rather than appearing and
  // failing on click.
  test('the button is offered only when markup was stored', () {
    expect(ViewOriginalButton.isAvailable(null), isFalse);
    expect(ViewOriginalButton.isAvailable(<String, dynamic>{}), isFalse);
    expect(
        ViewOriginalButton.isAvailable(
            <String, dynamic>{'email_html_path': ''}),
        isFalse);
    expect(
        ViewOriginalButton.isAvailable(
            <String, dynamic>{'email_html_path': '   '}),
        isFalse);
    expect(
        ViewOriginalButton.isAvailable(
            <String, dynamic>{'email_html_path': 'accounts/a/email-html/x'}),
        isTrue);
  });

  testWidgets('offers the safe view and the one that loads images',
      (tester) async {
    await _pump(tester);

    expect(find.text(t.attachment.view_original), findsOneWidget);
    expect(find.text(t.attachment.view_original_images), findsOneWidget);
  });

  // Loading remote images has a consequence the reader cannot see: it tells
  // the sender the message was opened. So it is asked, in those words.
  testWidgets('loading images asks first, and says why', (tester) async {
    await _pump(tester);

    await tester.tap(find.byKey(const Key('view_original_images_msg-1')));
    await tester.pumpAndSettle();

    expect(
        find.text(t.attachment.view_original_images_warning), findsOneWidget);
  });

  testWidgets('the plain view does not ask about images', (tester) async {
    await _pump(tester);

    await tester.tap(find.byKey(const Key('view_original_msg-1')));
    await tester.pumpAndSettle();

    expect(find.text(t.attachment.view_original_images_warning), findsNothing);
  });
}
