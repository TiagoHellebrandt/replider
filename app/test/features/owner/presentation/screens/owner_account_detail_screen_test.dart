import 'package:flutter/material.dart';
import 'dart:convert';
import 'dart:typed_data';
import 'package:dio/dio.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import 'package:app/design_system/app_theme.dart';
import 'package:app/features/owner/data/datasources/owner_remote_datasource.dart';
import 'package:app/features/owner/domain/models/owner_account_models.dart';
import 'package:app/features/owner/presentation/controllers/owner_controller.dart';
import 'package:app/features/owner/presentation/screens/owner_account_detail_screen.dart';
import 'package:app/i18n/strings.g.dart';

OwnerAccountDetail _detail({bool suspended = false, bool selfMatch = false}) {
  final acc = OwnerAccountSummary(
    id: 'acc-1',
    name: 'Acme Corp',
    subscriptionStatus: 'active',
    onboardingCompleted: true,
    csatEnabled: true,
    trialBypass: false,
    suspendedAt: suspended ? DateTime.utc(2026, 1, 1) : null,
    suspendedReason: suspended ? 'abuse' : null,
    createdAt: DateTime.utc(2026, 1, 1),
    updatedAt: DateTime.utc(2026, 1, 2),
  );
  return OwnerAccountDetail(
    account: acc,
    plan: OwnerPlanInfo(
      id: 'plan-1',
      name: 'Pro',
      maxAgents: 5,
      maxChannels: 3,
      maxConversationsPerMonth: 1000,
      maxAiTokensPerMonth: 500000,
    ),
    admin: OwnerAdminUserInfo(
      id: 'u-1',
      name: 'Admin',
      email: 'admin@acme.com',
      isEmailVerified: true,
    ),
    seller: selfMatch
        ? OwnerAdminUserInfo(
            id: 'u-seller',
            name: 'Admin',
            email: 'admin@acme.com',
            isEmailVerified: true,
          )
        : null,
    sellerSelfMatch: selfMatch,
  );
}

GoRouter _router() {
  return GoRouter(routes: [
    GoRoute(
      path: '/',
      builder: (_, __) => const OwnerAccountDetailScreen(accountId: 'acc-1'),
    ),
    GoRoute(path: '/owner/accounts', builder: (_, __) => const Scaffold()),
  ]);
}

Widget _harness({required OwnerAccountDetail seed}) {
  return ProviderScope(
    overrides: [
      ownerAccountDetailProvider('acc-1').overrideWith((ref) async => seed),
      ownerConversationsProvider('acc-1').overrideWith(
        (ref) async => OwnerConversationsResponse(items: const []),
      ),
      ownerUsageTrendProvider('acc-1').overrideWith(
        (ref) async => OwnerUsageTrendResponse(items: const []),
      ),
      ownerInvoicesProvider('acc-1').overrideWith(
        (ref) async => OwnerInvoicesResponse(items: const []),
      ),
    ],
    child: MaterialApp.router(
      theme: AppTheme.darkTheme,
      routerConfig: _router(),
    ),
  );
}

void _tallViewport(WidgetTester tester) {
  tester.view.physicalSize = const Size(1200, 4000);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
}

class _PlansAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(RequestOptions options,
      Stream<Uint8List>? requestStream, Future<void>? cancelFuture) async {
    if (options.path.contains('/owner/plans')) {
      return ResponseBody.fromString(
        jsonEncode([
          {
            'id': '01PLAN_ENTERPRISE_00000   ',
            'name': 'Enterprise',
            'price': 0,
            'currency': 'BRL',
            'ai_provider': 'google',
            'ai_model': 'gemini',
            'input_multiplier': 1.5,
            'output_multiplier': 1.5,
            'max_agents': 100,
            'max_channels': 200,
            'max_conversations_per_month': 200000,
            'max_ai_tokens_per_month': 50000000,
            'max_storage_mb': 1,
            'stripe_price_id': null,
            'created_at': '2026-01-01T00:00:00Z',
            'updated_at': '2026-01-01T00:00:00Z',
          },
        ]),
        200,
        headers: {
          Headers.contentTypeHeader: [Headers.jsonContentType],
        },
      );
    }
    return ResponseBody.fromString('{}', 404);
  }

  @override
  void close({bool force = false}) {}
}

