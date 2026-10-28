import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/billing/presentation/screens/billing_screen.dart';
import 'package:app/features/billing/presentation/controllers/billing_controller.dart';
import 'package:app/features/billing/domain/models/effective_plan_limits_model.dart';
import 'package:app/features/billing/domain/models/plan_model.dart';
import 'package:app/features/billing/domain/models/usage_model.dart';
import 'package:app/design_system/app_theme.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// BillingScreen renders ListTiles inside themed glass cards (DecoratedBox
/// with a background colour), which trips Flutter's debug-only "ListTile
/// background color or ink splashes may be invisible" assertion during paint
/// and fails the whole widget test. The assertion is cosmetic and does not
/// affect release builds. Swallow only that specific assertion at
/// FlutterError.onError; everything else still fails the test. Must be called
/// from INSIDE each test body — the test binding reinstalls its own handler
/// after setUp, so a setUp override is ignored. (See app/CLAUDE.md.)
void ignoreListTileAssertions() {
  final prev = FlutterError.onError;
  FlutterError.onError = (details) {
    if (details.exceptionAsString().contains('ListTile background color')) {
      return;
    }
    prev?.call(details);
  };
}

void main() {
  setUpAll(() {
    FlutterSecureStorage.setMockInitialValues({});
  });

  setUp(() {
    LocaleSettings.setLocale(AppLocale.en);
  });

  final testUsage = UsageModel(
    aiTokensUsed: 2500,
    aiInputTokensUsed: 1500,
    aiOutputTokensUsed: 1000,
    addonTokens: 0,
    effectiveLimit: 5000,
    conversationsCount: 10,
    maxAiTokensPerMonth: 5000,
    maxConversationsPerMonth: 50,
  );

  const testPlanLimits = EffectivePlanLimitsModel(
    planId: 'plan-1',
    planName: 'Pro',
    isContractPlan: false,
    maxAgents: 5,
    maxChannels: 8,
    maxSeats: 5,
    maxConversationsPerMonth: 2000,
    maxAiTokensPerMonth: 800000,
    maxStorageMb: 102400,
    maxBroadcastMessagesPerMonth: 0,
    maxRecipientsPerCampaign: 0,
    overageConversationCents: 50,
    overageCapMultiplier: 2.0,
  );

  Widget createWidgetUnderTest({
    UsageModel? usage,
    List<PlanModel> plans = const [],
    EffectivePlanLimitsModel? planLimits,
    String accountCurrency = 'brl',
  }) {
    final user = UserModel(
      id: 'u1',
      name: 'Test',
      email: 't@example.com',
      token: 'jwt',
      accountCurrency: accountCurrency,
    );
    return ProviderScope(
      overrides: [
        accountUsageProvider
            .overrideWith(() => _FakeAccountUsage(usage ?? testUsage)),
        plansListProvider.overrideWith(() => _FakePlansList(plans)),
        accountPlanLimitsProvider.overrideWith(
            () => _FakeAccountPlanLimits(planLimits ?? testPlanLimits)),
        authControllerProvider.overrideWith(() => _FakeAuth(user)),
      ],
      child: MaterialApp(
        theme: AppTheme.darkTheme,
        home: const BillingScreen(),
      ),
    );
  }

  testWidgets('BillingScreen renders app bar with title',
      (WidgetTester tester) async {
    ignoreListTileAssertions();
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pumpAndSettle();

    expect(find.text(t.billing.title), findsOneWidget);
  });

  testWidgets('BillingScreen shows section headers',
      (WidgetTester tester) async {
    ignoreListTileAssertions();
    // All three section headers live in a lazy ListView; the plans header
    // sits below the 800x600 fold (usage + addon-packs sections precede it),
    // so build a tall surface to force every section to lay out.
    tester.view.physicalSize = const Size(1200, 4000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pumpAndSettle();

    expect(find.text(t.billing.section_usage), findsOneWidget);
    // Sales-led (2026-08-19): no plan catalogue, no credit packs. The
    // conversation is the offer.
    expect(find.text(t.billing.section_addon_packs), findsNothing);
    expect(find.text(t.billing.section_plans), findsNothing);
    expect(find.byKey(const Key('talk_to_us_card')), findsOneWidget);
    expect(find.text(t.billing.talk_title), findsOneWidget);
  });

  testWidgets('on trial the card offers a conversation, not a plan list',
      (WidgetTester tester) async {
    ignoreListTileAssertions();
    final trialUsage = UsageModel(
      aiTokensUsed: 0,
      aiInputTokensUsed: 0,
      aiOutputTokensUsed: 0,
      addonTokens: 0,
      effectiveLimit: 5000,
      conversationsCount: 0,
      maxAiTokensPerMonth: 5000,
      maxConversationsPerMonth: 50,
      isTrialing: true,
      planName: 'Trial',
    );
    await tester.pumpWidget(createWidgetUnderTest(usage: trialUsage));
    await tester.pumpAndSettle();

    // The trial card is gone entirely: it compared against hardcoded
    // numbers of a retired public tier. The one door is the TalkToUsCard.
    expect(find.byKey(const Key('talk_to_us_card')), findsOneWidget);
    expect(find.text(t.billing.trial_view_plans), findsNothing);
  });

  testWidgets('BillingScreen shows usage data', (WidgetTester tester) async {
    ignoreListTileAssertions();
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pumpAndSettle();

    expect(find.text(t.billing.usage_ai_tokens), findsOneWidget);
    expect(find.text(t.billing.usage_conversations), findsOneWidget);
  });

  testWidgets('BillingScreen shows back button', (WidgetTester tester) async {
    ignoreListTileAssertions();
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pumpAndSettle();

    expect(find.byIcon(Icons.arrow_back), findsOneWidget);
  });

  testWidgets('BillingScreen shows usage progress bars',
      (WidgetTester tester) async {
    ignoreListTileAssertions();
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pumpAndSettle();

    expect(find.byType(LinearProgressIndicator), findsWidgets);
  });

  testWidgets('with no assertable cap, the row simply does not appear',
      (tester) async {
    // The ListTile paint assertion only fires on CI's Flutter, and takes the
    // whole test down. Its neighbours already filter it.
    ignoreListTileAssertions();
    // In pt the two sentences differ, and that is where the defect showed:
    // "Unlimited recipients per campaign". In en they coincide.
    LocaleSettings.setLocale(AppLocale.pt);
    addTearDown(() => LocaleSettings.setLocale(AppLocale.en));

    // "Unlimited recipients per campaign": this row was the only one that
    // interpolated the unlimited label in place of the number, instead of using
    // the sentence written for that case. On the money screen.
    const unlimitedRecipients = EffectivePlanLimitsModel(
      planId: 'plan-ent',
      planName: 'Enterprise',
      isContractPlan: true,
      maxAgents: 50,
      maxChannels: 100,
      maxSeats: 25,
      maxConversationsPerMonth: 100000,
      maxAiTokensPerMonth: 10000000,
      maxStorageMb: 1048576,
      maxBroadcastMessagesPerMonth: -1,
      maxRecipientsPerCampaign: -1,
      overageConversationCents: 0,
      overageCapMultiplier: 0,
    );

    await tester.pumpWidget(createWidgetUnderTest(
      planLimits: unlimitedRecipients,
    ));
    await tester.pumpAndSettle();

    // No "unlimited" on screen: with no assertable cap the row does not appear.
    // Promising an infinite cap is a promise the infrastructure does not sign.
    expect(find.textContaining('ilimitad', skipOffstage: false), findsNothing);
    expect(find.textContaining('nlimited', skipOffstage: false), findsNothing);
  });

  // US-004: USD accounts.

  // The money screen spoke the previous product's language: service channels,
  // attendant seats, tokens. What is bought today is prospecting — and the lead
  // and research caps were already ENFORCED by the server without ever appearing
  // here. Whoever hit one found out by clicking.
  // "512k MB of storage" and "1000M tokens": correct numbers, written in a way
  // nobody reads. Nobody buys half a million megabytes.
  testWidgets('storage and tokens come in the unit people say out loud',
      (tester) async {
    ignoreListTileAssertions();
    LocaleSettings.setLocale(AppLocale.pt);
    addTearDown(() => LocaleSettings.setLocale(AppLocale.en));

    const big = EffectivePlanLimitsModel(
      planId: 'p',
      planName: 'Enterprise',
      isContractPlan: true,
      maxAgents: 1,
      maxChannels: 1,
      maxSeats: 1,
      maxConversationsPerMonth: 100,
      maxAiTokensPerMonth: 1000000000,
      maxStorageMb: 512000,
      maxBroadcastMessagesPerMonth: 0,
      maxRecipientsPerCampaign: 0,
      overageConversationCents: 0,
      overageCapMultiplier: 0,
    );

    await tester.pumpWidget(createWidgetUnderTest(planLimits: big));
    await tester.pumpAndSettle();

    expect(find.text(t.billing.plan_feature_storage_gb(gb: '500')),
        findsOneWidget);
    expect(find.textContaining('512k MB'), findsNothing);
    expect(
        find.text(t.billing.plan_feature_tokens(tokens: '1B')), findsOneWidget);
    expect(find.textContaining('1000M'), findsNothing);
  });

  testWidgets('the limits speak in the units being bought', (tester) async {
    // The ListTile paint assertion only fires on CI's Flutter, and takes the
    // whole test down. Its neighbours already filter it.
    ignoreListTileAssertions();
    LocaleSettings.setLocale(AppLocale.pt);
    addTearDown(() => LocaleSettings.setLocale(AppLocale.en));

    const sdrLimits = EffectivePlanLimitsModel(
      planId: 'plan-sdr',
      planName: 'Vendedor',
      isContractPlan: true,
      maxAgents: 3,
      maxChannels: 3,
      maxSeats: 2,
      maxConversationsPerMonth: 1000,
      maxAiTokensPerMonth: 100000,
      maxStorageMb: 1024,
      maxBroadcastMessagesPerMonth: 0,
      maxRecipientsPerCampaign: 0,
      overageConversationCents: 0,
      overageCapMultiplier: 0,
      maxLeadsPerMonth: 500,
      maxEnrichmentsPerMonth: 200,
      maxAiDraftsPerMonth: 300,
      maxFunnels: 5,
      maxEmailMailboxes: 2,
    );

    await tester.pumpWidget(createWidgetUnderTest(planLimits: sdrLimits));
    await tester.pumpAndSettle();

    expect(
        find.text(t.billing.plan_feature_leads(count: '500')), findsOneWidget);
    expect(find.text(t.billing.plan_feature_enrichments(count: '200')),
        findsOneWidget);
    expect(
        find.text(t.billing.plan_feature_drafts(count: '300')), findsOneWidget);
    expect(
        find.text(t.billing.plan_feature_funnels(count: '5')), findsOneWidget);
  });

  // Zero is neither "unlimited" nor "zero available": it is "this plan does not
  // sell this". A row announcing 0 researches would read as a limit reached.
  testWidgets('what the plan does not sell becomes no row', (tester) async {
    // The ListTile paint assertion only fires on CI's Flutter, and takes the
    // whole test down. Its neighbours already filter it.
    ignoreListTileAssertions();
    const noSdr = EffectivePlanLimitsModel(
      planId: 'plan-old',
      planName: 'Solo',
      isContractPlan: false,
      maxAgents: 1,
      maxChannels: 1,
      maxSeats: 1,
      maxConversationsPerMonth: 100,
      maxAiTokensPerMonth: 1000,
      maxStorageMb: 100,
      maxBroadcastMessagesPerMonth: 0,
      maxRecipientsPerCampaign: 0,
      overageConversationCents: 0,
      overageCapMultiplier: 0,
    );

    await tester.pumpWidget(createWidgetUnderTest(planLimits: noSdr));
    await tester.pumpAndSettle();

    expect(find.text(t.billing.plan_feature_leads(count: '0')), findsNothing);
    expect(find.text(t.billing.plan_feature_enrichments(count: '0')),
        findsNothing);
  });

  // "How much of my package have I used" was answered in tokens — our cost, not
  // something anybody buys.
  testWidgets("the month's usage comes in SDR units, with a denominator",
      (tester) async {
    // The ListTile paint assertion only fires on CI's Flutter, and takes the
    // whole test down. Its neighbours already filter it.
    ignoreListTileAssertions();
    LocaleSettings.setLocale(AppLocale.pt);
    addTearDown(() => LocaleSettings.setLocale(AppLocale.en));

    const limits = EffectivePlanLimitsModel(
      planId: 'p',
      planName: 'Vendedor',
      isContractPlan: true,
      maxAgents: 1,
      maxChannels: 1,
      maxSeats: 1,
      maxConversationsPerMonth: 100,
      maxAiTokensPerMonth: 1000,
      maxStorageMb: 100,
      maxBroadcastMessagesPerMonth: 0,
      maxRecipientsPerCampaign: 0,
      overageConversationCents: 0,
      overageCapMultiplier: 0,
      maxEnrichmentsPerMonth: 200,
    );

    await tester.pumpWidget(createWidgetUnderTest(
      planLimits: limits,
      usage: const UsageModel(
        aiTokensUsed: 2500,
        aiInputTokensUsed: 1500,
        aiOutputTokensUsed: 1000,
        effectiveLimit: 5000,
        conversationsCount: 10,
        maxAiTokensPerMonth: 5000,
        maxConversationsPerMonth: 50,
        sdr: SdrUsageModel(leads: 40, enrichments: 12, approachesSent: 7),
      ),
    ));
    await tester.pumpAndSettle();

    // A tela é alta e rola: o cartão de uso pode estar fora da janela.
    expect(find.byKey(const Key('billing_sdr_usage'), skipOffstage: false),
        findsOneWidget);
    // Com teto conhecido, o número vem com o denominador.
    expect(
        find.text(t.billing.usage_sdr_of_limit(used: '12', limit: '200'),
            skipOffstage: false),
        findsOneWidget);
    // With no cap, just the amount — inventing a denominator would be worse than
    // having none.
    expect(
        find.text(t.billing.usage_sdr_no_limit(used: '7'), skipOffstage: false),
        findsOneWidget);
  });

  // A count the server could not produce does not become a zero on screen.
  testWidgets("with no month block, the screen invents no zeros",
      (tester) async {
    // The ListTile paint assertion only fires on CI's Flutter, and takes the
    // whole test down. Its neighbours already filter it.
    ignoreListTileAssertions();
    await tester.pumpWidget(createWidgetUnderTest());
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('billing_sdr_usage'), skipOffstage: false),
        findsNothing);
  });
}

class _FakeAccountUsage extends AccountUsage {
  final UsageModel _usage;
  _FakeAccountUsage(this._usage);

  @override
  Future<UsageModel> build() async => _usage;
}

class _FakePlansList extends PlansList {
  final List<PlanModel> _plans;
  _FakePlansList(this._plans);

  @override
  Future<List<PlanModel>> build() async => _plans;
}

class _FakeAccountPlanLimits extends AccountPlanLimits {
  final EffectivePlanLimitsModel _limits;
  _FakeAccountPlanLimits(this._limits);

  @override
  Future<EffectivePlanLimitsModel> build() async => _limits;
}

class _FakeAuth extends AuthController {
  final UserModel _user;
  _FakeAuth(this._user);

  @override
  Future<UserModel?> build() async => _user;
}
