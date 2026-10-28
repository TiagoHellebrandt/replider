import 'dart:typed_data';

import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/onboarding/domain/onboarding_variant.dart';
import 'package:app/features/onboarding/presentation/widgets/step_import_products.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:dio/dio.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

// The second step of the shipped flow, and the one that replaced "create an
// agent". These pin the promise the step makes: what you SELL, not what bot you
// want — and that skipping it is real, because not everybody has the file open.

// A picker that always hands back one brochure, and a server that always
// refuses it. Together they reproduce the state a person actually saw: files
// chosen, nothing read, and — before this — no reason anywhere on the screen.
class _OneFilePicker extends FilePicker {
  @override
  Future<FilePickerResult?> pickFiles({
    String? dialogTitle,
    String? initialDirectory,
    FileType type = FileType.any,
    List<String>? allowedExtensions,
    // Typed, not bare `Function`: strict-inference cannot infer a bare
    // Function's return type, and that is a WARNING, which the CI analyze
    // gate fails on. The upstream signature is void-returning.
    void Function(FilePickerStatus)? onFileLoading,
    @Deprecated('allowCompression is deprecated and has no effect. '
        'Use compressionQuality instead.')
    bool allowCompression = false,
    int compressionQuality = 0,
    bool allowMultiple = false,
    bool withData = false,
    bool withReadStream = false,
    bool lockParentWindow = false,
    bool readSequential = false,
  }) async =>
      FilePickerResult([
        PlatformFile(
          name: 'catalogo.pdf',
          size: 8,
          bytes: Uint8List.fromList('%PDF-1.4'.codeUnits),
        ),
      ]);
}

class _RefusingAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(RequestOptions options,
      Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async {
    return ResponseBody.fromString(
      '{"code":"PRODUCT_PDF_PROVIDER_CANT_READ",'
      '"message":"o provedor de IA desta conta nao le arquivos"}',
      400,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType]
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  Future<void> pump(WidgetTester tester) async {
    await tester.pumpWidget(ProviderScope(
      overrides: [dioProvider.overrideWithValue(Dio())],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const Scaffold(body: StepImportProducts()),
      ),
    ));
    await tester.pumpAndSettle();
  }

  testWidgets('asks what she sells, and how to bring it', (tester) async {
    await pump(tester);

    expect(find.text(t.onboarding.products_title), findsOneWidget);
    expect(find.text(t.onboarding.products_subtitle), findsOneWidget);
    expect(find.byKey(const Key('onboarding_import_pick')), findsOneWidget);
    // What the file has to look like, before she goes looking for it.
    expect(find.text(t.product_import.what_we_read), findsOneWidget);
  });

  // Nothing has been read yet, so there is nothing to confirm and no counts to
  // state. An empty review here would be a claim about a file nobody chose.
  testWidgets('shows no review until a file has been read', (tester) async {
    await pump(tester);

    expect(find.byKey(const Key('product_import_confirm')), findsNothing);
    expect(find.byKey(const Key('product_import_summary')), findsNothing);
    expect(find.byKey(const Key('onboarding_import_result')), findsNothing);
  });

  // The whole point of the change: the flow no longer opens by asking for a
  // chatbot, and the channel stays terminal because completing it completes
  // onboarding.
  test(
      'the shipped flow is company, AI key, price list, customers, channel, offer',
      () {
    // The key comes second because the price list spends it: a contract
    // account met that wall on a step it was never given a chance to prepare.
    expect(OnboardingVariant.reduced.stepIndices, [0, 1, 5, 6, 4, 7]);
    expect(OnboardingVariant.reduced.stepIndices, isNot(contains(2)));
    expect(OnboardingVariant.reduced.stepIndices.last, 7);
  });

  // The failure used to live in a snack that faded, leaving "0 linhas lidas"
  // as the only thing on screen — which reads as "your brochures are empty"
  // when the truth was a service refusing every file. The reason stays put.
  testWidgets('a refused file leaves its reason on the screen', (tester) async {
    FilePicker.platform = _OneFilePicker();
    final dio = Dio(BaseOptions(baseUrl: 'https://api.test/v1'));
    dio.httpClientAdapter = _RefusingAdapter();

    await tester.pumpWidget(ProviderScope(
      overrides: [dioProvider.overrideWithValue(dio)],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const Scaffold(body: StepImportProducts()),
      ),
    ));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('onboarding_import_pick')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('onboarding_import_failures')), findsOneWidget);
    expect(find.textContaining('catalogo.pdf'), findsOneWidget);
  });
}
