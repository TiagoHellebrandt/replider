import 'dart:convert';
import 'dart:typed_data';

import 'package:app/design_system/app_theme.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/features/billing/presentation/widgets/ai_cost_card.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

class _FixedAdapter implements HttpClientAdapter {
  _FixedAdapter(this.body);
  final Map<String, dynamic> body;

  @override
  Future<ResponseBody> fetch(RequestOptions options,
      Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async {
    return ResponseBody.fromString(jsonEncode(body), 200, headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    });
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  Widget harness(Map<String, dynamic> report) {
    final dio = Dio(BaseOptions(baseUrl: 'http://x'))
      ..httpClientAdapter = _FixedAdapter(report);
    return ProviderScope(
      overrides: [dioProvider.overrideWithValue(dio)],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const Scaffold(body: AICostCard()),
      ),
    );
  }

  testWidgets('shows cycle spend, pace and the ceiling with percentage',
      (tester) async {
    await tester.pumpWidget(harness({
      'cycle': {'total_brl_cents': 4210},
      'projected_monthly_brl_cents': 9000,
      'budget_cents': 10000,
      'budget_used_pct': 42,
    }));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('ai_cost_card')), findsOneWidget);
    expect(find.text('R\$ 42,10'), findsOneWidget);
    expect(find.text('R\$ 90,00'), findsOneWidget);
    expect(find.text('R\$ 100,00 (42%)'), findsOneWidget);
  });

  testWidgets('no history renders a dash, never a fake zero-cost promise',
      (tester) async {
    await tester.pumpWidget(harness({
      'cycle': {'total_brl_cents': 0},
      'projected_monthly_brl_cents': 0,
    }));
    await tester.pumpAndSettle();

    expect(find.text('—'), findsOneWidget);
    expect(find.text(t.billing.ai_cost_no_budget), findsOneWidget);
  });
}
