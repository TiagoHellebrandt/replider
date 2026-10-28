import 'package:app/design_system/app_theme.dart';
import 'package:app/features/billing/domain/models/plan_model.dart';
import 'package:app/features/billing/presentation/widgets/talk_to_us_card.dart';
import 'package:app/features/billing/presentation/widgets/upgrade_plan_modal.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

// Every "upgrade" entry point opens this sheet. Since 2026-08-19 it shows no
// plans: the product is sales-led, and the sheet offers the conversation —
// the booking page when one is configured, WhatsApp and e-mail always.

const _channels = ContactSalesChannels(
  email: 'support@replider.com',
  whatsapp: '+55 11 90000-0000',
  whatsappLink: 'https://wa.me/5511900000000',
  whatsappText: 'Olá',
  bookingUrl: 'https://calendar.app.google/abc',
);

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.pt));

  Widget harness(ContactSalesChannels channels) {
    return ProviderScope(
      overrides: [
        contactSalesProvider.overrideWith((ref) async => channels),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: Scaffold(
          body: Builder(
            builder: (context) => Center(
              child: ElevatedButton(
                onPressed: () => showUpgradePlanModal(context),
                child: const Text('open'),
              ),
            ),
          ),
        ),
      ),
    );
  }

  Future<void> openModal(WidgetTester tester,
      {ContactSalesChannels channels = _channels}) async {
    await tester.pumpWidget(harness(channels));
    await tester.pumpAndSettle();
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
  }

  testWidgets('offers the conversation, not a price list', (tester) async {
    await openModal(tester);

    expect(find.byKey(const Key('upgrade_plan_modal')), findsOneWidget);
    expect(find.byKey(const Key('talk_to_us_card')), findsOneWidget);
    expect(find.text(t.billing.talk_title), findsOneWidget);
    // Nothing priced, nothing to subscribe to.
    expect(find.textContaining('R\$'), findsNothing);
    expect(find.text(t.common.subscribe), findsNothing);
  });

  testWidgets('with a booking page, booking leads; WhatsApp and e-mail follow',
      (tester) async {
    await openModal(tester);

    expect(find.byKey(const Key('talk_to_us_book')), findsOneWidget);
    expect(find.text(t.billing.talk_book_cta), findsOneWidget);
    expect(find.byKey(const Key('talk_to_us_whatsapp')), findsOneWidget);
    expect(find.byKey(const Key('talk_to_us_email')), findsOneWidget);
  });

  testWidgets('without a booking page there is no booking button',
      (tester) async {
    await openModal(
      tester,
      channels: const ContactSalesChannels(
        email: 'support@replider.com',
        whatsapp: '+55 11 90000-0000',
        whatsappLink: 'https://wa.me/5511900000000',
        whatsappText: 'Olá',
      ),
    );

    expect(find.byKey(const Key('talk_to_us_book')), findsNothing);
    expect(find.byKey(const Key('talk_to_us_whatsapp')), findsOneWidget);
  });

  testWidgets('the sheet closes', (tester) async {
    await openModal(tester);
    await tester.tap(find.text(t.common.close));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('upgrade_plan_modal')), findsNothing);
  });
}
