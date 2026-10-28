import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:mocktail/mocktail.dart';
import 'package:app/core/network/dio_client.dart';
import 'package:app/features/auth/data/datasources/auth_local_datasource.dart';
import 'package:app/features/auth/domain/models/user_model.dart';
import 'package:app/features/auth/domain/repositories/auth_repository.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/settings/presentation/controllers/profile_controller.dart';

class MockDio extends Mock implements Dio {}

class MockAuthRepository extends Mock implements AuthRepository {}

class MockAuthLocalDataSource extends Mock implements AuthLocalDataSource {}

class FakeUserModel extends Fake implements UserModel {}

void main() {
  late MockDio mockDio;
  late MockAuthRepository mockAuthRepository;
  late MockAuthLocalDataSource mockLocalDataSource;
  late ProviderContainer container;

  setUpAll(() {
    registerFallbackValue(FakeUserModel());
  });

  const tUser = UserModel(
    id: 'user-1',
    name: 'Test User',
    email: 'test@example.com',
    token: 'jwt-token',
  );

  setUp(() {
    mockDio = MockDio();
    mockAuthRepository = MockAuthRepository();
    mockLocalDataSource = MockAuthLocalDataSource();
  });

  tearDown(() {
    container.dispose();
  });

  ProviderContainer createContainer({UserModel? initialUser}) {
    when(() => mockAuthRepository.getCurrentUser())
        .thenAnswer((_) async => initialUser);

    container = ProviderContainer(
      overrides: [
        dioProvider.overrideWithValue(mockDio),
        authRepositoryProvider.overrideWithValue(mockAuthRepository),
        authLocalDataSourceProvider.overrideWithValue(mockLocalDataSource),
      ],
    );
    return container;
  }

  group('ProfileController', () {
    group('updateProfile', () {
      test('should return null on successful profile update', () async {
        createContainer(initialUser: tUser);
        await container.read(authControllerProvider.future);

        when(() => mockDio.put<dynamic>(
              any(),
              data: any(named: 'data'),
              queryParameters: any(named: 'queryParameters'),
              options: any(named: 'options'),
              cancelToken: any(named: 'cancelToken'),
              onSendProgress: any(named: 'onSendProgress'),
              onReceiveProgress: any(named: 'onReceiveProgress'),
            )).thenAnswer((_) async => Response(
              requestOptions: RequestOptions(path: '/me'),
              statusCode: 200,
              data: {'name': 'New Name', 'email': 'new@example.com'},
            ));
        when(() => mockLocalDataSource.saveUser(any()))
            .thenAnswer((_) async {});

        final notifier = container.read(profileControllerProvider.notifier);
        final result = await notifier.updateProfile(
          name: 'New Name',
          email: 'new@example.com',
        );

        expect(result, isNull);
      });

      test('should return error message on DioException with 404', () async {
        createContainer(initialUser: tUser);
        await container.read(authControllerProvider.future);

        when(() => mockDio.put<dynamic>(
              any(),
              data: any(named: 'data'),
              queryParameters: any(named: 'queryParameters'),
              options: any(named: 'options'),
              cancelToken: any(named: 'cancelToken'),
              onSendProgress: any(named: 'onSendProgress'),
              onReceiveProgress: any(named: 'onReceiveProgress'),
            )).thenThrow(
          DioException(
            requestOptions: RequestOptions(path: '/me'),
            response: Response(
              requestOptions: RequestOptions(path: '/me'),
              statusCode: 404,
            ),
          ),
        );

        final notifier = container.read(profileControllerProvider.notifier);
        final result = await notifier.updateProfile(
          name: 'Name',
          email: 'email@test.com',
        );

        expect(result, isNotNull);
        expect(result, isA<String>());
      });

      test('should return error message on DioException with error body',
          () async {
        createContainer(initialUser: tUser);
        await container.read(authControllerProvider.future);

        when(() => mockDio.put<dynamic>(
              any(),
              data: any(named: 'data'),
              queryParameters: any(named: 'queryParameters'),
              options: any(named: 'options'),
              cancelToken: any(named: 'cancelToken'),
              onSendProgress: any(named: 'onSendProgress'),
              onReceiveProgress: any(named: 'onReceiveProgress'),
            )).thenThrow(
          DioException(
            requestOptions: RequestOptions(path: '/me'),
            response: Response(
              requestOptions: RequestOptions(path: '/me'),
              statusCode: 422,
              data: {'message': 'Email already taken'},
            ),
          ),
        );

        final notifier = container.read(profileControllerProvider.notifier);
        final result = await notifier.updateProfile(
          name: 'Name',
          email: 'dup@test.com',
        );

        expect(result, 'Email already taken');
      });
    });

    group('changePassword', () {
      test('should return null on successful password change', () async {
        createContainer(initialUser: tUser);
        await container.read(authControllerProvider.future);

        when(() => mockDio.patch<dynamic>(
              any(),
              data: any(named: 'data'),
              queryParameters: any(named: 'queryParameters'),
              options: any(named: 'options'),
              cancelToken: any(named: 'cancelToken'),
              onSendProgress: any(named: 'onSendProgress'),
              onReceiveProgress: any(named: 'onReceiveProgress'),
            )).thenAnswer((_) async => Response(
              requestOptions: RequestOptions(path: '/me/password'),
              statusCode: 200,
            ));

        final notifier = container.read(profileControllerProvider.notifier);
        final result = await notifier.changePassword(
          currentPassword: 'oldpass',
          newPassword: 'newpass',
        );

        expect(result, isNull);
      });

      test('should return error on 401 (wrong current password)', () async {
        createContainer(initialUser: tUser);
        await container.read(authControllerProvider.future);

        when(() => mockDio.patch<dynamic>(
              any(),
              data: any(named: 'data'),
              queryParameters: any(named: 'queryParameters'),
              options: any(named: 'options'),
              cancelToken: any(named: 'cancelToken'),
              onSendProgress: any(named: 'onSendProgress'),
              onReceiveProgress: any(named: 'onReceiveProgress'),
            )).thenThrow(
          DioException(
            requestOptions: RequestOptions(path: '/me/password'),
            response: Response(
              requestOptions: RequestOptions(path: '/me/password'),
              statusCode: 401,
            ),
          ),
        );

        final notifier = container.read(profileControllerProvider.notifier);
        final result = await notifier.changePassword(
          currentPassword: 'wrong',
          newPassword: 'newpass',
        );

        expect(result, isNotNull);
        expect(result, isA<String>());
      });

      test('should return error on 404/405 (endpoint unavailable)', () async {
        createContainer(initialUser: tUser);
        await container.read(authControllerProvider.future);

        when(() => mockDio.patch<dynamic>(
              any(),
              data: any(named: 'data'),
              queryParameters: any(named: 'queryParameters'),
              options: any(named: 'options'),
              cancelToken: any(named: 'cancelToken'),
              onSendProgress: any(named: 'onSendProgress'),
              onReceiveProgress: any(named: 'onReceiveProgress'),
            )).thenThrow(
          DioException(
            requestOptions: RequestOptions(path: '/me/password'),
            response: Response(
              requestOptions: RequestOptions(path: '/me/password'),
              statusCode: 405,
            ),
          ),
        );

        final notifier = container.read(profileControllerProvider.notifier);
        final result = await notifier.changePassword(
          currentPassword: 'pass',
          newPassword: 'newpass',
        );

        expect(result, isNotNull);
        expect(result, isA<String>());
      });
    });
  });
}
