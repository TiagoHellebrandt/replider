import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/feedback/presentation/widgets/cancellation_survey_modal.dart';
import 'package:app/i18n/strings.g.dart';

class _FakeAdapter implements HttpClientAdapter {
  RequestOptions? lastRequest;
  String? lastBody;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<dynamic>? cancelFuture,
  ) async {
    lastRequest = options;
    lastBody = options.data?.toString();
    return ResponseBody.fromString('', 204);
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  late _FakeAdapter adapter;

  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
    adapter = _FakeAdapter();
    resetCancellationSurveyShownForTest();
  });

  // The survey is taller than the default 600px test surface; give it room so
  // every reason row + the close button are on-screen and hit-testable.
  void useTallSurface(WidgetTester tester) {
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
  }

  Widget harness(ProviderContainer container) {
    return UncontrolledProviderScope(
      container: container,
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: Builder(
          builder: (context) => Scaffold(
            body: Center(
              child: ElevatedButton(
                onPressed: () => showCancellationSurveyModal(context),
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

  testWidgets('renders reasons with submit disabled until one is picked',
      (tester) async {
    useTallSurface(tester);
    await tester.pumpWidget(harness(makeContainer()));
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    expect(find.text(t.feedback.cancel_title), findsOneWidget);
    expect(find.text(t.feedback.cancel_reasons.too_expensive), findsOneWidget);

    final submit = tester.widget<ElevatedButton>(
      find.widgetWithText(ElevatedButton, t.feedback.cancel_submit),
    );
    expect(submit.onPressed, isNull); // disabled before a reason is selected
  });

  testWidgets('picking a reason and submitting posts to the survey endpoint',
      (tester) async {
    useTallSurface(tester);
    await tester.pumpWidget(harness(makeContainer()));
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    await tester.tap(find.text(t.feedback.cancel_reasons.too_expensive));
    await tester.pump();
    await tester
        .tap(find.widgetWithText(ElevatedButton, t.feedback.cancel_submit));
    await tester.pumpAndSettle();

    expect(adapter.lastRequest?.path, '/me/cancellation-survey');
    expect(adapter.lastBody, contains('too_expensive'));
    // Modal closed after a successful submit.
    expect(find.text(t.feedback.cancel_title), findsNothing);
  });

  testWidgets('close dismisses without posting', (tester) async {
    useTallSurface(tester);
    await tester.pumpWidget(harness(makeContainer()));
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    await tester.tap(find.text(t.common.close));
    await tester.pumpAndSettle();

    expect(adapter.lastRequest, isNull);
    expect(find.text(t.feedback.cancel_title), findsNothing);
  });

  testWidgets(
      'maybeShowCancellationSurvey shows once, and not when trigger is '
      'false', (tester) async {
    // trigger=false → nothing shown.
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: makeContainer(),
        child: MaterialApp(
          theme: AppTheme.darkTheme,
          home: Builder(builder: (context) {
            maybeShowCancellationSurvey(context, trigger: false);
            return const Scaffold(body: SizedBox.shrink());
          }),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text(t.feedback.cancel_title), findsNothing);

    // trigger=true → shown exactly once for the session.
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: makeContainer(),
        child: MaterialApp(
          theme: AppTheme.darkTheme,
          home: Builder(builder: (context) {
            maybeShowCancellationSurvey(context, trigger: true);
            return const Scaffold(body: SizedBox.shrink());
          }),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text(t.feedback.cancel_title), findsOneWidget);
  });
}
