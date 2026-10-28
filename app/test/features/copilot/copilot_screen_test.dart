import 'dart:async';
import 'dart:convert';
import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/copilot/presentation/screens/copilot_screen.dart';
import 'package:app/features/sales/presentation/controllers/audiences_controller.dart';
import 'package:app/i18n/strings.g.dart';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

// US-M026. A chat box teaches nothing on its own, so the tests pin what
// makes it usable: it says what it can do before you type, it shows the
// work behind an answer, and a failed turn says so instead of vanishing.

/// A Dio whose adapter answers from a canned handler — the copilot's only
/// dependency is one POST.
Dio _dio(ResponseBody Function(RequestOptions) handler) {
  final dio = Dio(BaseOptions(baseUrl: 'https://api.test/v1'));
  dio.httpClientAdapter = _FakeAdapter(handler);
  return dio;
}

class _FakeAdapter implements HttpClientAdapter {
  _FakeAdapter(this.handler);

  final ResponseBody Function(RequestOptions) handler;
  int calls = 0;

  @override
  Future<ResponseBody> fetch(
      RequestOptions options, Stream<Uint8List>? _, Future<void>? __) async {
    calls++;
    return handler(options);
  }

  @override
  void close({bool force = false}) {}
}

/// Holds the response until the test releases it — that is how the wait stays
/// visible long enough for the patience threshold to fire.
class _SlowAdapter implements HttpClientAdapter {
  _SlowAdapter(this.pending);

  final Future<ResponseBody> pending;

  @override
  Future<ResponseBody> fetch(
          RequestOptions options, Stream<Uint8List>? _, Future<void>? __) =>
      pending;

  @override
  void close({bool force = false}) {}
}

ResponseBody _ok(String body) => ResponseBody.fromString(body, 200, headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    });

