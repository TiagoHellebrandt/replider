import 'package:dio/dio.dart';

/// Mixin for controllers/notifiers that interact with the API via Dio.
///
/// Centralizes the repeated pattern of:
/// ```dart
/// } on DioException catch (e) {
///   final data = e.response?.data;
///   if (data is Map<String, dynamic>) {
///     return data['message'] ?? data['error'] ?? 'Error';
///   }
///   return 'Network error';
/// }
/// ```
///
/// Usage:
/// ```dart
/// class MyController extends _$MyController with ApiErrorHandler {
///   Future<String?> doSomething() async {
///     try {
///       await datasource.call();
///       return null; // success
///     } on DioException catch (e) {
///       return extractDioError(e);
///     } catch (e) {
///       return 'Unexpected error: $e';
///     }
///   }
/// }
/// ```
mixin ApiErrorHandler {
  /// Extracts a user-friendly error message from a [DioException].
  ///
  /// Tries to read `message` or `error` from the JSON response body.
  /// Falls back to the HTTP status code or a generic network error message.
  String extractDioError(DioException e) {
    final data = e.response?.data;
    if (data is Map<String, dynamic>) {
      return data['message'] as String? ??
          data['error'] as String? ??
          'Error: ${e.response?.statusCode}';
    }
    if (e.type == DioExceptionType.connectionTimeout ||
        e.type == DioExceptionType.connectionError) {
      return 'Connection error. Check your internet.';
    }
    return 'Network error: ${e.message}';
  }
}