Dio _plansDio() {
  final dio = Dio(BaseOptions(baseUrl: 'https://api.test/v1'));
  dio.httpClientAdapter = _PlansAdapter();
  return dio;
}

void main() {
  setUpAll(() => FlutterSecureStorage.setMockInitialValues({}));
  setUp(() => LocaleSettings.setLocale(AppLocale.en));

  group('OwnerAccountDetailScreen', () {
    testWidgets('hero header renders name, email, plan and status badge',
        (tester) async {
      await tester.pumpWidget(_harness(seed: _detail()));
      await tester.pumpAndSettle();

      expect(find.text(t.owner.detail_title), findsWidgets);
      // Name appears only in the hero.
      expect(find.text('Acme Corp'), findsOneWidget);
      // Email/plan appear in the hero AND the overview key/value list.
      expect(find.text('admin@acme.com'), findsWidgets);
      expect(find.text('Pro'), findsWidgets);
      // Status badge is the uppercased subscription status.
      expect(find.text('ACTIVE'), findsOneWidget);
    });

    testWidgets('renders the four tab labels', (tester) async {
      await tester.pumpWidget(_harness(seed: _detail()));
      await tester.pumpAndSettle();

      expect(find.text(t.owner.tab_overview), findsOneWidget);
      expect(find.text(t.owner.tab_usage), findsOneWidget);
      expect(find.text(t.owner.tab_invoices), findsOneWidget);
      expect(find.text(t.owner.tab_conversations), findsOneWidget);
    });

    testWidgets('overview tab shows Suspend button when account is active',
        (tester) async {
      _tallViewport(tester);
      await tester.pumpWidget(_harness(seed: _detail()));
      await tester.pumpAndSettle();

      expect(find.text(t.owner.suspend_cta), findsOneWidget);
    });

    testWidgets('suspended account shows Reactivate, not Suspend, on overview',
        (tester) async {
      _tallViewport(tester);
      await tester.pumpWidget(_harness(seed: _detail(suspended: true)));
      await tester.pumpAndSettle();

      expect(find.text(t.owner.reactivate_cta), findsOneWidget);
      expect(find.text(t.owner.suspend_cta), findsNothing);
    });

    testWidgets(
        'seller self-match flag renders the warning in the seller section (US-013)',
        (tester) async {
      _tallViewport(tester);
      await tester.pumpWidget(_harness(seed: _detail(selfMatch: true)));
      await tester.pumpAndSettle();

      expect(find.text(t.owner.seller_self_match_warning), findsOneWidget);
    });

    testWidgets('no self-match warning when seller and admin differ',
        (tester) async {
      _tallViewport(tester);
      await tester.pumpWidget(_harness(seed: _detail()));
      await tester.pumpAndSettle();

      expect(find.text(t.owner.seller_self_match_warning), findsNothing);
    });

    testWidgets('actions menu exposes grant / extend / impersonate',
        (tester) async {
      await tester.pumpWidget(_harness(seed: _detail()));
      await tester.pumpAndSettle();

      await tester.tap(find.byIcon(Icons.more_vert));
      await tester.pumpAndSettle();

      expect(find.text(t.owner.grant_cta), findsOneWidget);
      expect(find.text(t.owner.extend_cta), findsOneWidget);
      expect(find.text(t.owner.impersonate_cta), findsOneWidget);
      // The new "set plan / contract" (Enterprise/Partner) action.
      expect(find.text(t.owner.apply_plan_cta), findsOneWidget);
    });

    testWidgets('conversations tab renders title + empty state',
        (tester) async {
      await tester.pumpWidget(_harness(seed: _detail()));
      await tester.pumpAndSettle();

      await tester.tap(find.text(t.owner.tab_conversations));
      await tester.pumpAndSettle();

      expect(find.text(t.owner.conversations_label), findsOneWidget);
      expect(find.text(t.owner.conversations_empty), findsOneWidget);
    });

    testWidgets(
        'conversations tab renders sms and instagram channel types (US-014)',
        (tester) async {
      OwnerConversationRow row(String id, String type, String contact) =>
          OwnerConversationRow(
            id: id,
            channelId: 'ch-$type',
            contactName: contact,
            status: 'open',
            isBotPaused: false,
            channelType: type,
            messageCount: 3,
            createdAt: DateTime.utc(2026, 7, 1),
          );
      await tester.pumpWidget(ProviderScope(
        overrides: [
          ownerAccountDetailProvider('acc-1')
              .overrideWith((ref) async => _detail()),
          ownerConversationsProvider('acc-1').overrideWith(
            (ref) async => OwnerConversationsResponse(items: [
              row('c1', 'sms', '+15550001111'),
              row('c2', 'instagram', 'ig_user'),
              row('c3', 'whatsapp', 'Maria'),
            ]),
          ),
          ownerUsageTrendProvider('acc-1').overrideWith(
            (ref) async => OwnerUsageTrendResponse(items: const []),
          ),
          ownerInvoicesProvider('acc-1').overrideWith(
            (ref) async => OwnerInvoicesResponse(items: const []),
          ),
        ],
        child: MaterialApp.router(
          theme: AppTheme.darkTheme,
          routerConfig: _router(),
        ),
      ));
      await tester.pumpAndSettle();

      await tester.tap(find.text(t.owner.tab_conversations));
      await tester.pumpAndSettle();

      expect(tester.takeException(), isNull);
      expect(find.text('+15550001111'), findsOneWidget);
      expect(find.text('ig_user'), findsOneWidget);
      expect(find.text('Maria'), findsOneWidget);
    });

    testWidgets('usage tab renders the usage + trend sections', (tester) async {
      await tester.pumpWidget(_harness(seed: _detail()));
      await tester.pumpAndSettle();

      await tester.tap(find.text(t.owner.tab_usage));
      await tester.pumpAndSettle();

      expect(find.text(t.owner.usage_trend_label), findsOneWidget);
    });

    testWidgets('applying a contract shows success, never a false error alert',
        (tester) async {
      // Regression: apply-contract succeeded server-side (200) but the
      // client surfaced a false "unexpected error" because the controller
      // parsed/used the response. applyContract is now void — the flow
      // must end on the success snackbar with no error text.
      final router = GoRouter(routes: [
        GoRoute(
          path: '/',
          builder: (_, __) =>
              const OwnerAccountDetailScreen(accountId: 'acc-1'),
        ),
        GoRoute(path: '/owner/accounts', builder: (_, __) => const Scaffold()),
        GoRoute(path: '/home', builder: (_, __) => const Scaffold()),
      ]);
      await tester.pumpWidget(ProviderScope(
        overrides: [
          ownerDataSourceProvider.overrideWithValue(_FakeOwnerDS()),
          dioProvider.overrideWithValue(_plansDio()),
          ownerConversationsProvider('acc-1').overrideWith(
            (ref) async => OwnerConversationsResponse(items: const []),
          ),
          ownerUsageTrendProvider('acc-1').overrideWith(
            (ref) async => OwnerUsageTrendResponse(items: const []),
          ),
          ownerInvoicesProvider('acc-1').overrideWith(
            (ref) async => OwnerInvoicesResponse(items: const []),
          ),
        ],
        child: MaterialApp.router(
          theme: AppTheme.darkTheme,
          routerConfig: router,
        ),
      ));
      await tester.pumpAndSettle();

      await tester.tap(find.byIcon(Icons.more_vert));
      await tester.pumpAndSettle();
      await tester.tap(find.text(t.owner.apply_plan_cta));
      await tester.pumpAndSettle();
      await tester.tap(find.byType(DropdownButtonFormField<String>));
      await tester.pumpAndSettle();
      await tester.tap(find.textContaining('Enterprise').last);
      await tester.pumpAndSettle();
      await tester.tap(find.text(t.common.save));
      await tester.pumpAndSettle();

      expect(find.text(t.common.success), findsOneWidget);
      expect(find.text(t.errors.UNKNOWN), findsNothing);
    });
  });
}

class _FakeOwnerDS implements OwnerRemoteDataSource {
  @override
  Future<void> applyContract(String id, Map<String, dynamic> body) async {}
  @override
  Future<OwnerAccountDetail> getOne(String id) async => _detail();
  @override
  dynamic noSuchMethod(Invocation i) =>
      throw UnimplementedError(i.memberName.toString());
}
