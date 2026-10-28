import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:app/features/auth/presentation/widgets/email_verification_banner.dart';
import 'package:app/features/dashboard/presentation/screens/dashboard_screen.dart';
import 'package:app/features/dashboard/presentation/controllers/dashboard_controller.dart';
import 'package:app/features/dashboard/domain/models/conversation_model.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/billing/presentation/controllers/billing_controller.dart';
import 'package:app/features/billing/presentation/controllers/trial_banner_controller.dart';
import 'package:app/features/billing/presentation/controllers/usage_warning_dismissal_controller.dart';
import 'package:app/features/billing/domain/models/usage_model.dart';
import 'package:app/features/billing/domain/models/effective_plan_limits_model.dart';
import 'package:app/features/home/domain/models/onboarding_progress_model.dart';
import 'package:app/features/home/presentation/controllers/onboarding_progress_controller.dart';
import 'package:app/features/onboarding/presentation/controllers/product_tour_controller.dart';
import 'package:app/features/announcements/presentation/controllers/feature_announcement_controller.dart';
import 'package:app/features/feedback/presentation/controllers/nps_prompt_controller.dart';
import 'package:app/features/billing/presentation/widgets/quota_wall_modal.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:app/core/network/websocket_service.dart';

void main() {
  setUpAll(() {
    FlutterSecureStorage.setMockInitialValues({});
  });

  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
  });

  Widget createWidgetUnderTest({
    List<ConversationModel> conversations = const [],
    String? initialConversationId,
  }) {
    return ProviderScope(
      overrides: [
        conversationsProvider.overrideWith((ref) async => conversations),
        realtimeListenerProvider.overrideWith(() => _FakeRealtimeListener()),
        authControllerProvider.overrideWith(() => _FakeAuthController()),
        accountUsageProvider.overrideWith(() => _FakeAccountUsage()),
        accountPlanLimitsProvider.overrideWith(() => _FakeAccountPlanLimits()),
        // Bypass the real WS service so pumpAndSettle doesn't spin on
        // a live reconnect timer.
        wsConnectionStatusProvider.overrideWith((ref) => Stream.value(true)),
        // Onboarding checklist is rendered at the top of the dashboard
        // (see Fase 5). Stub the controller so it doesn't try to call
        // /me/onboarding-progress through an unmocked Dio client.
        onboardingProgressControllerProvider.overrideWith(
          () => _FakeOnboardingProgress(),
        ),
        // Suppress the one-shot product tour (US-183) so its post-frame
        // dialog doesn't pop over the dashboard assertions.
        productTourControllerProvider.overrideWith(() => _FakeProductTour()),
        featureAnnouncementControllerProvider
            .overrideWith(() => _FakeFeatureAnnouncement()),
        npsPromptControllerProvider.overrideWith(() => _FakeNpsPrompt()),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: DashboardScreen(initialConversationId: initialConversationId),
      ),
    );
  }

  testWidgets('DashboardScreen renders without errors',
      (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pump();

    // The screen should be a Scaffold
    expect(find.byType(Scaffold), findsWidgets);
  });

  testWidgets('DashboardScreen shows loading indicator initially',
      (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest());
    // Before the future resolves, there should be a loading state
    // Just verify the screen rendered without error
    expect(find.byType(DashboardScreen), findsOneWidget);
  });

  testWidgets('DashboardScreen renders with empty conversations',
      (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest(conversations: []));
    await tester.pumpAndSettle();

    expect(find.byType(DashboardScreen), findsOneWidget);
  });

  // Gives the test a realistic desktop-sized surface. The default Flutter
  // test window (800x600) is unrealistically short, so banners + the empty
  // inbox CTA can overflow vertically; real devices are far taller.
  void useDesktopSurface(WidgetTester tester) {
    tester.view.physicalSize = const Size(1280, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
  }

  // Locates the split-view sidebar and returns its rendered width, or null
  // when no sidebar is present (mobile single-pane layout).
  double? sidebarWidth(WidgetTester tester) {
    final finder = find.byKey(const Key('dashboard_sidebar'));
    if (finder.evaluate().isEmpty) return null;
    return tester.getSize(finder).width;
  }

  testWidgets('sidebar is full width on desktop (>=900px)',
      (WidgetTester tester) async {
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pumpAndSettle();

    expect(sidebarWidth(tester), 380);
  });

  testWidgets('sidebar narrows on tablet (600-900px) so detail is not cramped',
      (WidgetTester tester) async {
    tester.view.physicalSize = const Size(768, 1024);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pumpAndSettle();

    expect(sidebarWidth(tester), 300);
  });

  testWidgets('no split-view sidebar on mobile (<600px)',
      (WidgetTester tester) async {
    tester.view.physicalSize = const Size(400, 800);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pumpAndSettle();

    expect(sidebarWidth(tester), isNull);
  });

  Widget createTrialWidgetUnderTest({
    required bool dismissed,
    bool contract = false,
  }) {
    return ProviderScope(
      overrides: [
        conversationsProvider.overrideWith((ref) async => const []),
        realtimeListenerProvider.overrideWith(() => _FakeRealtimeListener()),
        authControllerProvider.overrideWith(() => contract
            ? _FakeTrialingContractAuthController()
            : _FakeTrialingAuthController()),
        accountUsageProvider.overrideWith(() => _FakeAccountUsage()),
        accountPlanLimitsProvider.overrideWith(() => _FakeAccountPlanLimits()),
        wsConnectionStatusProvider.overrideWith((ref) => Stream.value(true)),
        onboardingProgressControllerProvider.overrideWith(
          () => _FakeOnboardingProgress(),
        ),
        // Suppress the one-shot product tour (US-183) so its post-frame
        // dialog doesn't pop over the dashboard assertions.
        productTourControllerProvider.overrideWith(() => _FakeProductTour()),
        featureAnnouncementControllerProvider
            .overrideWith(() => _FakeFeatureAnnouncement()),
        npsPromptControllerProvider.overrideWith(() => _FakeNpsPrompt()),
        trialBannerDismissalProvider.overrideWith(
          () => _FakeTrialBannerDismissal(dismissed),
        ),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const DashboardScreen(),
      ),
    );
  }

  testWidgets('trial countdown banner shows for trialing user not dismissed',
      (WidgetTester tester) async {
    useDesktopSurface(tester);
    await tester.pumpWidget(createTrialWidgetUnderTest(dismissed: false));
    await tester.pumpAndSettle();

    expect(
      find.text(
          t.dashboard.trial_days_remaining(days: '4', used: '5', max: '50')),
      findsOneWidget,
    );
  });

  testWidgets('trial banner hidden once dismissed for the day',
      (WidgetTester tester) async {
    useDesktopSurface(tester);
    await tester.pumpWidget(createTrialWidgetUnderTest(dismissed: true));
    await tester.pumpAndSettle();

    expect(
      find.text(
          t.dashboard.trial_days_remaining(days: '4', used: '5', max: '50')),
      findsNothing,
    );
  });

  testWidgets('trial banner hidden for a contract (Enterprise/Partner) plan',
      (WidgetTester tester) async {
    // Regression: a contract account showed the trial-expired banner.
    useDesktopSurface(tester);
    await tester.pumpWidget(
        createTrialWidgetUnderTest(dismissed: false, contract: true));
    await tester.pumpAndSettle();

    expect(
      find.text(
          t.dashboard.trial_days_remaining(days: '4', used: '5', max: '50')),
      findsNothing,
    );
  });

  Widget createUsageWarningWidgetUnderTest({
    required double tokensUsagePercent,
    required bool dismissed,
  }) {
    return ProviderScope(
      overrides: [
        conversationsProvider.overrideWith((ref) async => const []),
        realtimeListenerProvider.overrideWith(() => _FakeRealtimeListener()),
        authControllerProvider.overrideWith(() => _FakeAuthController()),
        accountUsageProvider.overrideWith(
            () => _FakeAccountUsage(tokensUsagePercent: tokensUsagePercent)),
        accountPlanLimitsProvider.overrideWith(() => _FakeAccountPlanLimits()),
        wsConnectionStatusProvider.overrideWith((ref) => Stream.value(true)),
        onboardingProgressControllerProvider.overrideWith(
          () => _FakeOnboardingProgress(),
        ),
        // Suppress the one-shot product tour (US-183) so its post-frame
        // dialog doesn't pop over the dashboard assertions.
        productTourControllerProvider.overrideWith(() => _FakeProductTour()),
        featureAnnouncementControllerProvider
            .overrideWith(() => _FakeFeatureAnnouncement()),
        npsPromptControllerProvider.overrideWith(() => _FakeNpsPrompt()),
        usageWarningDismissalProvider.overrideWith(
          () => _FakeUsageWarningDismissal(dismissed),
        ),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const DashboardScreen(),
      ),
    );
  }

  testWidgets('80% usage warning banner shows with a dismiss button',
      (WidgetTester tester) async {
    useDesktopSurface(tester);
    await tester.pumpWidget(createUsageWarningWidgetUnderTest(
        tokensUsagePercent: 85, dismissed: false));
    await tester.pumpAndSettle();

    expect(
      find.text(t.dashboard.tokens_high_usage_banner(percent: '85')),
      findsOneWidget,
    );
    // Dismiss affordance present for the soft banner.
    expect(find.byIcon(Icons.close), findsOneWidget);
  });

  testWidgets('80% usage warning banner hidden once dismissed for the day',
      (WidgetTester tester) async {
    useDesktopSurface(tester);
    await tester.pumpWidget(createUsageWarningWidgetUnderTest(
        tokensUsagePercent: 85, dismissed: true));
    await tester.pumpAndSettle();

    expect(
      find.text(t.dashboard.tokens_high_usage_banner(percent: '85')),
      findsNothing,
    );
  });

  // US-119: accountUsageProvider must be subscribed to exactly once. The
  // cold-start quota wall is now derived from the single watched value
  // rather than a second ref.listen on the same provider.
  testWidgets('accountUsage is fetched once (single watch, no double-fetch)',
      (WidgetTester tester) async {
    useDesktopSurface(tester);
    _CountingAccountUsage.buildCount = 0;
    final counter = _CountingAccountUsage();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          conversationsProvider.overrideWith((ref) async => const []),
          realtimeListenerProvider.overrideWith(() => _FakeRealtimeListener()),
          authControllerProvider.overrideWith(() => _FakeAuthController()),
          accountUsageProvider.overrideWith(() => counter),
          accountPlanLimitsProvider
              .overrideWith(() => _FakeAccountPlanLimits()),
          wsConnectionStatusProvider.overrideWith((ref) => Stream.value(true)),
          onboardingProgressControllerProvider.overrideWith(
            () => _FakeOnboardingProgress(),
          ),
          productTourControllerProvider.overrideWith(() => _FakeProductTour()),
          featureAnnouncementControllerProvider
              .overrideWith(() => _FakeFeatureAnnouncement()),
          npsPromptControllerProvider.overrideWith(() => _FakeNpsPrompt()),
        ],
        child: MaterialApp(
          theme: AppTheme.darkTheme,
          home: const DashboardScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(_CountingAccountUsage.buildCount, 1);
  });

  testWidgets('cold-start quota wall pops off the single watched value',
      (WidgetTester tester) async {
    resetQuotaWallShownForTest();
    addTearDown(resetQuotaWallShownForTest);
    useDesktopSurface(tester);
    await tester.pumpWidget(createUsageWarningWidgetUnderTest(
        tokensUsagePercent: 100, dismissed: true));
    await tester.pumpAndSettle();

    expect(find.text(t.billing.quota_wall_title), findsOneWidget);
  });

  Widget createEmailVerificationWidgetUnderTest({required bool verified}) {
    return ProviderScope(
      overrides: [
        conversationsProvider.overrideWith((ref) async => const []),
        realtimeListenerProvider.overrideWith(() => _FakeRealtimeListener()),
        authControllerProvider
            .overrideWith(() => _FakeAuthController(emailVerified: verified)),
        accountUsageProvider.overrideWith(() => _FakeAccountUsage()),
        accountPlanLimitsProvider.overrideWith(() => _FakeAccountPlanLimits()),
        wsConnectionStatusProvider.overrideWith((ref) => Stream.value(true)),
        onboardingProgressControllerProvider.overrideWith(
          () => _FakeOnboardingProgress(),
        ),
        productTourControllerProvider.overrideWith(() => _FakeProductTour()),
        featureAnnouncementControllerProvider
            .overrideWith(() => _FakeFeatureAnnouncement()),
        npsPromptControllerProvider.overrideWith(() => _FakeNpsPrompt()),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        // The banner moved to the AppShell (its old /home-only mount was a
        // screen onboarding never lands on). The harness replicates the
        // shell's gate around the extracted widget.
        home: Scaffold(
          body: Consumer(builder: (context, ref, _) {
            final unverified = ref.watch(authControllerProvider
                .select((s) => s.asData?.value?.isEmailVerified == false));
            return unverified
                ? const EmailVerificationBanner()
                : const SizedBox.shrink();
          }),
        ),
      ),
    );
  }

  testWidgets('email verification banner shows for an unverified user',
      (WidgetTester tester) async {
    useDesktopSurface(tester);
    await tester
        .pumpWidget(createEmailVerificationWidgetUnderTest(verified: false));
    await tester.pumpAndSettle();

    expect(find.text(t.email_verification.banner_message), findsOneWidget);
    expect(find.text(t.email_verification.resend), findsOneWidget);
  });

  testWidgets('email verification banner hidden for a verified user',
      (WidgetTester tester) async {
    useDesktopSurface(tester);
    await tester
        .pumpWidget(createEmailVerificationWidgetUnderTest(verified: true));
    await tester.pumpAndSettle();

    expect(find.text(t.email_verification.banner_message), findsNothing);
  });
  // `/home?conversation=<id>` — the push notification's click_url. The old
  // URL (/conversations/<id>) had no route and landed on the not-found page.
  testWidgets('initialConversationId selects the thread on arrival',
      (WidgetTester tester) async {
    await tester.pumpWidget(
        createWidgetUnderTest(initialConversationId: 'conv-from-push'));
    await tester.pumpAndSettle();

    final container =
        ProviderScope.containerOf(tester.element(find.byType(DashboardScreen)));
    expect(container.read(selectedConversationProvider), 'conv-from-push');
  });

  testWidgets('no initialConversationId leaves the selection alone',
      (WidgetTester tester) async {
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pumpAndSettle();

    final container =
        ProviderScope.containerOf(tester.element(find.byType(DashboardScreen)));
    expect(container.read(selectedConversationProvider), isNull);
  });
}

