import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:mocktail/mocktail.dart';
import 'package:retrofit/retrofit.dart';
import 'package:app/features/settings/data/datasources/channels_remote_datasource.dart';
import 'package:app/features/settings/domain/models/channel_model.dart';
import 'package:app/features/settings/presentation/controllers/channels_controller.dart';

class MockChannelsRemoteDataSource extends Mock
    implements ChannelsRemoteDataSource {}

void main() {
  late MockChannelsRemoteDataSource mockDatasource;
  late ProviderContainer container;

  final tChannelJson1 = {
    'id': 'channel-1',
    'account_id': 'acc-1',
    'agent_id': 'agent-1',
    'name': 'WhatsApp Business',
    'type': 'whatsapp',
    'is_active': true,
    'config': {'phone_number_id': '123'},
    'created_at': '2024-01-01T00:00:00Z',
  };

  final tChannelJson2 = {
    'id': 'channel-2',
    'account_id': 'acc-1',
    'name': 'Telegram Human-Only',
    'type': 'telegram',
    'is_active': false,
    'config': {'bot_token': 'token123'},
    'created_at': '2024-01-02T00:00:00Z',
  };

  setUp(() {
    mockDatasource = MockChannelsRemoteDataSource();
  });

  tearDown(() {
    container.dispose();
  });

  ProviderContainer createContainer() {
    container = ProviderContainer(
      overrides: [
        channelsRemoteDataSourceProvider.overrideWithValue(mockDatasource),
      ],
    );
    return container;
  }

  HttpResponse<dynamic> _makeResponse(dynamic data) {
    return HttpResponse(
      data,
      Response(
        requestOptions: RequestOptions(path: '/channels'),
        data: data,
        statusCode: 200,
      ),
    );
  }

  group('ChannelsList', () {
    group('build', () {
      test('should start with empty list', () async {
        createContainer();

        final channels = await container.read(channelsListProvider.future);

        expect(channels, isEmpty);
      });
    });

    group('load', () {
      test('should load all channels for the account', () async {
        when(() => mockDatasource.getChannels()).thenAnswer(
            (_) async => _makeResponse([tChannelJson1, tChannelJson2]));

        createContainer();

        final notifier = container.read(channelsListProvider.notifier);
        await notifier.load();

        final state = container.read(channelsListProvider);
        expect(state.hasValue, isTrue);
        expect(state.value, hasLength(2));
        expect(state.value![0].name, 'WhatsApp Business');
        expect(state.value![0].agentId, 'agent-1');
        expect(state.value![0].hasAgent, isTrue);
        expect(state.value![1].name, 'Telegram Human-Only');
        expect(state.value![1].agentId, isNull);
        expect(state.value![1].hasAgent, isFalse);
      });

      test('should handle empty channel list', () async {
        when(() => mockDatasource.getChannels())
            .thenAnswer((_) async => _makeResponse(<dynamic>[]));

        createContainer();

        final notifier = container.read(channelsListProvider.notifier);
        await notifier.load();

        final state = container.read(channelsListProvider);
        expect(state.hasValue, isTrue);
        expect(state.value, isEmpty);
      });

      test('should set error state on failure', () async {
        when(() => mockDatasource.getChannels())
            .thenThrow(Exception('Network error'));

        createContainer();

        final notifier = container.read(channelsListProvider.notifier);
        await notifier.load();

        final state = container.read(channelsListProvider);
        expect(state.hasError, isTrue);
      });
    });

    group('addChannel', () {
      test('should create channel without agent (human-only)', () async {
        when(() => mockDatasource.getChannels())
            .thenAnswer((_) async => _makeResponse([tChannelJson2]));
        when(() => mockDatasource.createChannel(any()))
            .thenAnswer((_) async => _makeResponse(tChannelJson2));

        createContainer();

        final notifier = container.read(channelsListProvider.notifier);
        await notifier.load();

        final result = await notifier.addChannel(
          name: 'Telegram Human-Only',
          type: ChannelType.telegram,
          config: {'bot_token': 'token123'},
        );

        expect(result.error, isNull);
        expect(result.raw, isNull);
        final captured =
            verify(() => mockDatasource.createChannel(captureAny()))
                .captured
                .single as Map<String, dynamic>;
        expect(captured.containsKey('agent_id'), isFalse);
      });

      test('should return error message on DioException', () async {
        when(() => mockDatasource.getChannels())
            .thenAnswer((_) async => _makeResponse(<dynamic>[]));
        when(() => mockDatasource.createChannel(any())).thenThrow(
          DioException(
            requestOptions: RequestOptions(path: '/channels'),
            response: Response(
              requestOptions: RequestOptions(path: '/channels'),
              statusCode: 422,
              data: {'message': 'Channel limit reached'},
            ),
          ),
        );

        createContainer();

        final notifier = container.read(channelsListProvider.notifier);
        await notifier.load();

        final result = await notifier.addChannel(
          name: 'New Channel',
          type: ChannelType.whatsapp,
          config: {},
        );

        expect(result.error, 'Channel limit reached');
        expect(result.raw, isA<DioException>());
      });
    });

    group('toggleChannel', () {
      test('should return null on successful toggle', () async {
        when(() => mockDatasource.getChannels())
            .thenAnswer((_) async => _makeResponse([tChannelJson1]));
        when(() => mockDatasource.toggleChannel('channel-1'))
            .thenAnswer((_) async => _makeResponse(null));

        createContainer();

        final notifier = container.read(channelsListProvider.notifier);
        await notifier.load();

        final result = await notifier.toggleChannel('channel-1');

        expect(result, isNull);
        verify(() => mockDatasource.toggleChannel('channel-1')).called(1);
      });
    });

    group('deleteChannel', () {
      test('should return null on successful deletion', () async {
        when(() => mockDatasource.getChannels())
            .thenAnswer((_) async => _makeResponse([tChannelJson1]));
        when(() => mockDatasource.deleteChannel('channel-1'))
            .thenAnswer((_) async => _makeResponse(null));

        createContainer();

        final notifier = container.read(channelsListProvider.notifier);
        await notifier.load();

        final result = await notifier.deleteChannel('channel-1');

        expect(result, isNull);
        verify(() => mockDatasource.deleteChannel('channel-1')).called(1);
      });

      test('should return error message on DioException', () async {
        when(() => mockDatasource.getChannels())
            .thenAnswer((_) async => _makeResponse(<dynamic>[]));
        when(() => mockDatasource.deleteChannel(any())).thenThrow(
          DioException(
            requestOptions: RequestOptions(path: '/channels/channel-1'),
            response: Response(
              requestOptions: RequestOptions(path: '/channels/channel-1'),
              statusCode: 404,
              data: {'error': 'Channel not found'},
            ),
          ),
        );

        createContainer();

        final notifier = container.read(channelsListProvider.notifier);
        await notifier.load();

        final result = await notifier.deleteChannel('channel-1');

        expect(result, 'Channel not found');
      });
    });

    group('testChannel', () {
      test('should return test result on success', () async {
        when(() => mockDatasource.testChannel('channel-1')).thenAnswer(
            (_) async => _makeResponse({'ok': true, 'message': 'Connected'}));

        createContainer();

        final notifier = container.read(channelsListProvider.notifier);

        final result = await notifier.testChannel('channel-1');

        expect(result, isNotNull);
        expect(result!['ok'], isTrue);
      });
    });

    group('refresh', () {
      test('should reload channels', () async {
        when(() => mockDatasource.getChannels())
            .thenAnswer((_) async => _makeResponse([tChannelJson1]));

        createContainer();

        final notifier = container.read(channelsListProvider.notifier);
        await notifier.load();

        when(() => mockDatasource.getChannels()).thenAnswer(
            (_) async => _makeResponse([tChannelJson1, tChannelJson2]));

        await notifier.refresh();

        final state = container.read(channelsListProvider);
        expect(state.value, hasLength(2));
      });
    });
  });
}
