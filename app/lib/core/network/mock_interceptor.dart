import 'package:dio/dio.dart';
import 'dart:async';

class MockInterceptor extends Interceptor {
  @override
  void onRequest(
      RequestOptions options, RequestInterceptorHandler handler) async {
    // Artificial delay to simulate network latency
    await Future<void>.delayed(const Duration(milliseconds: 800));

    final path = options.path;

    if (path.endsWith('/auth/login') ||
        path.endsWith('/auth/google') ||
        path.endsWith('/auth/meta')) {
      return handler.resolve(
        Response(
          requestOptions: options,
          statusCode: 200,
          data: {'token': 'mock-jwt-token-12345'},
        ),
      );
    }

    if (path.endsWith('/accounts') && options.method == 'POST') {
      // Register
      return handler.resolve(
        Response(
          requestOptions: options,
          statusCode: 201,
          data: {'token': 'mock-jwt-token-12345'},
        ),
      );
    }

    if (path.endsWith('/auth/forgot-password')) {
      return handler.resolve(
        Response(
          requestOptions: options,
          statusCode: 200,
          data: <String, dynamic>{},
        ),
      );
    }

    if (path.endsWith('/me')) {
      final authHeader = options.headers['Authorization'] as String?;
      if (authHeader == null || !authHeader.startsWith('Bearer ')) {
        return handler.reject(
          DioException(
            requestOptions: options,
            response: Response(requestOptions: options, statusCode: 401),
            type: DioExceptionType.badResponse,
          ),
        );
      }
      return handler.resolve(
        Response(
          requestOptions: options,
          statusCode: 200,
          data: {
            'user': {
              'id': '123',
              'name': 'Test User',
              'email': 'test@replider.com',
              'role': 'user',
              'avatarUrl': 'https://i.pravatar.cc/150?u=test@replider.com',
            }
          },
        ),
      );
    }

    // Default: pass through
    handler.next(options);
  }
}
