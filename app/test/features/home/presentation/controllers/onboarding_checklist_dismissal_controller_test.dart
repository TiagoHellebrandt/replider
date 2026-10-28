import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/features/home/presentation/controllers/onboarding_checklist_dismissal_controller.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  ProviderContainer makeContainer() {
    final c = ProviderContainer();
    addTearDown(c.dispose);
    return c;
  }

  test('build returns false when nothing stored', () async {
    FlutterSecureStorage.setMockInitialValues({});
    final container = makeContainer();
    final value =
        await container.read(onboardingChecklistDismissalProvider.future);
    expect(value, isFalse);
  });

  test('build returns true within the 24h window', () async {
    final recent =
        DateTime.now().subtract(const Duration(hours: 1)).toIso8601String();
    FlutterSecureStorage.setMockInitialValues(
        {'onboarding_checklist_dismissed_at': recent});
    final container = makeContainer();
    final value =
        await container.read(onboardingChecklistDismissalProvider.future);
    expect(value, isTrue);
  });

  test('build returns false once the 24h window has elapsed', () async {
    final old =
        DateTime.now().subtract(const Duration(hours: 25)).toIso8601String();
    FlutterSecureStorage.setMockInitialValues(
        {'onboarding_checklist_dismissed_at': old});
    final container = makeContainer();
    final value =
        await container.read(onboardingChecklistDismissalProvider.future);
    expect(value, isFalse);
  });

  test('build returns false on an unparseable stored value', () async {
    FlutterSecureStorage.setMockInitialValues(
        {'onboarding_checklist_dismissed_at': 'not-a-date'});
    final container = makeContainer();
    final value =
        await container.read(onboardingChecklistDismissalProvider.future);
    expect(value, isFalse);
  });

  test('dismiss auto-returns to false once the window elapses', () async {
    OnboardingChecklistDismissal.window = const Duration(milliseconds: 40);
    addTearDown(
        () => OnboardingChecklistDismissal.window = const Duration(hours: 24));
    FlutterSecureStorage.setMockInitialValues({});
    final container = makeContainer();
    await container.read(onboardingChecklistDismissalProvider.future);

    await container
        .read(onboardingChecklistDismissalProvider.notifier)
        .dismiss();
    expect(
      container.read(onboardingChecklistDismissalProvider).asData?.value,
      isTrue,
    );

    // The keepAlive provider would otherwise stay dismissed for the whole
    // session; the scheduled timer must flip it back so the card returns.
    await Future<void>.delayed(const Duration(milliseconds: 80));
    expect(
      container.read(onboardingChecklistDismissalProvider).asData?.value,
      isFalse,
    );
  });

  test('build auto-returns after the remaining window for a fresh dismissal',
      () async {
    OnboardingChecklistDismissal.window = const Duration(milliseconds: 60);
    addTearDown(
        () => OnboardingChecklistDismissal.window = const Duration(hours: 24));
    final recent = DateTime.now()
        .subtract(const Duration(milliseconds: 20))
        .toIso8601String();
    FlutterSecureStorage.setMockInitialValues(
        {'onboarding_checklist_dismissed_at': recent});
    final container = makeContainer();
    final initial =
        await container.read(onboardingChecklistDismissalProvider.future);
    expect(initial, isTrue);

    await Future<void>.delayed(const Duration(milliseconds: 80));
    expect(
      container.read(onboardingChecklistDismissalProvider).asData?.value,
      isFalse,
    );
  });

  test('dismiss sets state true and persists a timestamp', () async {
    FlutterSecureStorage.setMockInitialValues({});
    final container = makeContainer();
    await container.read(onboardingChecklistDismissalProvider.future);

    await container
        .read(onboardingChecklistDismissalProvider.notifier)
        .dismiss();

    expect(
      container.read(onboardingChecklistDismissalProvider).asData?.value,
      isTrue,
    );

    // A fresh container reading the same mock storage stays dismissed.
    final reload = makeContainer();
    final value =
        await reload.read(onboardingChecklistDismissalProvider.future);
    expect(value, isTrue);
  });
}
