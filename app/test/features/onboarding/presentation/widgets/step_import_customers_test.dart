import 'package:app/core/network/dio_client.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/onboarding/presentation/widgets/step_import_customers.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

// The third step of the shipped flow (2026-08-18): the customers she already
// has. These pin the promise the step makes — who already buys from you, as
// a file — and that nothing is claimed before a file was read.

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  Future<void> pump(WidgetTester tester) async {
    await tester.pumpWidget(ProviderScope(
      overrides: [dioProvider.overrideWithValue(Dio())],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const Scaffold(body: StepImportCustomers()),
      ),
    ));
    await tester.pumpAndSettle();
  }

  testWidgets('asks who already buys from her, and how to bring them',
      (tester) async {
    await pump(tester);

    expect(find.text(t.onboarding.customers_title), findsOneWidget);
    expect(find.text(t.onboarding.customers_subtitle), findsOneWidget);
    expect(find.byKey(const Key('onboarding_customers_pick')), findsOneWidget);
    // What the file has to look like, before she goes looking for it.
    expect(find.text(t.customer_import.what_we_read), findsOneWidget);
  });

  testWidgets('shows no review until a file has been read', (tester) async {
    await pump(tester);

    expect(find.byKey(const Key('customer_import_confirm')), findsNothing);
    expect(find.byKey(const Key('onboarding_customers_result')), findsNothing);
  });
}
