import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:app/core/network/auth_interceptor.dart';
import 'package:app/features/auth/domain/models/user_model.dart';

class MockFlutterSecureStorage extends Mock implements FlutterSecureStorage {}

void main() {
  late AuthInterceptor interceptor;
  late MockFlutterSecureStorage mockStorage;

  setUp(() {
    mockStorage = MockFlutterSecureStorage();
    interceptor = AuthInterceptor(mockStorage);
  });

  group('AuthInterceptor.onRequest', () {
    test('should add Bearer token when user is stored', () async {
      const user = UserModel(
        id: '1',
        name: 'Test',
        email: 'test@test.com',
        token: 'jwt-token-123',
      );

      when(() => mockStorage.read(key: 'cached_user_model'))
          .thenAnswer((_) async => json.encode(user.toJson()));

      final options = RequestOptions(path: '/agents');
      final handler = _MockRequestInterceptorHandler();

      interceptor.onRequest(options, handler);

      // Allow the async work inside onRequest to complete.
      await Future<void>.delayed(Duration.zero);

      expect(handler.nextCalled, isTrue);
      expect(handler.lastOptions?.headers['Authorization'],
          'Bearer jwt-token-123');
    });

    test('should not add Authorization header when no user is stored',
        () async {
      when(() => mockStorage.read(key: 'cached_user_model'))
          .thenAnswer((_) async => null);

      final options = RequestOptions(path: '/agents');
      final handler = _MockRequestInterceptorHandler();

      interceptor.onRequest(options, handler);

      await Future<void>.delayed(Duration.zero);

      expect(handler.nextCalled, isTrue);
      expect(
          handler.lastOptions?.headers.containsKey('Authorization'), isFalse);
    });

    test('should skip if Authorization header is already set', () async {
      final options = RequestOptions(path: '/me');
      options.headers['Authorization'] = 'Bearer existing-token';
      final handler = _MockRequestInterceptorHandler();

      interceptor.onRequest(options, handler);

      await Future<void>.delayed(Duration.zero);

      expect(handler.nextCalled, isTrue);
      // Should keep the existing token, not read from storage.
      expect(options.headers['Authorization'], 'Bearer existing-token');
      verifyNever(() => mockStorage.read(key: any(named: 'key')));
    });

    test('should proceed without token when storage throws', () async {
      when(() => mockStorage.read(key: 'cached_user_model'))
          .thenThrow(Exception('storage corrupted'));

      final options = RequestOptions(path: '/agents');
      final handler = _MockRequestInterceptorHandler();

      interceptor.onRequest(options, handler);

      await Future<void>.delayed(Duration.zero);

      expect(handler.nextCalled, isTrue);
      expect(
          handler.lastOptions?.headers.containsKey('Authorization'), isFalse);
    });

    test('should proceed without token when stored JSON is invalid', () async {
      when(() => mockStorage.read(key: 'cached_user_model'))
          .thenAnswer((_) async => 'not-valid-json');

      final options = RequestOptions(path: '/agents');
      final handler = _MockRequestInterceptorHandler();

      interceptor.onRequest(options, handler);

      await Future<void>.delayed(Duration.zero);

      expect(handler.nextCalled, isTrue);
      expect(
          handler.lastOptions?.headers.containsKey('Authorization'), isFalse);
    });

    test('should not add header when stored user has empty token', () async {
      const user = UserModel(
        id: '1',
        name: 'Test',
        email: 'test@test.com',
        token: '',
      );

      when(() => mockStorage.read(key: 'cached_user_model'))
          .thenAnswer((_) async => json.encode(user.toJson()));

      final options = RequestOptions(path: '/agents');
      final handler = _MockRequestInterceptorHandler();

      interceptor.onRequest(options, handler);

      await Future<void>.delayed(Duration.zero);

      expect(handler.nextCalled, isTrue);
      expect(
          handler.lastOptions?.headers.containsKey('Authorization'), isFalse);
    });
  });
}

/// A simple test double for [RequestInterceptorHandler] that records calls.
class _MockRequestInterceptorHandler extends RequestInterceptorHandler {
  bool nextCalled = false;
  RequestOptions? lastOptions;

  @override
  void next(RequestOptions requestOptions) {
    nextCalled = true;
    lastOptions = requestOptions;
  }
}
