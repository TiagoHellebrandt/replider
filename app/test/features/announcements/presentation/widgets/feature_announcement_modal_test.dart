import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/features/announcements/presentation/controllers/feature_announcement_controller.dart';
import 'package:app/features/announcements/presentation/widgets/feature_announcement_modal.dart';
import 'package:app/i18n/strings.g.dart';

void main() {
  setUpAll(() {
    FlutterSecureStorage.setMockInitialValues({});
  });

  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
    // An older stored release so the controller resolves to "show".
    FlutterSecureStorage.setMockInitialValues(
      {'feature_announcement_seen_release': '2020-01'},
    );
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
                onPressed: () => showFeatureAnnouncementModal(context),
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

  testWidgets('renders the title and the what\'s-new items', (tester) async {
    final container = makeContainer();
    await container.read(featureAnnouncementControllerProvider.future);

    await tester.pumpWidget(harness(container));
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    expect(find.text(t.feature_announcement.title), findsOneWidget);
    expect(find.text(t.feature_announcement.item1_title), findsOneWidget);
    expect(find.text(t.feature_announcement.item2_title), findsOneWidget);
    expect(find.text(t.feature_announcement.item3_title), findsOneWidget);
    expect(find.text(t.feature_announcement.cta), findsOneWidget);
  });

  testWidgets('the CTA marks the release seen and closes the modal',
      (tester) async {
    final container = makeContainer();
    await container.read(featureAnnouncementControllerProvider.future);
    expect(
        container.read(featureAnnouncementControllerProvider).value, isFalse);

    await tester.pumpWidget(harness(container));
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    await tester.tap(find.text(t.feature_announcement.cta));
    await tester.pumpAndSettle();

    expect(find.text(t.feature_announcement.item1_title), findsNothing);
    expect(container.read(featureAnnouncementControllerProvider).value, isTrue);
  });

  testWidgets('the close icon also dismisses and stamps the release',
      (tester) async {
    final container = makeContainer();
    await container.read(featureAnnouncementControllerProvider.future);

    await tester.pumpWidget(harness(container));
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    await tester.tap(find.byIcon(Icons.close));
    await tester.pumpAndSettle();

    expect(find.text(t.feature_announcement.title), findsNothing);
    expect(container.read(featureAnnouncementControllerProvider).value, isTrue);
  });
}
