import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/features/announcements/presentation/controllers/feature_announcement_controller.dart';

void main() {
  setUp(() {
    FlutterSecureStorage.setMockInitialValues({});
  });

  test('a brand-new install is suppressed and silently stamped', () async {
    final container = ProviderContainer();
    await container.read(featureAnnouncementControllerProvider.future);
    // Nothing to announce to a first-time user -> hidden.
    expect(container.read(featureAnnouncementControllerProvider).value, isTrue);
    container.dispose();

    // The current release was stamped, so a fresh container stays suppressed
    // (it must NOT replay the modal to the same user on the next launch).
    final second = ProviderContainer();
    addTearDown(second.dispose);
    expect(
      await second.read(featureAnnouncementControllerProvider.future),
      isTrue,
    );
  });

  test('an older stored release surfaces the announcement', () async {
    FlutterSecureStorage.setMockInitialValues(
      {'feature_announcement_seen_release': '2020-01'},
    );
    final container = ProviderContainer();
    addTearDown(container.dispose);

    // stored != currentRelease -> the modal should show.
    expect(
      await container.read(featureAnnouncementControllerProvider.future),
      isFalse,
    );
  });

  test('the current release is suppressed', () async {
    FlutterSecureStorage.setMockInitialValues({
      'feature_announcement_seen_release':
          FeatureAnnouncementController.currentRelease,
    });
    final container = ProviderContainer();
    addTearDown(container.dispose);

    expect(
      await container.read(featureAnnouncementControllerProvider.future),
      isTrue,
    );
  });

  test('markSeen stamps the current release and persists', () async {
    FlutterSecureStorage.setMockInitialValues(
      {'feature_announcement_seen_release': '2020-01'},
    );
    final first = ProviderContainer();
    await first.read(featureAnnouncementControllerProvider.future);
    expect(first.read(featureAnnouncementControllerProvider).value, isFalse);

    await first.read(featureAnnouncementControllerProvider.notifier).markSeen();
    expect(first.read(featureAnnouncementControllerProvider).value, isTrue);
    first.dispose();

    final second = ProviderContainer();
    addTearDown(second.dispose);
    expect(
      await second.read(featureAnnouncementControllerProvider.future),
      isTrue,
    );
  });
}
