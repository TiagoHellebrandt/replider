import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:app/core/network/websocket_service.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/dashboard/presentation/controllers/dashboard_controller.dart';
import 'package:app/features/dashboard/presentation/controllers/inbox_assignment_filter_controller.dart';
import 'package:app/features/dashboard/presentation/widgets/conversation_list_panel.dart';
import 'package:app/features/home/domain/models/onboarding_progress_model.dart';
import 'package:app/features/home/presentation/controllers/onboarding_progress_controller.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/i18n/strings.g.dart';

// US-017 + US-020 — inbox scope "Minhas / Não atribuídas / Todas".
// Storage is mocked empty so the provider falls through to the default,
// which is gated to the PLATFORM account: only a platform-account
// attendant lands on "Minhas"; customer-account teams keep "Todas".
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
    FlutterSecureStorage.setMockInitialValues({});
  });

  UserModel user({required String role, bool platform = false}) => UserModel(
        id: '01HZXK5T9GXW2M3N4P5Q6R7S8T',
        name: 'Test User',
        email: 'user@example.com',
        token: 'jwt',
        role: role,
        isPlatformAccount: platform,
      );

  Widget createPanel({required String role, bool platform = false}) {
    return ProviderScope(
      overrides: [
        authControllerProvider.overrideWith(
          () => _FakeAuthController(user(role: role, platform: platform)),
        ),
        conversationsProvider.overrideWith((ref) async => const []),
        wsEventsProvider.overrideWith((ref) => const Stream<WSEvent>.empty()),
        onboardingProgressControllerProvider.overrideWith(
          _FakeOnboardingProgress.new,
        ),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const Scaffold(body: ConversationListPanel()),
      ),
    );
  }

  Set<InboxAssignmentScope> selectedScope(WidgetTester tester) => tester
      .widget<SegmentedButton<InboxAssignmentScope>>(
        find.byType(SegmentedButton<InboxAssignmentScope>),
      )
      .selected;

  testWidgets('platform attendant defaults to "Mine"',
      (WidgetTester tester) async {
    await tester.pumpWidget(createPanel(role: 'attendant', platform: true));
    await tester.pumpAndSettle();

    expect(find.text(t.dashboard.assignment_mine), findsOneWidget);
    expect(selectedScope(tester), {InboxAssignmentScope.mine});
  });

  // US-020: an attendant of a CUSTOMER account must keep the historical
  // full view — the "Minhas" default was a platform-scoped feature that
  // silently changed every account. FAILS on the pre-US-020 code.
  testWidgets('customer-account attendant defaults to "All"',
      (WidgetTester tester) async {
    await tester.pumpWidget(createPanel(role: 'attendant'));
    await tester.pumpAndSettle();

    expect(selectedScope(tester), {InboxAssignmentScope.all});
  });

  testWidgets('admin defaults to "All"', (WidgetTester tester) async {
    await tester.pumpWidget(createPanel(role: 'admin', platform: true));
    await tester.pumpAndSettle();

    expect(selectedScope(tester), {InboxAssignmentScope.all});
  });

  // US-020: conversations nobody picked up must stay reachable — the
  // "Não atribuídas" bucket is a visible segment of the toggle.
  testWidgets('unassigned bucket is selectable', (WidgetTester tester) async {
    await tester.pumpWidget(createPanel(role: 'attendant', platform: true));
    await tester.pumpAndSettle();

    await tester.tap(find.descendant(
      of: find.byType(SegmentedButton<InboxAssignmentScope>),
      matching: find.text(t.dashboard.assignment_unassigned),
    ));
    await tester.pumpAndSettle();

    expect(selectedScope(tester), {InboxAssignmentScope.unassigned});
  });

  testWidgets('tapping "All" switches the attendant scope',
      (WidgetTester tester) async {
    await tester.pumpWidget(createPanel(role: 'attendant', platform: true));
    await tester.pumpAndSettle();

    // "All" also labels the status dropdown — scope the tap to the toggle.
    await tester.tap(find.descendant(
      of: find.byType(SegmentedButton<InboxAssignmentScope>),
      matching: find.text(t.dashboard.assignment_all),
    ));
    await tester.pumpAndSettle();

    expect(selectedScope(tester), {InboxAssignmentScope.all});
  });

  test('provider defaults: platform gates the attendant "mine" default',
      () async {
    for (final (role, platform, expected) in [
      ('attendant', true, InboxAssignmentScope.mine),
      ('attendant', false, InboxAssignmentScope.all),
      ('admin', true, InboxAssignmentScope.all),
      ('admin', false, InboxAssignmentScope.all),
    ]) {
      final container = ProviderContainer(overrides: [
        authControllerProvider.overrideWith(
          () => _FakeAuthController(user(role: role, platform: platform)),
        ),
      ]);
      addTearDown(container.dispose);
      expect(
        await container.read(inboxAssignmentFilterProvider.future),
        expected,
        reason: 'role=$role platform=$platform',
      );
    }
  });

  test('stored choice wins over the role default (legacy values included)',
      () async {
    for (final (stored, expected) in [
      ('mine', InboxAssignmentScope.mine),
      ('all', InboxAssignmentScope.all),
      ('unassigned', InboxAssignmentScope.unassigned),
    ]) {
      FlutterSecureStorage.setMockInitialValues({
        'inbox_assignment_filter_01HZXK5T9GXW2M3N4P5Q6R7S8T': stored,
      });
      final container = ProviderContainer(overrides: [
        authControllerProvider.overrideWith(
          () => _FakeAuthController(user(role: 'admin')),
        ),
      ]);
      addTearDown(container.dispose);
      expect(
        await container.read(inboxAssignmentFilterProvider.future),
        expected,
        reason: 'stored=$stored',
      );
    }
  });
}

class _FakeAuthController extends AuthController {
  final UserModel? _user;
  _FakeAuthController(this._user);

  @override
  FutureOr<UserModel?> build() async => _user;
}

class _FakeOnboardingProgress extends OnboardingProgressController {
  @override
  Future<OnboardingProgressModel?> build() async {
    return const OnboardingProgressModel(
      agentCreated: true,
      previewChatTested: true,
      channelConnected: true,
      knowledgeAdded: true,
      firstInboundReceived: true,
      onboardingCompleted: true,
      completionPercentage: 100,
    );
  }
}
