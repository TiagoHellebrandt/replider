import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/features/onboarding/presentation/controllers/product_tour_controller.dart';
import 'package:app/features/onboarding/presentation/widgets/product_tour_modal.dart';
import 'package:app/i18n/strings.g.dart';

void main() {
  setUpAll(() {
    FlutterSecureStorage.setMockInitialValues({});
  });

  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
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
                onPressed: () => showProductTourModal(context),
                child: const Text('open'),
              ),
            ),
          ),
        ),
      ),
    );
  }

  ProviderContainer makeContainer() {
    final c = ProviderContainer();
    addTearDown(c.dispose);
    return c;
  }

  testWidgets('opens on the first step with skip + next affordances',
      (tester) async {
    final container = makeContainer();
    await container.read(productTourControllerProvider.future);

    await tester.pumpWidget(harness(container));
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    expect(find.text(t.product_tour.step1_title), findsOneWidget);
    expect(find.text(t.product_tour.skip), findsOneWidget);
    expect(find.text(t.product_tour.next), findsOneWidget);
    // Not on the last step yet, so no "done".
    expect(find.text(t.product_tour.done), findsNothing);
  });

  testWidgets('stepping through to the end completes and closes the tour',
      (tester) async {
    final container = makeContainer();
    await container.read(productTourControllerProvider.future);

    await tester.pumpWidget(harness(container));
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    // Advance through every step until the final CTA flips to "done".
    for (var i = 0; i < 3; i++) {
      await tester.tap(find.text(t.product_tour.next));
      await tester.pumpAndSettle();
    }
    expect(find.text(t.product_tour.step4_title), findsOneWidget);

    await tester.tap(find.text(t.product_tour.done));
    await tester.pumpAndSettle();

    // Modal closed + flag stamped so it never replays.
    expect(find.text(t.product_tour.step4_title), findsNothing);
    expect(container.read(productTourControllerProvider).value, isTrue);
  });

  testWidgets('skip completes the tour without stepping through',
      (tester) async {
    final container = makeContainer();
    await container.read(productTourControllerProvider.future);

    await tester.pumpWidget(harness(container));
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    await tester.tap(find.text(t.product_tour.skip));
    await tester.pumpAndSettle();

    expect(find.text(t.product_tour.step1_title), findsNothing);
    expect(container.read(productTourControllerProvider).value, isTrue);
  });
}
