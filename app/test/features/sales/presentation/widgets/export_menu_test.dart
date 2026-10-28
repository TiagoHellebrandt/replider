import 'dart:convert';
import 'dart:typed_data';

import 'package:app/core/network/dio_client.dart';
import 'package:app/core/utils/file_download.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/sales/presentation/widgets/export_menu.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

// The export button (I3).
//
// The promise is "what you are looking at, in a file", so what the tests pin is
// the request: the screen's active filters go with it, empty ones do not, and
// the user is told when the file is not the whole list.

class _RecordingAdapter implements HttpClientAdapter {
  _RecordingAdapter({this.truncated = false, this.rows = '12'});

  final bool truncated;
  final String rows;
  RequestOptions? lastRequest;

  @override
  Future<ResponseBody> fetch(RequestOptions options,
      Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async {
    lastRequest = options;
    return ResponseBody.fromBytes(
      utf8.encode('id;contato\nl1;Clínica'),
      200,
      headers: {
        'content-disposition': ['attachment; filename=leads_2026-07-30.csv'],
        'x-export-rows': [rows],
        if (truncated) 'x-export-truncated': ['true'],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

Future<void> _pump(
  WidgetTester tester, {
  required HttpClientAdapter adapter,
  Map<String, String> filters = const {},
}) async {
  final dio = Dio()..httpClientAdapter = adapter;

  await tester.pumpWidget(ProviderScope(
    overrides: [dioProvider.overrideWithValue(dio)],
    child: MaterialApp(
      theme: AppTheme.darkTheme,
      home: Scaffold(
        appBar: AppBar(
          actions: [ExportMenu(entity: 'leads', filters: filters)],
        ),
      ),
    ),
  ));
  await tester.pumpAndSettle();
}

Future<void> _choose(WidgetTester tester, String label) async {
  await tester.tap(find.byKey(const Key('export_menu')));
  await tester.pumpAndSettle();
  await tester.tap(find.text(label));
  await tester.pumpAndSettle();
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  testWidgets('offers both formats and says the download is recorded',
      (tester) async {
    await _pump(tester, adapter: _RecordingAdapter());

    await tester.tap(find.byKey(const Key('export_menu')));
    await tester.pumpAndSettle();

    expect(find.text(t.export.xlsx), findsOneWidget);
    expect(find.text(t.export.csv), findsOneWidget);
    // Said BEFORE the download, not after: somebody taking the customer list
    // should know it is not anonymous while they can still decide.
    expect(find.text(t.export.audited), findsOneWidget);
  });

  testWidgets('the screen\'s active filters go with the request',
      (tester) async {
    final adapter = _RecordingAdapter();
    await _pump(tester, adapter: adapter, filters: {
      'funnel_id': 'f1',
      'status': 'responded',
      // A cleared filter must not be sent as an empty string, which the server
      // would treat as a filter nobody asked for.
      'source': '',
    });

    await _choose(tester, t.export.csv);

    final query = adapter.lastRequest!.queryParameters;
    expect(query['format'], 'csv');
    expect(query['funnel_id'], 'f1');
    expect(query['status'], 'responded');
    expect(query.containsKey('source'), isFalse);
  });

  testWidgets('choosing xlsx asks for xlsx', (tester) async {
    final adapter = _RecordingAdapter();
    await _pump(tester, adapter: adapter);

    await _choose(tester, t.export.xlsx);

    expect(adapter.lastRequest!.queryParameters['format'], 'xlsx');
    // Bytes, not JSON: an XLSX is a zip and the CSV carries a BOM that must
    // survive untouched.
    expect(adapter.lastRequest!.responseType, ResponseType.bytes);
  });

  testWidgets('a complete export reports how many rows', (tester) async {
    await _pump(tester, adapter: _RecordingAdapter(rows: '12'));

    await _choose(tester, t.export.csv);

    expect(find.text(t.export.done(rows: '12')), findsOneWidget);
  });

  testWidgets('a truncated export says the file is not the whole list',
      (tester) async {
    await _pump(tester,
        adapter: _RecordingAdapter(truncated: true, rows: '50000'));

    await _choose(tester, t.export.csv);

    // Otherwise somebody reconciles the file against a dashboard and concludes
    // the dashboard is wrong.
    expect(find.text(t.export.truncated(rows: '50000')), findsOneWidget);
  });

  test('the server names the file', () {
    final name = filenameFrom({
      'content-disposition': ['attachment; filename=leads_2026-07-30.xlsx'],
    });

    // A downloads folder with four files called "export.csv" is a downloads
    // folder nobody can use.
    expect(name, 'leads_2026-07-30.xlsx');
  });

  test('a missing or malformed disposition falls back rather than throwing',
      () {
    expect(filenameFrom(const {}), isNull);
    expect(
        filenameFrom({
          'content-disposition': const ['attachment']
        }),
        isNull);
    expect(
      filenameFrom({
        'content-disposition': const ['attachment; filename=""']
      }),
      isNull,
    );
  });
}