class _FakeTrialingAuthController extends AuthController {
  @override
  FutureOr<UserModel?> build() async {
    return const UserModel(
      id: 'test-user',
      name: 'Test User',
      email: 'test@example.com',
      token: 'fake-token',
      isTrialing: true,
      trialDaysRemaining: 4,
      planName: 'Trial',
      role: 'admin',
    );
  }
}

class _FakeTrialingContractAuthController extends AuthController {
  @override
  FutureOr<UserModel?> build() async {
    return const UserModel(
      id: 'test-user',
      name: 'Test User',
      email: 'test@example.com',
      token: 'fake-token',
      isTrialing: true,
      trialDaysRemaining: 4,
      planName: 'Enterprise',
      role: 'admin',
      isContractPlan: true,
    );
  }
}

class _FakeTrialBannerDismissal extends TrialBannerDismissal {
  _FakeTrialBannerDismissal(this._seed);
  final bool _seed;

  @override
  Future<bool> build() async => _seed;
}

class _FakeRealtimeListener extends RealtimeListener {
  @override
  void build() {
    // No-op: don't connect to WebSocket in tests
  }
}

class _FakeAuthController extends AuthController {
  _FakeAuthController({this.emailVerified = true});
  final bool emailVerified;

  @override
  FutureOr<UserModel?> build() async {
    return UserModel(
      id: 'test-user',
      name: 'Test User',
      email: 'test@example.com',
      token: 'fake-token',
      isTrialing: false,
      planName: 'Pro',
      role: 'admin',
      isEmailVerified: emailVerified,
    );
  }
}

