import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:mocktail/mocktail.dart';
import 'package:retrofit/retrofit.dart';
import 'package:app/features/dashboard/data/datasources/dashboard_remote_datasource.dart';
import 'package:app/features/dashboard/presentation/controllers/dashboard_controller.dart';

class MockDashboardRemoteDataSource extends Mock
    implements DashboardRemoteDataSource {}

void main() {
  late MockDashboardRemoteDataSource mockDatasource;
  late ProviderContainer container;

  setUp(() {
    mockDatasource = MockDashboardRemoteDataSource();
  });

  tearDown(() {
    container.dispose();
  });

  ProviderContainer createContainer() {
    container = ProviderContainer(
      overrides: [
        dashboardRemoteDataSourceProvider.overrideWithValue(mockDatasource),
      ],
    );
    return container;
  }

  group('ConversationStatusFilter', () {
    test('should start with null state', () {
      createContainer();
      final state = container.read(conversationStatusFilterProvider);
      expect(state, isNull);
    });

    test('should update state when update is called', () {
      createContainer();
      container.read(conversationStatusFilterProvider.notifier).update('open');
      expect(container.read(conversationStatusFilterProvider), 'open');
    });

    test('should allow setting null to clear filter', () {
      createContainer();
      final notifier =
          container.read(conversationStatusFilterProvider.notifier);
      notifier.update('open');
      notifier.update(null);
      expect(container.read(conversationStatusFilterProvider), isNull);
    });
  });

  group('TypingState', () {
    test('should start with empty map', () {
      createContainer();
      final state = container.read(typingStateProvider);
      expect(state, isEmpty);
    });

    test('should set typing state for a conversation', () {
      createContainer();
      final notifier = container.read(typingStateProvider.notifier);
      notifier.setTyping('conv-1', isTyping: true);

      expect(notifier.isTyping('conv-1'), isTrue);
      expect(notifier.isTyping('conv-2'), isFalse);
    });

    test('should clear typing state', () {
      createContainer();
      final notifier = container.read(typingStateProvider.notifier);
      notifier.setTyping('conv-1', isTyping: true);
      notifier.setTyping('conv-1', isTyping: false);

      expect(notifier.isTyping('conv-1'), isFalse);
    });
  });

  group('UsageAlert', () {
    test('should start with null', () {
      createContainer();
      final state = container.read(usageAlertProvider);
      expect(state, isNull);
    });

    test('should set alert percentage', () {
      createContainer();
      container.read(usageAlertProvider.notifier).setAlert(85.5);
      expect(container.read(usageAlertProvider), 85.5);
    });

    test('should clear alert', () {
      createContainer();
      final notifier = container.read(usageAlertProvider.notifier);
      notifier.setAlert(90.0);
      notifier.clear();
      expect(container.read(usageAlertProvider), isNull);
    });
  });

  group('SelectedConversation', () {
    test('should start with null', () {
      createContainer();
      final state = container.read(selectedConversationProvider);
      expect(state, isNull);
    });

    test('should update selection', () {
      createContainer();
      container.read(selectedConversationProvider.notifier).select('conv-1');
      expect(container.read(selectedConversationProvider), 'conv-1');
    });

    test('should allow deselecting', () {
      createContainer();
      final notifier = container.read(selectedConversationProvider.notifier);
      notifier.select('conv-1');
      notifier.select(null);
      expect(container.read(selectedConversationProvider), isNull);
    });
  });

  group('ConversationSearch', () {
    test('should start with empty string', () {
      createContainer();
      final state = container.read(conversationSearchProvider);
      expect(state, '');
    });

    test('should update search query', () {
      createContainer();
      container.read(conversationSearchProvider.notifier).update('john');
      expect(container.read(conversationSearchProvider), 'john');
    });

    test('should clear search query', () {
      createContainer();
      final notifier = container.read(conversationSearchProvider.notifier);
      notifier.update('john');
      notifier.clear();
      expect(container.read(conversationSearchProvider), '');
    });
  });

  group('MessageSearch', () {
    test('should start with empty string', () {
      createContainer();
      final state = container.read(messageSearchProvider);
      expect(state, '');
    });

    test('should update search query', () {
      createContainer();
      container.read(messageSearchProvider.notifier).update('hello');
      expect(container.read(messageSearchProvider), 'hello');
    });

    test('should clear search query', () {
      createContainer();
      final notifier = container.read(messageSearchProvider.notifier);
      notifier.update('hello');
      notifier.clear();
      expect(container.read(messageSearchProvider), '');
    });
  });

  group('ConversationActions', () {
    group('reply', () {
      test('should call datasource and invalidate providers on success',
          () async {
        when(() => mockDatasource.replyToConversation(
              'conv-1',
              {'message': 'Hello back'},
            )).thenAnswer((_) async {});

        createContainer();

        final notifier = container.read(conversationActionsProvider.notifier);
        await notifier.reply('conv-1', 'Hello back');

        final state = container.read(conversationActionsProvider);
        expect(state.hasError, isFalse);
        verify(() => mockDatasource.replyToConversation(
              'conv-1',
              {'message': 'Hello back'},
            )).called(1);
      });

      test('should set error state on failure', () async {
        when(() => mockDatasource.replyToConversation(any(), any()))
            .thenThrow(Exception('Failed to send'));

        createContainer();

        final notifier = container.read(conversationActionsProvider.notifier);
        await notifier.reply('conv-1', 'Hello');

        final state = container.read(conversationActionsProvider);
        expect(state.hasError, isTrue);
      });
    });

    group('updateLeadMetadata', () {
      HttpResponse<dynamic> okResponse() => HttpResponse<dynamic>(
            null,
            Response<dynamic>(requestOptions: RequestOptions(path: '')),
          );

      test('should PATCH the field with field_key/value on success', () async {
        when(() => mockDatasource.updateLeadMetadata(
              'lead-1',
              {'field_key': 'name', 'value': 'Janet'},
            )).thenAnswer((_) async => okResponse());

        createContainer();

        final notifier = container.read(conversationActionsProvider.notifier);
        await notifier.updateLeadMetadata('conv-1', 'lead-1', 'name', 'Janet');

        final state = container.read(conversationActionsProvider);
        expect(state.hasError, isFalse);
        verify(() => mockDatasource.updateLeadMetadata(
              'lead-1',
              {'field_key': 'name', 'value': 'Janet'},
            )).called(1);
      });

      test('should set error state and rethrow on failure', () async {
        when(() => mockDatasource.updateLeadMetadata(any(), any()))
            .thenThrow(Exception('save failed'));

        createContainer();

        final notifier = container.read(conversationActionsProvider.notifier);
        await expectLater(
          notifier.updateLeadMetadata('conv-1', 'lead-1', 'name', 'Janet'),
          throwsA(isA<Exception>()),
        );

        final state = container.read(conversationActionsProvider);
        expect(state.hasError, isTrue);
      });
    });

    group('resumeBot', () {
      test('should call datasource resumeBot on success', () async {
        when(() => mockDatasource.resumeBot('conv-1')).thenAnswer((_) async {});

        createContainer();

        final notifier = container.read(conversationActionsProvider.notifier);
        await notifier.resumeBot('conv-1');

        final state = container.read(conversationActionsProvider);
        expect(state.hasError, isFalse);
        verify(() => mockDatasource.resumeBot('conv-1')).called(1);
      });

      test('should set error state on resumeBot failure', () async {
        when(() => mockDatasource.resumeBot(any()))
            .thenThrow(Exception('Bot resume failed'));

        createContainer();

        final notifier = container.read(conversationActionsProvider.notifier);
        await notifier.resumeBot('conv-1');

        final state = container.read(conversationActionsProvider);
        expect(state.hasError, isTrue);
      });
    });
  });
}
