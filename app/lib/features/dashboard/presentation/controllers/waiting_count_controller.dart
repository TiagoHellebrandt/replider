import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'package:app/core/network/dio_client.dart';
import 'package:app/core/network/websocket_service.dart';

part 'waiting_count_controller.g.dart';

/// How many customers are waiting on a human reply — the number on the
/// Conversas tab. Before this badge existed the only way to learn a reply
/// had arrived was to open the screen; silence looked identical to no news.
///
/// keepAlive because the SHELL wears it: an autoDispose provider would die
/// whenever the inbox is closed, which is exactly when the badge matters.
/// Refreshed by the same WebSocket events that refresh the inbox, debounced
/// so a burst of messages costs one request, not one each.
@Riverpod(keepAlive: true)
class WaitingCount extends _$WaitingCount {
  Timer? _debounce;

  @override
  Future<int> build() async {
    final sub = ref.listen(wsEventsProvider, (_, next) {
      final event = next.asData?.value;
      if (event == null) return;
      if (event.event == 'new_message' ||
          event.event == 'conversation_updated') {
        _debounce?.cancel();
        // Set the state directly instead of invalidating: a keepAlive
        // provider nobody is reading at that instant would only rebuild on
        // the NEXT read, and the badge would lag one navigation behind.
        _debounce = Timer(const Duration(seconds: 2), () async {
          state = AsyncData(await _fetch());
        });
      }
    });
    ref.onDispose(() {
      _debounce?.cancel();
      sub.close();
    });
    return _fetch();
  }

  Future<int> _fetch() async {
    final dio = ref.read(dioProvider);
    try {
      final resp = await dio.get<dynamic>('/conversations/waiting-count');
      final data = resp.data;
      if (data is Map<String, dynamic>) {
        return (data['count'] as num?)?.toInt() ?? 0;
      }
      return 0;
    } catch (_) {
      // The badge is a courtesy: a failed count renders as no badge, never
      // as an error state in the navigation bar.
      return 0;
    }
  }
}
