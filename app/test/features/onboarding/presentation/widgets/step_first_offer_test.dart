import 'dart:async';

import 'package:app/design_system/app_theme.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/features/onboarding/presentation/controllers/first_offer_controller.dart';
import 'package:app/features/onboarding/presentation/controllers/onboarding_controller.dart';
import 'package:app/features/onboarding/presentation/widgets/step_first_offer.dart';
import 'package:app/features/sales/domain/models/lead_model.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:flutter/material.dart';
import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

// The last step, and the reason the wizard exists: one real, approvable
// message, read before onboarding is over. These pin the four things the
// step can show and that it never claims more than the server said.

class _Fixed extends FirstOfferController {
  _Fixed(this.value);
  final FirstOffer value;
  @override
  Future<FirstOffer> build() async => value;
}

class _Failing extends FirstOfferController {
  int builds = 0;
  @override
  Future<FirstOffer> build() async {
    builds++;
    if (builds == 1) throw Exception('provider down');
    return const FirstOffer(status: 'ready', draft: 'segunda tentativa');
  }
}

/// The server refused with a named code — the shape the API answers with.
class _Refusing extends FirstOfferController {
  _Refusing(this.code);
  final String code;
  @override
  Future<FirstOffer> build() async => throw DioException(
        requestOptions: RequestOptions(path: '/me/onboarding/first-offer'),
        response: Response<dynamic>(
          requestOptions: RequestOptions(path: '/me/onboarding/first-offer'),
          statusCode: 400,
          data: <String, dynamic>{'code': code, 'message': 'refused'},
        ),
      );
}

class _MockDio extends Mock implements Dio {}

class _Hanging extends FirstOfferController {
  // A future that never completes and leaves no timer behind.
  @override
  Future<FirstOffer> build() => Completer<FirstOffer>().future;
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  Future<void> pump(
      WidgetTester tester, FirstOfferController controller) async {
    await tester.pumpWidget(ProviderScope(
      // No automatic retry: the error state has to be observable, and the
      // Retry button is what re-asks.
      retry: (_, __) => null,
      overrides: [
        firstOfferControllerProvider.overrideWith(() => controller),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const Scaffold(body: StepFirstOffer()),
      ),
    ));
    await tester.pump();
  }

  testWidgets('shows the draft, who it is for, and that it is in the queue',
      (tester) async {
    await pump(
      tester,
      _Fixed(FirstOffer(
        status: 'ready',
        source: 'customer',
        draft: 'Bom dia, Hospital Santa Rosa. Vi que…',
        lead: const LeadModel(
          id: 'l1',
          contactId: '5565999',
          enrichment: {'company_name': 'Hospital Santa Rosa'},
        ),
      )),
    );
    await tester.pumpAndSettle();

    expect(find.text(t.onboarding.first_offer_title), findsOneWidget);
    expect(find.byKey(const Key('first_offer_card')), findsOneWidget);
    expect(find.textContaining('Bom dia, Hospital Santa Rosa'), findsOneWidget);
    expect(
      find.text(
          t.onboarding.first_offer_for_customer(name: 'Hospital Santa Rosa')),
      findsOneWidget,
    );
    expect(find.text(t.onboarding.first_offer_in_queue), findsOneWidget);
    // Nothing here sends: no approve button on this screen.
    expect(find.text(t.common.confirm), findsNothing);
  });

  testWidgets('a prospect is introduced as found, not as a customer',
      (tester) async {
    await pump(
      tester,
      _Fixed(FirstOffer(
        status: 'ready',
        source: 'prospect',
        draft: 'Olá',
        lead: const LeadModel(id: 'l1', contactId: 'Hospital Geral'),
      )),
    );
    await tester.pumpAndSettle();

    expect(
      find.text(t.onboarding.first_offer_for_prospect(name: 'Hospital Geral')),
      findsOneWidget,
    );
  });

