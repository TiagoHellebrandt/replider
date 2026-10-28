import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/features/support/domain/support_message.dart';

part 'support_repository.g.dart';

/// Thin Dio wrapper over the internal support thread endpoints
/// (GET/POST /me/support/messages). Kept as a plain class behind a
/// provider so widget tests swap in a fake without touching Dio.
class SupportRepository {
  SupportRepository(this._dio);

  final Dio _dio;

  /// Fetches the user's thread, oldest first. The API stamps the thread
  /// as read on this call (clears the email-fallback window server-side).
  Future<SupportThread> fetchThread() async {
    final res = await _dio.get<dynamic>(
      '/me/support/messages',
      queryParameters: {'limit': 200},
    );
    final data = res.data;
    if (data is Map<String, dynamic>) return SupportThread.fromJson(data);
    return const SupportThread();
  }

  Future<SupportSendResult> send(String content) async {
    final res = await _dio.post<dynamic>(
      '/me/support/messages',
      data: {'content': content},
    );
    final data = res.data as Map<String, dynamic>;
    return SupportSendResult(
      conversationId: data['conversation_id'] as String?,
      message: SupportMessage.fromJson(data['message'] as Map<String, dynamic>),
    );
  }
}

@riverpod
SupportRepository supportRepository(Ref ref) =>
    SupportRepository(ref.watch(dioProvider));
