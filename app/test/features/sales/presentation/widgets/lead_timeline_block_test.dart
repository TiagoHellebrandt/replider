import 'dart:typed_data';

import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/sales/domain/models/lead_timeline.dart';
import 'package:app/features/sales/presentation/controllers/lead_timeline_controller.dart';
import 'package:app/features/sales/presentation/widgets/lead_timeline_block.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

// The reply, where the person who sent the proposal can see it. Before this a
// vendedor sent and never knew they had been answered: the reply landed in an
// inbox their role is not allowed to open.

TimelineMessage _msg({
  required String id,
  required TimelineAuthor author,
  String text = 'oi',
  String authorName = '',
  String kind = 'text',
}) =>
    TimelineMessage(
      id: id,
      author: author,
      text: text,
      authorName: authorName,
      kind: kind,
      sentAt: DateTime(2026, 8, 12, 14, 30),
    );

/// Records what the composer posts and answers 201, so a send never reaches
/// out. `status` lets a test make the server refuse.
class _RecordingAdapter implements HttpClientAdapter {
  _RecordingAdapter({this.status = 201, this.body = ''});

  final int status;
  final String body;
  final requests = <RequestOptions>[];

  @override
  Future<ResponseBody> fetch(
      RequestOptions options, Stream<Uint8List>? _, Future<void>? __) async {
    requests.add(options);
    return ResponseBody.fromString(body, status, headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType]
    });
  }

  @override
  void close({bool force = false}) {}
}

