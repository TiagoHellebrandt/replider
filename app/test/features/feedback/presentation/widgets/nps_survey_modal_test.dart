import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/feedback/presentation/controllers/nps_prompt_controller.dart';
import 'package:app/features/feedback/presentation/widgets/nps_survey_modal.dart';
import 'package:app/i18n/strings.g.dart';

class _FakeAdapter implements HttpClientAdapter {
  /// Path of the last non-GET request (i.e. the submit/dismiss POST). The
  /// GET /me/nps-prompt from the controller's build() is ignored here so
  /// it doesn't mask which action endpoint was hit.
  String? lastPostPath;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<dynamic>? cancelFuture,
  ) async {
    if (options.method == 'GET') {
      // Eligibility probe — answer with JSON so build() resolves cleanly.
      return ResponseBody.fromString(
        '{"eligible": true}',
        200,
        headers: {
          Headers.contentTypeHeader: [Headers.jsonContentType],
        },
      );
    }
    lastPostPath = options.path;
    return ResponseBody.fromString('', 204);
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  late _FakeAdapter adapter;

  setUpAll(() {
    FlutterSecureStorage.setMockInitialValues({});
  });

  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
    adapter = _FakeAdapter();
  });

  Widget harness(ProviderContainer container) {
    return UncontrolledProviderScope(
      container: container,
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: Builder(
          builder: (context) => Scaffold(
            body: Center(
              child: ElevatedButton(
                onPressed: () => showNpsSurveyModal(context),
                child: const Text('open'),
              ),
            ),
          ),
        ),
      ),
    );
  }

  ProviderContainer makeContainer() {
    final dio = Dio(BaseOptions(baseUrl: 'http://test/v1'))
      ..httpClientAdapter = adapter;
    final c = ProviderContainer(
      overrides: [dioProvider.overrideWithValue(dio)],
    );
    addTearDown(c.dispose);
    return c;
  }

  testWidgets('renders the NPS question with submit disabled until a score',
      (tester) async {
    final container = makeContainer();
    await tester.pumpWidget(harness(container));
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    expect(find.text(t.feedback.nps_title), findsOneWidget);
    // 0..10 score chips are present.
    expect(find.text('10'), findsOneWidget);

    final submitButton = tester.widget<ElevatedButton>(
      find.widgetWithText(ElevatedButton, t.feedback.nps_submit),
    );
    expect(submitButton.onPressed, isNull); // disabled before a score
  });

  testWidgets('submitting a score posts to /me/nps and closes the modal',
      (tester) async {
    final container = makeContainer();
    await tester.pumpWidget(harness(container));
    // Mirror production: the eligibility probe (build()) resolves before the
    // user interacts (maybeShowNpsPrompt watches it first). runAsync escapes
    // the widget-test fake-async zone so the real dio future completes.
    await tester
        .runAsync(() => container.read(npsPromptControllerProvider.future));
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    await tester.tap(find.text('9'));
    await tester.pump();
    await tester
        .tap(find.widgetWithText(ElevatedButton, t.feedback.nps_submit));
    await tester.pumpAndSettle();

    expect(adapter.lastPostPath, '/me/nps');
    // Modal closed + quarter stamped so it won't re-prompt.
    expect(find.text(t.feedback.nps_title), findsNothing);
    expect(container.read(npsPromptControllerProvider).value, isTrue);
  });

  testWidgets('"maybe later" dismisses via the dismiss endpoint',
      (tester) async {
    final container = makeContainer();
    await tester.pumpWidget(harness(container));
    // Let the eligibility probe resolve before interacting (see above).
    await tester
        .runAsync(() => container.read(npsPromptControllerProvider.future));
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    await tester.tap(find.text(t.feedback.nps_later));
    await tester.pumpAndSettle();

    // "Maybe later" stamps the quarter via the dismiss endpoint, never the
    // score-submit endpoint.
    expect(adapter.lastPostPath, '/me/nps/dismiss');
    expect(find.text(t.feedback.nps_title), findsNothing);
    expect(container.read(npsPromptControllerProvider).value, isTrue);
  });
}