Future<void> _pump(WidgetTester tester, Dio dio,
    {String? initialPrompt, List<Audience> audiences = const []}) async {
  tester.view.physicalSize = const Size(900, 1400);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(ProviderScope(
    overrides: [
      dioProvider.overrideWithValue(dio),
      audiencesProvider.overrideWith((ref) async => audiences),
    ],
    child: MaterialApp.router(
      theme: AppTheme.darkTheme,
      routerConfig: GoRouter(
        initialLocation: initialPrompt == null
            ? '/copilot'
            : '/copilot?prompt=${Uri.encodeQueryComponent(initialPrompt)}',
        routes: [
          // Mirrors the real route: the prompt rides in the query string.
          GoRoute(
              path: '/copilot',
              builder: (_, state) => CopilotScreen(
                  initialPrompt: state.uri.queryParameters['prompt'])),
          GoRoute(
              path: '/prospect',
              builder: (_, __) => const Scaffold(body: Text('prospect'))),
        ],
      ),
    ),
  ));
  await tester.pumpAndSettle();
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  // An empty chat box is the hardest screen in any product: it has to say
  // what it can do before anyone types.
  testWidgets('the empty state offers concrete things to ask', (tester) async {
    await _pump(tester, _dio((_) => _ok('{}')));

    expect(find.text(t.copilot.empty_title), findsOneWidget);
    expect(find.text(t.copilot.sample_prospect), findsOneWidget);
    expect(find.byKey(const Key('copilot_transcript')), findsNothing);
  });

  // The home screen's one suggestion chip travels as `/copilot?prompt=…`.
  // Nothing read it: the chip landed on an empty composer and the suggestion
  // had to be retyped — which is the one thing carrying it exists to avoid.
  testWidgets('a prompt in the link lands in the composer', (tester) async {
    await _pump(tester, _dio((_) => _ok('{"message":"ok"}')),
        initialPrompt: 'Liste os leads sem responsável');

    final field =
        tester.widget<TextField>(find.byKey(const Key('copilot_input')));
    expect(field.controller!.text, 'Liste os leads sem responsável');
  });

  // ...and stops there. Auto-sending would let a URL fire a tool that WRITES.
  testWidgets('a prompt in the link is not sent by itself', (tester) async {
    var calls = 0;
    await _pump(tester, _dio((_) {
      calls++;
      return _ok('{"message":"ok"}');
    }), initialPrompt: 'Ache 20 clínicas e importe');
    await tester.pumpAndSettle();

    expect(calls, 0, reason: 'a link must not run a tool on its own');
  });

  testWidgets('an answer shows the work behind it', (tester) async {
    await _pump(
      tester,
      _dio((_) => _ok(
          '{"message":"Achei 20 e gerei 20 abordagens.","tools_used":["draft_approaches","list_funnels"],'
          '"actions":[{"label":"Ver fila","route":"/prospect"}]}')),
    );

    await tester.enterText(
        find.byKey(const Key('copilot_input')), 'acha 20 clínicas');
    await tester.tap(find.byKey(const Key('copilot_send')));
    await tester.pumpAndSettle();

    expect(find.text('Achei 20 e gerei 20 abordagens.'), findsOneWidget);
    // Showing the work is what makes the number believable — but the work is
    // what the tool DID, not the identifier it has in the source. The screen
    // used to print `draft_approaches` at a salesperson.
    expect(find.byKey(const Key('copilot_tools_used')), findsOneWidget);
    expect(
        find.textContaining(t.copilot.tool_draft_approaches), findsOneWidget);
    expect(find.textContaining('draft_approaches'), findsNothing);
  });

  // A prospecting run that created 56 leads was announced in the chat as "1 of
  // them was added as a new lead". The prose is the model's; on an action that
  // changes the account it does not serve as an account of what happened. The
  // receipt comes from the server and is printed as it came.
  testWidgets('what the tool wrote shows without passing through the model',
      (tester) async {
    await _pump(
      tester,
      _dio((_) => _ok('{"message":"1 lead adicionado.",'
          '"tools_used":["draft_approaches"],'
          '"receipts":[{"tool":"draft_approaches",'
          '"text":"60 empresas encontradas, 56 viraram leads novos (4 já existiam)"}]}')),
    );

    await tester.enterText(
        find.byKey(const Key('copilot_input')), 'ache clínicas');
    await tester.tap(find.byKey(const Key('copilot_send')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('copilot_receipt')), findsOneWidget);
    expect(find.textContaining('56 viraram leads novos'), findsOneWidget);
  });

  // With no receipt (a read), no box: paraphrasing a list costs one more
  // question, not an account changed in silence.
  testWidgets('an answer with no write gets no receipt box', (tester) async {
    await _pump(
      tester,
      _dio((_) => _ok('{"message":"Você tem 3 funis.",'
          '"tools_used":["list_funnels"]}')),
    );

    await tester.enterText(
        find.byKey(const Key('copilot_input')), 'quais funis eu tenho?');
    await tester.tap(find.byKey(const Key('copilot_send')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('copilot_receipt')), findsNothing);
  });

  // "searched companies · searched companies" reads worse than the truth,
  // which is "twice".
  testWidgets('a mesma ferramenta duas vezes vira ×2', (tester) async {
    await _pump(
      tester,
      _dio((_) => _ok('{"message":"pronto",'
          '"tools_used":["draft_approaches","draft_approaches"]}')),
    );

    await tester.enterText(find.byKey(const Key('copilot_input')), 'ache');
    await tester.tap(find.byKey(const Key('copilot_send')));
    await tester.pumpAndSettle();

    expect(find.text('${t.copilot.tool_draft_approaches} ×2'), findsOneWidget);
  });

  // The screen hides the id; the conversation must not forget it. Without this
  // the model asks "confirm which funnel", the person says "yes, use that one",
  // and the next call goes out with a guessed id — seen in production.
  testWidgets('history returns the text with the id, not the displayed text',
      (tester) async {
    const id = '01KYW9HYFXQK3Y41HAJFTG9CSS';
    final sent = <Map<String, dynamic>>[];
    await _pump(
      tester,
      _dio((req) {
        sent.add((req.data as Map).cast<String, dynamic>());
        return _ok(jsonEncode(<String, dynamic>{
          'message': 'O único funil é "TESTE QA".',
          'context_message': 'O único funil é "TESTE QA" (id: $id).',
        }));
      }),
    );

    await tester.enterText(
        find.byKey(const Key('copilot_input')), 'quais funis?');
    await tester.tap(find.byKey(const Key('copilot_send')));
    await tester.pumpAndSettle();

    // A tela mostra a versão limpa...
    expect(find.textContaining(id), findsNothing);

    await tester.enterText(
        find.byKey(const Key('copilot_input')), 'sim, use esse');
    await tester.tap(find.byKey(const Key('copilot_send')));
    await tester.pumpAndSettle();

    // ...e a próxima pergunta leva o id junto.
    final history =
        (sent.last['history'] as List).cast<Map<dynamic, dynamic>>();
    final assistant =
        history.firstWhere((h) => h['role'] == 'model')['content'] as String;
    expect(assistant, contains(id));
  });

  // A tool nobody translated still ran. Hiding it would be the one thing this
  // line exists to prevent.
  testWidgets('an unknown tool is still named', (tester) async {
    await _pump(
      tester,
      _dio(
          (_) => _ok('{"message":"Pronto.","tools_used":["ferramenta_nova"]}')),
    );

    await tester.enterText(find.byKey(const Key('copilot_input')), 'oi');
    await tester.tap(find.byKey(const Key('copilot_send')));
    await tester.pumpAndSettle();

    expect(find.textContaining('ferramenta_nova'), findsOneWidget);
  });

  testWidgets('an action button takes you to the work', (tester) async {
    await _pump(
      tester,
      _dio((_) => _ok(
          '{"message":"Prontas.","actions":[{"label":"Ver fila","route":"/prospect"}]}')),
    );

    await tester.enterText(find.byKey(const Key('copilot_input')), 'gera');
    await tester.tap(find.byKey(const Key('copilot_send')));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('copilot_action_/prospect')));
    await tester.pumpAndSettle();
    expect(find.text('prospect'), findsOneWidget);
  });

  // A turn that failed stays visible and says why. Dropping it leaves the
  // user staring at their own question, unsure whether anything happened.
  testWidgets('a failed turn says so instead of vanishing', (tester) async {
    await _pump(
      tester,
      _dio((options) => ResponseBody.fromString(
            '{"code":"COPILOT_UNAVAILABLE","message":"indisponível"}',
            503,
            headers: {
              Headers.contentTypeHeader: [Headers.jsonContentType],
            },
          )),
    );

    await tester.enterText(find.byKey(const Key('copilot_input')), 'oi');
    await tester.tap(find.byKey(const Key('copilot_send')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('copilot_error')), findsOneWidget);
    // ...and what the user typed is still on screen.
    expect(find.text('oi'), findsOneWidget);
  });

  // One turn is a model call plus however many tool round-trips it decides to
  // make, and the client waits up to 120s. A spinner reading "thinking…" frozen
  // for a minute and a half reads as stuck, and the person reloads the page in
  // the middle of a prospecting run that was working.
  testWidgets('a long wait explains itself instead of going mute',
      (tester) async {
    final completer = Completer<ResponseBody>();
    final dio = _dio((_) => throw UnimplementedError());
    dio.httpClientAdapter = _SlowAdapter(completer.future);
    await _pump(tester, dio);

    await tester.enterText(find.byKey(const Key('copilot_input')), 'prospecte');
    await tester.tap(find.byKey(const Key('copilot_send')));
    await tester.pump();

    expect(find.byKey(const Key('copilot_thinking')), findsOneWidget);
    // Before the threshold it does not justify itself: a fast answer must
    // never show the slowness notice.
    expect(find.byKey(const Key('copilot_thinking_long')), findsNothing);

    await tester.pump(const Duration(seconds: 16));
    expect(find.byKey(const Key('copilot_thinking_long')), findsOneWidget);

    completer.complete(_ok('{"message":"pronto"}'));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('copilot_thinking')), findsNothing);
  });

  // The prospecting suggestion used to run a tool that IMPORTS: one tap filled
  // the funnel with twenty companies from a segment and a city nobody chose,
  // and spent Places quota doing it. It is the same rule `initialPrompt`
  // already follows in writing.
  testWidgets('tapping a suggestion fills the field and does not fire',
      (tester) async {
    final dio = _dio((_) => _ok('{"message":"nunca"}'));
    await _pump(tester, dio);

    await tester.tap(find.byKey(const Key('copilot_sample_0')));
    await tester.pumpAndSettle();

    expect((dio.httpClientAdapter as _FakeAdapter).calls, 0);
    final field =
        tester.widget<TextField>(find.byKey(const Key('copilot_input')));
    expect(field.controller!.text, t.copilot.sample_prospect);
  });

  // "Find 20 imaging clinics in São Paulo" taught the format and chose the
  // vertical, in a product that is not about clinics. With no saved audience
  // the example becomes the template to fill in — honest about what is missing.
  testWidgets('with no saved audience, the example is a vertical-free template',
      (tester) async {
    final dio = _dio((_) => _ok('{"message":"x"}'));
    await _pump(tester, dio);

    expect(find.text(t.copilot.sample_prospect), findsOneWidget);
    expect(t.copilot.sample_prospect.toLowerCase(), isNot(contains('clínic')));
  });

  // With a saved audience it IS the right answer and is already stored: the
  // example stops being a template and becomes what the person actually sells.
  testWidgets("with a saved audience, the example uses the account's segment",
      (tester) async {
    final dio = _dio((_) => _ok('{"message":"x"}'));
    await _pump(tester, dio, audiences: const [
      Audience(
        id: 'a1',
        name: 'Distribuidoras',
        segment: 'distribuidoras de equipamento',
        keywords: ['distribuidora de equipamento médico'],
        regions: ['Campinas'],
      ),
    ]);

    expect(
      find.text(t.copilot.sample_prospect_audience(
          keyword: 'distribuidora de equipamento médico',
          location: 'Campinas')),
      findsOneWidget,
    );
  });

  testWidgets('the composer does not send an empty message', (tester) async {
    final dio = _dio((_) => _ok('{"message":"nunca"}'));
    await _pump(tester, dio);

    await tester.tap(find.byKey(const Key('copilot_send')));
    await tester.pumpAndSettle();

    expect((dio.httpClientAdapter as _FakeAdapter).calls, 0);
  });
}
