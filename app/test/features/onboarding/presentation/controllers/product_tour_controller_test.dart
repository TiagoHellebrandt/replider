import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/features/onboarding/presentation/controllers/product_tour_controller.dart';

void main() {
  setUp(() {
    FlutterSecureStorage.setMockInitialValues({});
  });

  test('build returns false when the tour has never been shown', () async {
    final container = ProviderContainer();
    addTearDown(container.dispose);

    final shown = await container.read(productTourControllerProvider.future);
    expect(shown, isFalse);
  });

  test('complete stamps the flag and persists across containers', () async {
    final first = ProviderContainer();
    await first.read(productTourControllerProvider.future);
    await first.read(productTourControllerProvider.notifier).complete();
    expect(first.read(productTourControllerProvider).value, isTrue);
    first.dispose();

    // A fresh container re-reads storage and must still see it completed.
    final second = ProviderContainer();
    addTearDown(second.dispose);
    expect(await second.read(productTourControllerProvider.future), isTrue);
  });

  test('a stored "true" flag suppresses the tour', () async {
    FlutterSecureStorage.setMockInitialValues(
      {'product_tour_completed': 'true'},
    );
    final container = ProviderContainer();
    addTearDown(container.dispose);

    expect(await container.read(productTourControllerProvider.future), isTrue);
  });
}