Future<void> _pump(
  WidgetTester tester, {
  LeadTimeline? timeline,
  Object? error,
  Dio? dio,
}) async {
  tester.view.physicalSize = const Size(900, 1600);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(ProviderScope(
    overrides: [
      dioProvider.overrideWithValue(dio ?? Dio()),
      leadTimelineProvider('lead-1').overrideWith((ref) async {
        if (error != null) throw error;
        return timeline ?? const LeadTimeline(messages: []);
      }),
    ],
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      home: const Scaffold(body: LeadTimelineBlock(leadId: 'lead-1')),
    ),
  ));
  await tester.pumpAndSettle();
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  testWidgets('shows what was said, and who said it', (tester) async {
    await _pump(
      tester,
      timeline: LeadTimeline(messages: [
        _msg(
            id: 'm1',
            author: TimelineAuthor.team,
            authorName: 'Rafa',
            text: 'Segue a proposta.'),
        _msg(
            id: 'm2',
            author: TimelineAuthor.customer,
            text: 'Recebi, vou avaliar.'),
        _msg(id: 'm3', author: TimelineAuthor.bot, text: 'Posso ajudar?'),
      ]),
    );

    expect(find.byKey(const Key('lead_timeline_list')), findsOneWidget);
    expect(find.text('Segue a proposta.'), findsOneWidget);
    expect(find.text('Recebi, vou avaliar.'), findsOneWidget);

    // A teammate is named; the customer and the bot are not — the customer's
    // name is on the card above and "bot" is not a person.
    expect(find.text('Rafa'), findsOneWidget);
    expect(find.text(t.lead_timeline.from_customer), findsOneWidget);
    expect(find.text(t.lead_timeline.from_bot), findsOneWidget);
  });

  // "We answered" and "the bot answered" are different facts about the account,
  // so an unnamed teammate still reads as us rather than as the bot.
  testWidgets('an unnamed teammate is still us, not the bot', (tester) async {
    await _pump(
      tester,
      timeline:
          LeadTimeline(messages: [_msg(id: 'm1', author: TimelineAuthor.team)]),
    );

    expect(find.text(t.lead_timeline.from_team), findsOneWidget);
    expect(find.text(t.lead_timeline.from_bot), findsNothing);
  });

  // An empty row reads as a loading defect, so a photo says it is a photo.
  testWidgets('a media message renders as what it is', (tester) async {
    await _pump(
      tester,
      timeline: LeadTimeline(messages: [
        _msg(id: 'm1', author: TimelineAuthor.customer, text: '', kind: 'image')
      ]),
    );

    expect(
        find.text(t.lead_timeline.attachment(kind: 'image')), findsOneWidget);
  });

  testWidgets('nothing sent yet says so plainly', (tester) async {
    await _pump(tester);

    expect(find.byKey(const Key('lead_timeline_empty')), findsOneWidget);
    expect(find.byKey(const Key('lead_timeline_list')), findsNothing);
    // No thread, no composer: the first contact goes through the queue, and
    // a reply box here would only collect a LEAD_NO_CONVERSATION.
    expect(find.byKey(const Key('lead_reply_field')), findsNothing);
    expect(
        find.byKey(const Key('lead_timeline_no_conversation')), findsOneWidget);
  });

  // The reply, where the person who read it can write back. She could see
  // "yes" here and every place to answer was attendant-gated.
  testWidgets('the composer posts the reply to the lead and clears',
      (tester) async {
    final adapter = _RecordingAdapter();
    final dio = Dio()..httpClientAdapter = adapter;
    await _pump(
      tester,
      dio: dio,
      timeline: LeadTimeline(messages: [
        _msg(id: 'm1', author: TimelineAuthor.customer, text: 'Recebi.'),
      ]),
    );

    expect(find.byKey(const Key('lead_reply_field')), findsOneWidget);
    await tester.enterText(
        find.byKey(const Key('lead_reply_field')), '  Ótimo, mando hoje.  ');
    await tester.tap(find.byKey(const Key('lead_reply_send')));
    await tester.pumpAndSettle();

    expect(adapter.requests, hasLength(1));
    final req = adapter.requests.single;
    expect(req.method, 'POST');
    expect(req.path, '/leads/lead-1/reply');
    expect((req.data as Map)['text'], 'Ótimo, mando hoje.');
    expect(find.text(t.lead_timeline.reply_sent), findsOneWidget);
    // Cleared for the next one.
    expect(
        tester
            .widget<TextField>(find.byKey(const Key('lead_reply_field')))
            .controller!
            .text,
        isEmpty);
  });

  // The click gets an answer: an empty reply is refused on the screen, not by
  // a round-trip to hear what the screen already knew.
  testWidgets('an empty reply is refused before any request', (tester) async {
    final adapter = _RecordingAdapter();
    final dio = Dio()..httpClientAdapter = adapter;
    await _pump(
      tester,
      dio: dio,
      timeline: LeadTimeline(
          messages: [_msg(id: 'm1', author: TimelineAuthor.customer)]),
    );

    await tester.tap(find.byKey(const Key('lead_reply_send')));
    await tester.pumpAndSettle();

    expect(adapter.requests, isEmpty);
    expect(find.byType(SnackBar), findsOneWidget);
  });

  // The server's refusal is shown in the person's language, not as a raw code.
  testWidgets('a refused send is explained', (tester) async {
    final adapter = _RecordingAdapter(
        status: 409,
        body: '{"code":"LEAD_NO_CONVERSATION","message":"no thread"}');
    final dio = Dio()..httpClientAdapter = adapter;
    await _pump(
      tester,
      dio: dio,
      timeline: LeadTimeline(
          messages: [_msg(id: 'm1', author: TimelineAuthor.customer)]),
    );

    await tester.enterText(find.byKey(const Key('lead_reply_field')), 'Oi');
    await tester.tap(find.byKey(const Key('lead_reply_send')));
    await tester.pumpAndSettle();

    expect(find.text(t.errors.LEAD_NO_CONVERSATION), findsOneWidget);
  });

  // A failed read must never render as "nobody wrote to this customer": that is
  // a claim a broken query has not earned, and it would send the salesperson
  // chasing somebody who already replied.
  testWidgets('a failed read says so instead of claiming silence',
      (tester) async {
    await _pump(tester, error: Exception('boom'));

    expect(find.byKey(const Key('lead_timeline_error')), findsOneWidget);
    expect(find.byKey(const Key('lead_timeline_empty')), findsNothing);
    expect(find.text(t.lead_timeline.empty), findsNothing);
  });

  // A list that silently starts mid-conversation reads as the beginning of the
  // relationship.
  testWidgets('a capped history says it is capped', (tester) async {
    await _pump(
      tester,
      timeline: LeadTimeline(
        truncated: true,
        messages: [_msg(id: 'm1', author: TimelineAuthor.customer)],
      ),
    );

    expect(find.byKey(const Key('lead_timeline_truncated')), findsOneWidget);
  });

  // An unrecognised author reads as the bot: claiming a person wrote something
  // they did not is the worse mistake.
  test('an unknown author is never attributed to a teammate', () {
    expect(TimelineAuthor.from('quem-sabe'), TimelineAuthor.bot);
    expect(TimelineAuthor.from(null), TimelineAuthor.bot);
    expect(TimelineAuthor.from('team'), TimelineAuthor.team);
    expect(TimelineAuthor.from('customer'), TimelineAuthor.customer);
  });
}
