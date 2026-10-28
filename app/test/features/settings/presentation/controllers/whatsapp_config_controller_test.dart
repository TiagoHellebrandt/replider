import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:app/i18n/strings.g.dart';
import 'package:mocktail/mocktail.dart';
import 'package:retrofit/retrofit.dart';

import 'package:app/features/settings/data/datasources/whatsapp_config_remote_datasource.dart';
import 'package:app/features/settings/domain/models/whatsapp_template_model.dart';
import 'package:app/features/settings/presentation/controllers/whatsapp_config_controller.dart';

class MockWAConfigDataSource extends Mock
    implements WhatsAppConfigRemoteDataSource {}

class FakeMultipartFile extends Fake implements MultipartFile {}

void main() {
  const channelId = 'ch-1';

  late MockWAConfigDataSource ds;
  late ProviderContainer container;

  setUpAll(() {
    registerFallbackValue(FakeMultipartFile());
  });

  setUp(() {
    ds = MockWAConfigDataSource();
  });

  tearDown(() => container.dispose());

  ProviderContainer makeContainer() {
    container = ProviderContainer(
      overrides: [
        whatsAppConfigRemoteDataSourceProvider.overrideWithValue(ds),
      ],
    );
    return container;
  }

  HttpResponse<dynamic> resp(dynamic data, {int status = 200}) {
    return HttpResponse(
      data,
      Response(
        requestOptions: RequestOptions(path: '/'),
        data: data,
        statusCode: status,
      ),
    );
  }

  group('WhatsAppTemplates', () {
    test('build returns parsed list', () async {
      when(() => ds.listTemplates(channelId)).thenAnswer((_) async => resp([
            {
              'name': 't1',
              'status': 'APPROVED',
              'language': 'pt_BR',
              'category': 'UTILITY',
              'components': [
                {'type': 'BODY', 'text': 'Olá'}
              ],
            },
            {
              'name': 't2',
              'status': 'REJECTED',
              'language': 'en_US',
              'category': 'MARKETING',
              'components': <dynamic>[],
            },
          ]));

      makeContainer();
      final list =
          await container.read(whatsAppTemplatesProvider(channelId).future);

      expect(list, hasLength(2));
      expect(list[0].name, 't1');
      expect(list[0].status, WhatsAppTemplateStatus.approved);
      expect(list[1].status, WhatsAppTemplateStatus.rejected);
    });

    test('create sends Meta-shaped payload and invalidates', () async {
      when(() => ds.listTemplates(channelId))
          .thenAnswer((_) async => resp(<dynamic>[]));
      when(() => ds.createTemplate(channelId, any()))
          .thenAnswer((_) async => resp(null, status: 201));

      makeContainer();
      await container.read(whatsAppTemplatesProvider(channelId).future);

      final err = await container
          .read(whatsAppTemplatesProvider(channelId).notifier)
          .create(
        name: 'boas_vindas',
        category: WhatsAppTemplateCategory.utility,
        language: 'pt_BR',
        components: [
          {'type': 'BODY', 'text': 'oi'}
        ],
      );

      expect(err, isNull);
      final captured = verify(() => ds.createTemplate(channelId, captureAny()))
          .captured
          .single as Map<String, dynamic>;
      expect(captured['name'], 'boas_vindas');
      expect(captured['category'], 'UTILITY');
      expect(captured['language'], 'pt_BR');
      expect(captured['components'], hasLength(1));
    });

    test('delete calls datasource with template name', () async {
      when(() => ds.listTemplates(channelId))
          .thenAnswer((_) async => resp(<dynamic>[]));
      when(() => ds.deleteTemplate(channelId, 'boas_vindas'))
          .thenAnswer((_) async => resp(null, status: 204));

      makeContainer();
      await container.read(whatsAppTemplatesProvider(channelId).future);
      final err = await container
          .read(whatsAppTemplatesProvider(channelId).notifier)
          .delete('boas_vindas');

      expect(err, isNull);
      verify(() => ds.deleteTemplate(channelId, 'boas_vindas')).called(1);
    });

    test('create surfaces non-Dio errors', () async {
      when(() => ds.listTemplates(channelId))
          .thenAnswer((_) async => resp(<dynamic>[]));
      when(() => ds.createTemplate(channelId, any()))
          .thenThrow(Exception('kaboom'));

      makeContainer();
      await container.read(whatsAppTemplatesProvider(channelId).future);
      final err = await container
          .read(whatsAppTemplatesProvider(channelId).notifier)
          .create(
        name: 'x',
        category: WhatsAppTemplateCategory.utility,
        language: 'pt_BR',
        components: const [],
      );

      expect(err, equals(t.errors.UNKNOWN));
    });

    test('create surfaces Meta error on DioException', () async {
      when(() => ds.listTemplates(channelId))
          .thenAnswer((_) async => resp(<dynamic>[]));
      when(() => ds.createTemplate(channelId, any())).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/'),
          response: Response(
            requestOptions: RequestOptions(path: '/'),
            statusCode: 502,
            data: {'message': 'meta down'},
          ),
        ),
      );

      makeContainer();
      await container.read(whatsAppTemplatesProvider(channelId).future);
      final err = await container
          .read(whatsAppTemplatesProvider(channelId).notifier)
          .create(
        name: 'x',
        category: WhatsAppTemplateCategory.marketing,
        language: 'en_US',
        components: const [],
      );

      expect(err, 'meta down');
    });

    test('delete surfaces non-Dio errors', () async {
      when(() => ds.listTemplates(channelId))
          .thenAnswer((_) async => resp(<dynamic>[]));
      when(() => ds.deleteTemplate(channelId, 'x'))
          .thenThrow(Exception('nope'));

      makeContainer();
      await container.read(whatsAppTemplatesProvider(channelId).future);
      final err = await container
          .read(whatsAppTemplatesProvider(channelId).notifier)
          .delete('x');

      expect(err, equals(t.errors.UNKNOWN));
    });

    test('_extractError falls back when response data not a Map', () async {
      // DioException with non-map data path
      when(() => ds.listTemplates(channelId))
          .thenAnswer((_) async => resp(<dynamic>[]));
      when(() => ds.deleteTemplate(channelId, 'x')).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/'),
          response: Response(
            requestOptions: RequestOptions(path: '/'),
            statusCode: 500,
            data: 'plain string',
          ),
          message: 'timeout',
        ),
      );

      makeContainer();
      await container.read(whatsAppTemplatesProvider(channelId).future);
      final err = await container
          .read(whatsAppTemplatesProvider(channelId).notifier)
          .delete('x');

      // Falls through to "Erro de rede" branch
      expect(err, equals(t.errors.UNKNOWN));
    });

    test('delete surfaces Meta error message', () async {
      when(() => ds.listTemplates(channelId))
          .thenAnswer((_) async => resp(<dynamic>[]));
      when(() => ds.deleteTemplate(channelId, 'x')).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/'),
          response: Response(
            requestOptions: RequestOptions(path: '/'),
            statusCode: 400,
            data: {'message': 'not found'},
          ),
        ),
      );

      makeContainer();
      await container.read(whatsAppTemplatesProvider(channelId).future);
      final err = await container
          .read(whatsAppTemplatesProvider(channelId).notifier)
          .delete('x');

      expect(err, 'not found');
    });
  });
}
