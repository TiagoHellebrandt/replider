import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:app/features/billing/presentation/controllers/trial_banner_controller.dart';

void main() {
  setUp(() {
    FlutterSecureStorage.setMockInitialValues({});
  });

  test('build returns false when nothing has been dismissed', () async {
    final container = ProviderContainer();
    addTearDown(container.dispose);

    final dismissed = await container.read(trialBannerDismissalProvider.future);

    expect(dismissed, isFalse);
  });

  test('dismissForToday flips state to true', () async {
    final container = ProviderContainer();
    addTearDown(container.dispose);

    await container.read(trialBannerDismissalProvider.future);
    await container
        .read(trialBannerDismissalProvider.notifier)
        .dismissForToday();

    expect(container.read(trialBannerDismissalProvider).value, isTrue);
  });

  test('dismissal persists across provider containers (same day)', () async {
    final first = ProviderContainer();
    await first.read(trialBannerDismissalProvider.future);
    await first.read(trialBannerDismissalProvider.notifier).dismissForToday();
    first.dispose();

    // A fresh container re-reads from storage and must still see it dismissed.
    final second = ProviderContainer();
    addTearDown(second.dispose);
    final dismissed = await second.read(trialBannerDismissalProvider.future);

    expect(dismissed, isTrue);
  });

  test('a stale (different-day) stored date does not dismiss', () async {
    FlutterSecureStorage.setMockInitialValues(
      {'trial_banner_dismissed_date': '2000-01-01'},
    );
    final container = ProviderContainer();
    addTearDown(container.dispose);

    final dismissed = await container.read(trialBannerDismissalProvider.future);

    expect(dismissed, isFalse);
  });
}
