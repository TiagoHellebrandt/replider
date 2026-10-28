import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:mocktail/mocktail.dart';
import 'package:app/features/billing/data/datasources/billing_remote_datasource.dart';
import 'package:app/features/billing/domain/models/plan_model.dart';
import 'package:app/features/billing/domain/models/usage_model.dart';
import 'package:app/features/billing/presentation/controllers/billing_controller.dart';

class MockBillingRemoteDataSource extends Mock
    implements BillingRemoteDataSource {}

void main() {
  late MockBillingRemoteDataSource mockDatasource;
  late ProviderContainer container;

  setUpAll(() {
    FlutterSecureStorage.setMockInitialValues({});
  });

  final tPlan = const PlanModel(
    id: 'plan-1',
    name: 'Pro',
    price: 4990,
    currency: 'BRL',
    aiProvider: 'openai',
    aiModel: 'gpt-4',
    inputMultiplier: 1.0,
    outputMultiplier: 1.0,
    maxAgents: 5,
    maxChannels: 10,
    maxConversationsPerMonth: 1000,
    maxAiTokensPerMonth: 100000,
    maxStorageMb: 5120,
    createdAt: '2024-01-01T00:00:00Z',
    updatedAt: '2024-01-01T00:00:00Z',
  );

  final tUsage = const UsageModel(
    aiTokensUsed: 5000,
    conversationsCount: 50,
    maxAiTokensPerMonth: 100000,
    maxConversationsPerMonth: 1000,
  );

  setUp(() {
    mockDatasource = MockBillingRemoteDataSource();
  });

  tearDown(() {
    container.dispose();
  });

  ProviderContainer createContainer() {
    container = ProviderContainer(
      overrides: [
        billingRemoteDataSourceProvider.overrideWithValue(mockDatasource),
      ],
    );
    return container;
  }

  group('PlansList', () {
    test('should load plans on initialization', () async {
      when(() => mockDatasource.getPlans()).thenAnswer((_) async => [tPlan]);

      createContainer();

      final plans = await container.read(plansListProvider.future);

      expect(plans, hasLength(1));
      expect(plans[0].name, 'Pro');
      expect(plans[0].price, 4990);
      verify(() => mockDatasource.getPlans()).called(1);
    });

    test('should set error state when loading plans fails', () async {
      when(() => mockDatasource.getPlans())
          .thenThrow(Exception('Network error'));

      createContainer();

      final sub = container.listen(plansListProvider, (_, __) {});
      await Future<void>.delayed(const Duration(milliseconds: 100));
      final state = container.read(plansListProvider);
      expect(state.hasError, isTrue);
      sub.close();
    });

    test('should reload plans on refresh', () async {
      when(() => mockDatasource.getPlans()).thenAnswer((_) async => [tPlan]);

      createContainer();
      await container.read(plansListProvider.future);

      final tPlan2 = const PlanModel(
        id: 'plan-2',
        name: 'Enterprise',
        price: 9990,
        currency: 'BRL',
        aiProvider: 'openai',
        aiModel: 'gpt-4',
        inputMultiplier: 1.0,
        outputMultiplier: 1.0,
        maxAgents: 20,
        maxChannels: 50,
        maxConversationsPerMonth: 10000,
        maxAiTokensPerMonth: 1000000,
        maxStorageMb: 51200,
        createdAt: '2024-01-01T00:00:00Z',
        updatedAt: '2024-01-01T00:00:00Z',
      );

      when(() => mockDatasource.getPlans())
          .thenAnswer((_) async => [tPlan, tPlan2]);

      await container.read(plansListProvider.notifier).refresh();

      final plans = container.read(plansListProvider).value;
      expect(plans, hasLength(2));
    });
  });

  group('AccountUsage', () {
    test('should load usage on initialization', () async {
      when(() => mockDatasource.getUsage()).thenAnswer((_) async => tUsage);

      createContainer();

      final usage = await container.read(accountUsageProvider.future);

      expect(usage.aiTokensUsed, 5000);
      expect(usage.conversationsCount, 50);
      verify(() => mockDatasource.getUsage()).called(1);
    });

    test('should set error state when loading usage fails', () async {
      when(() => mockDatasource.getUsage())
          .thenThrow(Exception('Network error'));

      createContainer();

      final sub = container.listen(accountUsageProvider, (_, __) {});
      await Future<void>.delayed(const Duration(milliseconds: 100));
      final state = container.read(accountUsageProvider);
      expect(state.hasError, isTrue);
      sub.close();
    });

    test('should reload usage on refresh', () async {
      when(() => mockDatasource.getUsage()).thenAnswer((_) async => tUsage);

      createContainer();
      await container.read(accountUsageProvider.future);

      final updatedUsage = const UsageModel(
        aiTokensUsed: 10000,
        conversationsCount: 100,
        maxAiTokensPerMonth: 100000,
        maxConversationsPerMonth: 1000,
      );

      when(() => mockDatasource.getUsage())
          .thenAnswer((_) async => updatedUsage);

      await container.read(accountUsageProvider.notifier).refresh();

      final usage = container.read(accountUsageProvider).value;
      expect(usage!.aiTokensUsed, 10000);
    });
  });

  group('BillingActions', () {
    group('openBillingPortal', () {
      test('should return UX message when no Stripe customer yet', () async {
        when(() => mockDatasource.openBillingPortal(any())).thenThrow(
          DioException(
            requestOptions: RequestOptions(path: '/payments/billing-portal'),
            response: Response(
              requestOptions: RequestOptions(path: '/payments/billing-portal'),
              statusCode: 400,
              data: {
                'code': 'BILLING_NO_CUSTOMER',
                'message': 'account has no Stripe customer',
              },
            ),
          ),
        );

        createContainer();

        final notifier = container.read(billingActionsProvider.notifier);
        final result = await notifier.openBillingPortal();

        expect(result, equals(t.billing.no_customer_yet));
      });

      test('should return parsed error message on other DioException',
          () async {
        when(() => mockDatasource.openBillingPortal(any())).thenThrow(
          DioException(
            requestOptions: RequestOptions(path: '/payments/billing-portal'),
            response: Response(
              requestOptions: RequestOptions(path: '/payments/billing-portal'),
              statusCode: 500,
              data: {'message': 'Stripe unreachable'},
            ),
          ),
        );

        createContainer();

        final notifier = container.read(billingActionsProvider.notifier);
        final result = await notifier.openBillingPortal();

        expect(result, 'Stripe unreachable');
      });

      test('should return generic error on unexpected exception', () async {
        when(() => mockDatasource.openBillingPortal(any()))
            .thenThrow(Exception('unexpected'));

        createContainer();

        final notifier = container.read(billingActionsProvider.notifier);
        final result = await notifier.openBillingPortal();

        expect(result, equals(t.errors.UNKNOWN));
      });
    });
  });
}
