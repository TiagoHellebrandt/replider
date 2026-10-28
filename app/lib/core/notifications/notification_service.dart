import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'package:app/core/network/websocket_service.dart';
import 'package:app/features/dashboard/presentation/controllers/dashboard_controller.dart';

part 'notification_service.g.dart';

/// Minimal cross-platform "you have a new message" notifier. Plays a
/// short system alert sound and (on web) tweaks the document title with
/// an unread counter so an attendant on another tab notices the ping.
///
/// Mobile push notifications would need flutter_local_notifications +
/// platform setup. We deliberately don't add that dependency in this
/// pass — the sound + title combo covers the much bigger gap of
/// "attendant has the dashboard open in a background tab and hears
/// nothing when a customer messages".
class NotificationService {
  NotificationService();

  int _unreadCount = 0;
  String? _titleBackup;
  DateTime _lastPlayed = DateTime.fromMillisecondsSinceEpoch(0);

  /// Bumped on every event we want to nudge for; reset by [clearUnread].
  int get unreadCount => _unreadCount;

  /// Fired by the WS listener whenever an inbound message lands on a
  /// conversation the user isn't currently looking at. Plays a sound
  /// (rate-limited so 10 messages don't drum the user) and updates the
  /// browser tab title.
  void onIncomingMessage() {
    _unreadCount += 1;
    _updateTitle();
    final now = DateTime.now();
    // Throttle: at most one beep per 2s, even with bursts of inbound
    // messages.  Anything tighter is annoying without adding signal.
    if (now.difference(_lastPlayed).inMilliseconds > 2000) {
      _lastPlayed = now;
      // SystemSoundType.alert is the OS-level "ding" — works on web
      // (Web Audio fallback) and mobile without any extra package.
      SystemSound.play(SystemSoundType.alert);
    }
  }

  /// Called when the user opens the conversation that produced the
  /// unread, or when they click anywhere that signals "I see them".
  void clearUnread() {
    if (_unreadCount == 0) return;
    _unreadCount = 0;
    _updateTitle();
  }

  void _updateTitle() {
    if (!kIsWeb) return;
    // SystemChrome.setApplicationSwitcherDescription works on web and
    // on Android. On web specifically it updates document.title, which
    // is exactly the surface a multi-tab user notices first.
    final base = _titleBackup ??= 'Replider';
    final next = _unreadCount > 0 ? '($_unreadCount) $base' : base;
    SystemChrome.setApplicationSwitcherDescription(
      ApplicationSwitcherDescription(label: next, primaryColor: 0),
    );
  }
}

/// Singleton-ish provider so the WS listener always pokes the same
/// instance; counter state would otherwise reset on every read.
@Riverpod(keepAlive: true)
NotificationService notificationService(Ref ref) => NotificationService();

/// Registers the WS → notifier pipe. Listens to inbound `new_message`
/// events and only pings when the user is NOT looking at the
/// originating conversation. Wire this provider once at app start
/// (e.g. via `ref.watch` in the dashboard root) and forget it.
@Riverpod(keepAlive: true)
void inboundNotificationListener(Ref ref) {
  final svc = ref.watch(notificationServiceProvider);
  // ref.listen on a stream provider keeps the subscription alive for
  // the lifetime of the (keep-alive) provider itself.
  ref.listen<AsyncValue<WSEvent>>(wsEventsProvider, (_, next) {
    final event = next.asData?.value;
    if (event == null) return;
    if (event.event != 'new_message') return;

    // Don't ping for the conversation the user is actively viewing —
    // they already see the bubble appear inline.
    final selectedId = ref.read(selectedConversationProvider);
    if (selectedId == event.conversationId) return;

    svc.onIncomingMessage();
  });
}
