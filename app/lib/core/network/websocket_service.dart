import 'dart:async';
import 'dart:convert';
import 'dart:developer' as developer;
import 'package:flutter/foundation.dart' show visibleForTesting;
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:web_socket_channel/web_socket_channel.dart';
import 'package:app/features/auth/domain/models/user_model.dart';

part 'websocket_service.g.dart';

/// Parsed real-time event from the WebSocket.
class WSEvent {
  final String event;
  final String? conversationId;
  final Map<String, dynamic>? message;
  final Map<String, dynamic> raw;

  WSEvent({
    required this.event,
    this.conversationId,
    this.message,
    required this.raw,
  });

  factory WSEvent.fromJson(Map<String, dynamic> json) {
    return WSEvent(
      event: json['event'] as String? ?? 'unknown',
      conversationId: json['conversation_id'] as String?,
      message: json['message'] as Map<String, dynamic>?,
      raw: json,
    );
  }
}

/// Manages a persistent WebSocket connection to the API.
/// Broadcasts [WSEvent]s to all listeners and auto-reconnects on disconnect.
class WebSocketService {
  WebSocketChannel? _channel;

  /// Subscription to the current channel's stream. Stored so it can be
  /// cancelled explicitly on reconnect and dispose — relying on the
  /// socket closing to auto-complete the stream leaks the listener
  /// across reconnect churn (US-098). Cancelling on dispose also avoids
  /// a stray frame arriving between `sink.close()` and
  /// `_controller.close()` invoking `add` on a closed broadcast
  /// controller and throwing `StateError` from inside the WebSocket
  /// isolate.
  StreamSubscription<dynamic>? _channelSub;
  final _controller = StreamController<WSEvent>.broadcast();

  /// Separate stream for connection-status transitions — the app uses
  /// this to drive the "Reconectando..." banner on the dashboard. Kept
  /// broadcast so every listener sees every transition.
  final _statusController = StreamController<bool>.broadcast();
  Timer? _reconnectTimer;
  bool _disposed = false;
  bool _connected = false;

  /// Consecutive failed attempts since the last frame that proved the socket
  /// alive. Drives the backoff in [_scheduleReconnect].
  int _attempts = 0;

  /// True once the server refused THIS session's auth frame. The server
  /// answers `{"error":"unauthorized"}` and closes — for a vendedor that is
  /// policy (the socket mirrors the inbox's role gate), not a blip, and the
  /// old 3s loop reconnected against it forever: a request every three
  /// seconds for the whole session, each one refused. A refusal ends the
  /// reconnect cycle; a new session (login → new service) starts a fresh one.
  bool _authRefused = false;

  /// Exposed so the dashboard can tell "reconnecting" from "not for this
  /// role" — the banner must not promise a reconnect that will never come.
  bool get authRefused => _authRefused;

  /// Attempts so far, for the backoff tests.
  @visibleForTesting
  int get reconnectAttempts => _attempts;

  static const Duration _reconnectBase = Duration(seconds: 3);
  static const Duration _reconnectCap = Duration(minutes: 5);

  /// Exponential backoff: 3s, 6s, 12s, … capped at five minutes. A dead API
  /// used to be polled every 3s by every open tab for as long as it stayed
  /// down; now the pressure halves with every miss.
  @visibleForTesting
  static Duration reconnectDelay(int attempt) {
    var delay = _reconnectBase;
    for (var i = 0; i < attempt && delay < _reconnectCap; i++) {
      delay *= 2;
    }
    return delay > _reconnectCap ? _reconnectCap : delay;
  }

  Stream<WSEvent> get events => _controller.stream;

  /// True after the first frame goes out on a freshly-opened socket;
  /// false after an error/onDone, BEFORE the reconnect timer fires.
  Stream<bool> get connectionStatus => _statusController.stream;
  bool get isConnected => _connected;

