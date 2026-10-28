import 'dart:async';

import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'package:app/core/network/websocket_service.dart';
import 'package:app/features/auth/presentation/controllers/auth_controller.dart';
import 'package:app/features/support/data/support_repository.dart';
import 'package:app/features/support/domain/support_message.dart';

part 'support_controller.g.dart';

/// The `support_message` WS event (canal-interno US-003) is the customer
/// side of an internal thread message. It is broadcast to EVERY user of
/// the account, carrying the thread OWNER's user_id — both providers
/// below filter on it so a user only ever sees their own thread
/// (per-user privacy inside the same account).
bool _isMySupportEvent(WSEvent event, String? myUserId) {
  if (event.event != 'support_message') return false;
  if (myUserId == null || myUserId.isEmpty) return false;
  return event.raw['user_id'] == myUserId;
}

/// The logged-in user's support thread: hydrated from GET on open,
/// appended in real time from `support_message` WS events.
@riverpod
class SupportThreadController extends _$SupportThreadController {
  String? _myUserId;

  @override
  FutureOr<SupportThread> build() {
    // Captured via watch (not read-in-handler): the auth provider is
    // async, so a bare read at event time can observe it still loading
    // and silently drop the message. The watch also rebuilds (and
    // refetches the thread) once auth resolves or the user changes.
    _myUserId = ref.watch(authControllerProvider).asData?.value?.id;
    ref.listen(wsEventsProvider, (previous, next) {
      final event = next.asData?.value;
      if (event != null) _onEvent(event);
    });
    return ref.watch(supportRepositoryProvider).fetchThread();
  }

  void _onEvent(WSEvent event) {
    if (!_isMySupportEvent(event, _myUserId)) return;
    final messageJson = event.message;
    if (messageJson == null) return;
    final message = SupportMessage.fromJson(messageJson);
    _append(message, conversationId: event.conversationId);
  }

  /// Appends deduped by message id — the user's own send comes back both
  /// as the POST response and as a WS echo, whichever lands first wins.
  void _append(SupportMessage message, {String? conversationId}) {
    final current = state.asData?.value;
    if (current == null) return;
    if (message.id.isNotEmpty &&
        current.messages.any((m) => m.id == message.id)) {
      return;
    }
    state = AsyncData(SupportThread(
      conversationId: conversationId ?? current.conversationId,
      messages: [...current.messages, message],
    ));
  }

  Future<void> sendMessage(String content) async {
    final result = await ref.read(supportRepositoryProvider).send(content);
    _append(result.message, conversationId: result.conversationId);
  }
}

/// Unread count for the support nav badge. keepAlive so it accumulates
/// while the user is anywhere in the app; the support screen clears it
/// on open and while it stays visible.
@Riverpod(keepAlive: true)
class SupportUnreadCounter extends _$SupportUnreadCounter {
  @override
  int build() {
    // Same watch rationale as SupportThreadController; an auth change
    // (login/logout) rebuilds and correctly resets the count.
    final myUserId = ref.watch(authControllerProvider).asData?.value?.id;
    ref.listen(wsEventsProvider, (previous, next) {
      final event = next.asData?.value;
      if (event == null) return;
      if (!_isMySupportEvent(event, myUserId)) return;
      // Only platform-side replies badge the user — their own message
      // echoing back (direction inbound) must not.
      if ((event.message?['direction'] as String?) != 'outbound') return;
      state = state + 1;
    });
    return 0;
  }

  void clear() {
    if (state != 0) state = 0;
  }
}
