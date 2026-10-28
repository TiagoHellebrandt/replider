import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:app/core/network/websocket_service.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/dashboard/presentation/controllers/dashboard_controller.dart';
import 'package:app/features/dashboard/presentation/widgets/conversation_list_panel.dart';
import 'package:app/features/home/domain/models/onboarding_progress_model.dart';
import 'package:app/features/home/presentation/controllers/onboarding_progress_controller.dart';
import 'package:app/i18n/strings.g.dart';

// Status, channel, order and category sat permanently open above the list:
// four stacked selectors, ~90px fixed in a 380px sidebar, over the list that is
// the reason for the screen. In a 900px window that left a little over three
// conversations.
//
// Hiding for the sake of hiding would be worse: a short list caused by a
// forgotten filter is indistinguishable from an empty inbox. Hence the button
// COUNTS.
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    LocaleSettings.setLocale(AppLocale.pt);
    FlutterSecureStorage.setMockInitialValues({});
  });

  Widget panel() => ProviderScope(
        overrides: [
          authControllerProvider.overrideWith(() => _FakeAuth()),
          conversationsProvider.overrideWith((ref) async => const []),
          wsEventsProvider.overrideWith((ref) => const Stream<WSEvent>.empty()),
          onboardingProgressControllerProvider
              .overrideWith(_FakeOnboardingProgress.new),
        ],
        child: MaterialApp(
          theme: AppTheme.darkTheme,
          home: const Scaffold(
            // The dashboard sidebar's real width.
            body: SizedBox(width: 380, child: ConversationListPanel()),
          ),
        ),
      );

  testWidgets('the four slices do not occupy the bar by default',
      (tester) async {
    await tester.pumpWidget(panel());
    await tester.pumpAndSettle();

    // The scope stays: it is the triage used all the time.
    expect(find.text(t.dashboard.assignment_unassigned), findsOneWidget);
    // The rest does not.
    expect(find.byKey(const Key('inbox_status_filter')), findsNothing);
    expect(find.byKey(const Key('inbox_channel_filter')), findsNothing);
    expect(find.byKey(const Key('inbox_sort_filter')), findsNothing);
    expect(find.byKey(const Key('inbox_category_filter')), findsNothing);

    expect(find.byKey(const Key('inbox_filters_button')), findsOneWidget);
    expect(find.text(t.dashboard.filters_button), findsOneWidget);
  });

  testWidgets('the dialog brings the four, each with its label',
      (tester) async {
    await tester.pumpWidget(panel());
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('inbox_filters_button')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('inbox_filters_sheet')), findsOneWidget);
    for (final key in [
      'inbox_status_filter',
      'inbox_channel_filter',
      'inbox_sort_filter',
      'inbox_category_filter',
    ]) {
      expect(find.byKey(Key(key)), findsOneWidget, reason: key);
    }
    // Loose in the bar they were four unnamed boxes — "All", "All", "Most
    // recent", "All" — and finding out which one moved what cost a click
    // each.
    expect(find.text(t.dashboard.filter_label_status), findsOneWidget);
    expect(find.text(t.dashboard.filter_label_channel), findsOneWidget);
    expect(find.text(t.dashboard.filter_label_sort), findsOneWidget);
    expect(find.text(t.dashboard.filter_label_category), findsOneWidget);
  });

  // The number is what separates "empty inbox" from "forgotten filter".
  // Without it, hiding the slices would trade density for confusion.
  testWidgets('the button counts how many slices are on', (tester) async {
    await tester.pumpWidget(panel());
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('inbox_filters_clear')), findsNothing);

    await tester.tap(find.byKey(const Key('inbox_filters_button')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('inbox_status_filter')));
    await tester.pumpAndSettle();
    await tester.tap(find.text(t.dashboard.filter_open).last);
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('inbox_filters_sheet_close')));
    await tester.pumpAndSettle();

    expect(find.text(t.dashboard.filters_button_n(count: 1)), findsOneWidget);
    expect(find.byKey(const Key('inbox_filters_clear')), findsOneWidget);
  });

  testWidgets('clearing returns the button to its no-slice state',
      (tester) async {
    await tester.pumpWidget(panel());
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('inbox_filters_button')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('inbox_category_filter')));
    await tester.pumpAndSettle();
    await tester.tap(find.text(t.dashboard.category_hot_lead).last);
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('inbox_filters_sheet_close')));
    await tester.pumpAndSettle();
    expect(find.text(t.dashboard.filters_button_n(count: 1)), findsOneWidget);

    await tester.tap(find.byKey(const Key('inbox_filters_clear')));
    await tester.pumpAndSettle();

    expect(find.text(t.dashboard.filters_button), findsOneWidget);
    expect(find.byKey(const Key('inbox_filters_clear')), findsNothing);
  });
}

class _FakeAuth extends AuthController {
  @override
  FutureOr<UserModel?> build() async => const UserModel(
        id: '01HZXK5T9GXW2M3N4P5Q6R7S8T',
        name: 'Test User',
        email: 'user@example.com',
        token: 'jwt',
        role: 'admin',
      );
}

class _FakeOnboardingProgress extends OnboardingProgressController {
  @override
  Future<OnboardingProgressModel?> build() async =>
      const OnboardingProgressModel(
        agentCreated: true,
        previewChatTested: true,
        channelConnected: true,
        knowledgeAdded: true,
        firstInboundReceived: true,
        onboardingCompleted: true,
        completionPercentage: 100,
      );
}
