import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/features/dashboard/domain/models/message_attachment.dart';

/// Attachments of one message.
///
/// Fetched per message rather than folded into the transcript because the
/// verdict CHANGES after the message is written: a rescan settles a file that
/// read "unverified" when the bubble first rendered, and the bubble has to
/// be able to catch up without reloading the whole conversation.
final messageAttachmentsProvider =
    FutureProvider.autoDispose.family<List<MessageAttachment>, String>(
  (ref, messageId) async {
    final dio = ref.watch(dioProvider);
    final res =
        await dio.get<List<dynamic>>('/messages/$messageId/attachments');
    return (res.data ?? const <dynamic>[])
        .whereType<Map<String, dynamic>>()
        .map(MessageAttachment.fromJson)
        // Returns EVERYTHING, inline furniture included: "view original" needs
        // the embedded images to render the message as it was sent. Deciding
        // what counts as "an anexo" is a presentation call and lives in the
        // widget — a data layer that strips rows leaves the other consumer
        // with no source.
        .toList();
  },
);

final attachmentActionsProvider = Provider<AttachmentActions>(
  (ref) => AttachmentActions(ref.watch(dioProvider)),
);

class AttachmentActions {
  const AttachmentActions(this._dio);

  final Dio _dio;

  /// Builds the URL that renders an email's original markup in a browser.
  ///
  /// Two steps because a browser tab carries no Authorization header: the app
  /// asks (authenticated) for a two-minute token scoped to this one message,
  /// and the tab carries that. The session JWT never goes in a query string,
  /// where it would land in history, proxy logs and the Referer of everything
  /// the page links to.
  Future<String> originalViewUrl(
    String messageId, {
    bool loadImages = false,
  }) async {
    final res = await _dio.post<Map<String, dynamic>>(
      '/messages/$messageId/original-token',
    );
    final token = res.data?['token'] as String? ?? '';
    if (token.isEmpty) throw StateError('no view token');

    final base = _dio.options.baseUrl.replaceAll(RegExp(r'/$'), '');
    final query = {
      'token': token,
      if (loadImages) 'load_images': 'true',
    };
    return Uri.parse('$base/messages/original')
        .replace(queryParameters: query)
        .toString();
  }

  /// Asks for a short-lived signed URL.
  ///
  /// [acknowledgeUnscanned] is the user saying, explicitly, that they
  /// understood nobody checked this file. It never unlocks an infected one —
  /// the server refuses that regardless, because a "download anyway" on a
  /// flagged file would turn the whole scan into decoration.
  Future<String> downloadUrl(
    String attachmentId, {
    bool acknowledgeUnscanned = false,
  }) async {
    final res = await _dio.get<Map<String, dynamic>>(
      '/attachments/$attachmentId/download',
      queryParameters: acknowledgeUnscanned
          ? <String, dynamic>{'acknowledge_unscanned': 'true'}
          : null,
    );
    return res.data?['url'] as String? ?? '';
  }
}
