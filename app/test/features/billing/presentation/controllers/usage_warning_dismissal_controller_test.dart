import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:app/features/billing/presentation/controllers/usage_warning_dismissal_controller.dart';

void main() {
  setUp(() {
    FlutterSecureStorage.setMockInitialValues({});
  });

  test('build returns false when nothing has been dismissed', () async {
    final container = ProviderContainer();
    addTearDown(container.dispose);

    final dismissed =
        await container.read(usageWarningDismissalProvider.future);

    expect(dismissed, isFalse);
  });

  test('dismissForToday flips state to true', () async {
    final container = ProviderContainer();
    addTearDown(container.dispose);

    await container.read(usageWarningDismissalProvider.future);
    await container
        .read(usageWarningDismissalProvider.notifier)
        .dismissForToday();

    expect(container.read(usageWarningDismissalProvider).value, isTrue);
  });

  test('dismissal persists across provider containers (same day)', () async {
    final first = ProviderContainer();
    await first.read(usageWarningDismissalProvider.future);
    await first.read(usageWarningDismissalProvider.notifier).dismissForToday();
    first.dispose();

    // A fresh container re-reads from storage and must still see it dismissed.
    final second = ProviderContainer();
    addTearDown(second.dispose);
    final dismissed = await second.read(usageWarningDismissalProvider.future);

    expect(dismissed, isTrue);
  });

  test('a stale (different-day) stored date does not dismiss', () async {
    FlutterSecureStorage.setMockInitialValues(
      {'usage_warning_dismissed_date': '2000-01-01'},
    );
    final container = ProviderContainer();
    addTearDown(container.dispose);

    final dismissed =
        await container.read(usageWarningDismissalProvider.future);

    expect(dismissed, isFalse);
  });
}