  Future<void> connect() async {
    if (_disposed || _authRefused) return;

    final token = await _readToken();
    // Re-check after the await: dispose() (e.g. logout invalidating
    // webSocketServiceProvider) could have run while we were reading
    // secure storage. Without this guard we'd open a fresh socket
    // AFTER the dispose path ran, leaking the channel and triggering
    // a reconnect storm against a token that's about to be revoked.
    if (_disposed) return;
    if (token == null) return;

    final wsUrl = _buildWsUrl();

    try {
      _channel = WebSocketChannel.connect(Uri.parse(wsUrl));

      // Authenticate via first frame instead of query parameter (avoids token in logs/history)
      _channel!.sink.add(jsonEncode({'type': 'auth', 'token': token}));
      _markConnected(true);

      await _channelSub?.cancel();
      _channelSub = _channel!.stream.listen(
        (data) {
          if (_disposed || _controller.isClosed) return;
          try {
            final json = jsonDecode(data as String) as Map<String, dynamic>;
            if (handleFrame(json)) return;
            _controller.add(WSEvent.fromJson(json));
          } catch (e) {
            // A malformed/unexpected frame must not kill the stream, but
            // swallowing it silently hid dropped new_message /
            // conversation_updated events (the dashboard went stale with no
            // trace). Log and drop so the failure is at least observable.
            developer.log('dropped malformed WS frame',
                name: 'websocket', error: e);
          }
        },
        onDone: _scheduleReconnect,
        onError: (_) => _scheduleReconnect(),
      );
    } catch (_) {
      _scheduleReconnect();
    }
  }

  void _markConnected(bool connected) {
    if (_connected == connected) return;
    _connected = connected;
    if (!_disposed) _statusController.add(connected);
  }

  /// Inspects a decoded frame for connection-level meaning. Returns true when
  /// the frame was consumed here and must not reach the event listeners.
  ///
  /// The server's refusal of the auth frame is `{"error":"unauthorized"}`
  /// followed by a close — the only frame that carries `error` without an
  /// `event`. Any other frame proves the socket alive and resets the backoff.
  @visibleForTesting
  bool handleFrame(Map<String, dynamic> json) {
    if (json['error'] == 'unauthorized' && json['event'] == null) {
      _authRefused = true;
      developer.log(
          'server refused the socket for this session; not reconnecting',
          name: 'websocket');
      return true;
    }
    _attempts = 0;
    return false;
  }

  void _scheduleReconnect() {
    if (_disposed) return;
    _markConnected(false);
    _channel = null;
    _reconnectTimer?.cancel();
    if (_authRefused) return;
    _reconnectTimer = Timer(reconnectDelay(_attempts), connect);
    _attempts++;
  }

  Future<String?> _readToken() async {
    const storage = FlutterSecureStorage();
    try {
      final userJson = await storage.read(key: 'cached_user_model');
      if (userJson == null) return null;
      final user = UserModel.fromJson(
        json.decode(userJson) as Map<String, dynamic>,
      );
      return user.token.isNotEmpty ? user.token : null;
    } catch (_) {
      return null;
    }
  }

  static const _httpBase = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: 'http://localhost:8080/v1',
  );

  String _buildWsUrl() {
    final wsBase = _httpBase
        .replaceFirst('https://', 'wss://')
        .replaceFirst('http://', 'ws://');
    return '$wsBase/ws/chat';
  }

  void dispose() {
    _disposed = true;
    _reconnectTimer?.cancel();
    _channelSub?.cancel();
    _channelSub = null;
    _channel?.sink.close();
    _controller.close();
    _statusController.close();
  }
}

@Riverpod(keepAlive: true)
WebSocketService webSocketService(Ref ref) {
  final service = WebSocketService();
  service.connect();
  ref.onDispose(() => service.dispose());
  return service;
}

@riverpod
Stream<WSEvent> wsEvents(Ref ref) {
  final service = ref.watch(webSocketServiceProvider);
  return service.events;
}

/// Reflects the underlying WebSocket connection state so UI can show a
/// "reconectando" banner. Defaults to `true` (connected) on first read
/// to avoid a flicker on cold start before the first handshake lands.
@riverpod
Stream<bool> wsConnectionStatus(Ref ref) async* {
  final service = ref.watch(webSocketServiceProvider);
  yield service.isConnected;
  yield* service.connectionStatus;
}