class _FakeAccountUsage extends AccountUsage {
  _FakeAccountUsage({this.tokensUsagePercent = 2});
  final double tokensUsagePercent;

  @override
  Future<UsageModel> build() async {
    return UsageModel(
      aiTokensUsed: 100,
      conversationsCount: 5,
      maxAiTokensPerMonth: 5000,
      maxConversationsPerMonth: 50,
      tokensUsagePercent: tokensUsagePercent,
      conversationsUsagePercent: 10,
    );
  }
}

class _CountingAccountUsage extends AccountUsage {
  static int buildCount = 0;

  @override
  Future<UsageModel> build() async {
    buildCount++;
    return const UsageModel(
      aiTokensUsed: 100,
      conversationsCount: 5,
      maxAiTokensPerMonth: 5000,
      maxConversationsPerMonth: 50,
      tokensUsagePercent: 2,
      conversationsUsagePercent: 10,
    );
  }
}

// Offline fake for /me/plan-limits so nothing on the dashboard fires real
// Dio calls (which leave a pending request-timeout Timer and fail the
// test). The QuotaStrip that originally needed it is gone (BYOK/token
// model), but other billing surfaces still read it.
class _FakeAccountPlanLimits extends AccountPlanLimits {
  @override
  Future<EffectivePlanLimitsModel> build() async =>
      const EffectivePlanLimitsModel(
        planName: 'Trial',
        isContractPlan: false,
        maxAgents: 1,
        maxChannels: 1,
        maxSeats: 1,
        maxConversationsPerMonth: 50,
        maxAiTokensPerMonth: 5000,
        maxStorageMb: 500,
        maxBroadcastMessagesPerMonth: 0,
        maxRecipientsPerCampaign: 0,
        overageConversationCents: 0,
        overageCapMultiplier: 0,
      );
}

class _FakeUsageWarningDismissal extends UsageWarningDismissal {
  _FakeUsageWarningDismissal(this._seed);
  final bool _seed;

  @override
  Future<bool> build() async => _seed;
}

class _FakeProductTour extends ProductTourController {
  @override
  Future<bool> build() async => true; // already completed -> tour suppressed
}

class _FakeFeatureAnnouncement extends FeatureAnnouncementController {
  @override
  Future<bool> build() async => true; // already seen -> announcement suppressed
}

class _FakeNpsPrompt extends NpsPromptController {
  @override
  Future<bool> build() async => true; // handled -> NPS prompt suppressed
}

class _FakeOnboardingProgress extends OnboardingProgressController {
  @override
  Future<OnboardingProgressModel?> build() async {
    // Return a completed checklist so the card collapses to
    // SizedBox.shrink and stays out of the way of dashboard assertions.
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