  testWidgets('nothing to offer says what to bring', (tester) async {
    await pump(
      tester,
      _Fixed(const FirstOffer(
          status: 'nothing_to_offer', reason: 'no_customers_no_territory')),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('first_offer_nothing')), findsOneWidget);
    // no_customers_no_territory is a PLATFORM gap (no Places prospector):
    // the copy must not re-ask for the territory the wizard already made
    // mandatory two steps back.
    expect(find.text(t.onboarding.first_offer_unavailable), findsOneWidget);
    expect(find.byKey(const Key('first_offer_card')), findsNothing);
  });

  testWidgets('a search that found nobody says so, differently',
      (tester) async {
    await pump(
      tester,
      _Fixed(const FirstOffer(
          status: 'nothing_to_offer', reason: 'no_business_found')),
    );
    await tester.pumpAndSettle();

    expect(find.text(t.onboarding.first_offer_no_business), findsOneWidget);
  });

  testWidgets('while writing, it says so', (tester) async {
    await pump(tester, _Hanging());

    expect(find.byKey(const Key('first_offer_writing')), findsOneWidget);
    expect(find.text(t.onboarding.first_offer_writing), findsOneWidget);
  });

  // The server is idempotent, so a retry is safe — and it must be offered:
  // a provider hiccup on the last step must not be the person's memory of
  // the product.
  testWidgets('a failure offers a retry that asks again', (tester) async {
    await pump(tester, _Failing());
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('first_offer_error')), findsOneWidget);

    await tester.tap(find.text(t.common.retry));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('first_offer_card')), findsOneWidget);
    expect(find.textContaining('segunda tentativa'), findsOneWidget);
  });

  // The refusal cards. Going back to a step reads the onboarding controller,
  // which hydrates through Dio on first read — stubbed so nothing leaks.
  Future<ProviderContainer> pumpRefusal(
      WidgetTester tester, String code) async {
    final dio = _MockDio();
    when(() => dio.get<dynamic>(
          any(),
          queryParameters: any(named: 'queryParameters'),
          options: any(named: 'options'),
          cancelToken: any(named: 'cancelToken'),
          onReceiveProgress: any(named: 'onReceiveProgress'),
        )).thenThrow(DioException(requestOptions: RequestOptions(path: '/')));
    when(() => dio.put<dynamic>(
          any(),
          data: any(named: 'data'),
          queryParameters: any(named: 'queryParameters'),
          options: any(named: 'options'),
          cancelToken: any(named: 'cancelToken'),
          onSendProgress: any(named: 'onSendProgress'),
          onReceiveProgress: any(named: 'onReceiveProgress'),
        )).thenThrow(DioException(requestOptions: RequestOptions(path: '/')));
    await tester.pumpWidget(ProviderScope(
      retry: (_, __) => null,
      overrides: [
        dioProvider.overrideWithValue(dio),
        firstOfferControllerProvider.overrideWith(() => _Refusing(code)),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const Scaffold(body: StepFirstOffer()),
      ),
    ));
    await tester.pumpAndSettle();
    final container =
        ProviderScope.containerOf(tester.element(find.byType(StepFirstOffer)));
    // The wizard screen watches the controller; this bare step does not,
    // and an autoDispose provider nobody listens to is rebuilt (step 0) on
    // the read-back. Hold it the way the screen would.
    final sub = container.listen(onboardingControllerProvider, (_, __) {});
    addTearDown(sub.close);
    return container;
  }

  testWidgets('an empty catalogue offers the way back to Products',
      (tester) async {
    final container = await pumpRefusal(tester, 'APPROACH_NEEDS_PRODUCTS');
    expect(find.byKey(const Key('first_offer_error')), findsOneWidget);
    expect(find.text(t.errors.APPROACH_NEEDS_PRODUCTS), findsOneWidget);

    await tester.tap(find.byKey(const Key('first_offer_cta_products')));
    await tester.pump();
    expect(container.read(onboardingControllerProvider).currentStep,
        kOnboardingProductsStep);
    await tester.pump(const Duration(milliseconds: 900));
  });

  testWidgets('a customer with no address offers the way back to Customers',
      (tester) async {
    final container = await pumpRefusal(tester, 'LEAD_CONTACT_NOT_MESSAGEABLE');
    expect(find.text(t.errors.LEAD_CONTACT_NOT_MESSAGEABLE), findsOneWidget);

    await tester.tap(find.byKey(const Key('first_offer_cta_customers')));
    await tester.pump();
    expect(container.read(onboardingControllerProvider).currentStep,
        kOnboardingCustomersStep);
    await tester.pump(const Duration(milliseconds: 900));
  });

  testWidgets('a missing AI key names Settings', (tester) async {
    await pumpRefusal(tester, 'BYOK_NOT_CONFIGURED');
    expect(find.text(t.errors.BYOK_NOT_CONFIGURED), findsOneWidget);
    expect(find.byKey(const Key('first_offer_cta_settings')), findsOneWidget);
    expect(find.text(t.onboarding.first_offer_cta_settings), findsOneWidget);
    // Retry stays available whatever the code.
    expect(find.text(t.common.retry), findsOneWidget);
  });

  testWidgets('an unknown refusal keeps the plain retry', (tester) async {
    await pumpRefusal(tester, 'SOMETHING_ELSE');
    expect(find.byKey(const Key('first_offer_error')), findsOneWidget);
    expect(find.byKey(const Key('first_offer_cta_products')), findsNothing);
    expect(find.byKey(const Key('first_offer_cta_customers')), findsNothing);
    expect(find.byKey(const Key('first_offer_cta_settings')), findsNothing);
  });

  // The platform has no prospecting provider at all: an operator gap, said
  // as such — not "no business found" for a search that never ran.
  testWidgets('an unconfigured prospector is named as the platform\'s gap',
      (tester) async {
    await pump(
      tester,
      _Fixed(const FirstOffer(
          status: 'nothing_to_offer', reason: 'prospecting_not_configured')),
    );
    await tester.pumpAndSettle();

    expect(find.text(t.onboarding.first_offer_prospecting_not_configured),
        findsOneWidget);
    expect(
        find.byKey(const Key('first_offer_back_to_customers')), findsOneWidget);
  });
}
